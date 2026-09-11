package github

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// This benchmark measures one completed provider transfer and one synchronous
// entry delivery, including the owned download context and worker join.
func BenchmarkReadTreeOneEntry(b *testing.B) {
	payload := marshalGitHubFixture(b, treeResponseFixture{SHA: parsedCommit(b).String(), URL: "https://api.github.com/tree", Tree: []treeEntryFixture{treeWire("main.go", "blob", parsedCommit(b).String())}})
	if len(payload) == 0 {
		b.Fatal("tree payload bytes = 0, want nonempty typed document")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
		if _, err := w.Write(payload); err != nil {
			b.Errorf("provider Write()=%v, want nil", err)
		}
	}))
	defer server.Close()
	client := clientFixture(b, server.URL)
	defer func() {
		if err := client.Close(); err != nil {
			b.Errorf("Client.Close()=%v, want nil", err)
		}
	}()
	visitor := &countingTreeVisitor{}
	request := TreeRequest{Repository: parsedRepository(b, "owner/repository"), Commit: parsedCommit(b), Visitor: visitor}
	var got TreeObservation
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	for b.Loop() {
		var err error
		got, err = client.ReadTree(b.Context(), request)
		if err != nil {
			b.Fatalf("ReadTree()=%v, want nil", err)
		}
	}
	if got.Entries != 1 || got.Bytes.Uint64() != uint64(len(payload)) || visitor.count != uint64(b.N) || visitor.bad {
		b.Fatalf("ReadTree()=%+v visits=%d bad=%t, want one entry, %d bytes, %d visits and no invalid entry", got, visitor.count, visitor.bad, len(payload), b.N)
	}
}
