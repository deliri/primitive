package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestScratchResetDiscardsBytesAndRestoresWriteCoordinate(t *testing.T) {
	t.Parallel()
	for _, extent := range []int{0, 1, 4095, 4096, 4097, 65535, 65536, 65537, 262145} {
		t.Run(strconv.Itoa(extent), func(t *testing.T) {
			t.Parallel()
			rootPath, err := core.ParseAbsolutePath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			root, err := filestore.OpenRoot(t.Context(), rootPath)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			}()
			path, err := core.ParseRelativePath("scratch")
			if err != nil {
				t.Fatal(err)
			}
			location := filestore.Location{Root: root, Path: path}
			writer, err := filestore.OpenScratch(t.Context(), filestore.ScratchRequest{Location: location, Mode: 0600})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := writer.Close(); err != nil {
					t.Error(err)
				}
			}()
			payload := bytes.Repeat([]byte{'x'}, extent)
			for cycle := range 3 {
				if n, err := writer.Write(payload); err != nil || n != len(payload) {
					t.Fatalf("write=(%d,%v)", n, err)
				}
				if err := filestore.ResetScratch(t.Context(), filestore.ScratchResetRequest{File: writer}); err != nil {
					t.Fatal(err)
				}
				if _, err := writer.Write([]byte("next")); err != nil {
					t.Fatal(err)
				}
				reader, err := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: location})
				if err != nil {
					t.Fatal(err)
				}
				data, readErr := io.ReadAll(reader)
				if err := errors.Join(readErr, reader.Close()); err != nil {
					t.Fatal(err)
				}
				if string(data) != "next" {
					t.Fatalf("cycle=%d extent=%d prefix=%q want 4 bytes next", cycle, len(data), data[:min(len(data),32)])
				}
			}
		})
	}
}

func TestScratchResetRefusalsPreserveExistingBytes(t *testing.T) {
	t.Parallel()
	if err := filestore.ResetScratch(t.Context(), filestore.ScratchResetRequest{}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatal(err)
	}
	rootPath, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, err := filestore.OpenRoot(t.Context(), rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	path, err := core.ParseRelativePath("scratch")
	if err != nil {
		t.Fatal(err)
	}
	location := filestore.Location{Root: root, Path: path}
	writer, err := filestore.OpenScratch(t.Context(), filestore.ScratchRequest{Location: location, Mode: 0600})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("preserve")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	cancel(nil)
	if err := filestore.ResetScratch(ctx, filestore.ScratchResetRequest{File: writer}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := filestore.ResetScratch(t.Context(), filestore.ScratchResetRequest{File: writer}); err == nil {
		t.Fatal("closed file accepted")
	}
	reader, err := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: location})
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := io.ReadAll(reader)
	if err := filestore.ResetScratch(t.Context(), filestore.ScratchResetRequest{File: reader}); err == nil {
		t.Fatal("read-only file accepted")
	}
	if err := errors.Join(readErr, reader.Close()); err != nil {
		t.Fatal(err)
	}
	if string(data) != "preserve" {
		t.Fatalf("bytes=%q", data)
	}
}
