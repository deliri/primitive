package filelock_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filelock"
)

// lockBackstop is a deadlock backstop, not a performance assertion. A blocking
// acquisition that is going to succeed does so as soon as the holder releases.
const lockBackstop = 10 * time.Second

// openLockFile opens one lock file. Each open is a separate open file
// description, which is what advisory locks are attached to, so two of them
// contend exactly as two processes would.
func openLockFile(t *testing.T, path string) *os.File {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("OpenFile(%s) error = %v, want nil", path, err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func requireHeld(t *testing.T, acquisition filelock.Acquisition, want bool, label string) {
	t.Helper()
	got, err := acquisition.Held()
	if err != nil {
		t.Fatalf("%s Held() error = %v, want nil", label, err)
	}
	if got != want {
		t.Fatalf("%s Held() = %t, want %t", label, got, want)
	}
}

func acquireImmediate(t *testing.T, file *os.File, exclusivity filelock.Exclusivity) filelock.Acquisition {
	t.Helper()
	acquisition, err := filelock.Acquire(t.Context(), filelock.Request{
		File:        file,
		Exclusivity: exclusivity,
		Patience:    filelock.Immediate,
	})
	if err != nil {
		t.Fatalf("Acquire(%v, immediate) error = %v, want nil", exclusivity, err)
	}
	return acquisition
}

// TestExclusionMatrixAdmitsExactlyTheCompatibleCombinations exhausts the closed
// product of what one holder can be and what a second caller can ask for. The
// whole point of the package is that a second process is told the truth, so
// every cell is checked rather than sampled.
func TestExclusionMatrixAdmitsExactlyTheCompatibleCombinations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		holder   filelock.Exclusivity
		second   filelock.Exclusivity
		wantHeld bool
	}{
		{name: "exclusive holder excludes a second exclusive caller", holder: filelock.Exclusive, second: filelock.Exclusive},
		{name: "exclusive holder excludes a shared caller", holder: filelock.Exclusive, second: filelock.Shared},
		{name: "shared holder excludes an exclusive caller", holder: filelock.Shared, second: filelock.Exclusive},
		{name: "shared holder admits another shared caller", holder: filelock.Shared, second: filelock.Shared, wantHeld: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "lock")
			first := openLockFile(t, path)
			requireHeld(t, acquireImmediate(t, first, tc.holder), true, "holder")

			second := openLockFile(t, path)
			requireHeld(t, acquireImmediate(t, second, tc.second), tc.wantHeld, "second caller")
		})
	}
}

// TestReleaseHandsTheLockToTheNextCaller proves the release path is real. A
// package that acquired correctly but never released would pass every
// contention test above and still wedge a product after its first run.
func TestReleaseHandsTheLockToTheNextCaller(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "lock")
	first := openLockFile(t, path)
	second := openLockFile(t, path)

	requireHeld(t, acquireImmediate(t, first, filelock.Exclusive), true, "first")
	requireHeld(t, acquireImmediate(t, second, filelock.Exclusive), false, "second while held")

	if err := filelock.Release(t.Context(), first); err != nil {
		t.Fatalf("Release() error = %v, want nil", err)
	}
	requireHeld(t, acquireImmediate(t, second, filelock.Exclusive), true, "second after release")
}

// TestClosingTheDescriptorReleasesTheLock pins the property that makes advisory
// locking safe against a crash: the operating system drops the lock when the
// descriptor goes away, so a process that dies without cleanup does not leave
// the directory permanently locked.
func TestClosingTheDescriptorReleasesTheLock(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "lock")
	holder, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("OpenFile() error = %v, want nil", err)
	}
	requireHeld(t, acquireImmediate(t, holder, filelock.Exclusive), true, "holder")

	successor := openLockFile(t, path)
	requireHeld(t, acquireImmediate(t, successor, filelock.Exclusive), false, "successor while held")

	if err := holder.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	requireHeld(t, acquireImmediate(t, successor, filelock.Exclusive), true, "successor after the holder closed")
}

// TestBlockingAcquisitionWaitsForTheHolderRatherThanRefusing proves Blocking
// and Immediate are genuinely different. A blocking caller must not come back
// with held=false, because it has no refusal to report.
func TestBlockingAcquisitionWaitsForTheHolderRatherThanRefusing(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "lock")
	holder := openLockFile(t, path)
	waiter := openLockFile(t, path)
	requireHeld(t, acquireImmediate(t, holder, filelock.Exclusive), true, "holder")

	type outcome struct {
		acquisition filelock.Acquisition
		err         error
	}
	done := make(chan outcome, 1)
	go func() {
		acquisition, err := filelock.Acquire(context.Background(), filelock.Request{
			File:        waiter,
			Exclusivity: filelock.Exclusive,
			Patience:    filelock.Blocking,
		})
		done <- outcome{acquisition: acquisition, err: err}
	}()

	// The waiter must still be blocked. Releasing is the only thing that can
	// let it through, so its completion is evidence of waiting rather than of
	// timing.
	select {
	case got := <-done:
		t.Fatalf("blocking Acquire returned %v/%v before the holder released", got.acquisition, got.err)
	case <-time.After(50 * time.Millisecond):
	}

	if err := filelock.Release(t.Context(), holder); err != nil {
		t.Fatalf("Release() error = %v, want nil", err)
	}

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("blocking Acquire() error = %v, want nil", got.err)
		}
		requireHeld(t, got.acquisition, true, "blocking waiter")
	case <-time.After(lockBackstop):
		t.Fatalf("blocking Acquire did not return within %v of the holder releasing", lockBackstop)
	}
}

// TestAcquireRefusesAnUnusableRequestBeforeTouchingTheFile keeps the contract
// gate ahead of the effect and proves every closed domain is closed.
func TestAcquireRefusesAnUnusableRequestBeforeTouchingTheFile(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		withFile    bool
		exclusivity filelock.Exclusivity
		patience    filelock.Patience
	}{
		{name: "missing file", exclusivity: filelock.Exclusive, patience: filelock.Immediate},
		{name: "unset exclusivity", withFile: true, patience: filelock.Immediate},
		{name: "unset patience", withFile: true, exclusivity: filelock.Exclusive},
		{name: "exclusivity above the closed domain", withFile: true, exclusivity: filelock.Exclusivity(200), patience: filelock.Immediate},
		{name: "patience above the closed domain", withFile: true, exclusivity: filelock.Exclusive, patience: filelock.Patience(200)},
		{name: "both intents unset", withFile: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := filelock.Request{Exclusivity: tc.exclusivity, Patience: tc.patience}
			if tc.withFile {
				request.File = openLockFile(t, filepath.Join(t.TempDir(), "lock"))
			}
			acquisition, err := filelock.Acquire(t.Context(), request)
			if !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("Acquire() error = %v, want errors.Is %v", err, core.ErrPrimitiveContract)
			}
			if _, heldErr := acquisition.Held(); !errors.Is(heldErr, core.ErrPrimitiveContract) {
				t.Fatalf("Held() on a refused acquisition error = %v, want errors.Is %v",
					heldErr, core.ErrPrimitiveContract)
			}
		})
	}
}

// TestAcquisitionNobodyProducedCannotClaimAHold proves the sealed outcome. A
// caller that assembled its own Acquisition never attempted anything, so it
// must not be able to report holding a lock it does not hold.
func TestAcquisitionNobodyProducedCannotClaimAHold(t *testing.T) {
	t.Parallel()

	if _, err := (filelock.Acquisition{}).Held(); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("Held(zero acquisition) error = %v, want errors.Is %v", err, core.ErrPrimitiveContract)
	}
}

// TestReleaseRefusesAMissingFile keeps the release path's gate symmetric with
// the acquisition path's.
func TestReleaseRefusesAMissingFile(t *testing.T) {
	t.Parallel()

	if err := filelock.Release(t.Context(), nil); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("Release(nil file) error = %v, want errors.Is %v", err, core.ErrPrimitiveContract)
	}
}

// TestAcquireRefusesACancelledContextBeforeAnyEffect proves the context is a
// real gate on entry, which is the only place it can be one: no cancellation
// reaches a process already parked in a blocking lock call.
func TestAcquireRefusesACancelledContextBeforeAnyEffect(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "lock")
	file := openLockFile(t, path)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := filelock.Acquire(ctx, filelock.Request{
		File:        file,
		Exclusivity: filelock.Exclusive,
		Patience:    filelock.Immediate,
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Acquire(cancelled) error = %v, want errors.Is %v", err, context.Canceled)
	}

	// Nothing was locked, so a second caller still gets the lock.
	other := openLockFile(t, path)
	requireHeld(t, acquireImmediate(t, other, filelock.Exclusive), true, "caller after a refused acquisition")
}
