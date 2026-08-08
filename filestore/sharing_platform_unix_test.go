//go:build !windows

package filestore_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// TestObserveSharingRefusesWhereOpensDoNotContend pins the platform split:
// on a POSIX host the question has no kernel answer, and the refusal names
// the contract identity rather than inventing an observation.
func TestObserveSharingRefusesWhereOpensDoNotContend(t *testing.T) {
	t.Parallel()

	path, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath() error = %v, want nil", err)
	}
	got, gotErr := filestore.ObserveSharing(t.Context(), path)
	if !errors.Is(gotErr, core.ErrFilestoreContract) {
		t.Fatalf("ObserveSharing(posix host) error = %v, want errors.Is %v", gotErr, core.ErrFilestoreContract)
	}
	if got != filestore.SharingUnknown {
		t.Fatalf("ObserveSharing(posix host) = %v, want %v", got, filestore.SharingUnknown)
	}
}
