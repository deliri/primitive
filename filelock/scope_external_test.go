package filelock_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filelock"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestLockScopeExcludesContendersAndReleasesEveryConsumerOutcome(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                          string
		failure                       error
		panicConsumer, cancelConsumer bool
	}{
		{name: "successful operation"},
		{name: "refused operation", failure: context.DeadlineExceeded},
		{name: "canceled operation", failure: context.Canceled, cancelConsumer: true},
		{name: "panicking operation", failure: core.ErrFilestoreContract, panicConsumer: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			proveScopeExclusionAndSuccessor(t, tc.failure, tc.panicConsumer, tc.cancelConsumer)
		})
	}
}

func proveScopeExclusionAndSuccessor(t *testing.T, failure error, panicConsumer, cancelConsumer bool) {
	t.Helper()
	directory, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path, err := core.ParseRelativePath("carrier")
	if err != nil {
		t.Fatal(err)
	}
	rootScope, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: directory, Use: func(ctx context.Context, root *os.Root) error {
		ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: ctx})
		if err != nil {
			return err
		}
		defer cancel(nil)
		carrier := filestore.LockFileRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o600}
		calls, contenderCalls := 0, 0
		acquisition, err := filelock.WithScope(ctx, filelock.ScopeRequest{Carrier: carrier, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate, Use: func(ctx context.Context) error {
			calls++
			contender, err := filelock.WithScope(ctx, filelock.ScopeRequest{Carrier: carrier, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate, Use: func(context.Context) error { contenderCalls++; return nil }})
			held, heldErr := contender.Held()
			if err != nil || heldErr != nil || held || contenderCalls != 0 {
				t.Fatalf("contender = held:%t / %v / %v / %d calls, want refused native hold and no operation", held, err, heldErr, contenderCalls)
			}
			if cancelConsumer {
				cancel(context.Canceled)
			}
			if panicConsumer {
				panic(context.DeadlineExceeded)
			}
			return failure
		}})
		held, heldErr := acquisition.Held()
		if !errors.Is(err, failure) || heldErr != nil || !held || calls != 1 {
			t.Fatalf("scope = held:%t / %v / %v / %d calls, want admitted native operation and %v", held, err, heldErr, calls, failure)
		}
		successorCalls := 0
		successor, err := filelock.WithScope(context.WithoutCancel(ctx), filelock.ScopeRequest{Carrier: carrier, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate, Use: func(context.Context) error { successorCalls++; return nil }})
		held, heldErr = successor.Held()
		if err != nil || heldErr != nil || !held || successorCalls != 1 {
			t.Fatalf("successor = held:%t / %v / %v / %d calls, want actual unlocked carrier after cleanup", held, err, heldErr, successorCalls)
		}
		return nil
	}})
	if err := errors.Join(err, rootScope.Validate(), rootScope.OperationError, rootScope.CleanupError); err != nil {
		t.Fatal(err)
	}
}

func FuzzLockScopeExclusionAndCleanup(f *testing.F) {
	f.Add(uint8(0))
	f.Add(uint8(1))
	f.Add(uint8(2))
	f.Add(uint8(3))
	f.Fuzz(func(t *testing.T, disposition uint8) {
		var failure error
		switch disposition % 4 {
		case 1:
			failure = context.DeadlineExceeded
		case 2:
			failure = context.Canceled
		case 3:
			failure = core.ErrFilestoreContract
		}
		proveScopeExclusionAndSuccessor(t, failure, disposition%4 == 3, disposition%4 == 2)
	})
}

func TestLockScopeRefusesInvalidIntentBeforeCreatingCarrier(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                                      string
		missingUse, unknownExclusivity, unknownPatience, canceled bool
		want                                                      error
	}{
		{name: "missing operation", missingUse: true, want: core.ErrPrimitiveContract},
		{name: "unknown exclusivity", unknownExclusivity: true, want: core.ErrPrimitiveContract},
		{name: "unknown patience", unknownPatience: true, want: core.ErrPrimitiveContract},
		{name: "canceled parent", canceled: true, want: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory, err := core.ParseAbsolutePath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			path, err := core.ParseRelativePath("uncreated")
			if err != nil {
				t.Fatal(err)
			}
			rootScope, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: directory, Use: func(ctx context.Context, root *os.Root) error {
				ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: ctx})
				if err != nil {
					return err
				}
				defer cancel(nil)
				calls := 0
				request := filelock.ScopeRequest{Carrier: filestore.LockFileRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o600}, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate, Use: func(context.Context) error { calls++; return nil }}
				if tc.missingUse {
					request.Use = nil
				}
				if tc.unknownExclusivity {
					request.Exclusivity = filelock.ExclusivityUnknown
				}
				if tc.unknownPatience {
					request.Patience = filelock.PatienceUnknown
				}
				if tc.canceled {
					cancel(context.Canceled)
				}
				acquisition, err := filelock.WithScope(ctx, request)
				if !errors.Is(err, tc.want) || acquisition != (filelock.Acquisition{}) || calls != 0 {
					t.Fatalf("invalid scope = (%v, %v, %d calls), want zero observation, %v and no operation", acquisition, err, calls, tc.want)
				}
				absolute, err := directory.Resolve(path.String())
				if err != nil {
					return err
				}
				inspection, err := filestore.Inspect(context.WithoutCancel(ctx), absolute)
				if err != nil {
					return err
				}
				kind, err := inspection.Kind()
				if err != nil || kind != filestore.PathKindAbsent {
					t.Fatalf("invalid scope carrier = (%v, %v), want absent without a creation effect", kind, err)
				}
				return nil
			}})
			if err := errors.Join(err, rootScope.Validate(), rootScope.OperationError, rootScope.CleanupError); err != nil {
				t.Fatal(err)
			}
		})
	}
}
