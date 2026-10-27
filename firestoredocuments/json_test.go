package firestoredocuments

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync/atomic"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// This is the actual JSON-tagged caller record; no Firestore DTO mirrors it.
type jsonCallerRecord struct {
	Owner string `json:"owner"`
	Value int64  `json:"value"`
}

func (r jsonCallerRecord) Validate() error {
	return nativeRecord(r).Validate()
}

// Provider fixtures deliberately admit facts the caller's schema can refuse.
type jsonProviderRecord struct {
	Owner string `firestore:"owner"`
	Value int64  `firestore:"value"`
	Extra string `firestore:"extra,omitempty"`
}

func (jsonProviderRecord) Validate() error { return nil }

var _ func(context.Context, GetRequest) (JSONDocument[jsonCallerRecord], error) = GetJSON[jsonCallerRecord]

func TestGetJSONRefusesMissingReferencesAndContext(t *testing.T) {
	t.Parallel()
	cause := errors.New("JSON admission cancellation fixture")
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(cause)
	for _, tc := range []struct {
		name      string
		ctx       context.Context
		wantCause error
	}{
		{name: "missing reference refuses before native read", ctx: t.Context()},
		{name: "nil context retains typed refusal", wantCause: core.ErrNilContext},
		{name: "cancelled context retains custom cause", ctx: ctx, wantCause: cause},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := GetJSON[jsonCallerRecord](tc.ctx, GetRequest{})
			if !errors.Is(err, core.ErrFirestoreDocumentsContract) || tc.wantCause != nil && !errors.Is(err, tc.wantCause) || got != (JSONDocument[jsonCallerRecord]{}) {
				t.Fatalf("unbound JSON = (%+v, %v), want zero, contract and %v", got, err, tc.wantCause)
			}
		})
	}
}

func TestGetJSONEmulatorStrictCallerRecordLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		record    jsonProviderRecord
		wrongType bool
		wantErr   error
		wantJSON  bool
	}{
		{name: "JSON member names decode without Firestore tags", record: jsonProviderRecord{Owner: "caller", Value: 17}},
		{name: "zero integer is a real admitted value", record: jsonProviderRecord{Owner: "zero"}},
		{name: "minimum native integer remains exact", record: jsonProviderRecord{Owner: "minimum", Value: math.MinInt64}},
		{name: "maximum native integer remains exact", record: jsonProviderRecord{Owner: "maximum", Value: math.MaxInt64}},
		{name: "unicode and escaped member value remain exact", record: jsonProviderRecord{Owner: "Élaine\n\"日本語", Value: -1}},
		{name: "valid multiwindow field is not a transfer quota", record: jsonProviderRecord{Owner: strings.Repeat("x", 32*1024+1), Value: 1}},
		{name: "unknown provider member cannot silently disappear", record: jsonProviderRecord{Owner: "caller", Value: 17, Extra: "future-field"}, wantErr: core.ErrFirestoreDocumentsOperation, wantJSON: true},
		{name: "wrong native member type releases no partial caller", record: jsonProviderRecord{Owner: "caller"}, wrongType: true, wantErr: core.ErrFirestoreDocumentsOperation, wantJSON: true},
		{name: "decoded record still crosses owning validation", record: jsonProviderRecord{Value: 17}, wantErr: core.ErrFirestoreDocumentsContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			ref := nativeRef(client, t)
			_, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
				if tc.wrongType {
					return Set(ctx, SetRequest[incompatibleRecord]{Reference: ref, Transaction: tx, Record: incompatibleRecord{Owner: tc.record.Owner, Value: "not an integer"}})
				}
				return Set(ctx, SetRequest[jsonProviderRecord]{Reference: ref, Transaction: tx, Record: tc.record})
			}})
			if err != nil {
				t.Fatal(err)
			}
			got, err := GetJSON[jsonCallerRecord](t.Context(), GetRequest{Reference: ref})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (JSONDocument[jsonCallerRecord]{}) {
					t.Fatalf("refused JSON record = (%+v, %v), want zero and %v", got, err, tc.wantErr)
				}
				if _, ok := errors.AsType[*json.SemanticError](err); tc.wantJSON && !ok {
					t.Fatalf("JSON refusal = %v, want retained native SemanticError", err)
				}
			} else {
				want := jsonCallerRecord{Owner: tc.record.Owner, Value: tc.record.Value}
				if err != nil || got.Record != want || got.Validate() != nil {
					t.Fatalf("JSON record = (%+v, %v), want (%+v, nil)", got, err, want)
				}
			}
			if reads, writes := costs.reads.Load(), costs.writes.Load(); reads != 1 || writes != 1 {
				t.Fatalf("native exercised reads/writes = %d/%d, want 1/1", reads, writes)
			}
			reportNativeCosts(t, costs)
		})
	}
}

func TestGetJSONEmulatorTransactionAndRefusalBoundaries(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	ref := nativeRef(client, t)
	want := jsonProviderRecord{Owner: "transaction-owner", Value: 23}
	if _, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
		return Set(ctx, SetRequest[jsonProviderRecord]{Reference: ref, Transaction: tx, Record: want})
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
		got, err := GetJSON[jsonCallerRecord](ctx, GetRequest{Reference: ref, Transaction: tx})
		if err != nil || got.Record != (jsonCallerRecord{Owner: want.Owner, Value: want.Value}) {
			t.Fatalf("transaction JSON = (%+v, %v), want exact committed source", got, err)
		}
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	missing := ref.Parent.Doc(ref.ID + "-missing")
	got, err := GetJSON[jsonCallerRecord](t.Context(), GetRequest{Reference: missing})
	if !errors.Is(err, core.ErrFirestoreDocumentsOperation) || status.Code(err) != codes.NotFound || got != (JSONDocument[jsonCallerRecord]{}) {
		t.Fatalf("absent JSON = (%+v, %v), want zero and native NotFound", got, err)
	}
	before := costs.reads.Load()
	cause := errors.New("JSON caller cancellation fixture")
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(cause)
	got, err = GetJSON[jsonCallerRecord](ctx, GetRequest{Reference: ref})
	if !errors.Is(err, cause) || !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrFirestoreDocumentsContract) || got != (JSONDocument[jsonCallerRecord]{}) {
		t.Fatalf("cancelled JSON = (%+v, %v), want zero and caller cause", got, err)
	}
	scalar, scalarErr := GetJSON[nonStructRecord](t.Context(), GetRequest{Reference: ref})
	if !errors.Is(scalarErr, core.ErrFirestoreDocumentsContract) || scalar != (JSONDocument[nonStructRecord]{}) || costs.reads.Load() != before {
		t.Fatalf("scalar/cancel native reads = (%v, %d), want contract and unchanged %d", scalarErr, costs.reads.Load(), before)
	}
	reportNativeCosts(t, costs)
}

func FuzzGetJSONEmulatorStrictTypedClosure(f *testing.F) {
	client, _ := nativeClient(f)
	seed := jsonProviderRecord{Owner: "canonical-provider", Value: 17}
	seedRef := client.Collection("json-fuzz").Doc("canonical-seed")
	if _, err := RunTransaction(f.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
		return Set(ctx, SetRequest[jsonProviderRecord]{Reference: seedRef, Transaction: tx, Record: seed})
	}}); err != nil {
		f.Fatal(err)
	}
	canonical, err := GetJSON[jsonCallerRecord](f.Context(), GetRequest{Reference: seedRef})
	if err != nil || canonical.Record != (jsonCallerRecord{Owner: seed.Owner, Value: seed.Value}) || canonical.Validate() != nil {
		f.Fatalf("native canonical seed = (%+v, %v), want validated producer facts", canonical, err)
	}
	f.Add(canonical.Record.Owner, canonical.Record.Value, uint8(0))
	f.Add("", int64(0), uint8(0))
	f.Add("unknown-member", int64(3), uint8(1))
	f.Add("wrong-type", int64(-1), uint8(2))
	var ordinal atomic.Uint64
	f.Fuzz(func(t *testing.T, owner string, value int64, mode uint8) {
		ref := client.Collection("json-fuzz").Doc(fmt.Sprintf("mutation-%d", ordinal.Add(1)))
		defer func() {
			if _, err := ref.Delete(t.Context()); err != nil {
				t.Errorf("native fuzz fixture cleanup = %v, want nil", err)
			}
		}()
		mode %= 3
		input := jsonProviderRecord{Owner: owner, Value: value}
		if mode == 1 {
			input.Extra = "unknown-member"
		}
		commit, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
			if mode == 2 {
				return Set(ctx, SetRequest[incompatibleRecord]{Reference: ref, Transaction: tx, Record: incompatibleRecord{Owner: owner, Value: "wrong-type"}})
			}
			return Set(ctx, SetRequest[jsonProviderRecord]{Reference: ref, Transaction: tx, Record: input})
		}})
		if err != nil {
			if !errors.Is(err, core.ErrFirestoreDocumentsContract) || commit != (TransactionResult{}) {
				t.Fatalf("native producer refusal = (%+v, %v), want zero and SDK identity", commit, err)
			}
			return
		}
		got, err := GetJSON[jsonCallerRecord](t.Context(), GetRequest{Reference: ref})
		if mode != 0 || owner == "" {
			if !errors.Is(err, core.ErrFirestoreDocumentsContract) || got != (JSONDocument[jsonCallerRecord]{}) {
				t.Fatalf("strict JSON refusal = (%+v, %v), want zero and typed refusal", got, err)
			}
			return
		}
		want := jsonCallerRecord{Owner: owner, Value: value}
		wire, encodeErr := core.MarshalCanonicalJSONDocument(want)
		if err != nil || encodeErr != nil || got.Record != want || got.Validate() != nil || got.Bytes.Uint64() != uint64(len(wire)) {
			t.Fatalf("strict JSON admission = (%+v, %v), want (%+v, nil)", got, err, want)
		}
	})
}
