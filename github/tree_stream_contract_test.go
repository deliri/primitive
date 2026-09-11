package github

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"io"
	"testing"
	"unicode/utf8"
)

func TestTreeEntryStreamOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	payload := marshalGitHubFixture(t, treeResponseFixture{SHA: parsedCommit(t).String(), URL: "https://api.github.com/tree", Tree: []treeEntryFixture{treeWire("main.go", "blob", parsedCommit(t).String())}})
	cases := []struct {
		name    string
		consume bool
		wantErr error
	}{
		{name: "complete path permits metadata", consume: true},
		{name: "unread path cannot claim completion", wantErr: core.ErrGitHubResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var borrowed *TreeEntryStream
			visitor := nilFuncVisitor(func(stream *TreeEntryStream) error {
				borrowed = stream
				got, err := stream.Observation()
				if !errors.Is(err, core.ErrGitHubResponse) || got != (TreeEntry{}) {
					t.Fatalf("early metadata=%+v/%v, want zero typed refusal", got, err)
				}
				if tc.consume {
					var path bytes.Buffer
					if _, err := io.Copy(&path, stream); err != nil {
						return err
					}
					if path.String() != "main.go" {
						t.Fatalf("path=%q, want main.go", path.String())
					}
					got, err := stream.Observation()
					if err != nil || got.PathLength.Uint64() != 7 || got.PathSHA256 != core.SHA256Of(path.Bytes()) || got.Kind != TreeEntryBlob {
						t.Fatalf("EOF metadata=%+v/%v, want exact path proof", got, err)
					}
				}
				return nil
			})
			got, err := decodeTree(bytes.NewReader(payload), visitor)
			if !errors.Is(err, tc.wantErr) || got != boolTreeCount(tc.consume) {
				t.Fatalf("tree=%d/%v, want %d/%v", got, err, boolTreeCount(tc.consume), tc.wantErr)
			}
			var scratch [1]byte
			n, readErr := borrowed.Read(scratch[:])
			if tc.consume {
				if n != 0 || !errors.Is(readErr, io.EOF) {
					t.Fatalf("completed reader=%d/%v, want zero/EOF", n, readErr)
				}
			} else if n != 0 || !errors.Is(readErr, core.ErrGitHubContract) {
				t.Fatalf("expired reader=%d/%v, want zero typed lifetime refusal", n, readErr)
			}
		})
	}
	var zero TreeEntryStream
	got, err := zero.Observation()
	if !errors.Is(err, core.ErrGitHubResponse) || got != (TreeEntry{}) {
		t.Fatalf("zero metadata=%+v/%v, want zero typed refusal", got, err)
	}
}
func boolTreeCount(complete bool) uint64 {
	if complete {
		return 1
	}
	return 0
}

func FuzzGitHubStreamedPathMatchesCore(f *testing.F) {
	path, err := core.ParseSourcePath("directory/main.go")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(path.String())
	for _, hostile := range []string{"", "..", "a/../b", "/absolute", "a/", "a//b", "a\\b", " a", "a ", "a\n", "a/./b"} {
		f.Add(hostile)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 256<<10 {
			t.Skip("input exceeds bounded source-path oracle custody")
		}
		want, wantErr := core.ParseSourcePath(raw)
		if !utf8.ValidString(raw) {
			if wantErr == nil {
				t.Fatal("invalid UTF8 source path accepted=true, want false")
			}
			return // Raw malformed UTF8 wire is exercised by the JSON string fuzz target.
		}
		payload, err := json.Marshal(treeResponseFixture{SHA: parsedCommit(t).String(), URL: "https://api.github.com/tree", Tree: []treeEntryFixture{treeWire(raw, "blob", parsedCommit(t).String())}})
		if err != nil {
			t.Fatal(err)
		}
		visitor := &collectingTreeVisitor{}
		got, err := decodeTree(bytes.NewReader(payload), visitor)
		if wantErr != nil {
			if !errors.Is(err, core.ErrGitHubResponse) || got != 0 || len(visitor.entries) != 0 {
				t.Fatalf("path refusal=%d/%v entries=%d, want typed refusal and zero complete metadata", got, err, len(visitor.entries))
			}
			return
		}
		if err != nil || got != 1 || len(visitor.entries) != 1 || visitor.paths[0] != want.String() || visitor.entries[0].PathSHA256 != core.SHA256Of([]byte(raw)) {
			t.Fatalf("path=%d/%v/%q, want exact core-admitted %q", got, err, visitor.paths, want.String())
		}
	})
}
