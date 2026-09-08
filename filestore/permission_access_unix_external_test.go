//go:build darwin || linux

package filestore_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestPermissionSyncCapabilityRefusalPreservesMetadata(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the owner permission boundary")
	}
	for _, tc := range []struct {
		name      string
		mode      fs.FileMode
		directory bool
	}{
		{name: "unreadable and unwritable regular file cannot acknowledge durable chmod", mode: 0},
		{name: "execute-only regular file cannot lend a synchronization handle", mode: 0o100},
		{name: "search-only directory cannot lend a directory sync handle", mode: 0o100, directory: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			subject := filepath.Join(directory, "subject")
			contentPath := subject
			if tc.directory {
				if err := os.Mkdir(subject, 0o700); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(subject, 0o700); err != nil {
						t.Error(err)
					}
				})
				contentPath = filepath.Join(subject, "child")
			}
			payload := []byte{0, 255, 7, 31}
			if err := os.WriteFile(contentPath, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			reader, err := os.Open(contentPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := reader.Close(); err != nil {
					t.Error(err)
				}
			})
			if err := os.Chmod(subject, tc.mode); err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(subject)
			if err != nil {
				t.Fatal(err)
			}
			readFile, readErr := os.Open(subject)
			if readFile != nil {
				if err := readFile.Close(); err != nil {
					t.Error(err)
				}
			}
			writeFile, writeErr := os.OpenFile(subject, os.O_WRONLY, 0)
			if writeFile != nil {
				if err := writeFile.Close(); err != nil {
					t.Error(err)
				}
			}
			var readCause, writeCause *fs.PathError
			if !errors.As(readErr, &readCause) || !errors.Is(readErr, fs.ErrPermission) || !errors.As(writeErr, &writeCause) {
				t.Fatalf("native access = (%v,%v), want denied read and write handles", readErr, writeErr)
			}
			gotErr := filestore.SetPermissions(t.Context(), filestore.PermissionRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "subject")}, Mode: 0o600})
			if !errors.Is(gotErr, core.ErrFilestoreActivation) || errors.Is(gotErr, core.ErrFilestoreActivationIndeterminate) || !errors.Is(gotErr, readCause.Err) || !errors.Is(gotErr, writeCause.Err) {
				t.Errorf("acquisition refusal = %v, want definite activation refusal with both Go causes (%v,%v)", gotErr, readCause.Err, writeCause.Err)
			}
			after, err := os.Lstat(subject)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before, after) || before.Mode() != after.Mode() || before.ModTime().UnixNano() != after.ModTime().UnixNano() {
				t.Errorf("refused metadata = %v, want original %v", after, before)
			}
			got := make([]byte, len(payload))
			n, err := reader.ReadAt(got, 0)
			if err != nil || n != len(payload) || !bytes.Equal(got, payload) {
				t.Errorf("refused bytes = (%d,%v,%v), want %v", n, got, err, payload)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 1 {
				t.Errorf("namespace = (%v,%v), want only original subject", entries, err)
			}
		})
	}
}

func TestPermissionNamedPipeUsesNativeSynchronization(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		mode fs.FileMode
	}{
		{name: "a named FIFO uses the hosts actual Go synchronization result", mode: 0o400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			path := filepath.Join(directory, "pipe")
			if err := syscallMkfifo(path); err != nil {
				t.Fatal(err)
			}
			// A real read/write peer makes this type-refusal test safe even
			// under a mutant that opens without NONBLOCK. Separate subprocess
			// tests own the no-peer blocking-open contract.
			peer, err := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := peer.Close(); err != nil {
					t.Error(err)
				}
			})
			nativeFile, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
			if err != nil {
				t.Fatal(err)
			}
			nativeErr := nativeFile.Sync()
			if err := nativeFile.Close(); err != nil {
				t.Fatal(err)
			}
			var native *fs.PathError
			if nativeErr != nil && (!errors.As(nativeErr, &native) || native.Err == nil) {
				t.Fatalf("Go FIFO Sync = %v, want nil or typed native refusal", nativeErr)
			}
			before, err := peer.Stat()
			if err != nil {
				t.Fatal(err)
			}
			gotErr := filestore.SetPermissions(t.Context(), filestore.PermissionRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "pipe")}, Mode: tc.mode})
			if (gotErr == nil) != (nativeErr == nil) || nativeErr != nil && (!errors.Is(gotErr, core.ErrFilestoreActivationIndeterminate) || !errors.Is(gotErr, native.Err)) {
				t.Errorf("FIFO synchronization = %v, want Go result %v with indeterminate identity after a failed sync", gotErr, nativeErr)
			}
			after, err := peer.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before, after) || after.Mode().Perm() != tc.mode || before.Mode().Type() != after.Mode().Type() || before.ModTime().UnixNano() != after.ModTime().UnixNano() {
				t.Errorf("FIFO metadata = %v, want requested permissions on original %v", after, before)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 1 {
				t.Errorf("namespace = (%v,%v), want only original FIFO", entries, err)
			}
		})
	}
}
