package filestore_test

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Each iteration creates one real empty directory with Go and removes it
// durably through Filestore. Both namespace effects belong to this workload.
// Exclusive Mkdir plus the final observation prevent a no-op removal from
// being reported as a fast completed lifecycle.
func BenchmarkRemoveTreeEmptyDirectoryLifecycle(b *testing.B) {
	directory := b.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			b.Error(err)
		}
	}()
	path, err := core.ParseRelativePath("tree")
	if err != nil {
		b.Fatal(err)
	}
	request := filestore.TreeRemovalRequest{Location: filestore.Location{Root: root, Path: path}}
	if err := request.Validate(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := root.Mkdir(path.String(), 0o700); err != nil {
			b.Fatal(err)
		}
		if err := filestore.RemoveTree(b.Context(), request); err != nil {
			b.Fatal(err)
		}
	}
	if _, err := root.Lstat(path.String()); !errors.Is(err, fs.ErrNotExist) {
		b.Fatalf("removed directory = %v, want absent", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		b.Fatalf("remaining entries = (%v,%v), want empty", entries, err)
	}
}
