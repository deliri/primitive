//go:build darwin || linux

package filestore_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Exhaust the three owner permission bits, with present and absent leaves.
// Search admits Lstat; directory listing permission must never be demanded.
func TestInspectGoSearchPermissionLayerTriad(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the owner-permission refusal boundary")
	}
	for _, tc := range []struct {
		mode    fs.FileMode
		wantErr error
	}{
		{mode: 0o000, wantErr: fs.ErrPermission},
		{mode: 0o100},
		{mode: 0o200, wantErr: fs.ErrPermission},
		{mode: 0o300},
		{mode: 0o400, wantErr: fs.ErrPermission},
		{mode: 0o500},
		{mode: 0o600, wantErr: fs.ErrPermission},
		{mode: 0o700},
	} {
		for _, leaf := range []struct {
			name     string
			exists   bool
			wantKind filestore.PathKind
		}{
			{name: "existing child", exists: true, wantKind: filestore.PathKindRegularFile},
			{name: "absent child", wantKind: filestore.PathKindAbsent},
		} {
			t.Run(fmt.Sprintf("parent mode %03o/%s", tc.mode, leaf.name), func(t *testing.T) {
				t.Parallel()
				directory := t.TempDir()
				parent := filepath.Join(directory, "parent")
				if err := os.Mkdir(parent, 0o700); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(parent, 0o700); err != nil {
						t.Error(err)
					}
				})
				name := filepath.Join(parent, "child")
				payload := []byte{0, 255, 1}
				if leaf.exists {
					if err := os.WriteFile(name, payload, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Chmod(parent, tc.mode); err != nil {
					t.Fatal(err)
				}
				_, nativeErr := os.Lstat(name)
				wantNative := tc.wantErr
				if wantNative == nil && !leaf.exists {
					wantNative = fs.ErrNotExist
				}
				if !errors.Is(nativeErr, wantNative) {
					t.Fatalf("native Lstat = %v, want %v; permission fixture did not reach its boundary", nativeErr, wantNative)
				}
				path, err := core.ParseAbsolutePath(name)
				if err != nil {
					t.Fatal(err)
				}
				got, gotErr := filestore.Inspect(t.Context(), path)
				if tc.wantErr != nil {
					var native *fs.PathError
					if got != (filestore.Inspection{}) || !errors.Is(gotErr, core.ErrFilestoreSource) || !errors.Is(gotErr, tc.wantErr) || !errors.As(gotErr, &native) {
						t.Fatalf("refused observation = (%v,%v), want zero and source/native permission error", got, gotErr)
					}
				} else {
					kind, kindErr := got.Kind()
					if gotErr != nil || kindErr != nil || kind != leaf.wantKind {
						t.Fatalf("searchable observation = (%v,%v,%v), want %v without listing permission", kind, kindErr, gotErr, leaf.wantKind)
					}
				}
				if err := os.Chmod(parent, 0o700); err != nil {
					t.Fatal(err)
				}
				entries, err := os.ReadDir(parent)
				wantEntries := 0
				if leaf.exists {
					wantEntries = 1
				}
				if err != nil || len(entries) != wantEntries {
					t.Fatalf("namespace = (%v,%v), want %d original entries", entries, err, wantEntries)
				}
				if leaf.exists {
					data, err := os.ReadFile(name)
					if err != nil || !bytes.Equal(data, payload) {
						t.Fatalf("retained child = (%v,%v), want untouched %v", data, err, payload)
					}
				}
			})
		}
	}
}
