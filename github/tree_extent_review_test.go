package github

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type countingTreeVisitor struct {
	count uint64
	bad   bool
}

func (v *countingTreeVisitor) VisitGitHubTreeEntry(stream *TreeEntryStream) error {
	var path [8]byte
	n, err := io.ReadFull(stream, path[:])
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return err
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) || n != len("main.go") || string(path[:n]) != "main.go" {
		v.bad = true
		return core.ErrGitHubResponse
	}
	entry, err := stream.Observation()
	if err != nil || entry.Validate() != nil || entry.Kind != TreeEntryBlob || entry.PathLength.Uint64() != 7 || entry.PathSHA256 != core.SHA256Of([]byte("main.go")) {
		v.bad = true
		return core.ErrGitHubResponse
	}
	v.count++
	return nil
}

type treeExtentWire struct {
	count         int
	truncated     bool
	malformedTail bool
}

func writeTreeExtent(w io.Writer, input treeExtentWire) error {
	encoder := jsontext.NewEncoder(w)
	for _, token := range []jsontext.Token{jsontext.BeginObject, jsontext.String("sha"), jsontext.String("commit"), jsontext.String("url"), jsontext.String("https://api.github.com/tree"), jsontext.String("tree"), jsontext.BeginArray} {
		if err := encoder.WriteToken(token); err != nil {
			return err
		}
	}
	entry := treeEntryFixture{Path: "main.go", Mode: "100644", Type: "blob", SHA: "commit", URL: "https://api.github.com/blob"}
	for range input.count {
		if err := json.MarshalEncode(encoder, entry); err != nil {
			return err
		}
	}
	if input.malformedTail {
		entry.Type = "unknown"
		if err := json.MarshalEncode(encoder, entry); err != nil {
			return err
		}
	}
	for _, token := range []jsontext.Token{jsontext.EndArray, jsontext.String("truncated"), jsontext.Bool(input.truncated), jsontext.EndObject} {
		if err := encoder.WriteToken(token); err != nil {
			return err
		}
	}
	return nil
}
func TestTreeStreamsBeyondFormerCountAndByteCeilingsLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		wire    treeExtentWire
		wantErr error
	}{
		{name: "below former count", wire: treeExtentWire{count: 99_999}},
		{name: "at former count", wire: treeExtentWire{count: 100_000}},
		{name: "above former count", wire: treeExtentWire{count: 100_001}},
		{name: "truncation after large prefix never claims completion", wire: treeExtentWire{count: 100_001, truncated: true}, wantErr: core.ErrGitHubResponse},
		{name: "invalid kind after large prefix remains invalid", wire: treeExtentWire{count: 100_001, malformedTail: true}, wantErr: core.ErrGitHubResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				// A refused tail closes the consumer pipe and may cancel the provider write.
				retainProviderWriteResult(t, writeTreeExtent(w, tc.wire))
			}))
			defer server.Close()
			visitor := &countingTreeVisitor{}
			got, err := clientFixture(t, server.URL).ReadTree(t.Context(), TreeRequest{Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t), Visitor: visitor})
			if !errors.Is(err, tc.wantErr) || visitor.count != uint64(tc.wire.count) || visitor.bad {
				t.Fatalf("tree=%+v/%v visits=%d bad=%t; want %d/%v", got, err, visitor.count, visitor.bad, tc.wire.count, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (TreeObservation{}) {
					t.Fatalf("failed tail published completion: %+v", got)
				}
				return
			}
			if got.Validate() != nil || got.Entries != visitor.count || got.Bytes.Uint64() <= 7_000_000 {
				t.Fatalf("completion failed exact large observation: %+v", got)
			}
		})
	}
}
