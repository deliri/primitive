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

// A real read/write peer admits every native access mode without waiting for
// another process. Rejection must happen on the acquired object before bytes
// are consumed. The peer's descriptor stays inside Go's Control callback.
func TestPublicHandleOpenersRefuseNamedFIFOWithoutConsumingPeer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		door    nativeHandleDoor
		wantErr error
	}{
		{name: "read opener refuses FIFO before exposing its stream", door: nativeHandleRead, wantErr: core.ErrFilestoreSource},
		{name: "update opener refuses FIFO despite native read-write access", door: nativeHandleUpdate, wantErr: core.ErrFilestoreActivation},
		{name: "append opener refuses writable FIFO with a live peer", door: nativeHandleAppend, wantErr: core.ErrFilestoreActivation},
		{name: "lock opener refuses FIFO before applying requested mode", door: nativeHandleLock, wantErr: core.ErrFilestoreDestination},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			path := filepath.Join(directory, "pipe")
			if err := syscall.Mkfifo(path, 0o600); err != nil {
				t.Fatal(err)
			}
			peer, err := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = peer.Close() })
			before, err := peer.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if before.Mode()&fs.ModeNamedPipe == 0 {
				t.Fatalf("native fixture = %v, want named FIFO", before)
			}
			payload := []byte{0, 255, 7}
			if n, err := peer.Write(payload); err != nil || n != len(payload) {
				t.Fatalf("peer seed = (%d,%v), want %d bytes", n, err, len(payload))
			}
			location := filestore.Location{Root: root, Path: mustRelativePath(t, "pipe")}
			var got *os.File
			var gotErr error
			switch tc.door {
			case nativeHandleRead:
				got, gotErr = filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: location})
			case nativeHandleUpdate:
				got, gotErr = filestore.OpenUpdate(t.Context(), filestore.UpdateHandleRequest{Location: location})
			case nativeHandleAppend:
				got, gotErr = filestore.OpenAppend(t.Context(), filestore.AppendRequest{Location: location, Mode: 0o640, Append: filestore.AppendExisting})
			case nativeHandleLock:
				got, gotErr = filestore.OpenLockFile(t.Context(), filestore.LockFileRequest{Location: location, Mode: 0o640})
			default:
				t.Fatalf("door = %d, want declared public opener", tc.door)
			}
			if got != nil {
				t.Cleanup(func() { _ = got.Close() })
			}
			if got != nil || !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, fs.ErrInvalid) {
				t.Fatalf("FIFO acquisition = (%v,%v), want nil and %v/native invalid", got, gotErr, tc.wantErr)
			}
			raw, err := peer.SyscallConn()
			if err != nil {
				t.Fatal(err)
			}
			var buffer [4]byte
			var n int
			var readErr error
			controlErr := raw.Control(func(descriptor uintptr) { n, readErr = syscall.Read(int(descriptor), buffer[:]) })
			if controlErr != nil || readErr != nil || n != len(payload) || !bytes.Equal(buffer[:max(n, 0)], payload) {
				t.Fatalf("retained FIFO bytes = (%d,%v,%v,%v), want exact queued %v", n, buffer, controlErr, readErr, payload)
			}
			after, err := peer.Stat()
			if err != nil || !os.SameFile(before, after) || after.Mode() != before.Mode() {
				t.Fatalf("caller peer = (%v,%v), want original live FIFO", after, err)
			}
			named, err := root.Lstat("pipe")
			if err != nil || !os.SameFile(before, named) || named.Mode() != before.Mode() {
				t.Fatalf("named FIFO = (%v,%v), want unchanged inode and mode", named, err)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 1 || entries[0].Name() != location.Path.String() {
				t.Fatalf("namespace = (%v,%v), want only original FIFO", entries, err)
			}
		})
	}
}
