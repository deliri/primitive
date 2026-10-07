package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"iter"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestContentStreamOwnedLifetime(t *testing.T) {
	t.Parallel()
	refusal := errors.New("source refused")
	one, err := core.NewByteLength(1)
	if err != nil {
		t.Fatal(err)
	}
	two, err := core.NewByteLength(2)
	if err != nil {
		t.Fatal(err)
	}
	a := filestore.ContentIndexEntry{Digest: core.SHA256Of([]byte("a")), Extent: one}
	b := filestore.ContentIndexEntry{Digest: core.SHA256Of([]byte("b")), Extent: one}
	cases := []struct {
		name             string
		records          []filestore.ContentIndexEntry
		fail             error
		want             error
		observed, unique uint64
	}{
		{"empty admitted stream", nil, nil, nil, 0, 0},
		{"unique records", []filestore.ContentIndexEntry{a, b}, nil, nil, 2, 2},
		{"repeated record", []filestore.ContentIndexEntry{b, a, b}, nil, nil, 3, 2},
		{"conflicting extent", []filestore.ContentIndexEntry{a, {Digest: a.Digest, Extent: two}}, nil, core.ErrFilestoreContract, 0, 0},
		{"source fails after prefix", []filestore.ContentIndexEntry{a}, refusal, refusal, 0, 0},
		{"invalid source record", []filestore.ContentIndexEntry{{}}, nil, core.ErrFilestoreContract, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			parent, err := core.ParseAbsolutePath(directory)
			if err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			source := func(yield func(filestore.ContentIndexEntry, error) bool) {
				for _, record := range tc.records {
					if !yield(record, nil) {
						return
					}
				}
				if tc.fail != nil {
					yield(filestore.ContentIndexEntry{}, tc.fail)
				}
			}
			got, err := filestore.SortContentStream(t.Context(), filestore.ContentStreamRequest{Parent: parent, Source: source, Destination: &output})
			if !errors.Is(err, tc.want) {
				t.Fatalf("SortContentStream() = %v, want %v", err, tc.want)
			}
			if got.Observed != tc.observed || got.Unique.Entries != tc.unique {
				t.Fatalf("summary = %+v, want observed=%d unique=%d", got, tc.observed, tc.unique)
			}
			if tc.want != nil && output.Len() != 0 {
				t.Fatalf("failed source/sort emitted %d destination bytes", output.Len())
			}
			if tc.want == nil {
				var emitted uint64
				for {
					_, err := filestore.ReadContentIndexEntry(&output)
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					emitted++
				}
				if emitted != tc.unique {
					t.Fatalf("emitted = %d, want %d", emitted, tc.unique)
				}
			}
			entries, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("owned sort left %d scratch entries", len(entries))
			}
		})
	}
}

func TestContentStreamTrustBoundary(t *testing.T) {
	t.Parallel()
	parent, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	empty := iter.Seq2[filestore.ContentIndexEntry, error](func(func(filestore.ContentIndexEntry, error) bool) {})
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	cases := []struct {
		name    string
		ctx     context.Context
		request filestore.ContentStreamRequest
		want    error
	}{
		{"nil context", nil, filestore.ContentStreamRequest{Parent: parent, Source: empty, Destination: io.Discard}, core.ErrNilContext},
		{"canceled admission", canceled, filestore.ContentStreamRequest{Parent: parent, Source: empty, Destination: io.Discard}, context.Canceled},
		{"missing parent", t.Context(), filestore.ContentStreamRequest{Source: empty, Destination: io.Discard}, core.ErrFilestoreContract},
		{"missing source", t.Context(), filestore.ContentStreamRequest{Parent: parent, Destination: io.Discard}, core.ErrFilestoreContract},
		{"missing destination", t.Context(), filestore.ContentStreamRequest{Parent: parent, Source: empty}, core.ErrFilestoreContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := filestore.SortContentStream(tc.ctx, tc.request)
			if !errors.Is(err, tc.want) || got != (filestore.ContentStreamSummary{}) {
				t.Fatalf("got (%+v,%v), want zero and %v", got, err, tc.want)
			}
		})
	}
}
