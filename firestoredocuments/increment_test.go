package firestoredocuments

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync/atomic"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type incrementMissingField struct {
	Owner string `firestore:"owner"`
}
type incrementScalarParent struct {
	Owner string `firestore:"owner"`
	Stats string `firestore:"stats"`
}
type incrementNested struct {
	Count   int64  `firestore:"count"`
	Sibling string `firestore:"sibling"`
}
type incrementNestedDocument struct {
	Owner string          `firestore:"owner"`
	Stats incrementNested `firestore:"stats"`
}
type incrementLiteralField struct {
	Owner   string `firestore:"owner"`
	Counter int64  `firestore:"literal.counter"`
}

func TestIncrementEmulatorNativeTransformAndCosts(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name        string
		seed        any
		field       firestore.FieldPath
		delta, want int64
		nested      bool
	}{
		{name: "first signup creates absent counter document", field: firestore.FieldPath{"value"}, delta: 1, want: 1},
		{name: "missing field becomes integer delta", seed: incrementMissingField{Owner: "preserved"}, field: firestore.FieldPath{"value"}, delta: 2, want: 2},
		{name: "nonnumeric field becomes delta", seed: incompatibleRecord{Owner: "preserved", Value: "not-numeric"}, field: firestore.FieldPath{"value"}, delta: -3, want: -3},
		{name: "existing integer is incremented", seed: nativeRecord{Owner: "preserved", Value: 41}, field: firestore.FieldPath{"value"}, delta: 1, want: 42},
		{name: "negative delta subtracts", seed: nativeRecord{Owner: "preserved", Value: 41}, field: firestore.FieldPath{"value"}, delta: -3, want: 38},
		{name: "zero delta preserves native integer", seed: nativeRecord{Owner: "preserved", Value: 41}, field: firestore.FieldPath{"value"}, want: 41},
		{name: "exact upper integer boundary", seed: nativeRecord{Owner: "preserved", Value: math.MaxInt64 - 1}, field: firestore.FieldPath{"value"}, delta: 1, want: math.MaxInt64},
		{name: "upper overflow retains native saturation", seed: nativeRecord{Owner: "preserved", Value: math.MaxInt64}, field: firestore.FieldPath{"value"}, delta: 1, want: math.MaxInt64},
		{name: "lower overflow retains native saturation", seed: nativeRecord{Owner: "preserved", Value: math.MinInt64}, field: firestore.FieldPath{"value"}, delta: -1, want: math.MinInt64},
		{name: "field component dot is literal", seed: incrementLiteralField{Owner: "preserved", Counter: 4}, field: firestore.FieldPath{"literal.counter"}, delta: 3, want: 7},
		{name: "nested path preserves neighboring fields", seed: incrementNestedDocument{Owner: "preserved", Stats: incrementNested{Count: 4, Sibling: "nested-preserved"}}, field: firestore.FieldPath{"stats", "count"}, delta: 3, want: 7, nested: true},
		{name: "nested path replaces a non-map parent", seed: incrementScalarParent{Owner: "preserved", Stats: "not-a-map"}, field: firestore.FieldPath{"stats", "count"}, delta: 3, want: 3},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			ref := nativeRef(client, t)
			if row.seed != nil {
				if _, err := ref.Set(t.Context(), row.seed); err != nil {
					t.Fatal(err)
				}
			}
			beforeReads, beforeWrites, beforeCommits := costs.reads.Load(), costs.writes.Load(), costs.commits.Load()
			if err := Increment(t.Context(), IncrementRequest{Reference: ref, Field: row.field, Delta: row.delta}); err != nil {
				t.Fatalf("native Increment error = %v, want nil", err)
			}
			if reads, writes, commits := costs.reads.Load()-beforeReads, costs.writes.Load()-beforeWrites, costs.commits.Load()-beforeCommits; reads != 0 || writes != 1 || commits != 1 {
				t.Fatalf("blind native reads/writes/commits = %d/%d/%d, want 0/1/1", reads, writes, commits)
			}
			snapshot, err := ref.Get(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			value, err := snapshot.DataAtPath(row.field)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := value.(int64)
			if !ok || got != row.want {
				t.Fatalf("native transformed field = %v (%T), want int64 %d", value, value, row.want)
			}
			if row.seed != nil {
				owner, err := snapshot.DataAt("owner")
				if err != nil || owner != "preserved" {
					t.Fatalf("sibling owner = %v/%v, want preserved/nil", owner, err)
				}
			}
			if row.nested {
				sibling, err := snapshot.DataAt("stats.sibling")
				if err != nil || sibling != "nested-preserved" {
					t.Fatalf("nested sibling = %v/%v, want nested-preserved/nil", sibling, err)
				}
			}
			if costs.reads.Load()-beforeReads != 1 || costs.queries.Load() != 0 {
				t.Fatalf("verification reads/query RPCs = %d/%d, want one point read/no query", costs.reads.Load()-beforeReads, costs.queries.Load())
			}
		})
	}
}

func TestIncrementEmulatorTransactionQueueRollbackAndRetry(t *testing.T) {
	t.Parallel()
	cause := errors.New("increment owner transaction refusal")
	for _, row := range []struct {
		name                    string
		rollback, cancel, retry bool
		wantCallbacks           int
		wantCommits             int64
	}{
		{name: "queued transform becomes visible only at outer commit", wantCallbacks: 1, wantCommits: 1},
		{name: "owner refusal rolls back queued transform", rollback: true, wantCallbacks: 1},
		{name: "cancellation after queue cannot commit", cancel: true, wantCallbacks: 1},
		{name: "native aborted first commit retries once without double increment", retry: true, wantCallbacks: 2, wantCommits: 2},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			var injected atomic.Bool
			fault := func(ctx context.Context, method string, request, reply any, conn *grpc.ClientConn, invoke grpc.UnaryInvoker, options ...grpc.CallOption) error {
				if row.retry && strings.HasSuffix(method, "/Commit") && injected.CompareAndSwap(false, true) {
					return status.Error(codes.Aborted, "controlled native transaction retry")
				}
				return invoke(ctx, method, request, reply, conn, options...)
			}
			client, costs := nativeClient(t, grpc.WithChainUnaryInterceptor(fault))
			ref := nativeRef(client, t)
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			callbacks := 0
			result, err := RunTransaction(ctx, TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
				callbacks++
				if err := Increment(ctx, IncrementRequest{Reference: ref, Transaction: tx, Field: firestore.FieldPath{"value"}, Delta: 7}); err != nil {
					return err
				}
				if costs.writes.Load() != 0 {
					t.Fatalf("queued native committed writes = %d, want zero", costs.writes.Load())
				}
				// This independent proof read is outside the borrowed transaction. Increment
				// itself has queued no read and no independent commit.
				if _, readErr := ref.Get(t.Context()); status.Code(readErr) != codes.NotFound {
					t.Fatalf("queued visibility = %v, want native NotFound", readErr)
				}
				if row.cancel {
					cancel(cause)
					return cause
				}
				if row.rollback {
					return cause
				}
				return nil
			}})
			refused := row.rollback || row.cancel
			if refused {
				if !errors.Is(err, cause) || !errors.Is(err, core.ErrFirestoreDocumentsOperation) || result != (TransactionResult{}) {
					t.Fatalf("refused transaction = %+v/%v, want zero and retained cause", result, err)
				}
			} else if err != nil || result.Validate() != nil {
				t.Fatalf("committed transaction = %+v/%v, want native commitment", result, err)
			}
			snapshot, readErr := ref.Get(t.Context())
			wantWrites := int64(1)
			if refused {
				wantWrites = 0
				if status.Code(readErr) != codes.NotFound {
					t.Fatalf("rolled back document = %v, want NotFound", readErr)
				}
			} else {
				if readErr != nil {
					t.Fatal(readErr)
				}
				got, err := snapshot.DataAt("value")
				if err != nil || got != int64(7) {
					t.Fatalf("committed increment = %v/%v, want 7/nil", got, err)
				}
			}
			if callbacks != row.wantCallbacks || costs.commits.Load() != row.wantCommits || costs.writes.Load() != wantWrites || costs.reads.Load() != int64(callbacks+1) || costs.queries.Load() != 0 {
				t.Fatalf("callbacks/commit attempts/committed writes/proof reads/queries = %d/%d/%d/%d/%d, want %d/%d/%d/%d/0", callbacks, costs.commits.Load(), costs.writes.Load(), costs.reads.Load(), costs.queries.Load(), row.wantCallbacks, row.wantCommits, wantWrites, callbacks+1)
			}
		})
	}
}

func TestIncrementRefusesBeforeNativeEffects(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	ref := nativeRef(client, t)
	for _, row := range []struct {
		name    string
		request IncrementRequest
	}{
		{name: "missing native reference", request: IncrementRequest{Field: firestore.FieldPath{"value"}, Delta: 1}},
		{name: "empty field path", request: IncrementRequest{Reference: ref, Delta: 1}},
		{name: "empty path component", request: IncrementRequest{Reference: ref, Field: firestore.FieldPath{"stats", ""}, Delta: 1}},
		{name: "invalid UTF8 component", request: IncrementRequest{Reference: ref, Field: firestore.FieldPath{string([]byte{255})}, Delta: 1}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if err := Increment(t.Context(), row.request); !errors.Is(err, core.ErrFirestoreDocumentsContract) {
				t.Fatalf("Increment refusal = %v, want contract", err)
			}
		})
	}
	request := IncrementRequest{Reference: ref, Field: firestore.FieldPath{"value"}, Delta: 1}
	cause := errors.New("increment pre-admission cancellation")
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(cause)
	canceled, stop := context.WithCancel(t.Context())
	stop()
	for _, row := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{name: "absent context is a typed refusal", ctx: nil, want: core.ErrNilContext},
		{name: "canceled context retains cancellation", ctx: canceled, want: context.Canceled},
		{name: "custom caller cause remains visible", ctx: ctx, want: cause},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			err := Increment(row.ctx, request)
			if !errors.Is(err, core.ErrFirestoreDocumentsContract) || !errors.Is(err, row.want) {
				t.Fatalf("Increment context refusal = %v, want contract and %v", err, row.want)
			}
		})
	}
	t.Cleanup(func() {
		if costs.reads.Load() != 0 || costs.writes.Load() != 0 || costs.commits.Load() != 0 {
			t.Fatalf("refused Increment reads/writes/commits = %d/%d/%d, want all zero", costs.reads.Load(), costs.writes.Load(), costs.commits.Load())
		}
	})
}

func TestIncrementPreservesProviderAndCancellationCauses(t *testing.T) {
	t.Parallel()
	cause := errors.New("increment provider/caller competing cause")
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	fault := func(ctx context.Context, method string, request, reply any, conn *grpc.ClientConn, invoke grpc.UnaryInvoker, options ...grpc.CallOption) error {
		if strings.HasSuffix(method, "/Commit") {
			cancel(cause)
			return status.Error(codes.PermissionDenied, "controlled native commit refusal")
		}
		return invoke(ctx, method, request, reply, conn, options...)
	}
	client, costs := nativeClient(t, grpc.WithChainUnaryInterceptor(fault))
	err := Increment(ctx, IncrementRequest{Reference: nativeRef(client, t), Field: firestore.FieldPath{"value"}, Delta: 1})
	if !errors.Is(err, core.ErrFirestoreDocumentsOperation) || !errors.Is(err, context.Canceled) || !errors.Is(err, cause) || status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Increment competing causes = %v, want operation/PermissionDenied/Canceled/custom cause", err)
	}
	if costs.reads.Load() != 0 || costs.writes.Load() != 0 || costs.commits.Load() != 1 {
		t.Fatalf("faulted Increment reads/committed writes/attempts = %d/%d/%d, want 0/0/1", costs.reads.Load(), costs.writes.Load(), costs.commits.Load())
	}
}
