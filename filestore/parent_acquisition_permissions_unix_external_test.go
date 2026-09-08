//go:build darwin || linux

package filestore_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// A root is a real Go capability, not a promise that every child operation
// will be permitted. Exhaust read/search/write separately at acquisition.
func TestOpenParentNativeOwnerPermissionLayerTriad(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses native owner-permission refusals")
	}
	for _, tc := range []struct {
		mode     fs.FileMode
		wantOpen bool
	}{
		{mode: 0o000}, {mode: 0o100}, {mode: 0o200}, {mode: 0o300},
		{mode: 0o400, wantOpen: true}, {mode: 0o500, wantOpen: true}, {mode: 0o600, wantOpen: true}, {mode: 0o700, wantOpen: true},
	} {
		t.Run(fmt.Sprintf("owner %03o retains native parent capability", tc.mode), func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			parent := filepath.Join(directory, "parent")
			payload := []byte{0, 255, 1}
			if err := os.Mkdir(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(parent, 0o700); err != nil {
					t.Error(err)
				}
			})
			childName := filepath.Join(parent, "child")
			if err := os.WriteFile(childName, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			original, err := os.Stat(parent)
			if err != nil {
				t.Fatal(err)
			}
			childBefore, err := os.Stat(childName)
			if err != nil {
				t.Fatal(err)
			}
			path, err := core.ParseAbsolutePath(childName)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(parent, tc.mode); err != nil {
				t.Fatal(err)
			}
			native, nativeErr := os.OpenFile(parent, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NONBLOCK, 0)
			if native != nil {
				if err := native.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if (nativeErr == nil) != tc.wantOpen || (!tc.wantOpen && !errors.Is(nativeErr, fs.ErrPermission)) {
				t.Fatalf("native acquisition = %v, want admitted %t with permission on refusal", nativeErr, tc.wantOpen)
			}
			got, gotErr := filestore.OpenParent(t.Context(), path)
			if got.Root != nil {
				t.Cleanup(func() {
					if err := got.Root.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			if tc.wantOpen {
				if gotErr != nil || got.Validate() != nil || got.Path != mustRelativePath(t, "child") {
					t.Fatalf("OpenParent = (%+v,%v), want admitted child without extra search/write policy", got, gotErr)
				}
			} else {
				var nativePath, gotPath *fs.PathError
				if got != (filestore.Location{}) || !errors.Is(gotErr, core.ErrFilestoreSource) || !errors.As(nativeErr, &nativePath) || !errors.As(gotErr, &gotPath) || !errors.Is(gotErr, nativePath.Err) {
					t.Fatalf("OpenParent = (%+v,%v), want zero and native source refusal %v", got, gotErr, nativeErr)
				}
			}
			observed, err := os.Stat(parent)
			if err != nil || observed.Mode().Perm() != tc.mode || !os.SameFile(original, observed) || original.ModTime().UnixNano() != observed.ModTime().UnixNano() {
				t.Fatalf("parent metadata = (%v,%v), want original identity/time and mode %#o", observed, err, tc.mode)
			}
			if err := os.Chmod(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(parent)
			if err != nil || len(entries) != 1 || entries[0].Name() != "child" {
				t.Fatalf("retained children = (%v,%v), want original child", entries, err)
			}
			childAfter, err := os.Stat(childName)
			if err != nil || !os.SameFile(childBefore, childAfter) || childBefore.Mode() != childAfter.Mode() || childBefore.ModTime().UnixNano() != childAfter.ModTime().UnixNano() {
				t.Fatalf("child metadata = (%v,%v), want unchanged %v", childAfter, err, childBefore)
			}
			data, err := os.ReadFile(childName)
			if err != nil || !bytes.Equal(data, payload) {
				t.Fatalf("child bytes = (%v,%v), want %v", data, err, payload)
			}
			if tc.wantOpen {
				held, err := got.Root.Stat(".")
				if err != nil || !os.SameFile(original, held) {
					t.Fatalf("acquired directory = (%v,%v), want original inode", held, err)
				}
				child, err := got.Root.Lstat(got.Path.String())
				if err != nil || !os.SameFile(childBefore, child) {
					t.Fatalf("acquired child = (%v,%v), want original inode", child, err)
				}
			}
		})
	}
}
