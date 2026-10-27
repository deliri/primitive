package firestoredocuments

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type setPriorRecord struct {
	Owner   string `firestore:"owner"`
	Value   int64  `firestore:"value"`
	Retired string `firestore:"retired"`
}

func (r setPriorRecord) Validate() error {
	return (nativeRecord{Owner: r.Owner, Value: r.Value}).Validate()
}

type setUnsupportedRecord struct {
	Channel chan int `firestore:"channel"`
}

func (setUnsupportedRecord) Validate() error { return nil }

var _ func(context.Context, SetRequest[nativeRecord]) error = Set[nativeRecord]

func TestSetRefusesUnboundCapabilitiesAndContext(t *testing.T) {
	t.Parallel()
	cause := errors.New("Set caller cancellation fixture")
	cancelled, cancel := context.WithCancelCause(t.Context())
	cancel(cause)
	for _, tc := range []struct {
		name      string
		ctx       context.Context
		wantCause error
	}{
		{name: "missing transaction and document are refused", ctx: t.Context()},
		{name: "nil context retains its SDK identity", wantCause: core.ErrNilContext},
		{name: "cancelled context retains custom cause", ctx: cancelled, wantCause: cause},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := Set(tc.ctx, SetRequest[nativeRecord]{}); !errors.Is(err, core.ErrFirestoreDocumentsContract) || tc.wantCause != nil && !errors.Is(err, tc.wantCause) {
				t.Fatalf("unbound Set error = %v, want contract and cause %v", err, tc.wantCause)
			}
		})
	}
}

func TestSetEmulatorTransactionLayerTriad(t *testing.T) {
	t.Parallel()
	cause := errors.New("Set caller admission fixture")
	for _, tc := range []struct {
		name      string
		existing  bool
		refuse    bool
		cancel    bool
		invalid   bool
		encoding  bool
		nonStruct bool
		wantCause error
		wantWrite int64
	}{
		{name: "new document is created at transaction commit", wantWrite: 1},
		{name: "replacement removes every omitted old field", existing: true, wantWrite: 1},
		{name: "queued replacement is absent after callback refusal", existing: true, refuse: true, wantCause: cause},
		{name: "cancelled Set does not queue a native write", existing: true, cancel: true, wantCause: cause},
		{name: "invalid owning record does not queue a native write", existing: true, invalid: true, wantCause: core.ErrFirestoreDocumentsContract},
		{name: "native encoding refusal preserves operation identity", existing: true, encoding: true, wantCause: core.ErrFirestoreDocumentsOperation},
		{name: "nonstruct typed value refuses before native serialization", existing: true, nonStruct: true, wantCause: core.ErrFirestoreDocumentsContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			ref := nativeRef(client, t)
			prior := setPriorRecord{Owner: "prior-owner", Value: 7, Retired: "must disappear on replace"}
			if tc.existing {
				_, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
					return Create(ctx, CreateRequest[setPriorRecord]{Reference: ref, Transaction: tx, Record: prior})
				}})
				if err != nil {
					t.Fatal(err)
				}
			}
			beforeWrites, beforeReads := costs.writes.Load(), costs.reads.Load()
			want := nativeRecord{Owner: "replacement-owner", Value: 29}
			setReturned := false
			result, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
				request := SetRequest[nativeRecord]{Reference: ref, Transaction: tx, Record: want}
				if tc.invalid {
					request.Record.Owner = ""
				}
				if tc.cancel {
					cancelled, cancel := context.WithCancelCause(ctx)
					cancel(cause)
					ctx = cancelled
				}
				var setErr error
				if tc.nonStruct {
					setErr = Set(ctx, SetRequest[nonStructRecord]{Reference: ref, Transaction: tx, Record: "not a document struct"})
				} else if tc.encoding {
					setErr = Set(ctx, SetRequest[setUnsupportedRecord]{Reference: ref, Transaction: tx, Record: setUnsupportedRecord{Channel: make(chan int)}})
				} else {
					setErr = Set(ctx, request)
				}
				setReturned = true
				if tc.refuse {
					if setErr != nil {
						return setErr
					}
					return cause
				}
				return setErr
			}})
			if !setReturned {
				t.Fatal("Set reached = false, want actual SDK call before verdict")
			}
			if tc.wantCause != nil {
				if !errors.Is(err, tc.wantCause) || !errors.Is(err, core.ErrFirestoreDocumentsOperation) || result != (TransactionResult{}) {
					t.Fatalf("refused transaction = (%+v, %v), want zero and operation plus %v", result, err, tc.wantCause)
				}
			} else if err != nil || result.Validate() != nil {
				t.Fatalf("committed transaction = (%+v, %v), want valid native commitment", result, err)
			}
			got, readErr := Get[setPriorRecord](t.Context(), GetRequest{Reference: ref})
			wantRecord := prior
			if tc.wantCause == nil {
				wantRecord = setPriorRecord{Owner: want.Owner, Value: want.Value}
			}
			if readErr != nil || got.Record != wantRecord {
				t.Fatalf("persisted replacement = (%+v, %v), want (%+v, nil)", got, readErr, wantRecord)
			}
			if writes, reads := costs.writes.Load()-beforeWrites, costs.reads.Load()-beforeReads; writes != tc.wantWrite || reads != 1 {
				t.Fatalf("native exercised writes/reads = %d/%d, want %d/1", writes, reads, tc.wantWrite)
			}
			reportNativeCosts(t, costs)
		})
	}
}

func TestSetEmulatorQueueDoesNotCommitIndependently(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	ref := nativeRef(client, t)
	want := nativeRecord{Owner: "queued-owner", Value: 31}
	result, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
		if err := Set(ctx, SetRequest[nativeRecord]{Reference: ref, Transaction: tx, Record: want}); err != nil {
			return err
		}
		before, readErr := Get[nativeRecord](ctx, GetRequest{Reference: ref})
		if status.Code(readErr) != codes.NotFound || before != (Document[nativeRecord]{}) || costs.writes.Load() != 0 {
			t.Fatalf("queued visibility = (%+v, %v, %d writes), want native NotFound, zero record and zero committed writes", before, readErr, costs.writes.Load())
		}
		return nil
	}})
	if err != nil || result.Validate() != nil {
		t.Fatalf("final commit = (%+v, %v), want valid native commitment", result, err)
	}
	got, err := Get[nativeRecord](t.Context(), GetRequest{Reference: ref})
	if err != nil || got.Record != want || costs.writes.Load() != 1 || costs.reads.Load() != 2 {
		t.Fatalf("committed visibility = (%+v, %v, %d writes, %d reads), want (%+v, nil, 1 write, 2 reads)", got, err, costs.writes.Load(), costs.reads.Load(), want)
	}
	reportNativeCosts(t, costs)
}
