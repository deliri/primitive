package firestoredocuments

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/core"
)

// jsonQueryRecord is JSON-shaped the way Kernel documents are: an unsigned
// revision Firestore's native decoder cannot set, a nested record and a text
// identity. The query must admit exactly what the point read admits.
type jsonQueryRecord struct {
	Owner    string               `json:"owner"`
	Revision uint64               `json:"revision"`
	Nested   jsonQueryNestedValue `json:"nested"`
}

type jsonQueryNestedValue struct {
	Index uint8 `json:"index"`
}

func (r jsonQueryRecord) Validate() error {
	if r.Owner == "" || r.Revision == 0 {
		return core.ErrFirestoreDocumentsContract
	}
	return nil
}

func jsonQueryCollection(t *testing.T, client *firestore.Client) *firestore.CollectionRef {
	t.Helper()
	return client.Collection("query-integer-json").Doc(strings.ReplaceAll(t.Name(), "/", "-")).Collection("rows")
}

func TestQueryIntegerJSONEmulatorReadsJSONShapedRowsInOrder(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	collection := jsonQueryCollection(t, client)
	for i, row := range []map[string]any{
		{"owner": "target", "revision": int64(7), "nested": map[string]any{"index": int64(2)}},
		{"owner": "target", "revision": int64(1 << 62), "nested": map[string]any{"index": int64(0)}},
		{"owner": "target", "revision": int64(3), "nested": map[string]any{"index": int64(1)}},
		{"owner": "foreign", "revision": int64(9), "nested": map[string]any{"index": int64(0)}},
	} {
		if _, err := collection.Doc(fmt.Sprint(i)).Set(t.Context(), row); err != nil {
			t.Fatal(err)
		}
	}
	beforeReads, beforeWrites := costs.reads.Load(), costs.writes.Load()
	query := collection.Where("owner", "==", "target").OrderBy("nested.index", firestore.Asc).OrderBy(firestore.DocumentID, firestore.Asc).Limit(3)
	var seen []jsonQueryRecord
	visitor := func(ctx context.Context, d Document[jsonQueryRecord]) (QueryVisitDecision, error) {
		if ctx != t.Context() {
			t.Fatal("visitor must receive the exact caller context")
		}
		seen = append(seen, d.Record)
		return QueryContinue, nil
	}
	want := []jsonQueryRecord{{Owner: "target", Revision: 1 << 62, Nested: jsonQueryNestedValue{Index: 0}}, {Owner: "target", Revision: 3, Nested: jsonQueryNestedValue{Index: 1}}, {Owner: "target", Revision: 7, Nested: jsonQueryNestedValue{Index: 2}}}
	if err := QueryIntegerJSON(t.Context(), QueryRequest[jsonQueryRecord]{Query: query, Visit: visitor}); err != nil || !slices.Equal(seen, want) {
		t.Fatalf("QueryIntegerJSON = %+v/%v, want %+v/nil", seen, err, want)
	}
	// The native decoder cannot set the unsigned member; the JSON door exists
	// for exactly this document shape.
	if err := Query(t.Context(), QueryRequest[jsonQueryRecord]{Query: query, Visit: visitor}); !errors.Is(err, core.ErrFirestoreDocumentsOperation) {
		t.Fatalf("native Query of an unsigned JSON member = %v, want ErrFirestoreDocumentsOperation", err)
	}
	seen = nil
	if err := QueryIntegerJSON(t.Context(), QueryRequest[jsonQueryRecord]{Query: collection.Where("owner", "==", "absent"), Visit: visitor}); err != nil || len(seen) != 0 {
		t.Fatalf("empty QueryIntegerJSON = %+v/%v, want no visits/nil", seen, err)
	}
	if reads, writes, queries := costs.reads.Load()-beforeReads, costs.writes.Load()-beforeWrites, costs.queries.Load(); reads != 0 || writes != 0 || queries != 3 {
		t.Fatalf("point reads/writes/query RPCs = %d/%d/%d, want 0/0/3 (not a billing claim)", reads, writes, queries)
	}
}

func TestQueryIntegerJSONEmulatorRefusesRowsThePointReadRefuses(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		data map[string]any
		want error
	}{
		{"native double revision refuses before projection", map[string]any{"owner": "target", "revision": float64(7), "nested": map[string]any{"index": int64(0)}}, core.ErrFirestoreDocumentsOperation},
		{"unknown member is not discarded", map[string]any{"owner": "target", "revision": int64(7), "nested": map[string]any{"index": int64(0)}, "extra": "member"}, core.ErrFirestoreDocumentsOperation},
		{"owner-invalid record never reaches the visitor", map[string]any{"owner": "target", "revision": int64(0), "nested": map[string]any{"index": int64(0)}}, core.ErrFirestoreDocumentsContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			collection := jsonQueryCollection(t, client)
			if _, err := collection.Doc("row").Set(t.Context(), row.data); err != nil {
				t.Fatal(err)
			}
			visits := 0
			err := QueryIntegerJSON(t.Context(), QueryRequest[jsonQueryRecord]{Query: collection.Where("owner", "==", "target"), Visit: func(context.Context, Document[jsonQueryRecord]) (QueryVisitDecision, error) {
				visits++
				return QueryContinue, nil
			}})
			if !errors.Is(err, row.want) || visits != 0 {
				t.Fatalf("QueryIntegerJSON = %v with %d visits, want %v and no visit", err, visits, row.want)
			}
			if queries := costs.queries.Load(); queries != 1 {
				t.Fatalf("query RPCs = %d, want exactly one refused query", queries)
			}
		})
	}
}

func TestQueryIntegerJSONRefusesBeforeNativeEffects(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	collection := jsonQueryCollection(t, client)
	visitor := func(context.Context, Document[jsonQueryRecord]) (QueryVisitDecision, error) {
		return QueryContinue, nil
	}
	if err := QueryIntegerJSON(t.Context(), QueryRequest[jsonQueryRecord]{Query: collection.Query}); !errors.Is(err, core.ErrFirestoreDocumentsContract) {
		t.Fatalf("absent visitor = %v, want ErrFirestoreDocumentsContract", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := QueryIntegerJSON(ctx, QueryRequest[jsonQueryRecord]{Query: collection.Query, Visit: visitor}); !errors.Is(err, core.ErrFirestoreDocumentsContract) {
		t.Fatalf("canceled admission = %v, want ErrFirestoreDocumentsContract", err)
	}
	if queries := costs.queries.Load(); queries != 0 {
		t.Fatalf("query RPCs = %d, want none before admission", queries)
	}
}
