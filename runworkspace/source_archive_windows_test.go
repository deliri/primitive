//go:build windows

package runworkspace

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestArchiveEntryPathUsesWindowsNativeSeparators(t *testing.T) {
	t.Parallel()

	checkout, err := core.ParseRelativePath("checkout")
	if err != nil {
		t.Fatalf("core.ParseRelativePath(checkout) error = %v, want nil", err)
	}
	got, gotErr := archiveEntryPath(checkout, "pkg/main.go", 2)
	if gotErr != nil {
		t.Fatalf("archiveEntryPath(pkg/main.go) error = %v, want nil", gotErr)
	}
	want, wantErr := core.ParseRelativePath(`checkout\pkg\main.go`)
	if wantErr != nil {
		t.Fatalf("core.ParseRelativePath(want) error = %v, want nil", wantErr)
	}
	if got != want {
		t.Fatalf("archiveEntryPath(pkg/main.go) = %q, want %q", got.String(), want.String())
	}
}
