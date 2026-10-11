package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func proveDirectoryContentsRemoval(t testing.TB, count uint8, payload []byte, canceled bool) {
	t.Helper()
	directory, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "retained"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	for index := range int(count) {
		entry := filepath.Join(directory, fmt.Sprint(index))
		if index%3 == 0 {
			if err := os.Mkdir(entry, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(entry, "nested"), payload, 0600); err != nil {
				t.Fatal(err)
			}
		} else if index%3 == 1 {
			if err := os.WriteFile(entry, payload, 0600); err != nil {
				t.Fatal(err)
			}
		} else if err := os.Symlink(outside, entry); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	path, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filestore.OpenRoot(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	relative, err := core.ParseRelativePath(".")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if canceled {
		cancel()
	}
	err = filestore.RemoveDirectoryContents(ctx, filestore.Location{Root: root, Path: relative})
	if canceled && !errors.Is(err, context.Canceled) || !canceled && err != nil {
		t.Fatalf("removal: %v", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	if canceled {
		want = int(count)
	}
	if len(entries) != want {
		t.Fatalf("native removal retained %d entries, want %d", len(entries), want)
	}
	after, err := os.Stat(directory)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("removal replaced its directory: %v", err)
	}
	retained, err := os.ReadFile(filepath.Join(outside, "retained"))
	if err != nil || !bytes.Equal(retained, payload) {
		t.Fatalf("removal escaped rooted custody: %v", err)
	}
}

func TestDirectoryContentsRemovalDrainsNativeEntries(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			count := [...]uint8{0, 1, 63, 64, 65, 129, 193, 255}[index%8]
			proveDirectoryContentsRemoval(t, count, []byte{0, 255, byte(index)}, index/8%2 == 1)
		})
	}
}

func FuzzDirectoryContentsRemovalConservesNativeNamespace(f *testing.F) {
	f.Add(uint8(65), []byte{0, 255}, false)
	f.Add(uint8(129), []byte("outside must survive"), true)
	f.Add(uint8(0), []byte{}, false)
	f.Fuzz(func(t *testing.T, count uint8, payload []byte, canceled bool) {
		proveDirectoryContentsRemoval(t, count, payload, canceled)
	})
}
