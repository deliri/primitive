package firestoredocuments

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type nativeRecord struct {
	Owner string `firestore:"owner"`
	Value int64  `firestore:"value"`
}

func (r nativeRecord) Validate() error {
	if r.Owner == "" {
		return core.ErrFirestoreDocumentsContract
	}
	return nil
}

var (
	_ func(context.Context, GetRequest) (Document[nativeRecord], error)    = Get[nativeRecord]
	_ func(context.Context, CreateRequest[nativeRecord]) error             = Create[nativeRecord]
	_ func(context.Context, TransactionRequest) (TransactionResult, error) = RunTransaction
)

func TestDocumentsRefuseAbsentCapabilitiesAndCancellation(t *testing.T) {
	t.Parallel()
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	cause := errors.New("caller admission cancellation fixture")
	custom, cancelCause := context.WithCancelCause(t.Context())
	cancelCause(cause)
	for _, tc := range []struct {
		name      string
		ctx       context.Context
		wantCause error
	}{
		{name: "missing_native_capability_is_a_contract_refusal", ctx: t.Context()},
		{name: "nil_context_retains_native_boundary_identity", wantCause: core.ErrNilContext},
		{name: "cancellation_retains_native_cause", ctx: cancelled, wantCause: context.Canceled},
		{name: "custom_cancel_cause_is_preserved_before_any_operation", ctx: custom, wantCause: cause},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			read, readErr := Get[nativeRecord](tc.ctx, GetRequest{})
			writeErr := Create(tc.ctx, CreateRequest[nativeRecord]{})
			commit, commitErr := RunTransaction(tc.ctx, TransactionRequest{})
			for _, err := range []error{readErr, writeErr, commitErr} {
				if !errors.Is(err, core.ErrFirestoreDocumentsContract) || (tc.wantCause != nil && !errors.Is(err, tc.wantCause)) {
					t.Fatalf("document refusal=%v, want contract and cause=%v", err, tc.wantCause)
				}
			}
			if read != (Document[nativeRecord]{}) || commit != (TransactionResult{}) || commit.Validate() == nil {
				t.Fatalf("refused read=%+v commit=%+v, want zero records and no committed result", read, commit)
			}
		})
	}
}

// The interceptors count actual native provider messages. Counts are exercised
// emulator operations, not a claim about production billing.
type nativeCosts struct {
	reads           atomic.Int64
	writes          atomic.Int64
	queries         atomic.Int64
	commits         atomic.Int64
	readOnlyBegins  atomic.Int64
	readWriteBegins atomic.Int64
}
type costStream struct {
	grpc.ClientStream
	costs  *nativeCosts
	method string
}

func (s *costStream) RecvMsg(value any) error {
	err := s.ClientStream.RecvMsg(value)
	if err == nil && strings.HasSuffix(s.method, "/BatchGetDocuments") {
		if _, ok := value.(*firestorepb.BatchGetDocumentsResponse); ok {
			s.costs.reads.Add(1)
		}
	}
	return err
}
func (c *nativeCosts) unary(ctx context.Context, method string, request, reply any, connection *grpc.ClientConn, invoke grpc.UnaryInvoker, options ...grpc.CallOption) error {
	if strings.HasSuffix(method, "/BeginTransaction") {
		if typed, ok := request.(*firestorepb.BeginTransactionRequest); ok {
			if typed.GetOptions().GetReadOnly() != nil {
				c.readOnlyBegins.Add(1)
			} else {
				c.readWriteBegins.Add(1)
			}
		}
	}
	if strings.HasSuffix(method, "/RunQuery") {
		c.queries.Add(1)
	}
	err := invoke(ctx, method, request, reply, connection, options...)
	if strings.HasSuffix(method, "/Commit") {
		c.commits.Add(1)
		if err == nil {
			if typed, ok := request.(*firestorepb.CommitRequest); ok {
				c.writes.Add(int64(len(typed.Writes)))
			}
		}
	}
	return err
}
func (c *nativeCosts) stream(ctx context.Context, description *grpc.StreamDesc, connection *grpc.ClientConn, method string, streamer grpc.Streamer, options ...grpc.CallOption) (grpc.ClientStream, error) {
	if strings.HasSuffix(method, "/RunQuery") {
		c.queries.Add(1)
	}
	stream, err := streamer(ctx, description, connection, method, options...)
	if err != nil {
		return nil, err
	}
	return &costStream{ClientStream: stream, costs: c, method: method}, nil
}
func nativeClient(t testing.TB, extra ...grpc.DialOption) (*firestore.Client, *nativeCosts) {
	t.Helper()
	endpoint := nativeEmulatorEndpoint
	if endpoint == "" {
		t.Fatalf("native emulator endpoint=%q, want configured local proof endpoint", endpoint)
	}
	costs := new(nativeCosts)
	options := append([]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(costs.unary), grpc.WithChainStreamInterceptor(costs.stream)}, extra...)
	connection, err := grpc.NewClient(endpoint, options...)
	if err != nil {
		t.Fatal(err)
	}
	client, err := firestore.NewClient(t.Context(), "primitive-documents-proof", option.WithGRPCConn(connection), option.WithoutAuthentication())
	if err != nil {
		_ = connection.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("native client close=%v, want nil", err)
		}
		if got := connection.GetState(); got != connectivity.Shutdown {
			t.Errorf("SDK-owned native connection state=%v, want Shutdown", got)
		}
	})
	return client, costs
}
func nativeRef(client *firestore.Client, t *testing.T) *firestore.DocumentRef {
	t.Helper()
	return client.Collection("documents").Doc(strings.ReplaceAll(t.Name(), "/", "-"))
}
func reportNativeCosts(t *testing.T, c *nativeCosts) {
	t.Helper()
	t.Logf("native emulator observations: reads=%d committed_writes=%d commit_attempts=%d queries=%d", c.reads.Load(), c.writes.Load(), c.commits.Load(), c.queries.Load())
	if c.queries.Load() != 0 {
		t.Fatalf("native queries=%d, want zero point-document operations", c.queries.Load())
	}
}

func TestDocumentsEmulatorCommitVisibilityAndReadCost(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	ref := nativeRef(client, t)
	want := nativeRecord{Owner: "owner-a", Value: 17}
	queued, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		result, err := RunTransaction(ctx, TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
			if err := Create(ctx, CreateRequest[nativeRecord]{Reference: ref, Transaction: tx, Record: want}); err != nil {
				return err
			}
			close(queued)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}})
		if err == nil {
			err = result.Validate()
		}
		done <- err
	}()
	defer func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(10 * time.Second):
			t.Errorf("native visibility transaction joined=false context=%v, want joined", ctx.Err())
		}
	}()
	select {
	case <-queued:
	case err := <-done:
		t.Fatalf("transaction before queue=%v, want queued write", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	before, err := Get[nativeRecord](ctx, GetRequest{Reference: ref})
	if status.Code(err) != codes.NotFound || before != (Document[nativeRecord]{}) {
		close(release)
		<-done
		t.Fatalf("queued visibility=(%+v,%v), want zero and native NotFound", before, err)
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	got, err := Get[nativeRecord](ctx, GetRequest{Reference: ref})
	if err != nil || got.Record != want {
		t.Fatalf("committed read=(%+v,%v), want (%+v,nil)", got, err, want)
	}
	if costs.writes.Load() != 1 || costs.reads.Load() != 2 {
		t.Fatalf("native reads=%d writes=%d, want2 and1", costs.reads.Load(), costs.writes.Load())
	}
	reportNativeCosts(t, costs)
}

func TestDocumentsEmulatorCallbackRefusalIsAtomic(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	ref := nativeRef(client, t)
	cause := errors.New("fixture admission refused")
	got, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
		if err := Create(ctx, CreateRequest[nativeRecord]{Reference: ref, Transaction: tx, Record: nativeRecord{Owner: "refused", Value: 3}}); err != nil {
			return err
		}
		return cause
	}})
	if !errors.Is(err, cause) || !errors.Is(err, core.ErrFirestoreDocumentsOperation) || got != (TransactionResult{}) {
		t.Fatalf("callback refusal=(%+v,%v), want zero and exact cause", got, err)
	}
	read, readErr := Get[nativeRecord](t.Context(), GetRequest{Reference: ref})
	if status.Code(readErr) != codes.NotFound || read != (Document[nativeRecord]{}) || costs.writes.Load() != 0 {
		t.Fatalf("aborted transaction read=(%+v,%v) writes=%d, want missing and0", read, readErr, costs.writes.Load())
	}
	reportNativeCosts(t, costs)
}

func TestDocumentsEmulatorConcurrentUniqueCreateHasOneOwner(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	ref := nativeRef(client, t)
	start := make(chan struct{})
	var joined sync.WaitGroup
	outcomes := make(chan error, 2)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	for _, owner := range []string{"owner-a", "owner-b"} {
		joined.Go(func() {
			<-start
			result, err := RunTransaction(ctx, TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
				return Create(ctx, CreateRequest[nativeRecord]{Reference: ref, Transaction: tx, Record: nativeRecord{Owner: owner, Value: 23}})
			}})
			if err == nil {
				err = result.Validate()
			} else if result != (TransactionResult{}) {
				err = fmt.Errorf("refused commit released result: %w", core.ErrFirestoreDocumentsContract)
			}
			outcomes <- err
		})
	}
	close(start)
	joined.Wait()
	close(outcomes)
	var committed, conflicted int
	for err := range outcomes {
		if err == nil {
			committed++
		} else if status.Code(err) == codes.AlreadyExists && errors.Is(err, core.ErrFirestoreDocumentsOperation) {
			conflicted++
		} else {
			t.Fatalf("concurrent result=%v, want committed or native AlreadyExists", err)
		}
	}
	got, err := Get[nativeRecord](ctx, GetRequest{Reference: ref})
	if err != nil || committed != 1 || conflicted != 1 || (got.Record.Owner != "owner-a" && got.Record.Owner != "owner-b") || got.Record.Value != 23 || costs.writes.Load() != 1 {
		t.Fatalf("unique owner=%+v error=%v committed=%d conflicted=%d writes=%d, want one genuine owner and1write", got, err, committed, conflicted, costs.writes.Load())
	}
	reportNativeCosts(t, costs)
}

func TestDocumentsEmulatorNativeRetryUsesFreshRead(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	ref := nativeRef(client, t)
	guard := client.Collection("guards").Doc(ref.ID)
	if _, err := guard.Create(t.Context(), nativeRecord{Owner: "guard", Value: 1}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	var attempt int
	result, err := RunTransaction(ctx, TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
		attempt++
		if attempt == 2 {
			// Fixture mutation happens after the SDK rolled back the lock. This
			// is native arrangement, not an application mutation door.
			if _, err := guard.Update(ctx, []firestore.Update{{Path: "value", Value: int64(2)}}); err != nil {
				return err
			}
		}
		read, err := Get[nativeRecord](ctx, GetRequest{Reference: guard, Transaction: tx})
		if err != nil {
			return err
		}
		if attempt == 1 {
			// A native non-transactional write cannot cross this read lock.
			// Feed its real Aborted cause back to the SDK retry owner.
			_, err := guard.Update(ctx, []firestore.Update{{Path: "value", Value: int64(2)}})
			if status.Code(err) != codes.Aborted {
				return errors.Join(core.ErrFirestoreDocumentsContract, err)
			}
			return err
		}
		return Create(ctx, CreateRequest[nativeRecord]{Reference: ref, Transaction: tx, Record: nativeRecord{Owner: "fresh", Value: read.Record.Value}})
	}})
	if err != nil || result.Validate() != nil || attempt != 2 {
		t.Fatalf("native retried transaction=(%+v,%v) attempts=%d, want committed after native Aborted retry", result, err, attempt)
	}
	got, err := Get[nativeRecord](ctx, GetRequest{Reference: ref})
	if err != nil || got.Record != (nativeRecord{Owner: "fresh", Value: 2}) {
		t.Fatalf("native retried record=(%+v,%v), want fresh value2", got, err)
	}
	if costs.reads.Load() != 3 || costs.writes.Load() != 3 || costs.commits.Load() != 4 {
		t.Fatalf("native retry reads=%d committed_writes=%d commit_attempts=%d, want3/3/4 including2 fixture writes", costs.reads.Load(), costs.writes.Load(), costs.commits.Load())
	}
	reportNativeCosts(t, costs)
}

func TestDocumentsEmulatorTypedRecordAdmission(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		record  nativeRecord
		wantErr error
	}{
		{name: "minimum_native_integer_preserved", record: nativeRecord{Owner: "minimum", Value: math.MinInt64}},
		{name: "maximum_native_integer_preserved", record: nativeRecord{Owner: "maximum", Value: math.MaxInt64}},
		{name: "zero_value_data_is_not_missing", record: nativeRecord{Owner: "zero"}},
		{name: "owner_validation_rejects_persisted_empty_owner", record: nativeRecord{Value: 4}, wantErr: core.ErrFirestoreDocumentsContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			ref := nativeRef(client, t)
			// Native SDK fixture arrangement deliberately includes a record that
			// Create refuses. Get must independently enforce its owning admission.
			if _, err := ref.Create(t.Context(), tc.record); err != nil {
				t.Fatal(err)
			}
			got, err := Get[nativeRecord](t.Context(), GetRequest{Reference: ref})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("persisted record admission=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (Document[nativeRecord]{}) {
					t.Fatalf("refused record=%+v, want zero", got)
				}
			} else if got.Record != tc.record {
				t.Fatalf("admitted record=%+v, want %+v", got, tc.record)
			}
			if costs.reads.Load() != 1 || costs.writes.Load() != 1 {
				t.Fatalf("record native reads=%d writes=%d, want1/1", costs.reads.Load(), costs.writes.Load())
			}
			reportNativeCosts(t, costs)
		})
	}
	t.Run("native_type_mismatch_retains_operation_identity_and_zero_record", func(t *testing.T) {
		t.Parallel()
		client, costs := nativeClient(t)
		ref := nativeRef(client, t)
		wrong := struct {
			Owner string `firestore:"owner"`
			Value string `firestore:"value"`
		}{Owner: "typed", Value: "not-an-integer"}
		if _, err := ref.Create(t.Context(), wrong); err != nil {
			t.Fatal(err)
		}
		got, err := Get[nativeRecord](t.Context(), GetRequest{Reference: ref})
		if !errors.Is(err, core.ErrFirestoreDocumentsOperation) || got != (Document[nativeRecord]{}) {
			t.Fatalf("native wrong type=(%+v,%v), want zero and typed native operation refusal", got, err)
		}
		reportNativeCosts(t, costs)
	})
}

type nonStructRecord string

func (nonStructRecord) Validate() error { return nil }

func TestDocumentsEmulatorRejectsNonStructBeforeNativeRead(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	ref := nativeRef(client, t)
	scalar, scalarErr := Get[nonStructRecord](t.Context(), GetRequest{Reference: ref})
	pointer, pointerErr := Get[*nativeRecord](t.Context(), GetRequest{Reference: ref})
	if !errors.Is(scalarErr, core.ErrFirestoreDocumentsContract) || !errors.Is(pointerErr, core.ErrFirestoreDocumentsContract) || scalar != (Document[nonStructRecord]{}) || pointer.Record != nil {
		t.Fatalf("nonstruct admissions=(%+v,%v),(%+v,%v), want zero and contract refusal", scalar, scalarErr, pointer, pointerErr)
	}
	result, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
		return Create(ctx, CreateRequest[nonStructRecord]{Reference: ref, Transaction: tx, Record: "not-a-record"})
	}})
	if !errors.Is(err, core.ErrFirestoreDocumentsContract) || result != (TransactionResult{}) || costs.reads.Load() != 0 || costs.writes.Load() != 0 {
		t.Fatalf("nonstruct queued create=(%+v,%v) reads=%d writes=%d, want typed refusal and no document effect", result, err, costs.reads.Load(), costs.writes.Load())
	}
	reportNativeCosts(t, costs)
}

func TestDocumentsEmulatorCancellationBeforeCommitLeavesNoRecord(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	ref := nativeRef(client, t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	queued, finished := make(chan struct{}), make(chan struct{})
	type outcome struct {
		result TransactionResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		defer close(finished)
		result, err := RunTransaction(ctx, TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
			if err := Create(ctx, CreateRequest[nativeRecord]{Reference: ref, Transaction: tx, Record: nativeRecord{Owner: "cancelled", Value: 7}}); err != nil {
				return err
			}
			close(queued)
			<-ctx.Done()
			return ctx.Err()
		}})
		done <- outcome{result, err}
	}()
	defer func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(10 * time.Second):
			t.Errorf("cancelled native transaction joined=false context=%v, want joined", ctx.Err())
		}
	}()
	select {
	case <-queued:
	case got := <-done:
		t.Fatalf("precommit=(%+v,%v), want queued native write", got.result, got.err)
	case <-time.After(10 * time.Second):
		t.Fatalf("native queued write observed=false context=%v, want queued write", ctx.Err())
	}
	cancel()
	select {
	case got := <-done:
		if !errors.Is(got.err, context.Canceled) || got.result != (TransactionResult{}) {
			t.Fatalf("cancelled commit=(%+v,%v), want zero and native cancellation", got.result, got.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("native cancellation completed=false context=%v, want completed", ctx.Err())
	}
	read, err := Get[nativeRecord](t.Context(), GetRequest{Reference: ref})
	if status.Code(err) != codes.NotFound || read != (Document[nativeRecord]{}) || costs.reads.Load() != 1 || costs.writes.Load() != 0 || costs.commits.Load() != 0 {
		t.Fatalf("cancelled visibility=(%+v,%v) reads=%d writes=%d commits=%d, want missing/1/0/0", read, err, costs.reads.Load(), costs.writes.Load(), costs.commits.Load())
	}
	reportNativeCosts(t, costs)
}
