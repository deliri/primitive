package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// This is a native descriptor-to-child handoff proof. The public acquisition
// already happened; changing its old path must not redirect later effects.
func TestScratchParentDescriptorSurvivesNamespaceReplacementLayerTriad(t *testing.T) {
	t.Parallel()
	for _, replacement := range []bool{false, true} {
		t.Run(scratchReplacementName(replacement), func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			owned := filepath.Join(directory, "owned")
			foreign := filepath.Join(directory, "foreign")
			link := filepath.Join(directory, "parent")
			for _, path := range []string{owned, foreign} {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(owned, link); err != nil {
				t.Fatal(err)
			}
			parent, err := os.OpenRoot(link)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := parent.Close(); err != nil {
					t.Error(err)
				}
			}()
			if replacement {
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(foreign, link); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			err = withScratchParent(t.Context(), parent, func(_ context.Context, root *os.Root) error {
				calls++
				if err := root.WriteFile("private", []byte{0, 255}, 0o600); err != nil {
					return err
				}
				ownedEntries, err := os.ReadDir(owned)
				if err != nil || len(ownedEntries) != 1 {
					t.Fatalf("owned scope = (%d,%v), want exactly one real directory", len(ownedEntries), err)
				}
				foreignEntries, err := os.ReadDir(foreign)
				if err != nil || len(foreignEntries) != 0 {
					t.Fatalf("foreign effects = (%d,%v), want zero", len(foreignEntries), err)
				}
				return nil
			})
			if err != nil || calls != 1 {
				t.Fatalf("scope = (%d,%v), want one use and completed cleanup", calls, err)
			}
			for _, path := range []string{owned, foreign} {
				entries, err := os.ReadDir(path)
				if err != nil || len(entries) != 0 {
					t.Fatalf("residue %s = (%d,%v), want none", path, len(entries), err)
				}
			}
		})
	}
}

func scratchReplacementName(replaced bool) string {
	if replaced {
		return "old symlink now names foreign parent"
	}
	return "unchanged parent owns its child"
}

func TestScratchScopeCleanupPreservesForeignReplacement(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	parent, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := parent.Close(); err != nil {
			t.Error(err)
		}
	}()
	var name string
	err = withScratchParent(t.Context(), parent, func(_ context.Context, root *os.Root) error {
		entries, err := fs.ReadDir(parent.FS(), ".")
		if err != nil || len(entries) != 1 {
			t.Fatalf("created scope = (%d,%v), want one child", len(entries), err)
		}
		name = entries[0].Name()
		if err := parent.Rename(name, "retained-owner"); err != nil {
			return err
		}
		if err := parent.Mkdir(name, 0o700); err != nil {
			return err
		}
		return parent.WriteFile(filepath.Join(name, "foreign"), []byte{255, 0}, 0o600)
	})
	if !errors.Is(err, core.ErrFilestoreCleanup) || !errors.Is(err, core.ErrFilestoreConflict) {
		t.Fatalf("cleanup = %v, want foreign ownership refusal", err)
	}
	got, readErr := parent.ReadFile(filepath.Join(name, "foreign"))
	if readErr != nil || len(got) != 2 || got[0] != 255 || got[1] != 0 {
		t.Fatalf("foreign entry = (%x,%v), want preserved bytes", got, readErr)
	}
	if _, err := parent.Stat("retained-owner"); errors.Is(err, fs.ErrNotExist) || err != nil {
		t.Fatalf("relocated owned directory = %v, want preserved caller relocation", err)
	}
}
