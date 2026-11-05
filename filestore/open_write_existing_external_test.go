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

func writeOnlyTestLocation(t *testing.T, directory, name string) filestore.Location {
	t.Helper()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := filestore.CloseRoot(filestore.RootHandleRequest{Root: root}); err != nil {
			t.Error(err)
		}
	})
	relative, err := core.ParseRelativePath(name)
	if err != nil {
		t.Fatal(err)
	}
	return filestore.Location{Root: root, Path: relative}
}

func TestOpenWriteExistingPreservesNativeFileAndNamespace(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name               string
		mode               fs.FileMode
		missing, directory bool
		want               error
	}{
		{name: "write-only file needs no read access", mode: 0o200},
		{name: "ordinary file is neither truncated nor appended", mode: 0o600},
		{name: "absent file is never created", missing: true, want: fs.ErrNotExist},
		{name: "directory is never a writable stream", directory: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			name := filepath.Join(dir, "entry")
			if row.directory {
				if err := os.Mkdir(name, 0o700); err != nil {
					t.Fatal(err)
				}
			} else if !row.missing {
				if err := os.WriteFile(name, []byte("sentinel-tail"), row.mode); err != nil {
					t.Fatal(err)
				}
			}
			location := writeOnlyTestLocation(t, dir, "entry")
			file, err := filestore.OpenWriteExisting(t.Context(), filestore.UpdateHandleRequest{Location: location})
			if row.missing || row.directory {
				if file != nil || err == nil || row.want != nil && !errors.Is(err, row.want) {
					t.Fatalf("open=%v/%v, want refusal %v", file, err, row.want)
				}
				if row.missing {
					var native *fs.PathError
					if !errors.As(err, &native) {
						t.Fatalf("cause=%v, want native PathError", err)
					}
					if _, err := os.Stat(name); !errors.Is(err, fs.ErrNotExist) {
						t.Fatalf("absent path=%v, want absent", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			count, writeErr := filestore.WriteChunk(t.Context(), filestore.ChunkWriteRequest{Destination: file, Bytes: []byte("new")})
			closeErr := filestore.CloseFile(filestore.FileHandleRequest{File: file})
			if writeErr != nil || closeErr != nil || count.Uint64() != 3 {
				t.Fatalf("write=%d/%v close=%v, want 3/nil/nil", count.Uint64(), writeErr, closeErr)
			}
			if err := os.Chmod(name, 0o600); err != nil {
				t.Fatal(err)
			}
			actual, err := os.ReadFile(name)
			if err != nil || string(actual) != "newtinel-tail" {
				t.Fatalf("file=%q/%v, want newtinel-tail", actual, err)
			}
		})
	}
}

func TestOpenWriteExistingRejectsCancelledAndInvalidAdmission(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	name := filepath.Join(dir, "entry")
	if err := os.WriteFile(name, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	location := writeOnlyTestLocation(t, dir, "entry")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	file, err := filestore.OpenWriteExisting(ctx, filestore.UpdateHandleRequest{Location: location})
	if file != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled=%v/%v, want cancellation", file, err)
	}
	file, err = filestore.OpenWriteExisting(t.Context(), filestore.UpdateHandleRequest{})
	if file != nil || !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("empty request=%v/%v, want contract", file, err)
	}
	actual, err := os.ReadFile(name)
	if err != nil || string(actual) != "original" {
		t.Fatalf("cancelled file=%q/%v, want original", actual, err)
	}
}

func FuzzOpenWriteExistingNativeCustody(f *testing.F) {
	for _, data := range [][]byte{nil, []byte("new"), []byte("longer-than-the-original-sentinel-tail")} {
		f.Add(data, false)
		f.Add(data, true)
	}
	f.Fuzz(func(t *testing.T, data []byte, missing bool) {
		if len(data) > 256 {
			return
		}
		dir := t.TempDir()
		name := filepath.Join(dir, "entry")
		initial := []byte("sentinel-tail")
		if !missing {
			if err := os.WriteFile(name, initial, 0o200); err != nil {
				t.Fatal(err)
			}
		}
		location := writeOnlyTestLocation(t, dir, "entry")
		file, err := filestore.OpenWriteExisting(t.Context(), filestore.UpdateHandleRequest{Location: location})
		if missing {
			if file != nil || !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("missing=%v/%v, want no creation", file, err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		count, writeErr := filestore.WriteChunk(t.Context(), filestore.ChunkWriteRequest{Destination: file, Bytes: data})
		closeErr := filestore.CloseFile(filestore.FileHandleRequest{File: file})
		if writeErr != nil || closeErr != nil || count.Uint64() != uint64(len(data)) {
			t.Fatalf("write=%d/%v close=%v, want %d/nil/nil", count.Uint64(), writeErr, closeErr, len(data))
		}
		if err := os.Chmod(name, 0o600); err != nil {
			t.Fatal(err)
		}
		want := make([]byte, max(len(initial), len(data)))
		copy(want, initial)
		copy(want, data)
		actual, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(actual, want) {
			t.Fatalf("file=%x/%v, want exact prefix update %x", actual, err, want)
		}
	})
}
