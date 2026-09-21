package filelock_test

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filelock"
	"github.com/deliri/primitive/v2026/temporal"
	"os"
	"path/filepath"
	"testing"
)

type lockTestOutcome struct {
	err         error
	acquisition filelock.Acquisition
}

func holdFixtureLock(t testing.TB, file *os.File, exclusivity filelock.Exclusivity) {
	t.Helper()
	got, err := filelock.Acquire(context.Background(), filelock.Request{File: file, Exclusivity: exclusivity, Patience: filelock.Immediate})
	if err != nil {
		t.Fatal(err)
	}
	held, err := got.Held()
	if err != nil || !held {
		t.Fatalf("fixture held=%v error=%v, want true,nil", held, err)
	}
}
func TestExclusionAndReleaseLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		holder, second filelock.Exclusivity
		close          bool
		wantHeld       bool
	}{
		{name: "exclusive_excludes_exclusive", holder: filelock.Exclusive, second: filelock.Exclusive, close: false, wantHeld: false},
		{name: "exclusive_excludes_shared", holder: filelock.Exclusive, second: filelock.Shared, close: false, wantHeld: false},
		{name: "shared_excludes_exclusive", holder: filelock.Shared, second: filelock.Exclusive, close: false, wantHeld: false},
		{name: "shared_admits_shared", holder: filelock.Shared, second: filelock.Shared, close: false, wantHeld: true},
		{name: "close_releases_exclusive", holder: filelock.Exclusive, second: filelock.Exclusive, close: true, wantHeld: false},
		{name: "close_releases_shared", holder: filelock.Shared, second: filelock.Exclusive, close: true, wantHeld: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "lock")
			holder, second := openFixtureLock(t, path), openFixtureLock(t, path)
			holdFixtureLock(t, holder, tc.holder)
			request := filelock.Request{File: second, Exclusivity: tc.second, Patience: filelock.Immediate}
			acquired, err := filelock.Acquire(t.Context(), request)
			held, heldErr := acquired.Held()
			if err != nil || heldErr != nil || held != tc.wantHeld || acquired.Validate() != nil {
				t.Fatalf("held=%v errors=%v/%v, want %v,nil,nil", held, err, heldErr, tc.wantHeld)
			}
			if held {
				if err := filelock.Release(t.Context(), second); err != nil {
					t.Fatal(err)
				}
			}
			if tc.close {
				err = holder.Close()
			} else {
				err = filelock.Release(t.Context(), holder)
			}
			if err != nil {
				t.Fatal(err)
			}
			acquired, err = filelock.Acquire(t.Context(), request)
			held, heldErr = acquired.Held()
			if err != nil || heldErr != nil || !held {
				t.Fatalf("successor held=%v errors=%v/%v, want true,nil,nil after release", held, err, heldErr)
			}
			if err := filelock.Release(t.Context(), second); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestBlockingAcquisitionHandoffLayerTriad(t *testing.T) {
	t.Parallel()
	// Exact Blocking native flags are independently checked in platform tests.
	// This table proves handoff and failure; a scheduling notification alone
	// does not prove that the waiter has entered the kernel.
	for _, tc := range []struct {
		name           string
		holder, waiter filelock.Exclusivity
		close          bool
	}{
		{name: "exclusive_to_exclusive", holder: filelock.Exclusive, waiter: filelock.Exclusive, close: false},
		{name: "shared_to_exclusive", holder: filelock.Shared, waiter: filelock.Exclusive, close: false},
		{name: "exclusive_to_shared_on_close", holder: filelock.Exclusive, waiter: filelock.Shared, close: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "lock")
			holder, waiter := openFixtureLock(t, path), openFixtureLock(t, path)
			holdFixtureLock(t, holder, tc.holder)
			budget, err := temporal.DurationFromSeconds(10)
			if err != nil {
				t.Fatal(err)
			}
			watchdog, stop, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: budget})
			if err != nil {
				t.Fatal(err)
			}
			defer stop()
			started := make(chan struct{})
			done := make(chan lockTestOutcome, 1)
			go func() {
				close(started)
				got, err := filelock.Acquire(context.Background(), filelock.Request{File: waiter, Exclusivity: tc.waiter, Patience: filelock.Blocking})
				done <- lockTestOutcome{acquisition: got, err: err}
			}()
			<-started
			if tc.close {
				err = holder.Close()
			} else {
				err = filelock.Release(t.Context(), holder)
			}
			if err != nil {
				err = errors.Join(err, holder.Close())
			}
			var got lockTestOutcome
			select {
			case got = <-done:
			case <-watchdog.Done():
				t.Fatal("blocking acquisition joined=false, want true after holder exit")
			}
			held, heldErr := got.acquisition.Held()
			if err != nil || got.err != nil || heldErr != nil || !held {
				t.Fatalf("held=%v errors=%v/%v/%v, want true and nil", held, err, got.err, heldErr)
			}
			if err := filelock.Release(t.Context(), waiter); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestRefusedRequestDoesNotChangeNativeOwnership(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		exclusivity filelock.Exclusivity
		patience    filelock.Patience
		missing     bool
	}{
		{name: "absent_file", exclusivity: filelock.Exclusive, patience: filelock.Immediate, missing: true},
		{name: "zero_exclusivity", exclusivity: filelock.ExclusivityUnknown, patience: filelock.Immediate, missing: false},
		{name: "zero_patience", exclusivity: filelock.Exclusive, patience: filelock.PatienceUnknown, missing: false},
		{name: "future_exclusivity", exclusivity: filelock.Exclusivity(3), patience: filelock.Immediate, missing: false},
		{name: "future_patience", exclusivity: filelock.Exclusive, patience: filelock.Patience(3), missing: false},
		{name: "all_bits_exclusivity", exclusivity: filelock.Exclusivity(255), patience: filelock.Immediate, missing: false},
		{name: "all_bits_patience", exclusivity: filelock.Exclusive, patience: filelock.Patience(255), missing: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "refused.lock")
			original, contender := openFixtureLock(t, path), openFixtureLock(t, path)
			holdFixtureLock(t, original, filelock.Shared)
			file := original
			if tc.missing {
				file = nil
			}
			got, err := filelock.Acquire(t.Context(), filelock.Request{File: file, Exclusivity: tc.exclusivity, Patience: tc.patience})
			if !errors.Is(err, core.ErrPrimitiveContract) || errors.Is(err, core.ErrFileLockUnavailable) || got != (filelock.Acquisition{}) {
				t.Fatalf("outcome=%v error=%v, want zero contract refusal", got, err)
			}
			// An invalid exclusive request must not upgrade or release the existing
			// shared lock: another shared holder must fit and an exclusive must not.
			shared, err := filelock.Acquire(t.Context(), filelock.Request{File: contender, Exclusivity: filelock.Shared, Patience: filelock.Immediate})
			held, heldErr := shared.Held()
			if err != nil || heldErr != nil || !held {
				t.Fatalf("shared probe held=%v errors=%v/%v, want unchanged shared ownership", held, err, heldErr)
			}
			if err := filelock.Release(t.Context(), contender); err != nil {
				t.Fatal(err)
			}
			exclusive, err := filelock.Acquire(t.Context(), filelock.Request{File: contender, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate})
			held, heldErr = exclusive.Held()
			if err != nil || heldErr != nil || held {
				t.Fatalf("exclusive probe held=%v errors=%v/%v, want contention with unchanged holder", held, err, heldErr)
			}
		})
	}
}

type filelockBrokenContext struct{ context.Context }

func expiredLockContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel, err := temporal.WithDeadline(temporal.DeadlineRequest{Parent: t.Context(), Deadline: temporal.InstantFromNanoseconds(0)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cancel)
	return ctx
}
func TestContextIngressPreservesLockStateLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr error
		context func(*testing.T) context.Context
		name    string
	}{
		{name: "active", context: func(t *testing.T) context.Context { return t.Context() }, wantErr: nil},
		{name: "nil", context: func(*testing.T) context.Context { return nil }, wantErr: core.ErrNilContext},
		{name: "typed_nil", context: func(*testing.T) context.Context { return (*filelockBrokenContext)(nil) }, wantErr: core.ErrContextObservation},
		{name: "cancelled", context: func(t *testing.T) context.Context {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			return ctx
		}, wantErr: context.Canceled},
		{name: "expired", context: expiredLockContext, wantErr: context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "context.lock")
			file, contender := openFixtureLock(t, path), openFixtureLock(t, path)
			ctx := tc.context(t)
			got, err := filelock.Acquire(ctx, filelock.Request{File: file, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("acquire error=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && got != (filelock.Acquisition{}) {
				t.Fatalf("refused outcome=%v, want zero", got)
			}
			probe, err := filelock.Acquire(t.Context(), filelock.Request{File: contender, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate})
			held, heldErr := probe.Held()
			if err != nil || heldErr != nil || held != (tc.wantErr != nil) {
				t.Fatalf("probe held=%v errors=%v/%v, want held=%v", held, err, heldErr, tc.wantErr != nil)
			}
			if held {
				if err := filelock.Release(t.Context(), contender); err != nil {
					t.Fatal(err)
				}
				holdFixtureLock(t, file, filelock.Exclusive)
			}
			if err := filelock.Release(ctx, file); !errors.Is(err, tc.wantErr) {
				t.Fatalf("release error=%v, want %v", err, tc.wantErr)
			}
			probe, err = filelock.Acquire(t.Context(), filelock.Request{File: contender, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate})
			held, heldErr = probe.Held()
			if err != nil || heldErr != nil || held != (tc.wantErr == nil) {
				t.Fatalf("after release held=%v errors=%v/%v, want held=%v", held, err, heldErr, tc.wantErr == nil)
			}
		})
	}
}
func TestAbsentCapabilityCannotClaimEffect(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		check func() error
		name  string
	}{
		{name: "zero_acquisition_validation", check: (filelock.Acquisition{}).Validate},
		{name: "zero_acquisition_disclosure", check: func() error {
			held, err := (filelock.Acquisition{}).Held()
			if held {
				return nil
			}
			return err
		}},
		{name: "nil_release", check: func() error { return filelock.Release(t.Context(), nil) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.check(); !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("error=%v, want primitive contract refusal", err)
			}
		})
	}
}
