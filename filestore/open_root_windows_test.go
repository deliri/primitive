//go:build windows

package filestore

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsOpenRootDirectoryAcceptsOnlyTheInspectedDirectory(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name               string
		directory, missing bool
		wantErr            error
	}{
		{name: "acquired root retains inspected directory inode", directory: true},
		{name: "regular file is refused without mutation", wantErr: fs.ErrInvalid},
		{name: "missing name cannot fabricate a root", missing: true, wantErr: fs.ErrNotExist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			name := filepath.Join(directory, "entry")
			if tc.directory {
				if err := os.Mkdir(name, 0o700); err != nil {
					t.Fatal(err)
				}
			} else if !tc.missing {
				if err := os.WriteFile(name, []byte{0, 255, 1}, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before, beforeErr := os.Lstat(name)
			root, gotErr := openRootDirectory(name)
			if root != nil {
				t.Cleanup(func() {
					if err := root.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			if !errors.Is(gotErr, tc.wantErr) || (root != nil) != tc.directory {
				t.Fatalf("openRootDirectory = (%v,%v), want directory=%t and %v", root, gotErr, tc.directory, tc.wantErr)
			}
			if tc.directory {
				after, err := root.Stat(".")
				if beforeErr != nil || err != nil || !os.SameFile(before, after) {
					t.Fatalf("root inode = (%v,%v), want inspected inode", after, err)
				}
			}
			after, err := os.Lstat(name)
			if tc.missing {
				if !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("missing after = %v, want native absence", err)
				}
			} else if beforeErr != nil || err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
				t.Fatalf("entry after = (%v,%v), want retained native facts", after, err)
			}
		})
	}
}
