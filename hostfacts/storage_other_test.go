//go:build !linux

package hostfacts

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestObserveDiskRotationAnswersUnsupportedOffLinux pins the one answer a
// platform without the kernel index may give for a real directory: the
// unsupported observation, never an invented rotation and never an error.
func TestObserveDiskRotationAnswersUnsupportedOffLinux(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	directory, err := core.ParseAbsolutePath(dir)
	if err != nil {
		t.Fatalf("ParseAbsolutePath(%q) error = %v, want nil", dir, err)
	}
	got, gotErr := ObserveDiskRotation(context.Background(), DiskRotationRequest{Directory: directory})
	if gotErr != nil || got != DiskRotationUnsupported {
		t.Fatalf("ObserveDiskRotation() = (%v, %v), want (%v, nil)", got, gotErr, DiskRotationUnsupported)
	}
}
