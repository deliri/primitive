package filestore

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"os"
	"path/filepath"
	"testing"
)

func TestParentDirectorySynchronizationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path string
		closed     bool
		wantErr    error
	}{
		{name: "nested parent sync cannot create target", path: filepath.Join("objects", "target")},
		{name: "root parent sync cannot create missing leaf", path: "target"},
		{name: "missing parent retains native absence", path: filepath.Join("missing", "target"), wantErr: os.ErrNotExist},
		{name: "closed root retains native closed identity", path: "target", closed: true, wantErr: os.ErrClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			if err := os.Mkdir(filepath.Join(directory, "objects"), 0o700); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(filepath.Join(directory, "objects"))
			if err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			if tc.closed {
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Cleanup(func() {
					if err := root.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			path, err := core.ParseRelativePath(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			gotErr := syncParent(root, path)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("syncParent = %v, want %v", gotErr, tc.wantErr)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 1 || entries[0].Name() != "objects" || !entries[0].IsDir() {
				t.Fatalf("namespace = (%v,%v), want original objects directory only", entries, err)
			}
			children, err := os.ReadDir(filepath.Join(directory, "objects"))
			if err != nil || len(children) != 0 {
				t.Fatalf("parent contents = (%v,%v), want empty", children, err)
			}
			after, err := os.Stat(filepath.Join(directory, "objects"))
			if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
				t.Fatalf("parent custody = (%v,%v), want original inode/mode/time", after, err)
			}
		})
	}
}
