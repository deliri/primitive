//go:build darwin || linux

package filestore_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestHeldStandingNativeSearchPermissionLayerTriad(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses native owner search refusals")
	}
	for _, tc := range []struct {
		mode    fs.FileMode
		want    filestore.HeldStanding
		wantErr error
	}{
		{mode: 0o000, want: filestore.HeldStandingUnknown, wantErr: core.ErrFilestoreSource},
		{mode: 0o100, want: filestore.HeldStandingSame},
		{mode: 0o200, want: filestore.HeldStandingUnknown, wantErr: core.ErrFilestoreSource},
		{mode: 0o300, want: filestore.HeldStandingSame},
		{mode: 0o400, want: filestore.HeldStandingUnknown, wantErr: core.ErrFilestoreSource},
		{mode: 0o500, want: filestore.HeldStandingSame},
		{mode: 0o600, want: filestore.HeldStandingUnknown, wantErr: core.ErrFilestoreSource},
		{mode: 0o700, want: filestore.HeldStandingSame},
	} {
		t.Run(fmt.Sprintf("owner %03o retains exact standing or native refusal", tc.mode), func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			parent := filepath.Join(directory, "parent")
			name := filepath.Join(parent, "held")
			if err := os.Mkdir(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(parent, 0o700); err != nil {
					t.Error(err)
				}
			})
			payload := []byte{0, 255, 1, 127}
			if err := os.WriteFile(name, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			held, err := os.Open(name)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := held.Close(); err != nil {
					t.Error(err)
				}
			})
			path, err := core.ParseAbsolutePath(name)
			if err != nil {
				t.Fatal(err)
			}
			heldBefore, err := held.Stat()
			if err != nil {
				t.Fatal(err)
			}
			parentBefore, err := os.Stat(parent)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(parent, tc.mode); err != nil {
				t.Fatal(err)
			}
			native, nativeErr := os.Lstat(name)
			got, gotErr := filestore.ObserveHeldStanding(t.Context(), held, path)
			if got != tc.want || !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("standing = (%v,%v), want (%v,%v)", got, gotErr, tc.want, tc.wantErr)
			}
			if tc.wantErr == nil {
				if nativeErr != nil || !os.SameFile(heldBefore, native) || got.Validate() != nil {
					t.Fatalf("native standing = (%v,%v), want held inode %v", native, nativeErr, heldBefore)
				}
			} else {
				var nativePath, gotPath *os.PathError
				if !errors.Is(nativeErr, fs.ErrPermission) || !errors.As(nativeErr, &nativePath) || !errors.As(gotErr, &gotPath) || !errors.Is(gotErr, nativePath.Err) || gotPath.Path != nativePath.Path || gotPath.Op != nativePath.Op {
					t.Fatalf("refused native observation = (%v,%v), want exact permission cause", gotErr, nativeErr)
				}
			}
			parentAfter, err := os.Stat(parent)
			if err != nil || !os.SameFile(parentBefore, parentAfter) || parentAfter.Mode().Perm() != tc.mode || parentBefore.ModTime().UnixNano() != parentAfter.ModTime().UnixNano() {
				t.Fatalf("parent = (%v,%v), want retained identity/time and mode %#o", parentAfter, err, tc.mode)
			}
			heldAfter, err := held.Stat()
			if err != nil || !os.SameFile(heldBefore, heldAfter) || heldBefore.Mode() != heldAfter.Mode() || heldBefore.ModTime().UnixNano() != heldAfter.ModTime().UnixNano() {
				t.Fatalf("held inode = (%v,%v), want %v", heldAfter, err, heldBefore)
			}
			data := make([]byte, len(payload))
			if n, err := io.ReadFull(held, data); err != nil || n != len(payload) || !bytes.Equal(data, payload) {
				t.Fatalf("held cursor/bytes = (%v,%d,%v), want %v", data, n, err, payload)
			}
			var extra [1]byte
			if n, err := held.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
				t.Fatalf("held completion = (%d,%v), want exact EOF", n, err)
			}
			if err := os.Chmod(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(parent)
			if err != nil || len(entries) != 1 || entries[0].Name() != "held" {
				t.Fatalf("retained namespace = (%v,%v), want only held", entries, err)
			}
			candidate, err := os.Lstat(name)
			if err != nil || !os.SameFile(heldBefore, candidate) || heldBefore.Mode() != candidate.Mode() || heldBefore.ModTime().UnixNano() != candidate.ModTime().UnixNano() {
				t.Fatalf("retained name = (%v,%v), want original inode %v", candidate, err, heldBefore)
			}
		})
	}
}
