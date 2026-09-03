package runworkspace

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestArchiveEntryPathRejectsPlatformDependentBackslashes(t *testing.T) {
	t.Parallel()

	checkout, err := core.ParseRelativePath("checkout")
	if err != nil {
		t.Fatalf("core.ParseRelativePath(checkout) error = %v, want nil", err)
	}
	got, gotErr := archiveEntryPath(checkout, `a\b\c`, 2)
	if !errors.Is(gotErr, core.ErrPrimitiveContract) || got != (core.RelativePath{}) {
		t.Fatalf("archiveEntryPath(platform-dependent backslashes) = (%q, %v), want zero and %v", got.String(), gotErr, core.ErrPrimitiveContract)
	}
}
