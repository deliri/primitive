package github

import (
	stdjson "encoding/json"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Each seed is emitted from a typed provider document. All three public doors
// run through the real HTTP client; the oracle separately decodes source facts.
func FuzzGitHubPublicJSONSemanticClosure(f *testing.F) {
	commit := parsedCommit(f)
	f.Add(uint8(0), marshalGitHubFixture(f, []headWire{{SHA: commit.String()}}))
	f.Add(uint8(1), marshalGitHubFixture(f, []tagWire{{Name: "v1.2.3", Commit: tagCommitWire{SHA: commit.String()}}}))
	f.Add(uint8(2), marshalGitHubFixture(f, treeResponseFixture{SHA: commit.String(), URL: "https://api.github.com/tree", Tree: []treeEntryFixture{treeWire("main.go", "blob", commit.String())}}))
	for kind := range uint8(3) {
		f.Add(kind, []byte{})
		f.Add(kind, []byte("null"))
	}
	f.Fuzz(func(t *testing.T, selector uint8, payload []byte) {
		if len(payload) > 4*(64<<10) {
			t.Skip("input exceeds bounded independent oracle custody")
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
			_, err := w.Write(payload)
			retainProviderWriteResult(t, err)
		}))
		defer server.Close()
		client := clientFixture(t, server.URL)
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("Client.Close()=%v, want nil", err)
			}
		}()
		repository := parsedRepository(t, "owner/repository")
		switch selector % 3 {
		case 0:
			got, err := client.ReadHead(t.Context(), HeadRequest{Repository: repository})
			if err != nil {
				if !errors.Is(err, core.ErrGitHubResponse) || got != (HeadObservation{}) {
					t.Fatalf("head refusal=%+v/%v, want zero typed response refusal", got, err)
				}
				return
			}
			var source []headWire
			if decodeErr := stdjson.Unmarshal(payload, &source); decodeErr != nil || len(source) != 1 {
				t.Fatalf("accepted head source=%+v/%v, want one commit", source, decodeErr)
			}
			if got.Validate() != nil || got.Repository != repository || got.Commit.String() != source[0].SHA {
				t.Fatalf("head=%+v, want original repository and exact source %+v", got, source)
			}
		case 1:
			got, err := client.ReadTagPage(t.Context(), TagPageRequest{Repository: repository, Page: 1})
			if err != nil {
				if !errors.Is(err, core.ErrGitHubResponse) || got.Repository != (Repository{}) || got.Page != 0 || got.NextPage != 0 || len(got.Tags) != 0 {
					t.Fatalf("tag refusal=%+v/%v, want zero typed response refusal", got, err)
				}
				return
			}
			var source []tagWire
			if decodeErr := stdjson.Unmarshal(payload, &source); decodeErr != nil {
				t.Fatalf("accepted tags source=%v, want nil", decodeErr)
			}
			if got.Validate() != nil || got.Repository != repository || got.Page != 1 || got.NextPage != 0 || len(got.Tags) != len(source) {
				t.Fatalf("tags=%+v, want exact source count %d and bound page", got, len(source))
			}
			for i, tag := range got.Tags {
				if tag.Name.String() != source[i].Name || tag.Commit.String() != source[i].Commit.SHA {
					t.Fatalf("tag[%d]=%+v, want source %+v", i, tag, source[i])
				}
			}
		case 2:
			visitor := &collectingTreeVisitor{}
			got, err := client.ReadTree(t.Context(), TreeRequest{Repository: repository, Commit: commit, Visitor: visitor})
			if err != nil {
				if !errors.Is(err, core.ErrGitHubResponse) || got != (TreeObservation{}) {
					t.Fatalf("tree refusal=%+v/%v, want zero typed response refusal", got, err)
				}
				for _, entry := range visitor.entries {
					if entry.Validate() != nil {
						t.Fatalf("delivered prefix=%+v, want validated entry", entry)
					}
				}
				return
			}
			var source struct {
				SHA       *string
				URL       *string
				Tree      *[]treeEntryFixture
				Truncated *bool
			}
			if decodeErr := stdjson.Unmarshal(payload, &source); decodeErr != nil || source.SHA == nil || *source.SHA == "" || source.URL == nil || *source.URL == "" || source.Tree == nil || source.Truncated == nil || *source.Truncated {
				t.Fatalf("tree source=%+v/%v, want complete untruncated document", source, decodeErr)
			}
			if got.Validate() != nil || got.Repository != repository || got.Commit != commit || got.Bytes.Uint64() != uint64(len(payload)) || got.Entries != uint64(len(*source.Tree)) || len(visitor.entries) != len(*source.Tree) {
				t.Fatalf("tree=%+v visits=%d, want exact source bytes=%d entries=%d", got, len(visitor.entries), len(payload), len(*source.Tree))
			}
			spellings := [...]string{"", "blob", "tree", "commit"}
			for i, entry := range visitor.entries {
				wire := (*source.Tree)[i]
				if int(entry.Kind) >= len(spellings) || visitor.paths[i] != wire.Path || entry.PathLength.Uint64() != uint64(len(wire.Path)) || entry.PathSHA256 != core.SHA256Of([]byte(wire.Path)) || spellings[entry.Kind] != wire.Type {
					t.Fatalf("entry[%d]=%+v, want source %+v", i, entry, wire)
				}
			}
		}
	})
}
