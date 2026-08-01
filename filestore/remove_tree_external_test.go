package filestore_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
)

func TestRemoveTreeDeletesOnlyTheNamedRootedTree(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatalf("OpenRoot(%q) error = %v, want nil", rootDirectory, err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("Root.Close() error = %v, want nil", closeErr)
		}
	}()
	tree := filepath.Join(rootDirectory, "tree")
	if err := os.MkdirAll(filepath.Join(tree, "nested"), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v, want nil", tree, err)
	}
	if err := os.WriteFile(filepath.Join(tree, "nested", "evidence"), []byte("owned"), 0o600); err != nil {
		t.Fatalf("WriteFile(tree evidence) error = %v, want nil", err)
	}
	keeper := filepath.Join(rootDirectory, "keeper")
	if err := os.WriteFile(keeper, []byte("retained"), 0o600); err != nil {
		t.Fatalf("WriteFile(keeper) error = %v, want nil", err)
	}
	request := filestore.TreeRemovalRequest{
		Location: filestore.Location{Root: root, Path: mustRelativePath(t, "tree")},
	}
	if err := filestore.RemoveTree(t.Context(), request); err != nil {
		t.Fatalf("RemoveTree() error = %v, want nil", err)
	}
	if _, err := os.Stat(tree); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(tree) error = %v, want %v", err, os.ErrNotExist)
	}
	if got, err := os.ReadFile(keeper); err != nil || string(got) != "retained" {
		t.Fatalf("keeper = %q, error = %v; want retained, nil", got, err)
	}
	if err := filestore.RemoveTree(t.Context(), request); err != nil {
		t.Fatalf("second RemoveTree() error = %v, want nil", err)
	}
}

func TestRemoveTreeRemovesSymlinkWithoutCrossingIt(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	outsideDirectory := t.TempDir()
	outside := filepath.Join(outsideDirectory, "evidence")
	if err := os.WriteFile(outside, []byte("must survive"), 0o600); err != nil {
		t.Fatalf("WriteFile(outside) error = %v, want nil", err)
	}
	link := filepath.Join(rootDirectory, "tree")
	if err := os.Symlink(outsideDirectory, link); err != nil {
		t.Fatalf("Symlink() error = %v, want nil", err)
	}
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatalf("OpenRoot(%q) error = %v, want nil", rootDirectory, err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("Root.Close() error = %v, want nil", closeErr)
		}
	}()
	if err := filestore.RemoveTree(t.Context(), filestore.TreeRemovalRequest{
		Location: filestore.Location{Root: root, Path: mustRelativePath(t, "tree")},
	}); err != nil {
		t.Fatalf("RemoveTree(symlink) error = %v, want nil", err)
	}
	if _, err := os.Lstat(link); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lstat(link) error = %v, want %v", err, os.ErrNotExist)
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "must survive" {
		t.Fatalf("outside evidence = %q, error = %v; want must survive, nil", got, err)
	}
}
