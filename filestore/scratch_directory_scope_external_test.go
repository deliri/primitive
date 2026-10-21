package filestore_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestScratchDirectoryNativeProcessPathAndCleanup(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		fail, cancel bool
	}{{"success", false, false}, {"consumer failure", true, false}, {"cancellation", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parentText := t.TempDir()
			parent, err := core.ParseAbsolutePath(parentText)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			calls := 0
			err = filestore.WithScratchDirectoryScope(ctx, filestore.ScratchDirectoryScopeRequest{Parent: parent, Use: func(ctx context.Context, root *os.Root, directory core.AbsolutePath) error {
				calls++
				if err := filestore.ValidateRootIdentity(root, directory); err != nil {
					return err
				}
				cmd := exec.CommandContext(ctx, "/bin/pwd")
				cmd.Dir = directory.String()
				out, err := cmd.Output()
				if err != nil {
					return err
				}
				observed, err := os.Stat(strings.TrimSpace(string(out)))
				if err != nil {
					return err
				}
				held, err := root.Stat(".")
				if err != nil {
					return err
				}
				if !os.SameFile(observed, held) {
					t.Fatal("child process entered a different directory")
				}
				if err := root.WriteFile("native-child", []byte{0, 255}, 0600); err != nil {
					return err
				}
				if tc.cancel {
					cancel()
					return ctx.Err()
				}
				if tc.fail {
					return core.ErrFilestoreConflict
				}
				return nil
			}})
			if calls != 1 {
				t.Fatalf("calls=%d, want one native use", calls)
			}
			if tc.cancel {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancel=%v, want native cancellation", err)
				}
			} else if tc.fail {
				if !errors.Is(err, core.ErrFilestoreConflict) {
					t.Fatalf("failure=%v, want preserved consumer error", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			entries, e := os.ReadDir(parentText)
			if e != nil || len(entries) != 0 {
				t.Fatalf("residue=%v/%v, want none", entries, e)
			}
		})
	}
}

func TestScratchDirectoryRefusesBeforeUse(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                 string
		invalidParent, missingUse, cancelled bool
	}{{"invalid parent", true, false, false}, {"missing consumer", false, true, false}, {"already cancelled", false, false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parentText := t.TempDir()
			parent, err := core.ParseAbsolutePath(parentText)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			request := filestore.ScratchDirectoryScopeRequest{Parent: parent, Use: func(context.Context, *os.Root, core.AbsolutePath) error { calls++; return nil }}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.invalidParent {
				request.Parent = core.AbsolutePath{}
			}
			if tc.missingUse {
				request.Use = nil
			}
			if tc.cancelled {
				cancel()
			}
			err = filestore.WithScratchDirectoryScope(ctx, request)
			want := core.ErrFilestoreContract
			if tc.cancelled {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("error=%v, want native cancellation", err)
				}
			} else if !errors.Is(err, want) {
				t.Fatalf("error=%v, want contract refusal", err)
			}
			if calls != 0 {
				t.Fatalf("calls=%d, want none", calls)
			}
			entries, e := os.ReadDir(parentText)
			if e != nil || len(entries) != 0 {
				t.Fatalf("residue=%v/%v, want none", entries, e)
			}
		})
	}
}

func TestScratchDirectoryPanicCleansOwnedTree(t *testing.T) {
	t.Parallel()
	parentText := t.TempDir()
	parent, err := core.ParseAbsolutePath(parentText)
	if err != nil {
		t.Fatal(err)
	}
	var caught bool
	func() {
		defer func() { caught = recover() == core.ErrFilestoreConflict }()
		_ = filestore.WithScratchDirectoryScope(t.Context(), filestore.ScratchDirectoryScopeRequest{Parent: parent, Use: func(context.Context, *os.Root, core.AbsolutePath) error { panic(core.ErrFilestoreConflict) }})
	}()
	if !caught {
		t.Fatal("consumer panic was not preserved")
	}
	entries, err := filepath.Glob(filepath.Join(parentText, "*"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("residue=%v/%v, want none", entries, err)
	}
}
