package temporal_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// These probes deliberately violate the Go Context contract at the methods
// consumed by its constructors. They contain no timer or cancellation model.
type temporalParentProbe struct {
	context.Context
	terminal                                       error
	panicErr, panicDeadline, panicDone, panicValue bool
}

func (p *temporalParentProbe) Err() error {
	if p.panicErr {
		panic(core.ErrContextObservation)
	}
	return p.terminal
}
func (p *temporalParentProbe) Deadline() (time.Time, bool) {
	if p.panicDeadline {
		panic(core.ErrContextObservation)
	}
	return p.Context.Deadline()
}
func (p *temporalParentProbe) Done() <-chan struct{} {
	if p.panicDone {
		panic(core.ErrContextObservation)
	}
	return p.Context.Done()
}

// witness:waiver doctrine/code_form/return_interface -- implements the exact context.Context.Value signature to probe Go cancellation propagation.
func (p *temporalParentProbe) Value(key any) any {
	if p.panicValue {
		panic(core.ErrContextObservation)
	}
	return p.Context.Value(key)
}

func TestContextConstructionContainsBrokenParentContracts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		probe    temporalParentProbe
		typedNil bool
	}{
		{name: "Err panics before construction", probe: temporalParentProbe{panicErr: true}},
		{name: "Deadline panics inside Go constructor", probe: temporalParentProbe{panicDeadline: true}},
		{name: "Done panics inside Go propagation", probe: temporalParentProbe{panicDone: true}},
		{name: "Value panics while Go discovers parent cancellation", probe: temporalParentProbe{panicValue: true}},
		{name: "nonstandard Err cannot masquerade as cancellation", probe: temporalParentProbe{terminal: errors.Join(context.Canceled, core.ErrTemporalContract)}},
		{name: "typed nil parent whose Err panics is refused", typedNil: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parent, stop := context.WithCancel(t.Context())
			defer stop()
			probe := tc.probe
			probe.Context = parent
			var supplied context.Context = &probe
			if tc.typedNil {
				supplied = (*temporalParentProbe)(nil)
			}
			duration, err := temporal.DurationFromHours(1)
			if err != nil {
				t.Fatal(err)
			}
			for _, call := range []struct {
				name      string
				construct func() (context.Context, context.CancelFunc, error)
			}{
				{name: "timeout", construct: func() (context.Context, context.CancelFunc, error) {
					return temporal.WithTimeout(temporal.TimeoutRequest{Parent: supplied, Duration: duration})
				}},
				{name: "deadline", construct: func() (context.Context, context.CancelFunc, error) {
					return temporal.WithDeadline(temporal.DeadlineRequest{Parent: supplied, Deadline: temporal.InstantFromNanoseconds(0)})
				}},
			} {
				got, cancel, gotErr := call.construct()
				if cancel != nil {
					cancel()
				}
				if got != nil || cancel != nil || !errors.Is(gotErr, core.ErrTemporalContract) || !errors.Is(gotErr, core.ErrContextObservation) {
					t.Fatalf("%s broken parent = (%v,%v,%v), want no capabilities and typed observation refusal", call.name, got, cancel, gotErr)
				}
			}
		})
	}
}
