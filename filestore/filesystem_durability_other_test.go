//go:build !linux || (!amd64 && !arm64)

package filestore

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestFilesystemDurabilityUnsupportedBeforeUse(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	path, err := core.ParseRelativePath(".")
	if err != nil {
		t.Fatal(err)
	}
	uses := 0
	err = WithFilesystemDurability(t.Context(), FilesystemDurabilityRequest{Location: Location{Root: root, Path: path}, Use: func(context.Context) error { uses++; return nil }})
	if !errors.Is(err, errors.ErrUnsupported) || !errors.Is(err, core.ErrFilestoreActivation) || uses != 0 {
		t.Fatalf("unsupported scope=%v callback uses=%d, want explicit native refusal before Use", err, uses)
	}
	if _, err := root.Stat("."); err != nil {
		t.Fatalf("borrowed root=%v, want unchanged ownership", err)
	}
}

func FilesystemDurabilityNativeProofForTest(t *testing.T) {
	t.Helper()
	t.Run("unsupported", TestFilesystemDurabilityUnsupportedBeforeUse)
}
