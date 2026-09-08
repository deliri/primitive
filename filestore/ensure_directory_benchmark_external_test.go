package filestore_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Each operation performs two real mode transitions on the same directory.
// Independent native observations reject a no-op, wrong inode or widened mode.
func BenchmarkEnsureExistingDirectoryModeTransitions(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name  string
		depth int
	}{{"Depth1", 1}, {"Depth16", 16}} {
		b.Run(tc.name, func(b *testing.B) {
			directory := b.TempDir()
			pathText := strings.Repeat("ancestor"+string(filepath.Separator), tc.depth-1) + "target"
			full := filepath.Join(directory, pathText)
			if err := os.MkdirAll(full, 0o700); err != nil {
				b.Fatal(err)
			}
			before, err := os.Stat(full)
			if err != nil {
				b.Fatal(err)
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() {
				if err := root.Close(); err != nil {
					b.Error(err)
				}
			})
			path, err := core.ParseRelativePath(pathText)
			if err != nil {
				b.Fatal(err)
			}
			request := filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o700}
			if err := request.Validate(); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				for _, mode := range [...]os.FileMode{0o750, 0o700} {
					request.Mode = mode
					if err := filestore.EnsureDirectory(b.Context(), request); err != nil {
						b.Fatal(err)
					}
					got, err := os.Stat(full)
					if err != nil || !got.IsDir() || !os.SameFile(before, got) || got.Mode().Perm() != mode {
						b.Fatalf("directory = (%v,%v), want same inode and mode %#o", got, err, mode)
					}
				}
			}
			entries, err := os.ReadDir(full)
			if err != nil || len(entries) != 0 {
				b.Fatalf("directory children = (%v,%v), want empty", entries, err)
			}
		})
	}
}

// Creation of a fixed two-entry chain and native removal are explicitly timed.
// Both exact directory modes and disappearance are independently observed.
func BenchmarkEnsureNewDirectoryNativeRemovalLifecycle(b *testing.B) {
	directory := b.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := root.Close(); err != nil {
			b.Error(err)
		}
	})
	path, err := core.ParseRelativePath(filepath.Join("parent", "target"))
	if err != nil {
		b.Fatal(err)
	}
	request := filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o750}
	if err := request.Validate(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := filestore.EnsureDirectory(b.Context(), request); err != nil {
			b.Fatal(err)
		}
		for _, name := range [...]string{"parent", path.String()} {
			got, err := root.Stat(name)
			if err != nil || !got.IsDir() || got.Mode().Perm() != request.Mode {
				b.Fatalf("created %s = (%v,%v), want directory mode %#o", name, got, err, request.Mode)
			}
		}
		if err := root.RemoveAll("parent"); err != nil {
			b.Fatal(err)
		}
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != 0 {
			b.Fatalf("removed namespace = (%v,%v), want empty", entries, err)
		}
	}
}
