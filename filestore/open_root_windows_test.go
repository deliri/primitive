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

	t.Run("opened root retains the inspected directory identity", func(t *testing.T) {
		t.Parallel()

		path := t.TempDir()
		before, beforeErr := os.Lstat(path)
		root, gotErr := openRootDirectory(path)
		if gotErr != nil {
			t.Fatalf("openRootDirectory(directory) error = %v, want nil", gotErr)
		}
		t.Cleanup(func() {
			if closeErr := root.Close(); closeErr != nil {
				t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
			}
		})
		after, afterErr := root.Stat(".")
		same := beforeErr == nil && afterErr == nil && os.SameFile(before, after)
		if !same {
			t.Fatalf("root identity = (before %v, after %v, same %t), want (nil, nil, true)", beforeErr, afterErr, same)
		}
	})

	t.Run("regular file is refused before root acquisition", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "regular")
		if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(regular) error = %v, want nil", err)
		}
		root, gotErr := openRootDirectory(path)
		if !errors.Is(gotErr, fs.ErrInvalid) || root != nil {
			t.Fatalf("openRootDirectory(regular) = (%v, %v), want nil and %v", root, gotErr, fs.ErrInvalid)
		}
	})
}
