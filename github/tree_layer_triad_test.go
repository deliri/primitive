package github

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	json "encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type collectingTreeVisitor struct {
	entries []TreeEntry
	paths   []string
}

func (v *collectingTreeVisitor) VisitGitHubTreeEntry(stream *TreeEntryStream) error {
	var path bytes.Buffer // caller-owned fixture aggregation, bounded by each fuzz oracle
	if _, err := io.Copy(&path, stream); err != nil {
		return err
	}
	entry, err := stream.Observation()
	if err != nil {
		return err
	}
	v.entries = append(v.entries, entry)
	v.paths = append(v.paths, path.String())
	return nil
}

func TestGitHubRecursiveTreeTransportLayerTriad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr     error
		name        string
		wireEntries []treeEntryFixture
		wantEntries uint64
		truncated   bool
	}{
		{
			name: "positive blob directory and submodule stream as closed kinds",
			wireEntries: []treeEntryFixture{
				{Path: "main.go", Mode: "100644", Type: "blob", SHA: parsedCommit(t).String(), URL: "https://api.github.com/blob"},
				{Path: "internal", Mode: "040000", Type: "tree", SHA: parsedCommit(t).String(), URL: "https://api.github.com/tree"},
				{Path: "vendor/module", Mode: "160000", Type: "commit", SHA: parsedCommit(t).String(), URL: "https://api.github.com/commit"},
			},
			wantEntries: 3,
		},
		{
			name:        "negative provider truncation cannot produce completed observation",
			wireEntries: []treeEntryFixture{{Path: "main.go", Mode: "100644", Type: "blob", SHA: parsedCommit(t).String(), URL: "https://api.github.com/blob"}},
			truncated:   true, wantErr: core.ErrGitHubResponse,
		},
		{
			name:        "neutral empty tree remains an exact zero-entry observation",
			wireEntries: []treeEntryFixture{}, wantEntries: 0,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, incoming *http.Request) {
				if incoming.URL.Path != "/repos/owner/repository/git/trees/"+parsedCommit(t).String() || incoming.URL.Query().Get("recursive") != "1" {
					t.Errorf("GitHub tree request = %s?%s, want exact recursive commit route", incoming.URL.Path, incoming.URL.RawQuery)
				}
				writeJSON(t, writer, struct {
					SHA       string             `json:"sha"`
					URL       string             `json:"url"`
					Tree      []treeEntryFixture `json:"tree"`
					Truncated bool               `json:"truncated"`
				}{SHA: parsedCommit(t).String(), URL: "https://api.github.com/tree", Tree: testCase.wireEntries, Truncated: testCase.truncated}, http.StatusOK)
			}))
			defer server.Close()

			visitor := &collectingTreeVisitor{}
			client := clientFixture(t, server.URL)
			got, gotErr := client.ReadTree(context.Background(), TreeRequest{
				Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t),
				Visitor: visitor,
			})
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("Client.ReadTree() error = %v, want %v", gotErr, testCase.wantErr)
			}
			if gotErr == nil && (got.Entries != testCase.wantEntries || uint64(len(visitor.entries)) != testCase.wantEntries) {
				t.Fatalf("Client.ReadTree() entries = (%d observed, %d visited), want %d", got.Entries, len(visitor.entries), testCase.wantEntries)
			}
			for _, entry := range visitor.entries {
				if err := entry.Validate(); err != nil {
					t.Fatalf("visited TreeEntry.Validate() error = %v, want nil", err)
				}
			}
		})
	}
}

func FuzzDecodeGitHubTreeSemanticClosure(f *testing.F) {
	seed, err := json.Marshal(struct {
		SHA       string             `json:"sha"`
		URL       string             `json:"url"`
		Tree      []treeEntryFixture `json:"tree"`
		Truncated bool               `json:"truncated"`
	}{
		SHA: parsedCommit(f).String(), URL: "https://api.github.com/tree",
		Tree: []treeEntryFixture{{Path: "main.go", Mode: "100644", Type: "blob", SHA: parsedCommit(f).String(), URL: "https://api.github.com/blob"}},
	})
	if err != nil {
		f.Fatalf("json.Marshal(tree seed) error = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`{"sha":"x","url":"x","tree":[],"truncated":true}`))
	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > 4*(64<<10) {
			t.Skip("input exceeds bounded aggregate oracle custody")
		}
		visitor := &collectingTreeVisitor{}
		got, gotErr := decodeTree(bytes.NewReader(payload), visitor)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrGitHubResponse) || got != 0 {
				t.Fatalf("decodeTree(rejected) error = %v, want typed GitHub or JSON rejection", gotErr)
			}
			return
		}
		var source struct {
			SHA       *string
			URL       *string
			Tree      *[]treeEntryFixture
			Truncated *bool
		}
		if err := stdjson.Unmarshal(payload, &source); err != nil || source.SHA == nil || *source.SHA == "" || source.URL == nil || *source.URL == "" || source.Tree == nil || source.Truncated == nil || *source.Truncated {
			t.Fatalf("accepted source=%+v/%v, want complete untruncated typed document", source, err)
		}
		if len(*source.Tree) != len(visitor.entries) {
			t.Fatalf("source entries=%d, want delivered count %d", len(*source.Tree), len(visitor.entries))
		}
		spellings := [...]string{"", "blob", "tree", "commit"}
		for i, entry := range visitor.entries {
			wire := (*source.Tree)[i]
			if int(entry.Kind) >= len(spellings) || visitor.paths[i] != wire.Path || entry.PathLength.Uint64() != uint64(len(wire.Path)) || entry.PathSHA256 != core.SHA256Of([]byte(wire.Path)) || spellings[entry.Kind] != wire.Type {
				t.Fatalf("entry[%d]=%+v, want exact source path/kind %+v", i, entry, wire)
			}
		}
		canonical, err := json.Marshal(treeResponseFixture{SHA: *source.SHA, URL: *source.URL, Tree: *source.Tree, Truncated: *source.Truncated})
		if err != nil {
			t.Fatalf("canonical source=%v, want nil", err)
		}
		roundVisitor := &collectingTreeVisitor{}
		roundCount, err := decodeTree(bytes.NewReader(canonical), roundVisitor)
		if err != nil || roundCount != got || !slices.Equal(roundVisitor.entries, visitor.entries) {
			t.Fatalf("canonical projection=%d/%v/%+v, want %d/nil/%+v", roundCount, err, roundVisitor.entries, got, visitor.entries)
		}
		if got != uint64(len(visitor.entries)) {
			t.Fatalf("decodeTree(accepted) = %d entries and %d visits, want exact conservation", got, len(visitor.entries))
		}
		for _, entry := range visitor.entries {
			if err := entry.Validate(); err != nil {
				t.Fatalf("decodeTree(accepted) entry.Validate() error = %v, want nil", err)
			}
		}
	})
}
