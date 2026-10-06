package temporal_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestCancellationLifetimeUsesGoFirstCauseAndParentPropagation(t *testing.T) {
	t.Parallel()
	first := errors.New("first consumer refusal")
	second := errors.New("later consumer refusal")
	parent, release, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	defer release(nil)
	child, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: parent})
	if err != nil {
		t.Fatal(err)
	}
	defer cancel(nil)
	if child.Err() != nil || context.Cause(child) != nil {
		t.Fatalf("fresh child = %v/%v, want live", child.Err(), context.Cause(child))
	}
	release(first)
	select {
	case <-child.Done():
	default:
		t.Fatal("parent cancellation did not synchronously propagate")
	}
	cancel(second)
	if !errors.Is(child.Err(), context.Canceled) || !errors.Is(context.Cause(child), first) || errors.Is(context.Cause(child), second) {
		t.Fatalf("child = %v/%v, want canceled with first parent cause", child.Err(), context.Cause(child))
	}
	late, stopLate, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: parent})
	if err != nil {
		t.Fatal(err)
	}
	defer stopLate(nil)
	if !errors.Is(late.Err(), context.Canceled) || !errors.Is(context.Cause(late), first) {
		t.Fatalf("already-canceled parent = %v/%v, want inherited first cause", late.Err(), context.Cause(late))
	}
	deadline, stopDeadline, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: temporal.Duration{}})
	if err != nil {
		t.Fatal(err)
	}
	defer stopDeadline()
	expired, stopExpired, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: deadline})
	if err != nil {
		t.Fatal(err)
	}
	defer stopExpired(nil)
	if !errors.Is(expired.Err(), context.DeadlineExceeded) || !errors.Is(context.Cause(expired), context.DeadlineExceeded) {
		t.Fatalf("expired parent = %v/%v, want inherited deadline identity", expired.Err(), context.Cause(expired))
	}
	sibling, stop, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	defer stop(nil)
	stop(nil)
	if !errors.Is(sibling.Err(), context.Canceled) || !errors.Is(context.Cause(sibling), context.Canceled) {
		t.Fatalf("nil-cause cancellation = %v/%v, want Go cancellation identity", sibling.Err(), context.Cause(sibling))
	}
}

func TestCancellationConstructionRefusesBrokenParents(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		parent context.Context
	}{
		{name: "nil parent"},
		{name: "Err panic", parent: &temporalParentProbe{Context: t.Context(), panicErr: true}},
		{name: "Done panic", parent: &temporalParentProbe{Context: t.Context(), panicDone: true}},
		{name: "Value panic", parent: &temporalParentProbe{Context: t.Context(), panicValue: true}},
		{name: "nonstandard terminal", parent: &temporalParentProbe{Context: t.Context(), terminal: core.ErrContextObservation}},
		{name: "typed nil", parent: (*temporalParentProbe)(nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: tc.parent})
			if cancel != nil {
				cancel(nil)
			}
			if ctx != nil || cancel != nil || !errors.Is(err, core.ErrTemporalContract) {
				t.Fatalf("broken parent = %v/%v, want absent lifetime and temporal refusal", ctx, err)
			}
		})
	}
}
