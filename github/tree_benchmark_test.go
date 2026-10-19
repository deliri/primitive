package github

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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

// drainingTreeVisitor consumes every path through a fixed window and checks
// the sealed observation, as a real tree consumer must.
type drainingTreeVisitor struct {
	entries, pathBytes uint64
}

func (v *drainingTreeVisitor) VisitGitHubTreeEntry(stream *TreeEntryStream) error {
	var window [64]byte
	for {
		n, err := stream.Read(window[:])
		v.pathBytes += uint64(n)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	entry, err := stream.Observation()
	if err != nil || entry.Kind != TreeEntryBlob {
		return errors.Join(core.ErrGitHubResponse, err)
	}
	v.entries++
	return nil
}

var benchmarkDecodeTreeSink uint64

// This benchmark isolates the streaming tree decoder from HTTP transport on a
// fixed 1024-entry document of repository-shaped paths.
func BenchmarkDecodeTreeEntries(b *testing.B) {
	const entries = 1024
	commit := parsedCommit(b).String()
	fixture := treeResponseFixture{SHA: commit, URL: "https://api.github.com/tree", Tree: make([]treeEntryFixture, 0, entries)}
	wantPathBytes := uint64(0)
	for index := range entries {
		path := "internal/package" + strconv.Itoa(index%32) + "/source_file_" + strconv.Itoa(index) + ".go"
		wantPathBytes += uint64(len(path))
		fixture.Tree = append(fixture.Tree, treeWire(path, "blob", commit))
	}
	payload := marshalGitHubFixture(b, fixture)
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	for b.Loop() {
		visitor := &drainingTreeVisitor{}
		got, err := decodeTree(bytes.NewReader(payload), visitor)
		if err != nil || visitor.pathBytes != wantPathBytes {
			b.Fatalf("decodeTree() = %d/%v path bytes=%d, want %d/nil path bytes=%d", got, err, visitor.pathBytes, entries, wantPathBytes)
		}
		benchmarkDecodeTreeSink = got
	}
	if benchmarkDecodeTreeSink != entries {
		b.Fatalf("decodeTree() entries = %d, want %d", benchmarkDecodeTreeSink, entries)
	}
}
