package temporal_test

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type detachedContextValueKey struct{}

func TestDetachedContextPreservesValuesWithoutParentLifetime(t *testing.T) {
	t.Parallel()
	for _, state := range [...]string{"live", "cancelled before detach", "cancelled after detach", "expired deadline", "live deadline"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			parent := context.WithValue(t.Context(), detachedContextValueKey{}, fs.ErrPermission)
			parent, cancel := context.WithCancelCause(parent)
			defer cancel(nil)
			switch state {
			case "cancelled before detach":
				cancel(fs.ErrClosed)
			case "expired deadline":
				var stop context.CancelFunc
				parent, stop = context.WithDeadline(parent, time.Time{})
				defer stop()
			case "live deadline":
				var stop context.CancelFunc
				parent, stop = context.WithDeadline(parent, time.Date(9999, time.January, 1, 0, 0, 0, 0, time.UTC))
				defer stop()
			}
			detached, err := temporal.WithoutCancellation(temporal.DetachedContextRequest{Parent: parent})
			if err != nil || detached == nil {
				t.Fatalf("detached context = (%v,%v), want usable context", detached, err)
			}
			if state == "cancelled after detach" {
				cancel(fs.ErrClosed)
			}
			if deadline, ok := detached.Deadline(); ok || !deadline.IsZero() || detached.Done() != nil || detached.Err() != nil || context.Cause(detached) != nil {
				t.Fatalf("detached lifetime = (%v,%t,%v,%v,%v), want absent parent lifetime", deadline, ok, detached.Done(), detached.Err(), context.Cause(detached))
			}
			if got := detached.Value(detachedContextValueKey{}); got != fs.ErrPermission {
				t.Fatalf("detached value = %v, want inherited permission identity", got)
			}
			child, stop := context.WithCancelCause(detached)
			stop(fs.ErrNotExist)
			if !errors.Is(child.Err(), context.Canceled) || !errors.Is(context.Cause(child), fs.ErrNotExist) || detached.Err() != nil {
				t.Fatalf("child cancellation = (%v,%v), want independently owned cause", child.Err(), context.Cause(child))
			}
		})
	}
}

func TestDetachedContextAdmissionDoesNotObserveParentMethods(t *testing.T) {
	t.Parallel()
	defer func() {
		if failure := recover(); failure != nil {
			t.Errorf("detached admission observed a parent method: %v", failure)
		}
	}()
	parent := &temporalParentProbe{Context: t.Context(), panicErr: true, panicDone: true, panicValue: true}
	detached, err := temporal.WithoutCancellation(temporal.DetachedContextRequest{Parent: parent})
	if err != nil || detached == nil {
		t.Fatalf("detaching unobserved parent = (%v,%v), want native admission", detached, err)
	}
	if detached.Err() != nil || detached.Done() != nil || context.Cause(detached) != nil {
		t.Fatal("detached context observed a forbidden parent method")
	}
	absent, err := temporal.WithoutCancellation(temporal.DetachedContextRequest{})
	if absent != nil || !errors.Is(err, core.ErrNilContext) || !errors.Is(err, core.ErrTemporalContract) {
		t.Fatalf("absent parent = (%v,%v), want both native and temporal identities", absent, err)
	}
}
