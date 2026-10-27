package gitrepo

import (
	"errors"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type referenceSink []ReferenceEntry

func (s *referenceSink) ConsumeReference(e ReferenceEntry) error { *s = append(*s, e); return nil }

type worktreeListSink []WorktreeListEntry

func (s *worktreeListSink) ConsumeWorktreeListEntry(e WorktreeListEntry) error {
	*s = append(*s, e)
	return nil
}

// A real repository with two branches, a linked branch worktree and a detached
// worktree: the streams match what native git itself reports, entry for entry.
func TestInventoryStreamsMatchNativeGit(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	main := filepath.Join(base, "main")
	runGitSetup(t, base, "init", "--quiet", "main")
	runGitSetup(t, main, "-c", "user.email=a@b", "-c", "user.name=a", "commit", "--quiet", "--allow-empty", "-m", "one")
	runGitSetup(t, main, "branch", "feature")
	runGitSetup(t, main, "worktree", "add", "--quiet", filepath.Join(base, "linked"), "feature")
	runGitSetup(t, main, "worktree", "add", "--quiet", "--detach", filepath.Join(base, "detached"))
	runGitSetup(t, main, "worktree", "lock", filepath.Join(base, "detached"))
	c, status := statusFixtureCapability(t, main)
	request := InventoryRequest{Root: status.Root}

	var refs referenceSink
	count, err := c.StreamReferences(t.Context(), request, &refs)
	if err != nil || count != uint64(len(refs)) {
		t.Fatalf("StreamReferences = (%d, %v) entries %d", count, err, len(refs))
	}
	native, err := exec.CommandContext(t.Context(), "git", "-C", main, "for-each-ref", "--format=%(objectname) %(refname)", "refs/heads", "refs/remotes").Output()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range refs {
		got = append(got, r.Commit.String()+" "+r.Name.String())
	}
	if want := strings.Split(strings.TrimSpace(string(native)), "\n"); strings.Join(got, "\n") != strings.Join(want, "\n") || len(got) != 2 {
		t.Fatalf("refs = %q, want native %q (two branches)", got, want)
	}

	var trees worktreeListSink
	count, err = c.StreamWorktreeList(t.Context(), request, &trees)
	if err != nil || count != 3 || len(trees) != 3 {
		t.Fatalf("StreamWorktreeList = (%d, %v) %+v, want three worktrees", count, err, trees)
	}
	byPath := map[string]WorktreeListEntry{}
	var paths []string
	for _, w := range trees {
		resolved, _ := filepath.EvalSymlinks(w.Path.String())
		byPath[resolved] = w
		paths = append(paths, resolved)
	}
	sort.Strings(paths)
	resolve := func(p string) string { r, _ := filepath.EvalSymlinks(p); return r }
	linked, detached, primary := byPath[resolve(filepath.Join(base, "linked"))], byPath[resolve(filepath.Join(base, "detached"))], byPath[resolve(main)]
	if linked.Branch == nil || linked.Branch.String() != "refs/heads/feature" || linked.Head == nil || linked.Detached {
		t.Fatalf("linked worktree = %+v, want branch refs/heads/feature with HEAD", linked)
	}
	if !detached.Detached || detached.Branch != nil || detached.Head == nil || !detached.Locked {
		t.Fatalf("detached worktree = %+v, want detached, locked, with HEAD", detached)
	}
	if primary.Branch == nil || primary.Head == nil || primary.Bare {
		t.Fatalf("main worktree = %+v (paths %q)", primary, paths)
	}
}

// Hostile native records are refused, never passed through or truncated.
func TestInventoryRecordWriterRefusesMalformedRecords(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		data string
		sep  byte
	}{
		{"reference without separator", "abc\n", '\n'},
		{"truncated record", "partial", '\n'},
		{"oversized record", strings.Repeat("x", inventoryRecordMaximum+1), '\n'},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			w := recordWriter{separator: row.sep, record: func(line []byte) error {
				if !strings.Contains(string(line), " ") {
					return core.ErrGitRepositoryOutput
				}
				return nil
			}}
			_, writeErr := w.Write([]byte(row.data))
			if err := errors.Join(writeErr, w.finish()); !errors.Is(err, core.ErrGitRepositoryOutput) {
				t.Fatalf("malformed %q error = %v, want %v", row.name, err, core.ErrGitRepositoryOutput)
			}
		})
	}
	bare := WorktreeListEntry{Bare: true, Detached: true}
	if path, err := core.ParseAbsolutePath("/tmp/x"); err == nil {
		bare.Path = path
	}
	if err := bare.Validate(); !errors.Is(err, core.ErrGitRepositoryOutput) {
		t.Fatalf("bare detached worktree = %v, want refused", err)
	}
}
