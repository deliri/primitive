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

func TestPermissionEffectLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		oldMode, newMode fs.FileMode
		directory        bool
		link             bool
		outside          bool
		missing          bool
		canceled         bool
		closed           bool
		wantErr          error
	}{
		{name: "read-only seal retains binary bytes and inode", oldMode: 0o600, newMode: 0o400},
		{name: "write-only result remains synchronizable through the held file", oldMode: 0o400, newMode: 0o200},
		{name: "write-only ingress can lend a write handle for synchronization", oldMode: 0o200, newMode: 0o400},
		{name: "execute-only result cannot lose the pre-acquired sync handle", oldMode: 0o600, newMode: 0o100},
		{name: "unchanged mode still acknowledges the existing dirty file", oldMode: 0o600, newMode: 0o600},
		{name: "directory permission change retains its child bytes", oldMode: 0o700, newMode: 0o500, directory: true},
		{name: "confined final link applies Go chmod to its target", oldMode: 0o600, newMode: 0o400, link: true},
		{name: "outside link cannot chmod an unowned inode", oldMode: 0o600, newMode: 0o400, link: true, outside: true, wantErr: core.ErrFilestoreActivation},
		{name: "missing target retains native absence", oldMode: 0o600, newMode: 0o400, missing: true, wantErr: core.ErrFilestoreActivation},
		{name: "canceled call preserves mode before acquisition", oldMode: 0o600, newMode: 0o400, canceled: true, wantErr: context.Canceled},
		{name: "closed root cannot acknowledge permission work", oldMode: 0o600, newMode: 0o400, closed: true, wantErr: core.ErrFilestoreActivation},
		{name: "unset permission request cannot produce a mode effect", oldMode: 0o600, wantErr: core.ErrFilestoreContract},
		{name: "file type bits cannot enter permission intent", oldMode: 0o600, newMode: fs.ModeDir | 0o400, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := []byte{0, 255, 7, 31}
			outside := t.TempDir()
			outsidePath := filepath.Join(outside, "outside")
			if err := os.WriteFile(outsidePath, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			outsideBefore, err := os.Lstat(outsidePath)
			if err != nil {
				t.Fatal(err)
			}
			directories := [2]string{t.TempDir(), t.TempDir()}
			var roots [2]*os.Root
			var readers [2]*os.File
			var before [2]fs.FileInfo
			subject := "subject"
			if tc.missing {
				subject = "missing"
			}
			for i, directory := range directories {
				root, err := os.OpenRoot(directory)
				if err != nil {
					t.Fatal(err)
				}
				roots[i] = root
				t.Cleanup(func() {
					if err := root.Close(); err != nil && !errors.Is(err, fs.ErrClosed) {
						t.Error(err)
					}
				})
				if err := os.WriteFile(filepath.Join(directory, "neighbor"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
				physical := filepath.Join(directory, "subject")
				if tc.link {
					physical = filepath.Join(directory, "target")
				}
				contentPath := physical
				if tc.directory {
					if err := os.Mkdir(physical, 0o700); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := os.Chmod(physical, 0o700); err != nil {
							t.Error(err)
						}
					})
					contentPath = filepath.Join(physical, "child")
				}
				if err := os.WriteFile(contentPath, payload, 0o600); err != nil {
					t.Fatal(err)
				}
				reader, err := os.Open(contentPath)
				if err != nil {
					t.Fatal(err)
				}
				readers[i] = reader
				t.Cleanup(func() {
					if err := reader.Close(); err != nil {
						t.Error(err)
					}
				})
				if err := os.Chmod(physical, tc.oldMode); err != nil {
					t.Fatal(err)
				}
				before[i], err = os.Stat(physical)
				if err != nil {
					t.Fatal(err)
				}
				if tc.link {
					target := "target"
					if tc.outside {
						target = outsidePath
					}
					if err := os.Symlink(target, filepath.Join(directory, "subject")); err != nil {
						t.Fatal(err)
					}
				}
			}
			request := filestore.PermissionRequest{Location: filestore.Location{Root: roots[0], Path: mustRelativePath(t, subject)}, Mode: tc.newMode}
			ctx := t.Context()
			if tc.canceled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if tc.closed {
				for _, root := range roots {
					if err := root.Close(); err != nil {
						t.Fatal(err)
					}
				}
			}
			invalidMode := tc.newMode == 0 || tc.newMode != tc.newMode.Perm()
			var nativeErr error
			if !tc.canceled && !invalidMode {
				nativeErr = roots[1].Chmod(subject, tc.newMode)
			}
			gotErr := filestore.SetPermissions(ctx, request)
			if (gotErr == nil) != (tc.wantErr == nil) || tc.wantErr != nil && !errors.Is(gotErr, tc.wantErr) {
				t.Errorf("SetPermissions = %v, want %v", gotErr, tc.wantErr)
			}
			if invalidMode {
				var gotIdentity core.ErrorIdentity
				if !errors.As(gotErr, &gotIdentity) || gotIdentity != core.ErrFilestoreContract {
					t.Errorf("contract identity = %v, want exact %v", gotIdentity, core.ErrFilestoreContract)
				}
			}
			if nativeErr != nil {
				var nativePath, gotPath *fs.PathError
				if !errors.As(nativeErr, &nativePath) || !errors.As(gotErr, &gotPath) || !errors.Is(gotErr, nativePath.Err) {
					t.Errorf("native cause = %v, want Go chmod cause %v", gotErr, nativeErr)
				}
			} else if !tc.canceled && !invalidMode && gotErr != nil {
				t.Errorf("permission result = %v, want admitted Go chmod result", gotErr)
			}
			physicalName := "subject"
			if tc.link {
				physicalName = "target"
			}
			gotInfo, err := os.Stat(filepath.Join(directories[0], physicalName))
			if err != nil {
				t.Fatal(err)
			}
			wantInfo, err := os.Stat(filepath.Join(directories[1], physicalName))
			if err != nil {
				t.Fatal(err)
			}
			if gotInfo.Mode() != wantInfo.Mode() || !os.SameFile(before[0], gotInfo) || before[0].ModTime().UnixNano() != gotInfo.ModTime().UnixNano() {
				t.Errorf("mode/identity = %v, want native mode %v and retained inode %v", gotInfo, wantInfo.Mode(), before[0])
			}
			gotBytes := make([]byte, len(payload))
			n, err := readers[0].ReadAt(gotBytes, 0)
			if err != nil || n != len(payload) || !bytes.Equal(gotBytes, payload) {
				t.Errorf("retained bytes = (%d,%v,%v), want %v", n, gotBytes, err, payload)
			}
			gotNeighbor, err := os.ReadFile(filepath.Join(directories[0], "neighbor"))
			if err != nil || !bytes.Equal(gotNeighbor, payload) {
				t.Errorf("neighbor = (%v,%v), want %v", gotNeighbor, err, payload)
			}
			outsideAfter, err := os.Lstat(outsidePath)
			if err != nil {
				t.Fatal(err)
			}
			gotOutside, err := os.ReadFile(outsidePath)
			if err != nil || !bytes.Equal(gotOutside, payload) || !os.SameFile(outsideBefore, outsideAfter) || outsideBefore.Mode() != outsideAfter.Mode() {
				t.Errorf("outside = (%v,%v,%v), want unchanged %v", outsideAfter, gotOutside, err, outsideBefore)
			}
			entries, err := os.ReadDir(directories[0])
			wantEntries := 2
			if tc.link {
				wantEntries++
			}
			if err != nil || len(entries) != wantEntries {
				t.Errorf("namespace = (%v,%v), want %d original entries", entries, err, wantEntries)
			}
			if tc.link {
				gotTarget, err := os.Readlink(filepath.Join(directories[0], "subject"))
				wantTarget := "target"
				if tc.outside {
					wantTarget = outsidePath
				}
				if err != nil || gotTarget != wantTarget {
					t.Errorf("retained link = (%q,%v), want %q", gotTarget, err, wantTarget)
				}
			}
		})
	}
}
