package filelock_test

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filelock"
	"path/filepath"
	"testing"
)

func FuzzAdvisoryLockOwnershipSemanticClosure(f *testing.F) {
	for _, seed := range []struct {
		exclusivity                    filelock.Exclusivity
		patience                       filelock.Patience
		holder, shared, closed, cancel bool
	}{
		{filelock.Exclusive, filelock.Immediate, false, false, false, false},
		{filelock.Shared, filelock.Immediate, true, true, false, false},
		{filelock.Exclusive, filelock.Immediate, true, false, false, false},
		{filelock.Exclusive, filelock.Blocking, false, false, false, false},
		{filelock.ExclusivityUnknown, filelock.Immediate, false, false, false, false},
		{filelock.Shared, filelock.Immediate, false, false, true, false},
		{filelock.Exclusive, filelock.Immediate, true, false, false, true},
	} {
		f.Add(uint8(seed.exclusivity), uint8(seed.patience), seed.holder, seed.shared, seed.closed, seed.cancel)
	}
	f.Fuzz(func(t *testing.T, exclusivity, patience uint8, withHolder, sharedHolder, closed, cancelled bool) {
		ex, p := filelock.Exclusivity(exclusivity), filelock.Patience(patience)
		valid := (ex == filelock.Exclusive || ex == filelock.Shared) && (p == filelock.Immediate || p == filelock.Blocking)
		path := filepath.Join(t.TempDir(), "fuzz.lock")
		holder, subject, probe := openFixtureLock(t, path), openFixtureLock(t, path), openFixtureLock(t, path)
		// Blocking with an unreleased holder would intentionally park forever.
		// The fuzz fixture puts contenders only on Immediate; blocking flags and
		// actual handoff are proved by deterministic tables.
		holderActive := withHolder && p == filelock.Immediate
		mode := filelock.Exclusive
		if sharedHolder {
			mode = filelock.Shared
		}
		if holderActive {
			holdFixtureLock(t, holder, mode)
		}
		if closed {
			if err := subject.Close(); err != nil {
				t.Fatal(err)
			}
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		if cancelled {
			cancel()
		}
		got, err := filelock.Acquire(ctx, filelock.Request{File: subject, Exclusivity: ex, Patience: p})
		var wantErr error
		switch {
		case cancelled:
			wantErr = context.Canceled
		case !valid:
			wantErr = core.ErrPrimitiveContract
		case closed:
			wantErr = core.ErrFileLockUnavailable
		}
		if !errors.Is(err, wantErr) || (errors.Is(wantErr, core.ErrPrimitiveContract) && errors.Is(err, core.ErrFileLockUnavailable)) {
			t.Fatalf("acquire error=%v, want %v for ex=%d patience=%d", err, wantErr, ex, p)
		}
		if wantErr != nil {
			if got != (filelock.Acquisition{}) {
				t.Fatalf("refused acquisition=%v, want zero", got)
			}
		} else {
			wantHeld := !holderActive || (mode == filelock.Shared && ex == filelock.Shared)
			held, heldErr := got.Held()
			if heldErr != nil || held != wantHeld || got.Validate() != nil {
				t.Fatalf("held=%v error=%v, want %v,nil", held, heldErr, wantHeld)
			}
			if held {
				if err := filelock.Release(ctx, subject); err != nil {
					t.Fatal(err)
				}
			}
		}
		if cancelled || closed {
			releaseErr := filelock.Release(ctx, subject)
			var desired error = core.ErrFileLockUnavailable
			if cancelled {
				desired = context.Canceled
			}
			if !errors.Is(releaseErr, desired) {
				t.Fatalf("release error=%v, want %v", releaseErr, desired)
			}
		}
		if holderActive {
			if err := filelock.Release(t.Context(), holder); err != nil {
				t.Fatal(err)
			}
		}
		// A fabricated refusal or a no-op Release leaves a real native lock behind.
		end, err := filelock.Acquire(t.Context(), filelock.Request{File: probe, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate})
		held, heldErr := end.Held()
		if err != nil || heldErr != nil || !held {
			t.Fatalf("final native probe held=%v errors=%v/%v, want true,nil,nil", held, err, heldErr)
		}
		if err := filelock.Release(t.Context(), probe); err != nil {
			t.Fatal(err)
		}
	})
}
