//go:build windows

package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Windows has neither POSIX permission enforcement nor O_DIRECTORY. Exercise
// existing native entries against Go's actual chmod/sync result, including a
// native durability refusal after the mode effect. No POSIX success is assumed.
func FuzzEnsureDirectoryNativeNamespaceCustody(f *testing.F) {
	seedDirectory := f.TempDir()
	seedPath, err := core.ParseAbsolutePath(seedDirectory)
	if err != nil {
		f.Fatal(err)
	}
	seed, err := filestore.Inspect(f.Context(), seedPath)
	if err != nil || seed.Validate() != nil {
		f.Fatalf("native seed = (%v,%v), want valid observation", seed, err)
	}
	permissions, err := seed.Permissions()
	if err != nil {
		f.Fatal(err)
	}
	mode, err := permissions.Bits()
	if err != nil {
		f.Fatal(err)
	}
	f.Add([]byte{}, uint16(mode), true, false, false)
	for _, tc := range []struct {
		mode                        uint16
		directory, canceled, closed bool
	}{
		{mode: 0, directory: true},
		{mode: 1, directory: true},
		{mode: 0o200, directory: true},
		{mode: 0o777, directory: true},
		{mode: 0o1000, directory: true},
		{mode: 0xffff, directory: true},
		{mode: 0o700},
		{mode: 0o700, directory: true, canceled: true},
		{mode: 0o700, directory: true, closed: true},
		{mode: 0, directory: true, canceled: true, closed: true},
	} {
		f.Add([]byte{0, 255, 1}, tc.mode, tc.directory, tc.canceled, tc.closed)
	}
	f.Fuzz(func(t *testing.T, payload []byte, rawMode uint16, directory, canceled, closed bool) {
		payload = payload[:min(len(payload), 1024)]
		work := t.TempDir()
		oracle := t.TempDir()
		roots := [2]*os.Root{requireTestRoot(t, work), requireTestRoot(t, oracle)}
		for _, root := range roots {
			if directory {
				if err := root.Mkdir("entry", 0o700); err != nil {
					t.Fatal(err)
				}
				if err := root.WriteFile(filepath.Join("entry", "child"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
			} else if err := root.WriteFile("entry", payload, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := root.WriteFile("neighbor", payload, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		before, err := roots[0].Lstat("entry")
		if err != nil {
			t.Fatal(err)
		}
		mode := fs.FileMode(rawMode)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		if canceled {
			cancel()
		}
		if closed {
			if err := roots[0].Close(); err != nil {
				t.Fatal(err)
			}
		}
		var wantErr, wantNative error
		switch {
		case canceled:
			wantErr = context.Canceled
		case mode == 0 || mode != mode.Perm():
			wantErr = core.ErrFilestoreContract
		case closed:
			wantErr, wantNative = core.ErrFilestoreActivation, fs.ErrClosed
		case !directory:
			wantErr, wantNative = core.ErrFilestoreActivation, fs.ErrExist
		default:
			file, err := roots[1].Open("entry")
			if err != nil {
				t.Fatal(err)
			}
			chmodErr := file.Chmod(mode)
			var syncErr error
			if chmodErr == nil {
				syncErr = file.Sync()
			}
			nativeErr := errors.Join(chmodErr, syncErr, file.Close())
			if nativeErr != nil {
				var pathErr *fs.PathError
				if !errors.As(nativeErr, &pathErr) {
					t.Fatalf("native mode/sync = %v, want PathError", nativeErr)
				}
				wantErr, wantNative = core.ErrFilestoreActivation, pathErr.Err
			}
		}
		request := filestore.DirectoryRequest{Location: filestore.Location{Root: roots[0], Path: mustRelativePath(t, "entry")}, Mode: mode}
		gotErr := filestore.EnsureDirectory(ctx, request)
		if !errors.Is(gotErr, wantErr) || wantNative != nil && !errors.Is(gotErr, wantNative) {
			t.Fatalf("EnsureDirectory = %v, want %v and native %v", gotErr, wantErr, wantNative)
		}
		for _, class := range []error{core.ErrFilestoreContract, core.ErrFilestoreActivation, core.ErrFilestoreActivationIndeterminate, core.ErrFilestoreSource, core.ErrFilestoreConflict, core.ErrFilestoreCleanup} {
			if errors.Is(gotErr, class) != errors.Is(wantErr, class) {
				t.Fatalf("error classes = %v, want exactly %v", gotErr, wantErr)
			}
		}
		got, err := removalFixtureSnapshot(work)
		if err != nil {
			t.Fatal(err)
		}
		want, err := removalFixtureSnapshot(oracle)
		if err != nil || len(got) != len(want) {
			t.Fatalf("namespace = (%v,%v), want native %v", got, err, want)
		}
		for i := range want {
			if got[i].name != want[i].name || got[i].mode != want[i].mode || got[i].target != want[i].target || !bytes.Equal(got[i].data, want[i].data) {
				t.Fatalf("entry %d = %+v, want native %+v", i, got[i], want[i])
			}
		}
		after, err := os.Lstat(filepath.Join(work, "entry"))
		if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
			t.Fatalf("entry custody = (%v,%v), want unchanged inode, extent, and timestamp", after, err)
		}
	})
}
