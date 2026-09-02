package filestore_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestOpenDirectoryLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive held directory exposes one valid filesystem identity and closes cleanly", func(t *testing.T) {
		t.Parallel()

		path := t.TempDir()
		directory, gotErr := filestore.OpenDirectory(t.Context(), openRootAbsolute(t, path))
		if gotErr != nil {
			t.Fatalf("filestore.OpenDirectory() error = %v, want nil", gotErr)
		}
		filesystem, filesystemErr := directory.Filesystem()
		_, identityErr := filesystem.Uint64()
		closeErr := directory.Close()
		if filesystemErr != nil || identityErr != nil || closeErr != nil {
			t.Fatalf("held directory filesystem/identity/close errors = (%v, %v, %v), want nil", filesystemErr, identityErr, closeErr)
		}
		if secondCloseErr := directory.Close(); !errors.Is(secondCloseErr, core.ErrFilestoreContract) {
			t.Fatalf("HeldDirectory.Close(second) error = %v, want %v", secondCloseErr, core.ErrFilestoreContract)
		}
	})

	t.Run("negative regular file never becomes a held directory", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "regular")
		if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(regular) error = %v, want nil", err)
		}
		directory, gotErr := filestore.OpenDirectory(t.Context(), openRootAbsolute(t, path))
		if !errors.Is(gotErr, core.ErrFilestoreSource) || directory != nil {
			t.Fatalf("filestore.OpenDirectory(regular) = (%v, %v), want nil and %v", directory, gotErr, core.ErrFilestoreSource)
		}
	})

	t.Run("neutral zero path performs no filesystem acquisition", func(t *testing.T) {
		t.Parallel()

		directory, gotErr := filestore.OpenDirectory(t.Context(), core.AbsolutePath{})
		if !errors.Is(gotErr, core.ErrFilestoreContract) || directory != nil {
			t.Fatalf("filestore.OpenDirectory(zero) = (%v, %v), want nil and %v", directory, gotErr, core.ErrFilestoreContract)
		}
	})
}
