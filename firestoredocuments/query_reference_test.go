package firestoredocuments

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// referenceRecord carries a well-formed identity inside its payload. The
// payload never names the physical row; only the observed reference does.
type referenceRecord struct {
	ID    string `firestore:"id" json:"id"`
	Value int64  `firestore:"value" json:"value"`
}

func (r referenceRecord) Validate() error {
	if r.ID == "" {
		return core.ErrFirestoreDocumentsContract
	}
	return nil
}

type referenceQueryDoor struct {
	name  string
	query func(context.Context, QueryRequest[referenceRecord]) error
}

func referenceQueryDoors() []referenceQueryDoor {
	return []referenceQueryDoor{
		{name: "native decoder", query: Query[referenceRecord]},
		{name: "integer JSON projection", query: QueryIntegerJSON[referenceRecord]},
	}
}

// queryNameRewrite replaces the actual emulator row name before the SDK builds
// its snapshot, so the production path observes a controlled native response.
type queryNameRewrite struct {
	name   func(string) string
	cancel context.CancelCauseFunc
	cause  error
}

type queryNameRewriteStream struct {
	grpc.ClientStream
	rewrite *queryNameRewrite
}

func (s *queryNameRewriteStream) RecvMsg(value any) error {
	err := s.ClientStream.RecvMsg(value)
	if response, ok := value.(*firestorepb.RunQueryResponse); err == nil && ok && response.Document != nil {
		response.Document.Name = s.rewrite.name(response.Document.Name)
		if s.rewrite.cancel != nil {
			s.rewrite.cancel(s.rewrite.cause)
		}
	}
	return err
}

func (r *queryNameRewrite) stream(ctx context.Context, description *grpc.StreamDesc, connection *grpc.ClientConn, method string, streamer grpc.Streamer, options ...grpc.CallOption) (grpc.ClientStream, error) {
	stream, err := streamer(ctx, description, connection, method, options...)
	if err != nil || !strings.HasSuffix(method, "/RunQuery") {
		return stream, err
	}
	return &queryNameRewriteStream{ClientStream: stream, rewrite: r}, nil
}

// A cursor taken from the observed reference pages every physical row exactly
// once even when payload identities name other rows or nothing at all.
func TestQueryDocumentReferenceIsTheActualNativeRow(t *testing.T) {
	t.Parallel()
	for _, door := range referenceQueryDoors() {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			observed := new(queryObservation)
			client, costs := nativeClient(t, grpc.WithChainStreamInterceptor(observed.stream))
			collection := client.Collection("query-reference").Doc(strings.ReplaceAll(t.Name(), "/", "-")).Collection("staff")
			ids := []string{"row-a", "row-b", "row-c"}
			stored := []referenceRecord{{ID: "row-c", Value: math.MinInt64}, {ID: "forged", Value: math.MaxInt64}, {ID: "row-c", Value: 0}}
			for i, id := range ids {
				if _, err := collection.Doc(id).Set(t.Context(), stored[i]); err != nil {
					t.Fatal(err)
				}
			}
			beforeReads, beforeWrites := costs.reads.Load(), costs.writes.Load()
			var gotPaths, gotIDs, gotParents []string
			var gotRecords []referenceRecord
			var cursor *firestore.DocumentRef
			pages := 0
			for {
				page := collection.OrderBy(firestore.DocumentID, firestore.Asc).Limit(1)
				if cursor != nil {
					page = page.StartAfter(cursor)
				}
				visits := 0
				err := door.query(t.Context(), QueryRequest[referenceRecord]{Query: page, Visit: func(_ context.Context, d Document[referenceRecord]) (QueryVisitDecision, error) {
					visits++
					if d.Reference == nil {
						t.Fatalf("visited record %+v has nil Reference, want actual native row reference", d.Record)
					}
					cursor = d.Reference
					gotPaths = append(gotPaths, d.Reference.Path)
					gotIDs = append(gotIDs, d.Reference.ID)
					gotParents = append(gotParents, d.Reference.Parent.Path)
					gotRecords = append(gotRecords, d.Record)
					return QueryContinue, nil
				}})
				pages++
				if err != nil {
					t.Fatalf("page %d query = %v, want nil", pages, err)
				}
				if visits == 0 || pages > len(ids)+1 {
					break
				}
			}
			wantPaths := []string{collection.Doc("row-a").Path, collection.Doc("row-b").Path, collection.Doc("row-c").Path}
			wantParents := []string{collection.Path, collection.Path, collection.Path}
			if !slices.Equal(gotPaths, wantPaths) || !slices.Equal(gotIDs, ids) || !slices.Equal(gotParents, wantParents) {
				t.Fatalf("observed reference paths/IDs/parents = %v/%v/%v, want %v/%v/%v", gotPaths, gotIDs, gotParents, wantPaths, ids, wantParents)
			}
			if !slices.Equal(gotRecords, stored) {
				t.Fatalf("records = %+v, want exact stored payloads including int64 extremes %+v", gotRecords, stored)
			}
			if reads, writes, queries := costs.reads.Load()-beforeReads, costs.writes.Load()-beforeWrites, costs.queries.Load(); reads != 0 || writes != 0 || queries != 4 || observed.documents.Load() != 3 || pages != 4 {
				t.Fatalf("point reads/writes/query RPCs/delivered rows/pages = %d/%d/%d/%d/%d, want 0/0/4/3/4", reads, writes, queries, observed.documents.Load(), pages)
			}
			queryContextsStopped(t, observed, 4)
		})
	}
}

func TestGetDocumentReferenceIsTheSnapshotReference(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	ref := client.Collection("get-reference").Doc(strings.ReplaceAll(t.Name(), "/", "-"))
	stored := referenceRecord{ID: "forged", Value: math.MaxInt64}
	if _, err := ref.Set(t.Context(), stored); err != nil {
		t.Fatal(err)
	}
	got, err := Get[referenceRecord](t.Context(), GetRequest{Reference: ref})
	if err != nil || got.Record != stored || got.Reference == nil || got.Reference.Path != ref.Path || got.Reference.ID != ref.ID {
		t.Fatalf("Get = %+v/%v, want record %+v at exact reference %s", got, err, stored, ref.Path)
	}
	var inTransaction Document[referenceRecord]
	if _, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Mode: TransactionReadOnly, Use: func(ctx context.Context, tx *firestore.Transaction) error {
		var err error
		inTransaction, err = Get[referenceRecord](ctx, GetRequest{Reference: ref, Transaction: tx})
		return err
	}}); err != nil || inTransaction.Record != stored || inTransaction.Reference == nil || inTransaction.Reference.Path != ref.Path {
		t.Fatalf("transactional Get = %+v/%v, want record %+v at exact reference %s", inTransaction, err, stored, ref.Path)
	}
	missing := ref.Parent.Doc(ref.ID + "-missing")
	absent, err := Get[referenceRecord](t.Context(), GetRequest{Reference: missing})
	if !errors.Is(err, core.ErrFirestoreDocumentsOperation) || status.Code(err) != codes.NotFound || absent != (Document[referenceRecord]{}) {
		t.Fatalf("missing Get = %+v/%v, want zero Document with nil Reference and operation/NotFound", absent, err)
	}
	if reads, queries := costs.reads.Load(), costs.queries.Load(); reads != 3 || queries != 0 {
		t.Fatalf("point reads/query RPCs = %d/%d, want 3/0", reads, queries)
	}
}

// The SDK builds Ref from the returned row name without checking that it lies
// inside the executed query. Each row rewrites only that name.
func TestQueryObservedReferenceMustLieInsideBoundParent(t *testing.T) {
	t.Parallel()
	const project, foreignProject = "projects/primitive-documents-proof/", "projects/primitive-foreign-proof/"
	cancelCause := errors.New("caller cancellation while a foreign row arrives")
	for _, row := range []struct {
		name       string
		rewrite    func(name, parentID string) string
		cancel     bool
		wantSuffix string
		wantErr    error
	}{
		{name: "unchanged native row is admitted with its exact reference", rewrite: func(name, _ string) string { return name }, wantSuffix: "/items/row"},
		{name: "descendant inside the bound parent is admitted exactly as returned", rewrite: func(name, _ string) string { return name + "/nested/deeper" }, wantSuffix: "/items/row/nested/deeper"},
		{name: "foreign project refuses", rewrite: func(name, _ string) string { return strings.Replace(name, project, foreignProject, 1) }, wantErr: core.ErrFirestoreDocumentsOperation},
		{name: "foreign database refuses", rewrite: func(name, _ string) string {
			return strings.Replace(name, "/databases/(default)/", "/databases/other-database/", 1)
		}, wantErr: core.ErrFirestoreDocumentsOperation},
		{name: "sibling root collection outside the bound parent refuses", rewrite: func(name, parentID string) string {
			return strings.Replace(name, "/query-reference-bound/"+parentID+"/items/row", "/other/row", 1)
		}, wantErr: core.ErrFirestoreDocumentsOperation},
		{name: "parent identity extended without separator refuses", rewrite: func(name, parentID string) string {
			return strings.Replace(name, "/"+parentID+"/", "/"+parentID+"x/", 1)
		}, wantErr: core.ErrFirestoreDocumentsOperation},
		{name: "empty document identity has no native reference and refuses", rewrite: func(name, _ string) string { return strings.TrimSuffix(name, "row") }, wantErr: core.ErrFirestoreDocumentsOperation},
		{name: "collection-shaped name remains the SDK refusal", rewrite: func(name, _ string) string { return name + "/nested" }, wantErr: core.ErrFirestoreDocumentsOperation},
		{name: "foreign row and caller cancellation both survive", rewrite: func(name, _ string) string { return strings.Replace(name, project, foreignProject, 1) }, cancel: true, wantErr: cancelCause},
	} {
		for _, door := range referenceQueryDoors() {
			t.Run(row.name+"/"+door.name, func(t *testing.T) {
				t.Parallel()
				ctx, cancel := context.WithCancelCause(t.Context())
				defer cancel(nil)
				parentID := strings.ReplaceAll(t.Name(), "/", "-")
				rewrite := &queryNameRewrite{name: func(name string) string { return row.rewrite(name, parentID) }, cause: cancelCause}
				if row.cancel {
					rewrite.cancel = cancel
				}
				observed := new(queryObservation)
				client, costs := nativeClient(t, grpc.WithChainStreamInterceptor(observed.stream, rewrite.stream))
				parent := client.Collection("query-reference-bound").Doc(parentID)
				stored := referenceRecord{ID: "payload", Value: math.MinInt64}
				if _, err := parent.Collection("items").Doc("row").Set(t.Context(), stored); err != nil {
					t.Fatal(err)
				}
				beforeReads, beforeWrites := costs.reads.Load(), costs.writes.Load()
				var visited []Document[referenceRecord]
				err := door.query(ctx, QueryRequest[referenceRecord]{Query: parent.Collection("items").Query, Visit: func(_ context.Context, d Document[referenceRecord]) (QueryVisitDecision, error) {
					visited = append(visited, d)
					return QueryContinue, nil
				}})
				if row.wantErr == nil {
					want := parent.Path + row.wantSuffix
					if err != nil || len(visited) != 1 || visited[0].Reference == nil || visited[0].Reference.Path != want || visited[0].Record != stored {
						t.Fatalf("bound query = %v with %d visits, want nil and one record %+v at exact reference %s", err, len(visited), stored, want)
					}
				} else {
					if !errors.Is(err, core.ErrFirestoreDocumentsOperation) || !errors.Is(err, row.wantErr) || len(visited) != 0 {
						t.Fatalf("refused query = %v with %d visits, want operation and %v with no visit", err, len(visited), row.wantErr)
					}
					if row.cancel != errors.Is(err, context.Canceled) {
						t.Fatalf("refused query cancellation = %v, want Canceled present=%t", err, row.cancel)
					}
				}
				if reads, writes, queries := costs.reads.Load()-beforeReads, costs.writes.Load()-beforeWrites, costs.queries.Load(); reads != 0 || writes != 0 || queries != 1 || observed.documents.Load() != 1 {
					t.Fatalf("point reads/writes/query RPCs/delivered rows = %d/%d/%d/%d, want 0/0/1/1", reads, writes, queries, observed.documents.Load())
				}
				queryContextsStopped(t, observed, 1)
			})
		}
	}
}
