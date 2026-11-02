package firestoredocuments

import (
	"context"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type integerWriteRecord struct {
	Owner string                  `json:"owner"`
	When  temporal.NumericInstant `json:"when"`
}

func (r integerWriteRecord) Validate() error {
	if r.Owner == "" {
		return core.ErrFirestoreDocumentsContract
	}
	return r.When.Validate()
}

func integerWriteFixture(t testing.TB, value int64) integerWriteRecord {
	t.Helper()
	when, err := temporal.NewNumericInstant(temporal.InstantFromNanoseconds(value))
	if err != nil {
		t.Fatal(err)
	}
	return integerWriteRecord{Owner: "actual-caller", When: when}
}

var (
	_ func(context.Context, CreateRequest[integerWriteRecord]) (core.ByteCount, error) = CreateIntegerJSON[integerWriteRecord]
	_ func(context.Context, SetRequest[integerWriteRecord]) (core.ByteCount, error)    = SetIntegerJSON[integerWriteRecord]
)

func TestIntegerJSONWriteUsesActualCallerCodecs(t *testing.T) {
	t.Parallel()
	for _, create := range []bool{true, false} {
		name := "replace follows caller JSON codecs"
		if create {
			name = "create follows caller JSON codecs"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			ref := nativeRef(client, t)
			want := integerWriteFixture(t, 1001)
			encoded, err := core.MarshalCanonicalJSONDocument(want)
			if err != nil {
				t.Fatal(err)
			}
			var extent core.ByteCount
			result, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
				var writeErr error
				if create {
					extent, writeErr = CreateIntegerJSON(ctx, CreateRequest[integerWriteRecord]{Reference: ref, Transaction: tx, Record: want})
				} else {
					extent, writeErr = SetIntegerJSON(ctx, SetRequest[integerWriteRecord]{Reference: ref, Transaction: tx, Record: want})
				}
				return writeErr
			}})
			if err != nil || result.Validate() != nil {
				t.Fatalf("JSON write commit=(%+v,%v), want committed", result, err)
			}
			snapshot, err := ref.Get(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			native := snapshot.Data()
			value, integer := native["when"].(int64)
			bytes, bytesErr := extent.Uint64()
			if !integer || value != 1001 || native["owner"] != want.Owner || len(native) != 2 || bytesErr != nil || bytes != uint64(len(encoded)) {
				t.Fatalf("stored native=%v extent=%d/%v, want JSON names, INTEGER1001 and exact extent%d", native, bytes, bytesErr, len(encoded))
			}
			if reads, writes := costs.reads.Load(), costs.writes.Load(); reads != 1 || writes != 1 {
				t.Fatalf("JSON write reads/writes=%d/%d, want1/1", reads, writes)
			}
			reportNativeCosts(t, costs)
		})
	}
}
