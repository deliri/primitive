//go:build unix

package filelock_test

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filelock"
)

type closedDescriptorPolicy struct {
	name        string
	exclusivity filelock.Exclusivity
	patience    filelock.Patience
}

// TestUnixClosedDescriptorPreservesNativeLockFailure exhausts the two-by-two
// acquisition policy product against one real descriptor failure. A closed
// *os.File is still a non-nil request capability, so it reaches the Unix
// effect leaf and the kernel answers EBADF. No acquisition produced beside
// that failure is valid.
func TestUnixClosedDescriptorPreservesNativeLockFailure(t *testing.T) {
	t.Parallel()

	cases := []closedDescriptorPolicy{
		{name: "immediate exclusive preserves EBADF", exclusivity: filelock.Exclusive, patience: filelock.Immediate},
		{name: "immediate shared preserves EBADF", exclusivity: filelock.Shared, patience: filelock.Immediate},
		{name: "blocking exclusive preserves EBADF", exclusivity: filelock.Exclusive, patience: filelock.Blocking},
		{name: "blocking shared preserves EBADF", exclusivity: filelock.Shared, patience: filelock.Blocking},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			proveClosedDescriptorAcquireFailure(t, tc)
		})
	}
}

func proveClosedDescriptorAcquireFailure(t *testing.T, policy closedDescriptorPolicy) {
	t.Helper()

	file := closedDescriptor(t)
	got, gotErr := filelock.Acquire(t.Context(), filelock.Request{
		File: file, Exclusivity: policy.exclusivity, Patience: policy.patience,
	})
	if !errors.Is(gotErr, core.ErrFileLockUnavailable) ||
		!errors.Is(gotErr, syscall.EBADF) {
		t.Fatalf("Acquire(closed descriptor) error = %v, want errors.Is(..., %v) and errors.Is(..., %v)", gotErr, core.ErrFileLockUnavailable, syscall.EBADF)
	}
	if err := got.Validate(); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("Acquire(closed descriptor) acquisition.Validate() error = %v, want errors.Is(..., %v)", err, core.ErrPrimitiveContract)
	}
}

// TestUnixClosedDescriptorReleasePreservesNativeLockFailure proves the same
// native identity crosses the unlock effect leaf.
func TestUnixClosedDescriptorReleasePreservesNativeLockFailure(t *testing.T) {
	t.Parallel()

	gotErr := filelock.Release(t.Context(), closedDescriptor(t))
	if !errors.Is(gotErr, core.ErrFileLockUnavailable) ||
		!errors.Is(gotErr, syscall.EBADF) {
		t.Fatalf("Release(closed descriptor) error = %v, want errors.Is(..., %v) and errors.Is(..., %v)", gotErr, core.ErrFileLockUnavailable, syscall.EBADF)
	}
}

func closedDescriptor(t *testing.T) *os.File {
	t.Helper()

	path := filepath.Join(t.TempDir(), "closed.lock")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(%s) error = %v, want nil", path, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(%s) error = %v, want nil", path, err)
	}
	return file
}
