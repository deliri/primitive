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

func TestTransactionModeExhaustsNativeSelection(t *testing.T) {
	t.Parallel()
	if TransactionReadWrite != 0 || TransactionReadOnly != 1 {
		t.Fatalf("native modes = %d/%d, want default0/read-only1", TransactionReadWrite, TransactionReadOnly)
	}
	for value := uint16(0); value < 256; value++ {
		mode := TransactionMode(value)
		request := TransactionRequest{Client: new(firestore.Client), Mode: mode, Use: func(context.Context, *firestore.Transaction) error { return nil }}
		gotErr := request.Validate()
		if value <= 1 {
			if gotErr != nil {
				t.Fatalf("native mode %d admission = %v, want nil", value, gotErr)
			}
		} else if !errors.Is(gotErr, core.ErrFirestoreDocumentsContract) {
			t.Fatalf("unknown native mode %d admission = %v, want contract refusal", value, gotErr)
		}
	}
}

type nilTransactionContext struct{ context.Context }

func TestReadOnlyAdmissionRefusesBeforeNativeBegin(t *testing.T) {
	t.Parallel()
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	cause := errors.New("read-only caller cancellation fixture")
	custom, cancelCause := context.WithCancelCause(t.Context())
	cancelCause(cause)
	var typedNil *nilTransactionContext
	for _, tc := range []struct {
		name      string
		ctx       context.Context
		mode      TransactionMode
		wantCause error
	}{
		{name: "nil caller refuses before SDK setup", mode: TransactionReadOnly, wantCause: core.ErrNilContext},
		{name: "typed nil caller observation is contained", ctx: typedNil, mode: TransactionReadOnly, wantCause: core.ErrContextObservation},
		{name: "cancelled caller retains native cancellation", ctx: cancelled, mode: TransactionReadOnly, wantCause: context.Canceled},
		{name: "custom caller cancellation retains exact cause", ctx: custom, mode: TransactionReadOnly, wantCause: cause},
		{name: "future mode refuses before callback or RPC", ctx: t.Context(), mode: TransactionMode(2), wantCause: core.ErrFirestoreDocumentsContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			calls := 0
			got, gotErr := RunTransaction(tc.ctx, TransactionRequest{Client: client, Mode: tc.mode, Use: func(context.Context, *firestore.Transaction) error {
				calls++
				return nil
			}})
			if !errors.Is(gotErr, core.ErrFirestoreDocumentsContract) || !errors.Is(gotErr, tc.wantCause) || got != (TransactionResult{}) {
				t.Fatalf("read-only admission = (%+v,%v), want zero/contract/cause %v", got, gotErr, tc.wantCause)
			}
			if calls != 0 || costs.readOnlyBegins.Load() != 0 || costs.readWriteBegins.Load() != 0 || costs.reads.Load() != 0 || costs.writes.Load() != 0 || costs.commits.Load() != 0 {
				t.Fatalf("refused native admission calls=%d begin=%d/%d read=%d write=%d commit=%d, want all zero", calls, costs.readOnlyBegins.Load(), costs.readWriteBegins.Load(), costs.reads.Load(), costs.writes.Load(), costs.commits.Load())
			}
		})
	}
}

func TestDocumentsReadOnlyLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		read          bool
		write         bool
		abort         bool
		cancel        bool
		wantCompleted bool
	}{
		{name: "empty native readonly confirms completion without document effects", wantCompleted: true},
		{name: "typed native read preserves snapshot and readonly provider mode", read: true, wantCompleted: true},
		{name: "native write refusal retains exact SDK cause and zero result", read: true, write: true},
		{name: "native Aborted callback stays single attempt in readonly mode", read: true, abort: true},
		{name: "custom cancellation after actual read retains caller cause", read: true, cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			ref := nativeRef(client, t)
			writeRef := client.Collection("refused-readonly-writes").Doc(ref.ID)
			want := nativeRecord{Owner: "read-only-owner", Value: 19}
			var fixtureWrites int64
			if tc.read {
				if _, err := ref.Create(t.Context(), want); err != nil {
					t.Fatal(err)
				}
				fixtureWrites = 1
			}
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			cause := errors.New("read-only cancellation after native read")
			aborted := status.Error(codes.Aborted, "native callback refusal fixture")
			var nativeWriteErr, sdkWriteErr error
			var read Document[nativeRecord]
			calls := 0
			got, gotErr := RunTransaction(ctx, TransactionRequest{Client: client, Mode: TransactionReadOnly, Use: func(ctx context.Context, tx *firestore.Transaction) error {
				calls++
				if tc.read {
					var err error
					read, err = Get[nativeRecord](ctx, GetRequest{Reference: ref, Transaction: tx})
					if err != nil {
						return err
					}
				}
				if tc.write {
					// The native sentinel is private to the official SDK. Capture its
					// actual identity, then require the public SDK path to retain it.
					nativeWriteErr = tx.Create(writeRef, want)
					sdkWriteErr = Create(ctx, CreateRequest[nativeRecord]{Reference: writeRef, Transaction: tx, Record: want})
					if sdkWriteErr == nil {
						// A broken mode must roll back the counterexample's queued
						// writes; the assertions below still reject that broken mode.
						return core.ErrFirestoreDocumentsContract
					}
					return sdkWriteErr
				}
				if tc.abort {
					return aborted
				}
				if tc.cancel {
					cancel(cause)
					return ctx.Err()
				}
				return nil
			}})
			if tc.wantCompleted {
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("read-only completion = (%+v,%v), want confirmed native completion", got, gotErr)
				}
			} else if got != (TransactionResult{}) || !errors.Is(gotErr, core.ErrFirestoreDocumentsOperation) {
				t.Fatalf("read-only refusal = (%+v,%v), want zero/native operation refusal", got, gotErr)
			}
			if tc.write && (nativeWriteErr == nil || !errors.Is(sdkWriteErr, nativeWriteErr) || !errors.Is(gotErr, nativeWriteErr) || !errors.Is(sdkWriteErr, core.ErrFirestoreDocumentsOperation)) {
				t.Fatalf("native write refusal = %v SDK=%v transaction=%v, want same native cause through both SDK boundaries", nativeWriteErr, sdkWriteErr, gotErr)
			}
			if tc.abort && !errors.Is(gotErr, aborted) {
				t.Fatalf("read-only callback refusal = %v, want native Aborted cause %v", gotErr, aborted)
			}
			if tc.cancel && (!errors.Is(gotErr, cause) || !errors.Is(gotErr, context.Canceled)) {
				t.Fatalf("cancelled read-only transaction = %v, want cancellation and exact cause %v", gotErr, cause)
			}
			if tc.read && read.Record != want {
				t.Fatalf("native snapshot = %+v, want %+v", read.Record, want)
			}
			var wantReads int64
			if tc.read {
				wantReads = 1
			}
			if tc.write {
				absent, err := Get[nativeRecord](t.Context(), GetRequest{Reference: writeRef})
				if status.Code(err) != codes.NotFound || absent != (Document[nativeRecord]{}) {
					t.Fatalf("refused write visibility = (%+v,%v), want zero/native NotFound", absent, err)
				}
				wantReads++
			}
			wantCommits := fixtureWrites
			if tc.wantCompleted {
				wantCommits++
			}
			if calls != 1 || costs.readOnlyBegins.Load() != 1 || costs.readWriteBegins.Load() != 0 || costs.reads.Load() != wantReads || costs.writes.Load() != fixtureWrites || costs.commits.Load() != wantCommits {
				t.Fatalf("read-only calls=%d native begin=%d/%d reads=%d writes=%d commits=%d, want1/1/0/%d/%d/%d", calls, costs.readOnlyBegins.Load(), costs.readWriteBegins.Load(), costs.reads.Load(), costs.writes.Load(), costs.commits.Load(), wantReads, fixtureWrites, wantCommits)
			}
			reportNativeCosts(t, costs)
		})
	}
}
