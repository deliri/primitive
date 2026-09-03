package github

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type collectingTreeVisitor struct{ entries []TreeEntry }

func (v *collectingTreeVisitor) VisitGitHubTreeEntry(entry TreeEntry) error {
	v.entries = append(v.entries, entry)
	return nil
}

func TestGitHubRecursiveTreeTransportLayerTriad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr     error
		name        string
		wireEntries []treeEntryWire
		maximum     uint64
		wantEntries uint64
		truncated   bool
	}{
		{
			name: "positive blob directory and submodule stream as closed kinds",
			wireEntries: []treeEntryWire{
				{Path: "main.go", Mode: "100644", Type: "blob", SHA: parsedCommit(t).String(), URL: "https://api.github.com/blob"},
				{Path: "internal", Mode: "040000", Type: "tree", SHA: parsedCommit(t).String(), URL: "https://api.github.com/tree"},
				{Path: "vendor/module", Mode: "160000", Type: "commit", SHA: parsedCommit(t).String(), URL: "https://api.github.com/commit"},
			},
			maximum: 3, wantEntries: 3,
		},
		{
			name:        "negative provider truncation cannot produce completed observation",
			wireEntries: []treeEntryWire{{Path: "main.go", Mode: "100644", Type: "blob", SHA: parsedCommit(t).String(), URL: "https://api.github.com/blob"}},
			truncated:   true, maximum: 1, wantErr: core.ErrGitHubResponse,
		},
		{
			name:        "neutral empty tree remains an exact zero-entry observation",
			wireEntries: []treeEntryWire{}, maximum: 1, wantEntries: 0,
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
					SHA       string          `json:"sha"`
					URL       string          `json:"url"`
					Tree      []treeEntryWire `json:"tree"`
					Truncated bool            `json:"truncated"`
				}{SHA: parsedCommit(t).String(), URL: "https://api.github.com/tree", Tree: testCase.wireEntries, Truncated: testCase.truncated}, http.StatusOK)
			}))
			defer server.Close()

			visitor := &collectingTreeVisitor{}
			client := clientFixture(t, server.URL)
			got, gotErr := client.ReadTree(context.Background(), TreeRequest{
				Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t),
				MaximumEntries: testCase.maximum, Visitor: visitor,
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
		SHA       string          `json:"sha"`
		URL       string          `json:"url"`
		Tree      []treeEntryWire `json:"tree"`
		Truncated bool            `json:"truncated"`
	}{
		SHA: parsedCommit(f).String(), URL: "https://api.github.com/tree",
		Tree: []treeEntryWire{{Path: "main.go", Mode: "100644", Type: "blob", SHA: parsedCommit(f).String(), URL: "https://api.github.com/blob"}},
	})
	if err != nil {
		f.Fatalf("json.Marshal(tree seed) error = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`{"sha":"x","url":"x","tree":[],"truncated":true}`))
	f.Fuzz(func(t *testing.T, payload []byte) {
		visitor := &collectingTreeVisitor{}
		got, gotErr := decodeTree(bytes.NewReader(payload), core.GitHubRecursiveTreeMaximumEntries, visitor)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrGitHubResponse) && !errors.Is(gotErr, core.ErrJSONContract) {
				t.Fatalf("decodeTree(rejected) error = %v, want typed GitHub or JSON rejection", gotErr)
			}
			return
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
