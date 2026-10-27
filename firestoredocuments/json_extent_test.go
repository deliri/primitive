package firestoredocuments

import (
	"context"
	"errors"
	"strings"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/core"
)

type jsonExtentProvider struct {
	Owner   string `firestore:"owner" json:"owner"`
	Padding string `firestore:"padding" json:"padding"`
}

func (r jsonExtentProvider) Validate() error {
	return (nativeRecord{Owner: r.Owner}).Validate()
}

type jsonExtentCaller struct {
	Owner   string `json:"owner"`
	Padding string `json:"padding,omitzero"`
}

func (r jsonExtentCaller) Validate() error {
	return (nativeRecord{Owner: r.Owner}).Validate()
}

func TestGetJSONEmulatorRetainsSourceExtentBeforeTypedOmission(t *testing.T) {
	t.Parallel()
	// This is an exercising caller's policy, not a Primitive transfer quota.
	const callerLimit = 512 * 1024
	base := jsonExtentProvider{Owner: "x"}
	encoded, err := core.MarshalCanonicalJSONDocument(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		size int
	}{
		{name: "source immediately below caller limit", size: callerLimit - 1},
		{name: "source exactly at caller limit", size: callerLimit},
		{name: "source above limit while typed projection is below", size: callerLimit + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			ref := nativeRef(client, t)
			input := jsonExtentProvider{Owner: strings.Repeat("x", tc.size-len(encoded)+1)}
			source, err := core.MarshalCanonicalJSONDocument(input)
			if err != nil || len(source) != tc.size {
				t.Fatalf("source oracle = (%d, %v), want %d bytes", len(source), err, tc.size)
			}
			if _, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
				return Set(ctx, SetRequest[jsonExtentProvider]{Reference: ref, Transaction: tx, Record: input})
			}}); err != nil {
				t.Fatal(err)
			}
			got, err := GetJSON[jsonExtentCaller](t.Context(), GetRequest{Reference: ref})
			if err != nil || got.Record != (jsonExtentCaller{Owner: input.Owner}) || got.Bytes.Uint64() != uint64(len(source)) || got.Validate() != nil {
				t.Fatalf("source extent = (%d, %v), want exact %d and validated original facts", got.Bytes.Uint64(), err, len(source))
			}
			projection, err := core.MarshalCanonicalJSONDocument(got.Record)
			if err != nil || len(projection) >= len(source) {
				t.Fatalf("typed omission = (%d, %v), want less than original %d", len(projection), err, len(source))
			}
			if tc.size > callerLimit && (got.Bytes.Uint64() <= callerLimit || len(projection) > callerLimit) {
				t.Fatalf("caller limit facts = (%d source, %d projection), want source refusal despite smaller projection", got.Bytes.Uint64(), len(projection))
			}
			if costs.reads.Load() != 1 || costs.writes.Load() != 1 {
				t.Fatalf("native point operations = %d reads/%d writes, want 1/1", costs.reads.Load(), costs.writes.Load())
			}
		})
	}
	zero := JSONDocument[jsonExtentCaller]{Record: jsonExtentCaller{Owner: "owned"}}
	if err := zero.Validate(); !errors.Is(err, core.ErrFirestoreDocumentsContract) {
		t.Fatalf("absent source extent = %v, want contract refusal", err)
	}
}
