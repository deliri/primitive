//go:build unix

package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func inspectEntry(t *testing.T, path string) filestore.Inspection {
	t.Helper()
	absolute, err := core.ParseAbsolutePath(path)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(%q) error = %v, want nil", path, err)
	}
	inspection, err := filestore.Inspect(context.Background(), absolute)
	if err != nil {
		t.Fatalf("filestore.Inspect(%q) error = %v, want nil", path, err)
	}
	return inspection
}

func TestInspectReportsRealStorageBehindARegularFile(t *testing.T) {
	t.Parallel()

	t.Run("a dense file is backed by at least its own bytes", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "dense")
		payload := bytes.Repeat([]byte("witness rides primitive\n"), 4096)
		if err := os.WriteFile(path, payload, 0o600); err != nil {
			t.Fatalf("write dense file: %v", err)
		}
		allocation, err := inspectEntry(t, path).Allocation()
		if err != nil {
			t.Fatalf("Allocation() error = %v, want nil", err)
		}
		if !allocation.Reported() {
			t.Fatalf("Allocation().Reported() = false, want true on a POSIX host")
		}
		allocated, err := allocation.Bytes()
		if err != nil {
			t.Fatalf("Allocation().Bytes() error = %v, want nil", err)
		}
		if got, want := allocated.Uint64(), uint64(len(payload)); got < want {
			t.Fatalf("dense file allocation = %d bytes, want at least %d", got, want)
		}
	})

	t.Run("a sparse file claims a size the device never granted", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "sparse")
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("create sparse file: %v", err)
		}
		const claimed = 8 << 20
		if err := file.Truncate(claimed); err != nil {
			t.Fatalf("truncate sparse file: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close sparse file: %v", err)
		}
		inspection := inspectEntry(t, path)
		size, err := inspection.SizeBytes()
		if err != nil {
			t.Fatalf("SizeBytes() error = %v, want nil", err)
		}
		if got := size.Uint64(); got != claimed {
			t.Fatalf("sparse file SizeBytes() = %d, want %d", got, claimed)
		}
		allocation, err := inspection.Allocation()
		if err != nil {
			t.Fatalf("Allocation() error = %v, want nil", err)
		}
		allocated, err := allocation.Bytes()
		if err != nil {
			t.Fatalf("Allocation().Bytes() error = %v, want nil", err)
		}
		if got := allocated.Uint64(); got >= claimed {
			t.Fatalf("sparse file allocation = %d bytes, want below the %d byte claim", got, claimed)
		}
	})

	t.Run("only a regular file has an allocation", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if _, err := inspectEntry(t, dir).Allocation(); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("directory Allocation() error = %v, want %v", err, core.ErrFilestoreContract)
		}

		absentPath := filepath.Join(dir, "never-written")
		if _, err := inspectEntry(t, absentPath).Allocation(); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("absent Allocation() error = %v, want %v", err, core.ErrFilestoreContract)
		}

		linkPath := filepath.Join(dir, "link")
		if err := os.Symlink(filepath.Join(dir, "target"), linkPath); err != nil {
			t.Fatalf("create symlink: %v", err)
		}
		if _, err := inspectEntry(t, linkPath).Allocation(); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("symlink Allocation() error = %v, want %v", err, core.ErrFilestoreContract)
		}
	})
}
