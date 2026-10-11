package runworkspace

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoSourceReadChecksExistingASTExtentBeforeAllocation(t *testing.T) {
	t.Parallel()
	for _, size := range [...]int{0, 1, 65535, 65536, 65537, GoDiscoverySourceMaximumBytes - 1, GoDiscoverySourceMaximumBytes, GoDiscoverySourceMaximumBytes + 1} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			want := bytes.Repeat([]byte{'x'}, size)
			if err := os.WriteFile(filepath.Join(directory, "subject.go"), want, 0o600); err != nil {
				t.Fatal(err)
			}
			root, err := core.ParseAbsolutePath(directory)
			if err != nil {
				t.Fatal(err)
			}
			manager, err := Open(t.Context(), Configuration{RunParent: root})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := manager.Close(); err != nil {
					t.Error(err)
				}
			})
			path, err := core.ParseRelativePath("subject.go")
			if err != nil {
				t.Fatal(err)
			}
			got, gotErr := manager.readGoSource(t.Context(), path)
			if size > GoDiscoverySourceMaximumBytes {
				if !errors.Is(gotErr, core.ErrPrimitiveContract) || got != nil {
					t.Fatalf("AST source refusal = (%d bytes,%v), want nil and primitive contract", len(got), gotErr)
				}
				return
			}
			if gotErr != nil || !bytes.Equal(got, want) {
				t.Fatalf("AST source = (%d bytes,%v), want exactly %d native bytes", len(got), gotErr, len(want))
			}
		})
	}
}

func TestGoSourceReadRefusesLargeSparseNativeFileWithoutReadingItsBody(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "subject.go")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	const size = 1 << 40
	if err := errors.Join(file.Truncate(size), file.Close()); err != nil {
		t.Fatal(err)
	}
	root, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := Open(t.Context(), Configuration{RunParent: root})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Error(err)
		}
	})
	relative, err := core.ParseRelativePath("subject.go")
	if err != nil {
		t.Fatal(err)
	}
	got, gotErr := manager.readGoSource(t.Context(), relative)
	if !errors.Is(gotErr, core.ErrPrimitiveContract) || got != nil {
		t.Fatalf("sparse source refusal = (%d bytes,%v), want nil and primitive contract before allocation", len(got), gotErr)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() != size {
		t.Fatalf("native sparse extent = (%v,%v), want %d unchanged", info, err, size)
	}
}
