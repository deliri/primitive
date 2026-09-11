package github

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

func TestTreeGrammarLayerTriad(t *testing.T) {
	t.Parallel()
	canonical := marshalGitHubFixture(t, treeResponseFixture{SHA: parsedCommit(t).String(), URL: "https://api.github.com/tree", Tree: []treeEntryFixture{treeWire("main.go", "blob", parsedCommit(t).String())}})
	cases := []struct {
		name, old, replacement string
		wantEntries            uint64
		wantVisits             int
		wantErr                error
	}{
		{name: "complete canonical entry", wantEntries: 1, wantVisits: 1},
		{name: "null completion flag cannot mean false", old: `"truncated":false`, replacement: `"truncated":null`, wantVisits: 1, wantErr: core.ErrGitHubResponse},
		{name: "numeric completion flag cannot mean false", old: `"truncated":false`, replacement: `"truncated":0`, wantVisits: 1, wantErr: core.ErrGitHubResponse},
		{name: "truncated boolean token remains typed refusal", old: `"truncated":false}`, replacement: `"truncated":fals`, wantVisits: 1, wantErr: core.ErrGitHubResponse},
		{name: "duplicate flag cannot overwrite earlier fact", old: `"truncated":false`, replacement: `"truncated":true,"truncated":false`, wantVisits: 1, wantErr: core.ErrGitHubResponse},
		{name: "escaped duplicate root name is still duplicate", old: `"truncated":false`, replacement: `"truncated":false,"\u0074runcated":false`, wantVisits: 1, wantErr: core.ErrGitHubResponse},
		{name: "duplicate path cannot overwrite streamed bytes", old: `"path":"main.go"`, replacement: `"path":"main.go","path":"other.go"`, wantErr: core.ErrGitHubResponse},
		{name: "conflicting kind cannot overwrite admitted kind", old: `"type":"blob"`, replacement: `"type":"blob","type":"tree"`, wantErr: core.ErrGitHubResponse},
		{name: "unknown field after path prevents metadata", old: `"type":"blob"`, replacement: `"type":"blob","future":true`, wantErr: core.ErrGitHubResponse},
		{name: "null path cannot become an empty nominal path", old: `"path":"main.go"`, replacement: `"path":null`, wantErr: core.ErrGitHubResponse},
		{name: "empty path cannot become metadata", old: `"path":"main.go"`, replacement: `"path":""`, wantErr: core.ErrGitHubResponse},
		{name: "wrong path container cannot be skipped", old: `"path":"main.go"`, replacement: `"path":[]`, wantErr: core.ErrGitHubResponse},
		{name: "unknown future kind is refused", old: `"type":"blob"`, replacement: `"type":"future"`, wantErr: core.ErrGitHubResponse},
		{name: "null kind is refused", old: `"type":"blob"`, replacement: `"type":null`, wantErr: core.ErrGitHubResponse},
		{name: "maximum uint64 provider size is admitted", old: `"type":"blob"`, replacement: `"type":"blob","size":18446744073709551615`, wantEntries: 1, wantVisits: 1},
		{name: "provider size overflow is a numeric refusal", old: `"type":"blob"`, replacement: `"type":"blob","size":18446744073709551616`, wantErr: core.ErrGitHubResponse},
		{name: "provider negative size is refused", old: `"type":"blob"`, replacement: `"type":"blob","size":-1`, wantErr: core.ErrGitHubResponse},
		{name: "provider fractional size is refused", old: `"type":"blob"`, replacement: `"type":"blob","size":1.5`, wantErr: core.ErrGitHubResponse},
		{name: "provider size leading zero is refused", old: `"type":"blob"`, replacement: `"type":"blob","size":01`, wantErr: core.ErrGitHubResponse},
		{name: "optional null metadata has no path meaning", old: `"mode":"100644"`, replacement: `"mode":null`, wantEntries: 1, wantVisits: 1},
		{name: "optional metadata wrong container is refused", old: `"mode":"100644"`, replacement: `"mode":{}`, wantErr: core.ErrGitHubResponse},
		{name: "trailing document cannot be silently ignored", old: `"truncated":false}`, replacement: `"truncated":false} {}`, wantVisits: 1, wantErr: core.ErrGitHubResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := canonical
			if tc.old != "" {
				input = bytes.Replace(canonical, []byte(tc.old), []byte(tc.replacement), 1)
				if bytes.Equal(input, canonical) {
					t.Fatalf("mutation %q changed=false, want true", tc.old)
				}
			}
			visitor := &collectingTreeVisitor{}
			got, err := decodeTree(bytes.NewReader(input), visitor)
			if !errors.Is(err, tc.wantErr) || got != tc.wantEntries || len(visitor.entries) != tc.wantVisits {
				t.Fatalf("tree=%d/%v visits=%d, want %d/%v/%d", got, err, len(visitor.entries), tc.wantEntries, tc.wantErr, tc.wantVisits)
			}
		})
	}
}
