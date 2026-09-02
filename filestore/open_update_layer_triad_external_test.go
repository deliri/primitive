package filestore_test

import (
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestOpenUpdateLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive rooted update changes only the selected bytes", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root, rootErr := filestore.OpenRoot(t.Context(), openRootAbsolute(t, directory))
		if rootErr != nil {
			t.Fatalf("filestore.OpenRoot() error = %v, want nil", rootErr)
		}
		t.Cleanup(func() { openRootClose(t, root) })
		path := mustRelativePath(t, "subject")
		writePermissionFixture(t, root, path)
		request := filestore.UpdateHandleRequest{Location: filestore.Location{Root: root, Path: path}}
		if gotErr := request.Validate(); gotErr != nil {
			t.Fatalf("UpdateHandleRequest.Validate() error = %v, want nil", gotErr)
		}
		handle, gotErr := filestore.OpenUpdate(t.Context(), request)
		if gotErr != nil {
			t.Fatalf("filestore.OpenUpdate() error = %v, want nil", gotErr)
		}
		gotBytes, writeErr := handle.WriteAt([]byte("X"), 1)
		closeErr := handle.Close()
		if writeErr != nil || closeErr != nil || gotBytes != 1 {
			t.Fatalf("update write/close = (%d, %v, %v), want (1, nil, nil)", gotBytes, writeErr, closeErr)
		}
		reader, readOpenErr := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest(request))
		if readOpenErr != nil {
			t.Fatalf("filestore.OpenRead(updated) error = %v, want nil", readOpenErr)
		}
		got, readErr := io.ReadAll(io.LimitReader(reader, 7))
		readCloseErr := reader.Close()
		if readErr != nil || readCloseErr != nil || string(got) != "sXaled" {
			t.Fatalf("updated bytes/read/close = (%q, %v, %v), want (%q, nil, nil)", got, readErr, readCloseErr, "sXaled")
		}
	})

	t.Run("negative rooted directory never becomes an update handle", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root, rootErr := filestore.OpenRoot(t.Context(), openRootAbsolute(t, directory))
		if rootErr != nil {
			t.Fatalf("filestore.OpenRoot() error = %v, want nil", rootErr)
		}
		t.Cleanup(func() { openRootClose(t, root) })
		path := mustRelativePath(t, "directory")
		gotErr := root.Mkdir(path.String(), 0o700)
		if gotErr != nil {
			t.Fatalf("os.Root.Mkdir() setup error = %v, want nil", gotErr)
		}
		handle, gotErr := filestore.OpenUpdate(t.Context(), filestore.UpdateHandleRequest{Location: filestore.Location{Root: root, Path: path}})
		if !errors.Is(gotErr, core.ErrFilestoreActivation) || handle != nil {
			t.Fatalf("filestore.OpenUpdate(directory) = (%v, %v), want nil and errors.Is(..., %v)", handle, gotErr, core.ErrFilestoreActivation)
		}
	})

	t.Run("neutral zero request performs no filesystem effect", func(t *testing.T) {
		t.Parallel()

		handle, gotErr := filestore.OpenUpdate(t.Context(), filestore.UpdateHandleRequest{})
		if !errors.Is(gotErr, core.ErrFilestoreContract) || handle != nil {
			t.Fatalf("filestore.OpenUpdate(zero request) = (%v, %v), want nil and errors.Is(..., %v)", handle, gotErr, core.ErrFilestoreContract)
		}
	})
}
