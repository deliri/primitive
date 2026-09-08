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

func FuzzPermissionNativeMetadataSemanticClosure(f *testing.F) {
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(emitted, uint16(0o600), uint16(0o400), false, false, false)
	for _, seed := range []struct {
		payload                   []byte
		before, after             uint16
		directory, link, canceled bool
	}{
		{payload: []byte{0, 255}, before: 0o600, after: 0o400},
		{payload: nil, before: 0o400, after: 0o400},
		{payload: []byte{0, 255}, before: 0o600, after: 0o100},
		{payload: []byte{0, 255}, before: 0o700, after: 0o500, directory: true},
		{payload: []byte{0, 255}, before: 0o600, after: 0o400, link: true},
		{payload: []byte{0, 255}, before: 0o600, after: 0o400, canceled: true},
	} {
		f.Add(seed.payload, seed.before, seed.after, seed.directory, seed.link, seed.canceled)
	}
	f.Fuzz(func(t *testing.T, payload []byte, rawBefore, rawAfter uint16, directory, link, canceled bool) {
		payload = payload[:min(len(payload), 2048)]
		// Admission of raw mode bits has an exhaustive finite-domain table. Here
		// filesystem content crosses the real effect boundary under valid intent.
		// Owner-read access supplies a pre-effect synchronization capability.
		beforeMode := fs.FileMode(rawBefore)&fs.ModePerm | 0o400
		afterMode := fs.FileMode(max(uint16(1), rawAfter&uint16(fs.ModePerm)))
		roots := [2]*os.Root{}
		directories := [2]string{t.TempDir(), t.TempDir()}
		readers := [2]*os.File{}
		var before [2]fs.FileInfo
		physicalName := "subject"
		if link {
			physicalName = "target"
		}
		for i, base := range directories {
			root, err := os.OpenRoot(base)
			if err != nil {
				t.Fatal(err)
			}
			roots[i] = root
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			})
			physical := filepath.Join(base, physicalName)
			contentPath := physical
			if directory {
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
			if err := os.Chmod(physical, beforeMode); err != nil {
				t.Fatal(err)
			}
			before[i], err = os.Stat(physical)
			if err != nil {
				t.Fatal(err)
			}
			if link {
				if err := os.Symlink(physicalName, filepath.Join(base, "subject")); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(base, "neighbor"), payload, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		request := filestore.PermissionRequest{Location: filestore.Location{Root: roots[0], Path: mustRelativePath(t, "subject")}, Mode: afterMode}
		if err := request.Validate(); err != nil {
			t.Fatal(err)
		}
		ctx := t.Context()
		var wantErr error
		if canceled {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
			wantErr = context.Canceled
		} else {
			wantErr = roots[1].Chmod(request.Location.Path.String(), afterMode)
		}
		gotErr := filestore.SetPermissions(ctx, request)
		if (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("SetPermissions = %v, want native %v", gotErr, wantErr)
		}
		if canceled && !errors.Is(gotErr, context.Canceled) {
			t.Fatalf("canceled effect = %v, want canceled", gotErr)
		}
		if wantErr != nil && !canceled {
			var nativePath, gotPath *fs.PathError
			if !errors.As(wantErr, &nativePath) || !errors.As(gotErr, &gotPath) || !errors.Is(gotErr, nativePath.Err) || !errors.Is(gotErr, core.ErrFilestoreActivation) {
				t.Fatalf("native cause = %v, want activation and %v", gotErr, wantErr)
			}
		}
		got, err := os.Stat(filepath.Join(directories[0], physicalName))
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.Stat(filepath.Join(directories[1], physicalName))
		if err != nil {
			t.Fatal(err)
		}
		if got.Mode() != want.Mode() || !os.SameFile(before[0], got) || before[0].ModTime().UnixNano() != got.ModTime().UnixNano() {
			t.Fatalf("mode/identity = %v, want Go mode %v and original inode %v", got, want.Mode(), before[0])
		}
		gotBytes := make([]byte, len(payload))
		n, err := readers[0].ReadAt(gotBytes, 0)
		if err != nil || n != len(payload) || !bytes.Equal(gotBytes, payload) {
			t.Fatalf("retained bytes = (%d,%v,%v), want %v", n, gotBytes, err, payload)
		}
		gotNeighbor, err := os.ReadFile(filepath.Join(directories[0], "neighbor"))
		if err != nil || !bytes.Equal(gotNeighbor, payload) {
			t.Fatalf("neighbor = (%v,%v), want %v", gotNeighbor, err, payload)
		}
		entries, err := os.ReadDir(directories[0])
		wantEntries := 2
		if link {
			wantEntries++
		}
		if err != nil || len(entries) != wantEntries {
			t.Fatalf("namespace = (%v,%v), want %d original entries", entries, err, wantEntries)
		}
		if link {
			gotTarget, err := os.Readlink(filepath.Join(directories[0], "subject"))
			if err != nil || gotTarget != physicalName {
				t.Fatalf("link = (%q,%v), want retained %q", gotTarget, err, physicalName)
			}
		}
	})
}
