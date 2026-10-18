package filestore_test

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestNativeScopesPreserveTerminalIdentityAndCustomCause(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ Deadline, Inside bool }{{}, {Inside: true}, {Deadline: true}} {
		deadline, inside := tc.Deadline, tc.Inside
		name := "cancelled"
		if deadline {
			name = "deadline"
		}
		if inside {
			name += " callback"
		} else {
			name += " acquisition"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			location, _ := inspectedReadLocation(t, []byte("native cancellation"), 0600)
			var ctx context.Context
			var cancel context.CancelFunc
			want := context.Canceled
			if deadline {
				ctx, cancel = context.WithDeadlineCause(t.Context(), time.Unix(1, 0), io.ErrNoProgress)
				want = context.DeadlineExceeded
			} else {
				var causeCancel context.CancelCauseFunc
				ctx, causeCancel = context.WithCancelCause(t.Context())
				cancel = func() { causeCancel(io.ErrNoProgress) }
				if !inside {
					cancel()
				}
			}
			defer cancel()
			// A past deadline is already terminal; callback paths use cancellable
			// contexts so no timer or scheduler race decides whether the loan occurs.

			parent, err := core.ParseAbsolutePath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			root, rootErr := filestore.WithRootScope(ctx, filestore.RootScopeRequest{Directory: parent, Use: func(ctx context.Context, _ *os.Root) error { calls++; cancel(); return ctx.Err() }})
			rootFailure := rootErr
			if inside {
				rootFailure = root.OperationError
			}
			if !errors.Is(rootFailure, want) || !errors.Is(rootFailure, io.ErrNoProgress) {
				t.Fatalf("native root cause=%v,want %v plus actual cause", rootFailure, want)
			}
			if inside {
				if root.Validate() != nil || root.CleanupError != nil || calls != 1 {
					t.Fatalf("completed native root=%+v calls=%d", root, calls)
				}
			} else if root.Validate() == nil || calls != 0 {
				t.Fatalf("refused acquisition invented root=%+v calls=%d", root, calls)
			}
			if inside {
				var causeCancel context.CancelCauseFunc
				ctx, causeCancel = context.WithCancelCause(t.Context())
				cancel = func() { causeCancel(io.ErrNoProgress) }
				defer cancel()
			}
			calls = 0
			read, readErr := filestore.WithInspectedReadScope(ctx, filestore.InspectedReadScopeRequest{Location: location, Use: func(ctx context.Context, _ filestore.InspectedRead) error { calls++; cancel(); return ctx.Err() }})
			readFailure := readErr
			if inside {
				readFailure = read.OperationError()
			}
			if !errors.Is(readFailure, want) || !errors.Is(readFailure, io.ErrNoProgress) {
				t.Fatalf("native read cause=%v,want %v plus actual cause", readFailure, want)
			}
			if inside {
				if read.Validate() != nil || read.CleanupError() != nil || calls != 1 {
					t.Fatalf("completed native read=%+v calls=%d", read, calls)
				}
			} else if read.Validate() == nil || calls != 0 {
				t.Fatalf("refused acquisition invented read=%+v calls=%d", read, calls)
			}
		})
	}
}

type scopeCausePanicContext struct{ context.Context }

func (scopeCausePanicContext) Value(any) any { panic("private context payload") }

func TestNativeScopeCancellationCausePanicKeepsTerminalAndTypedObservation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	parent, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	result, err := filestore.WithRootScope(scopeCausePanicContext{ctx}, filestore.RootScopeRequest{Directory: parent, Use: func(context.Context, *os.Root) error { t.Fatal("cancelled source entered callback"); return nil }})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrContextObservation) || result.Validate() == nil {
		t.Fatalf("contained native cause panic=%+v/%v", result, err)
	}
}
