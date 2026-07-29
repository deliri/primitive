package filestore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestParentDirectorySynchronizationLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive nested parent is synchronized through the real root handle", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		if err := os.Mkdir(filepath.Join(rootDirectory, "objects"), 0o700); err != nil {
			t.Fatal(err)
		}
		root, err := os.OpenRoot(rootDirectory)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if closeErr := root.Close(); closeErr != nil {
				t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
			}
		})
		path, err := core.ParseRelativePath(filepath.Join("objects", "target"))
		if err != nil {
			t.Fatal(err)
		}
		if gotErr := syncParent(root, path); gotErr != nil {
			t.Fatalf("syncParent(nested target) error = %v, want nil", gotErr)
		}
	})
	t.Run("negative closed root preserves the native closed-handle identity", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root, err := os.OpenRoot(rootDirectory)
		if err != nil {
			t.Fatal(err)
		}
		if err := root.Close(); err != nil {
			t.Fatal(err)
		}
		path, err := core.ParseRelativePath("target")
		if err != nil {
			t.Fatal(err)
		}
		gotErr := syncParent(root, path)
		if !errors.Is(gotErr, os.ErrClosed) {
			t.Fatalf("syncParent(closed root) error = %v, want %v", gotErr, os.ErrClosed)
		}
	})
	t.Run("neutral root-parent synchronization creates no namespace entries", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root, err := os.OpenRoot(rootDirectory)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if closeErr := root.Close(); closeErr != nil {
				t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
			}
		})
		path, err := core.ParseRelativePath("missing")
		if err != nil {
			t.Fatal(err)
		}
		if gotErr := syncParent(root, path); gotErr != nil {
			t.Fatalf("syncParent(root target) error = %v, want nil", gotErr)
		}
		entries, err := os.ReadDir(rootDirectory)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("entries after neutral parent sync = %v, want none", entries)
		}
	})
}
