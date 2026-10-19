package filestore_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func FuzzCreatedDirectoryScopeNativeOwnership(f *testing.F) {
	for _, body := range [][]byte{nil, []byte("native source\n"), {0, 255, '\n'}, bytes.Repeat([]byte{'x'}, 65537)} {
		f.Add(body, true)
		f.Add(body, false)
	}
	f.Fuzz(func(t *testing.T, body []byte, retain bool) {
		directory := t.TempDir()
		parent := requireTestRoot(t, directory)
		var borrowed *os.Root
		request := filestore.CreatedDirectoryScopeRequest{
			Directory: filestore.DirectoryRequest{Location: filestore.Location{Root: parent, Path: mustRelativePath(t, "owned")}, Mode: 0700},
			Use: func(_ context.Context, root *os.Root) error {
				borrowed = root
				file, err := root.OpenFile("content", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
				if err != nil {
					return err
				}
				_, writeErr := file.Write(body)
				if err := errors.Join(writeErr, file.Close()); err != nil {
					return err
				}
				if !retain {
					return io.ErrClosedPipe
				}
				return nil
			},
		}
		if err := request.Validate(); err != nil {
			t.Fatal(err)
		}
		err := filestore.WithCreatedDirectoryScope(t.Context(), request)
		if borrowed == nil {
			t.Fatal("native scope did not lend its root")
		}
		if _, err := borrowed.Stat("."); !errors.Is(err, fs.ErrClosed) {
			t.Fatalf("borrowed root escaped scope: %v", err)
		}
		if !retain {
			entries, readErr := os.ReadDir(directory)
			if !errors.Is(err, io.ErrClosedPipe) || readErr != nil || len(entries) != 0 {
				t.Fatalf("refused source=%v, native residue=%v/%v", err, entries, readErr)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(filepath.Join(directory, "owned", "content"))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.New()
		extent, readErr := io.Copy(digest, file)
		if err := errors.Join(readErr, file.Close()); err != nil {
			t.Fatal(err)
		}
		want := sha256.Sum256(body)
		if extent != int64(len(body)) || !bytes.Equal(digest.Sum(nil), want[:]) {
			t.Fatalf("retained native extent/hash=%d/%x, want %d/%x", extent, digest.Sum(nil), len(body), want)
		}
		called := false
		request.Use = func(context.Context, *os.Root) error { called = true; return nil }
		if err := filestore.WithCreatedDirectoryScope(t.Context(), request); !errors.Is(err, core.ErrFilestoreConflict) || !errors.Is(err, fs.ErrExist) || called {
			t.Fatalf("foreign acquisition=%v/%t, want native existence conflict before borrowing", err, called)
		}
	})
}
