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

// Each leaf measures acquisition of the same 128-byte inode, independent Go
// metadata observation, and caller-owned Close. No transfer throughput is claimed.
func BenchmarkNativeHandleAcquisition(b *testing.B) {
	b.ReportAllocs()
	for _, operation := range []struct {
		name string
		open func(context.Context, filestore.Location) (*os.File, error)
	}{
		{name: "Read", open: func(ctx context.Context, location filestore.Location) (*os.File, error) {
			return filestore.OpenRead(ctx, filestore.ReadHandleRequest{Location: location})
		}},
		{name: "Update", open: func(ctx context.Context, location filestore.Location) (*os.File, error) {
			return filestore.OpenUpdate(ctx, filestore.UpdateHandleRequest{Location: location})
		}},
		{name: "AppendExisting", open: func(ctx context.Context, location filestore.Location) (*os.File, error) {
			return filestore.OpenAppend(ctx, filestore.AppendRequest{Location: location, Mode: 0o640, Append: filestore.AppendExisting})
		}},
		{name: "Lock", open: func(ctx context.Context, location filestore.Location) (*os.File, error) {
			return filestore.OpenLockFile(ctx, filestore.LockFileRequest{Location: location, Mode: 0o640})
		}},
	} {
		b.Run(operation.name, func(b *testing.B) {
			directory := b.TempDir()
			root, err := os.OpenRoot(directory)
			if err != nil {
				b.Fatal(err)
			}
			defer func() {
				if err := root.Close(); err != nil {
					b.Error(err)
				}
			}()
			path, err := core.ParseRelativePath("file")
			if err != nil {
				b.Fatal(err)
			}
			payload := bytes.Repeat([]byte{0, 255, 7, 31}, 32)
			hostPath := filepath.Join(directory, path.String())
			if err := os.WriteFile(hostPath, payload, 0o640); err != nil {
				b.Fatal(err)
			}
			if err := os.Chmod(hostPath, 0o640); err != nil {
				b.Fatal(err)
			}
			original, err := os.Stat(hostPath)
			if err != nil {
				b.Fatal(err)
			}
			location := filestore.Location{Root: root, Path: path}
			if err := location.Validate(); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				file, err := operation.open(b.Context(), location)
				if err != nil || file == nil {
					b.Fatalf("acquisition = (%v,%v), want live file", file, err)
				}
				info, statErr := file.Stat()
				closeErr := file.Close()
				if statErr != nil || closeErr != nil || !os.SameFile(original, info) || info.Mode() != original.Mode() || info.Size() != int64(len(payload)) || !info.ModTime().Equal(original.ModTime()) {
					b.Fatalf("observed acquisition = (%v,%v,%v), want exact original inode metadata", info, statErr, closeErr)
				}
			}
			got, err := os.ReadFile(hostPath)
			if err != nil || !bytes.Equal(got, payload) {
				b.Fatalf("retained bytes = (%v,%v), want exact %v", got, err, payload)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 1 || entries[0].Name() != path.String() {
				b.Fatalf("namespace = (%v,%v), want one original file", entries, err)
			}
		})
	}
}

// Each operation reacquires a fixed outgoing file, transfers sync/close ownership
// to RotateAppend, observes its exclusively created empty incoming inode, closes
// that caller-owned handle, then durably removes the incoming name.
func BenchmarkRotateAppendOwnedHandleLifecycle(b *testing.B) {
	directory := b.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			b.Error(err)
		}
	}()
	current, err := core.ParseRelativePath("current")
	if err != nil {
		b.Fatal(err)
	}
	next, err := core.ParseRelativePath("next")
	if err != nil {
		b.Fatal(err)
	}
	payload := bytes.Repeat([]byte{0, 255, 7, 31}, 32)
	if err := os.WriteFile(filepath.Join(directory, current.String()), payload, 0o600); err != nil {
		b.Fatal(err)
	}
	original, err := root.Lstat(current.String())
	if err != nil {
		b.Fatal(err)
	}
	outgoingRequest := filestore.AppendRequest{Location: filestore.Location{Root: root, Path: current}, Mode: 0o600, Append: filestore.AppendExisting}
	incomingRequest := filestore.AppendRequest{Location: filestore.Location{Root: root, Path: next}, Mode: 0o640, Append: filestore.AppendCreate}
	removal := filestore.RemovalRequest{Location: incomingRequest.Location}
	if err := outgoingRequest.Validate(); err != nil {
		b.Fatal(err)
	}
	if err := incomingRequest.Validate(); err != nil {
		b.Fatal(err)
	}
	if err := removal.Validate(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		outgoing, err := filestore.OpenAppend(b.Context(), outgoingRequest)
		if err != nil {
			b.Fatal(err)
		}
		incoming, err := filestore.RotateAppend(b.Context(), filestore.RotationRequest{Outgoing: outgoing, Incoming: incomingRequest})
		if err != nil || incoming == nil {
			b.Fatalf("rotation = (%v,%v), want real incoming", incoming, err)
		}
		if _, err := outgoing.Stat(); !errors.Is(err, fs.ErrClosed) {
			b.Fatalf("outgoing = %v, want native closed", err)
		}
		info, statErr := incoming.Stat()
		closeErr := incoming.Close()
		if statErr != nil || closeErr != nil || !info.Mode().IsRegular() || info.Size() != 0 || info.Mode().Perm() != incomingRequest.Mode || os.SameFile(info, original) {
			b.Fatalf("incoming = (%v,%v,%v), want new empty inode with mode %#o", info, statErr, closeErr, incomingRequest.Mode)
		}
		if err := filestore.Remove(b.Context(), removal); err != nil {
			b.Fatal(err)
		}
	}
	got, err := os.ReadFile(filepath.Join(directory, current.String()))
	if err != nil || !bytes.Equal(got, payload) {
		b.Fatalf("outgoing bytes = (%v,%v), want exact %v", got, err, payload)
	}
	info, err := root.Lstat(current.String())
	if err != nil || !os.SameFile(info, original) || info.Mode() != original.Mode() || !info.ModTime().Equal(original.ModTime()) {
		b.Fatalf("outgoing metadata = (%v,%v), want original %v", info, err, original)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 || entries[0].Name() != current.String() {
		b.Fatalf("namespace = (%v,%v), want only original outgoing", entries, err)
	}
}
