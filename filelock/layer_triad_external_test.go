package filelock_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filelock"
)

func TestAdvisoryExclusionLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exclusive acquisition reports one held lock", func(t *testing.T) {
		t.Parallel()
		file := openLockFile(t, filepath.Join(t.TempDir(), "lock"))
		requireHeld(t, acquireImmediate(t, file, filelock.Exclusive), true, "exclusive holder")
	})

	t.Run("negative cancelled entry performs no lock effect", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "lock")
		refused := openLockFile(t, path)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		acquisition, err := filelock.Acquire(ctx, filelock.Request{
			File: refused, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate,
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Acquire(cancelled) error = %v, want %v", err, context.Canceled)
		}
		if _, heldErr := acquisition.Held(); !errors.Is(heldErr, core.ErrPrimitiveContract) {
			t.Fatalf("cancelled acquisition Held() error = %v, want %v", heldErr, core.ErrPrimitiveContract)
		}
		successor := openLockFile(t, path)
		requireHeld(t, acquireImmediate(t, successor, filelock.Exclusive), true, "successor after cancellation")
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
