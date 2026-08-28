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
		err         error
		acquisition filelock.Acquisition
	}
	started := make(chan struct{})
	done := make(chan outcome, 1)
	go func() {
		close(started)
		acquisition, err := filelock.Acquire(context.Background(), filelock.Request{
			File:        waiter,
			Exclusivity: filelock.Exclusive,
			Patience:    filelock.Blocking,
		})
		done <- outcome{acquisition: acquisition, err: err}
	}()
	<-started

	gotReleaseErr := filelock.Release(t.Context(), holder)
	if gotReleaseErr != nil {
		// Closing is the OS-owned abnormal completion path for a blocking lock.
		// The goroutine is still joined below before the test reports failure.
		_ = holder.Close()
	}

	select {
	case got := <-done:
		if gotReleaseErr != nil {
			t.Fatalf("Release() error = %v, want nil", gotReleaseErr)
		}
		if got.err != nil {
			t.Fatalf("blocking Acquire() error = %v, want nil", got.err)
		}
		requireHeld(t, got.acquisition, true, "blocking waiter")
	case <-time.After(lockBackstop):
		t.Fatalf("blocking Acquire did not return within %v of holder release or close", lockBackstop)
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

type contextGateOperation uint8

const (
	contextGateOperationUnknown contextGateOperation = iota
	contextGateAcquire
	contextGateRelease
)

// TestContextIngressRefusesTerminalStateBeforeLockMutation proves both public
// effect doors consult contextstate before touching the descriptor. The exact
// terminal states belong to contextstate; this table proves filelock preserves
// them and produces no lock or unlock beside a refusal.
func TestContextIngressRefusesTerminalStateBeforeLockMutation(t *testing.T) {
	t.Parallel()

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, stopExpired := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer stopExpired()

	cases := []struct {
		name      string
		operation contextGateOperation
		ctx       context.Context
		wantErr   error
	}{
		{name: "acquire refuses nil context before taking a lock", operation: contextGateAcquire, wantErr: core.ErrNilContext},
		{name: "acquire refuses cancelled context before taking a lock", operation: contextGateAcquire, ctx: cancelled, wantErr: context.Canceled},
		{name: "acquire refuses expired context before taking a lock", operation: contextGateAcquire, ctx: expired, wantErr: context.DeadlineExceeded},
		{name: "release refuses nil context without dropping the lock", operation: contextGateRelease, wantErr: core.ErrNilContext},
		{name: "release refuses cancelled context without dropping the lock", operation: contextGateRelease, ctx: cancelled, wantErr: context.Canceled},
		{name: "release refuses expired context without dropping the lock", operation: contextGateRelease, ctx: expired, wantErr: context.DeadlineExceeded},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "context.lock")
			first := openLockFile(t, path)
			second := openLockFile(t, path)

			switch tc.operation {
			case contextGateAcquire:
				got, gotErr := filelock.Acquire(tc.ctx, filelock.Request{
					File: first, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate,
				})
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("Acquire(terminal context) error = %v, want %v", gotErr, tc.wantErr)
				}
				if _, gotHeldErr := got.Held(); !errors.Is(gotHeldErr, core.ErrPrimitiveContract) {
					t.Fatalf("Acquire(terminal context) acquisition.Held() error = %v, want %v", gotHeldErr, core.ErrPrimitiveContract)
				}
				requireHeld(t, acquireImmediate(t, second, filelock.Exclusive), true, "successor after refused acquire")
			case contextGateRelease:
				requireHeld(t, acquireImmediate(t, first, filelock.Exclusive), true, "holder")
				gotErr := filelock.Release(tc.ctx, first)
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("Release(terminal context) error = %v, want %v", gotErr, tc.wantErr)
				}
				requireHeld(t, acquireImmediate(t, second, filelock.Exclusive), false, "contender after refused release")
				if gotReleaseErr := filelock.Release(t.Context(), first); gotReleaseErr != nil {
					t.Fatalf("Release(active context) error = %v, want nil", gotReleaseErr)
				}
				requireHeld(t, acquireImmediate(t, second, filelock.Exclusive), true, "successor after accepted release")
			default:
				t.Fatalf("context gate operation = %d, want admitted operation", tc.operation)
			}
		})
	}
}
