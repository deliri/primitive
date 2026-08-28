package filelock_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filelock"
)

func TestAdvisoryExclusionLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive release transfers exclusive ownership to the waiting descriptor", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "lock")
		holder := openLockFile(t, path)
		successor := openLockFile(t, path)
		requireHeld(t, acquireImmediate(t, holder, filelock.Exclusive), true, "exclusive holder")
		requireHeld(t, acquireImmediate(t, successor, filelock.Exclusive), false, "successor while held")
		if gotErr := filelock.Release(t.Context(), holder); gotErr != nil {
			t.Fatalf("Release(holder) error = %v, want nil", gotErr)
		}
		requireHeld(t, acquireImmediate(t, successor, filelock.Exclusive), true, "successor after release")
	})

	t.Run("negative closed descriptor preserves lock failure and emits no acquisition", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "closed.lock")
		closed, gotOpenErr := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
		if gotOpenErr != nil {
			t.Fatalf("OpenFile(%s) error = %v, want nil", path, gotOpenErr)
		}
		if gotCloseErr := closed.Close(); gotCloseErr != nil {
			t.Fatalf("Close(%s) error = %v, want nil", path, gotCloseErr)
		}
		acquisition, gotErr := filelock.Acquire(t.Context(), filelock.Request{
			File: closed, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate,
		})
		if !errors.Is(gotErr, core.ErrFileLockUnavailable) {
			t.Fatalf("Acquire(closed descriptor) error = %v, want %v", gotErr, core.ErrFileLockUnavailable)
		}
		if _, gotHeldErr := acquisition.Held(); !errors.Is(gotHeldErr, core.ErrPrimitiveContract) {
			t.Fatalf("Acquire(closed descriptor) acquisition.Held() error = %v, want %v", gotHeldErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral immediate contention reports not-held without fake failure", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "lock")
		holder := openLockFile(t, path)
		contender := openLockFile(t, path)
		requireHeld(t, acquireImmediate(t, holder, filelock.Exclusive), true, "holder")
		acquisition, err := filelock.Acquire(t.Context(), filelock.Request{
			File: contender, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate,
		})
		if err != nil {
			t.Fatalf("Acquire(contended immediate) error = %v, want nil typed outcome", err)
		}
		requireHeld(t, acquisition, false, "neutral contender")
	})
}
