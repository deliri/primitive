//go:build !darwin && !linux

package filestore_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestCreateNamedPipeNativeLayerTriad(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path, err := core.ParseAbsolutePath(filepath.Join(directory, "fifo"))
	if err != nil {
		t.Fatal(err)
	}
	err = filestore.CreateNamedPipe(t.Context(), filestore.NamedPipeRequest{Path: path, Mode: 0o600})
	if !errors.Is(err, errors.ErrUnsupported) || !errors.Is(err, core.ErrFilestoreActivation) {
		t.Fatalf("unsupported creation=%v, want unsupported activation", err)
	}
	if _, err := os.Lstat(path.String()); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unsupported creation entry=%v, want absent", err)
	}
}

func TestOpenNamedPipeReadWriteNativeLayerTriad(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	name, err := core.ParseRelativePath("absent")
	if err != nil {
		t.Fatal(err)
	}
	file, err := filestore.OpenNamedPipeReadWrite(t.Context(), filestore.Location{Root: root, Path: name})
	if file != nil {
		_ = file.Close()
		t.Fatalf("unsupported open handle=%v, want nil", file)
	}
	if !errors.Is(err, errors.ErrUnsupported) || !errors.Is(err, core.ErrFilestoreSource) {
		t.Fatalf("unsupported open=%v, want unsupported source", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("unsupported open entries=%v/%v, want empty", entries, err)
	}
}

func TestDuplicateBlockingFileNativeLayerTriad(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	file, err := os.Create(filepath.Join(directory, "source"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	duplicate, err := filestore.DuplicateBlockingFile(t.Context(), filestore.FileHandleRequest{File: file})
	if duplicate != nil {
		_ = duplicate.Close()
		t.Fatalf("unsupported duplicate=%v, want nil", duplicate)
	}
	if !errors.Is(err, errors.ErrUnsupported) || !errors.Is(err, core.ErrFilestoreSource) {
		t.Fatalf("unsupported duplicate=%v, want unsupported source", err)
	}
	if count, err := file.Write([]byte("kept")); count != 4 || err != nil {
		t.Fatalf("borrowed source acceptance=%d/%v, want still usable", count, err)
	}
}
