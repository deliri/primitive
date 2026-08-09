//go:build linux

package hostfacts

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestObserveDiskRotationAnswersFromTheKernelIndexOnLinux pins the native
// contract: Linux always supports the interface, so a real directory
// answers a rotation, a non-rotation, or the honest unavailable, and a
// refusal carries the package identity. Unsupported is never a Linux
// answer.
func TestObserveDiskRotationAnswersFromTheKernelIndexOnLinux(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	directory, err := core.ParseAbsolutePath(dir)
	if err != nil {
		t.Fatalf("ParseAbsolutePath(%q) error = %v, want nil", dir, err)
	}
	got, gotErr := ObserveDiskRotation(context.Background(), DiskRotationRequest{Directory: directory})
	if gotErr != nil {
		if !errors.Is(gotErr, core.ErrHostFacts) {
			t.Fatalf("ObserveDiskRotation() error = %v, want the hostfacts identity", gotErr)
		}
		if got != DiskRotationUnknown {
			t.Fatalf("ObserveDiskRotation() = %v on error, want %v", got, DiskRotationUnknown)
		}
		return
	}
	if !got.IsValid() || got == DiskRotationUnsupported {
		t.Fatalf("ObserveDiskRotation() = %v, want an admitted non-unsupported rotation", got)
	}
}
