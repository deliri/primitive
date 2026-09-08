package filestore_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// The fixed workload moves the same binary inode out and back. Every operation
// proves source disappearance, target inode, exact bytes and mode. Cross-parent
// synchronization is measured separately from same-parent synchronization.
func BenchmarkRenameBinaryInodeRoundTrip(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name  string
		cross bool
	}{{"SameParent", false}, {"CrossParent", true}} {
		b.Run(tc.name, func(b *testing.B) {
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
			payload := bytes.Repeat([]byte{0, 255, 1, 127}, 32)
			if err := os.WriteFile(filepath.Join(directory, "source"), payload, 0o600); err != nil {
				b.Fatal(err)
			}
			original, err := root.Stat("source")
			if err != nil {
				b.Fatal(err)
			}
			destination := "target"
			if tc.cross {
				if err := os.Mkdir(filepath.Join(directory, "other"), 0o700); err != nil {
					b.Fatal(err)
				}
				destination = filepath.Join("other", "target")
			}
			source, err := core.ParseRelativePath("source")
			if err != nil {
				b.Fatal(err)
			}
			target, err := core.ParseRelativePath(destination)
			if err != nil {
				b.Fatal(err)
			}
			requests := [...]filestore.RenameRequest{{Location: filestore.Location{Root: root, Path: source}, Target: target}, {Location: filestore.Location{Root: root, Path: target}, Target: source}}
			for _, request := range requests {
				if err := request.Validate(); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			for b.Loop() {
				for _, request := range requests {
					if err := filestore.Rename(b.Context(), request); err != nil {
						b.Fatal(err)
					}
					_, sourceErr := root.Lstat(request.Location.Path.String())
					got, statErr := root.Lstat(request.Target.String())
					data, readErr := os.ReadFile(filepath.Join(directory, request.Target.String()))
					if !errors.Is(sourceErr, fs.ErrNotExist) || statErr != nil || readErr != nil || !os.SameFile(original, got) || got.Mode() != original.Mode() || got.ModTime().UnixNano() != original.ModTime().UnixNano() || !bytes.Equal(data, payload) {
						b.Fatalf("rename inode/bytes/namespace = (%v,%v,%v,%v), want exact moved source", sourceErr, got, statErr, readErr)
					}
				}
			}
			entries, err := os.ReadDir(directory)
			wantEntries := 1
			if tc.cross {
				wantEntries = 2
			}
			if err != nil || len(entries) != wantEntries {
				b.Fatalf("final root entries = (%v,%v), want %d", entries, err, wantEntries)
			}
			if tc.cross {
				entries, err := os.ReadDir(filepath.Join(directory, "other"))
				if err != nil || len(entries) != 0 {
					b.Fatalf("other entries = (%v,%v), want empty", entries, err)
				}
			}
		})
	}
}
