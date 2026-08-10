package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestOpenStagedReadLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact staged inode reopens through the standard library", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		payload := deterministicPayload((32 << 10) + 1)
		staged, err := filestore.Stage(t.Context(), filestore.StageRequest{
			Source: bytes.NewReader(payload),
			Temporary: filestore.Location{
				Root: root, Path: mustRelativePath(t, ".read-stage"),
			},
			Mode: 0o600, MaximumBytes: mustByteCount(t, uint64(len(payload))),
		})
		if err != nil {
			t.Fatalf("Stage() error = %v, want nil", err)
		}
		file, err := filestore.OpenStagedRead(t.Context(), staged)
		if err != nil {
			t.Fatalf("OpenStagedRead() error = %v, want nil", err)
		}
		got, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("io.ReadAll(staged file) error = %v, want nil", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("os.File.Close() error = %v, want nil", err)
		}
		if !bytes.Equal(got, payload) {
			t.Fatalf("staged file bytes = %d, want exact %d", len(got), len(payload))
		}
		if err := filestore.Discard(t.Context(), staged); err != nil {
			t.Fatalf("Discard() error = %v, want nil", err)
		}
	})

	t.Run("negative substituted custody name is refused without opening stranger", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		staged, err := filestore.Stage(t.Context(), filestore.StageRequest{
			Source: bytes.NewReader([]byte{1}),
			Temporary: filestore.Location{
				Root: root, Path: mustRelativePath(t, ".owned-stage"),
			},
			Mode: 0o600, MaximumBytes: mustByteCount(t, 1),
		})
		if err != nil {
			t.Fatalf("Stage() error = %v, want nil", err)
		}
		if err := root.Rename(".owned-stage", ".moved-stage"); err != nil {
			t.Fatalf("os.Root.Rename() error = %v, want nil", err)
		}
		replacement, err := root.OpenFile(".owned-stage", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			t.Fatalf("os.Root.OpenFile(replacement) error = %v, want nil", err)
		}
		if _, err := replacement.Write([]byte{2}); err != nil {
			t.Fatalf("replacement.Write() error = %v, want nil", err)
		}
		if err := replacement.Close(); err != nil {
			t.Fatalf("replacement.Close() error = %v, want nil", err)
		}
		file, gotErr := filestore.OpenStagedRead(t.Context(), staged)
		if !errors.Is(gotErr, core.ErrFilestoreActivationIndeterminate) || file != nil {
			t.Fatalf("OpenStagedRead(substituted) = (%v, %v), want nil and errors.Is %v",
				file, gotErr, core.ErrFilestoreActivationIndeterminate)
		}
		got, err := os.ReadFile(filepath.Join(directory, ".owned-stage"))
		if err != nil || !bytes.Equal(got, []byte{2}) {
			t.Fatalf("replacement after refusal = (%v, %v), want ([2], nil)", got, err)
		}
	})

	t.Run("neutral canceled open leaves staged custody available", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		staged, err := filestore.Stage(t.Context(), filestore.StageRequest{
			Source: bytes.NewReader([]byte{3}),
			Temporary: filestore.Location{
				Root: root, Path: mustRelativePath(t, ".canceled-stage"),
			},
			Mode: 0o600, MaximumBytes: mustByteCount(t, 1),
		})
		if err != nil {
			t.Fatalf("Stage() error = %v, want nil", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		file, gotErr := filestore.OpenStagedRead(ctx, staged)
		if !errors.Is(gotErr, context.Canceled) || file != nil {
			t.Fatalf("OpenStagedRead(canceled) = (%v, %v), want nil and errors.Is %v", file, gotErr, context.Canceled)
		}
		if err := filestore.Discard(t.Context(), staged); err != nil {
			t.Fatalf("Discard(after canceled open) error = %v, want nil", err)
		}
	})
}
