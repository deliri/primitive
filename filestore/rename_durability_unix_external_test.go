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

func TestRenameNativePermissionAndDurabilityLayerTriad(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses native owner-permission refusals")
	}
	for _, tc := range []struct {
		mode                    fs.FileMode
		wantRename, wantDurable bool
	}{
		{mode: 0o000}, {mode: 0o100}, {mode: 0o200}, {mode: 0o300, wantRename: true},
		{mode: 0o400}, {mode: 0o500}, {mode: 0o600}, {mode: 0o700, wantRename: true, wantDurable: true},
	} {
		t.Run(fmt.Sprintf("owner %03o retains exact effect and durability identity", tc.mode), func(t *testing.T) {
			t.Parallel()
			directories := [2]string{t.TempDir(), t.TempDir()}
			var roots [2]*os.Root
			payload := []byte{0, 255, 1}
			oldTarget := []byte{1, 254, 0}
			for i, directory := range directories {
				for _, entry := range []struct {
					name string
					data []byte
				}{{"source", payload}, {"target", oldTarget}, {"neighbor", payload}} {
					if err := os.WriteFile(filepath.Join(directory, entry.name), entry.data, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				var err error
				roots[i], err = os.OpenRoot(directory)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(directory, 0o700); err != nil {
						t.Error(err)
					}
					if err := roots[i].Close(); err != nil {
						t.Error(err)
					}
				})
			}
			before, err := removalFixtureSnapshot(directories[0])
			if err != nil {
				t.Fatal(err)
			}
			infos := make([]fs.FileInfo, len(before))
			for i, entry := range before {
				infos[i], err = os.Stat(filepath.Join(directories[0], entry.name))
				if err != nil {
					t.Fatal(err)
				}
			}
			sourceBefore, err := roots[0].Stat("source")
			if err != nil {
				t.Fatal(err)
			}
			for _, directory := range directories {
				if err := os.Chmod(directory, tc.mode); err != nil {
					t.Fatal(err)
				}
			}
			nativeRenameErr := roots[1].Rename("source", "target")
			if (nativeRenameErr == nil) != tc.wantRename || (!tc.wantRename && !errors.Is(nativeRenameErr, fs.ErrPermission)) {
				t.Fatalf("native rename = %v, want admitted %t and permission on refusal", nativeRenameErr, tc.wantRename)
			}
			var nativeSyncErr error
			if tc.wantRename {
				directory, err := roots[1].OpenFile(".", os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NONBLOCK, 0)
				nativeSyncErr = err
				if directory != nil {
					nativeSyncErr = errors.Join(directory.Sync(), directory.Close())
				}
				if (nativeSyncErr == nil) != tc.wantDurable || (!tc.wantDurable && !errors.Is(nativeSyncErr, fs.ErrPermission)) {
					t.Fatalf("native parent settlement = %v, want durable %t and permission on refusal", nativeSyncErr, tc.wantDurable)
				}
			}
			request := filestore.RenameRequest{Location: filestore.Location{Root: roots[0], Path: mustRelativePath(t, "source")}, Target: mustRelativePath(t, "target")}
			gotErr := filestore.Rename(t.Context(), request)
			var wantErr error
			if !tc.wantRename {
				wantErr = core.ErrFilestoreActivation
			} else if !tc.wantDurable {
				wantErr = core.ErrFilestoreActivationIndeterminate
			}
			if !errors.Is(gotErr, wantErr) {
				t.Fatalf("rename = %v, want %v", gotErr, wantErr)
			}
			if !tc.wantRename {
				var native, gotNative *os.LinkError
				if !errors.As(nativeRenameErr, &native) || !errors.As(gotErr, &gotNative) || !errors.Is(gotErr, native.Err) || errors.Is(gotErr, core.ErrFilestoreActivationIndeterminate) {
					t.Fatalf("pre-effect refusal = %v, want native %v without indeterminate effect", gotErr, nativeRenameErr)
				}
			} else if !tc.wantDurable {
				var native, gotNative *fs.PathError
				if !errors.As(nativeSyncErr, &native) || !errors.As(gotErr, &gotNative) || !errors.Is(gotErr, native.Err) {
					t.Fatalf("post-effect refusal = %v, want indeterminate/native %v", gotErr, nativeSyncErr)
				}
			}
			for _, directory := range directories {
				info, err := os.Stat(directory)
				if err != nil || info.Mode().Perm() != tc.mode {
					t.Fatalf("parent mode = (%v,%v), want unchanged %#o", info, err, tc.mode)
				}
				if err := os.Chmod(directory, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			got, err := removalFixtureSnapshot(directories[0])
			if err != nil {
				t.Fatal(err)
			}
			want, err := removalFixtureSnapshot(directories[1])
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(want) {
				t.Fatalf("namespace = %+v, want Go %+v", got, want)
			}
			for i := range want {
				if got[i].name != want[i].name || got[i].mode != want[i].mode || !bytes.Equal(got[i].data, want[i].data) {
					t.Fatalf("entry %d = %+v, want Go %+v", i, got[i], want[i])
				}
			}
			if tc.wantRename {
				after, err := roots[0].Stat("target")
				if err != nil || !os.SameFile(sourceBefore, after) || sourceBefore.Mode() != after.Mode() || sourceBefore.ModTime().UnixNano() != after.ModTime().UnixNano() {
					t.Fatalf("completed move = (%v,%v), want exact source %v even on durability refusal", after, err, sourceBefore)
				}
			}
			for i, entry := range before {
				if tc.wantRename && entry.name != "neighbor" {
					continue
				}
				after, err := roots[0].Stat(entry.name)
				if err != nil || !os.SameFile(infos[i], after) || infos[i].Mode() != after.Mode() || infos[i].ModTime().UnixNano() != after.ModTime().UnixNano() {
					t.Fatalf("retained %s = (%v,%v), want %v", entry.name, after, err, infos[i])
				}
			}
		})
	}
}
