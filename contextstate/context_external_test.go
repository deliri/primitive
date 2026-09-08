package contextstate_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type contextValueKey uint8
type contextValue uint8

const testContextValueKey contextValueKey = iota
const testContextValue contextValue = iota

type contextErrBehavior uint8

const (
	contextErrReturns contextErrBehavior = iota
	contextErrPanics
	contextErrPanicsNil
)

type contextProbe struct {
	context.Context
	terminal    error
	done        <-chan struct{}
	errBehavior contextErrBehavior
	errCalls    int
}

func (c *contextProbe) Done() <-chan struct{} { return c.done }
func (c *contextProbe) Err() error {
	c.errCalls++
	if c.errBehavior == contextErrPanics {
		panic(core.ErrContextObservation)
	}
	if c.errBehavior == contextErrPanicsNil {
		panic(nil)
	}
	return c.terminal
}

type nilSafeContext struct {
	context.Context
}

func (*nilSafeContext) Err() error { return nil }

type errOnlyContext struct {
	context.Context
	terminal error
}

var _ context.Context = (*errOnlyContext)(nil)

func (*errOnlyContext) Deadline() (time.Time, bool) {
	panic(core.ErrContextObservation)
}

func (*errOnlyContext) Done() <-chan struct{} {
	panic(core.ErrContextObservation)
}

func (c *errOnlyContext) Err() error {
	return c.terminal
}

type hostileCancellationError struct{}

func (hostileCancellationError) Error() string {
	panic(core.ErrContextObservation)
}

func (hostileCancellationError) Is(target error) bool {
	return target == context.Canceled
}

func (hostileCancellationError) Unwrap() error {
	panic(core.ErrContextObservation)
}

type singleWrappedError struct {
	child error
}

func (e singleWrappedError) Error() string { return "" }
func (e singleWrappedError) Unwrap() error { return e.child }

type nilReceiverError struct{}

func (*nilReceiverError) Error() string { return "" }

type panickingIsError struct{}

func (panickingIsError) Error() string { return "" }
func (panickingIsError) Is(error) bool { panic(core.ErrContextObservation) }

type cycleError struct {
	child error
}

func (*cycleError) Error() string   { return "" }
func (e *cycleError) Unwrap() error { return e.child }

type nonComparableTerminalError []byte

func (nonComparableTerminalError) Error() string { return "" }

type contextFixture struct {
	ctx     context.Context
	cleanup context.CancelFunc
	probe   *contextProbe
	err     error
}

// Each row crosses all three public boundaries with a fresh owned fixture.
// Validate returns standard sentinels, Observe admits the active state, and
// ObserveAfterDone must refuse it. No verdict lives in a helper.
func TestContextstateObservationLayerTriadPreservesOnlyExactStandardTerminalFacts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		makeFixture func() contextFixture
		wantErr     error
		wantState   contextstate.State
		wantExact   bool
	}{
		{name: "nil interface is rejected", makeFixture: nilContextFixture, wantErr: core.ErrNilContext},
		{name: "background is usable now", makeFixture: backgroundContextFixture, wantState: contextstate.StateNone},
		{name: "TODO is usable now", makeFixture: todoContextFixture, wantState: contextstate.StateNone},
		{name: "value context is usable now", makeFixture: valueContextFixture, wantState: contextstate.StateNone},
		{name: "active cancellable context is usable now", makeFixture: activeContextFixture, wantState: contextstate.StateNone},
		{name: "future deadline remains usable until Err reports terminal", makeFixture: futureDeadlineContextFixture, wantState: contextstate.StateNone},
		{name: "without cancel masks cancelled parent", makeFixture: detachedContextFixture, wantState: contextstate.StateNone},
		{name: "nil-safe typed nil is undetectable and admitted", makeFixture: nilSafeContextFixture, wantState: contextstate.StateNone},
		{name: "custom active implementation is usable now", makeFixture: activeProbeFixture, wantState: contextstate.StateNone},
		{name: "Err is the sole context method observed at ingress", makeFixture: errOnlyActiveContextFixture, wantState: contextstate.StateNone},
		{name: "standard cancellation returns exact sentinel", makeFixture: cancelledContextFixture, wantState: contextstate.StateCancelled, wantErr: context.Canceled, wantExact: true},
		{name: "cancellation cause returns exact sentinel", makeFixture: cancellationCauseContextFixture, wantState: contextstate.StateCancelled, wantErr: context.Canceled, wantExact: true},
		{name: "a private deadline cause cannot relabel cancellation", makeFixture: cancellationDeadlineCauseFixture, wantState: contextstate.StateCancelled, wantErr: context.Canceled, wantExact: true},
		{name: "expired deadline returns exact sentinel", makeFixture: deadlineContextFixture, wantState: contextstate.StateDeadlineExceeded, wantErr: context.DeadlineExceeded, wantExact: true},
		{name: "wrapped cancellation violates the Context Err contract", makeFixture: wrappedCancellationProbeFixture, wantErr: core.ErrContextObservation},
		{name: "custom cancellation matcher cannot replace the exact sentinel", makeFixture: hostileCancellationProbeFixture, wantErr: core.ErrContextObservation},
		{name: "wrapped deadline violates the Context Err contract", makeFixture: wrappedDeadlineProbeFixture, wantErr: core.ErrContextObservation},
		{name: "joined terminal state violates the Context Err contract", makeFixture: contradictoryProbeFixture, wantErr: core.ErrContextObservation},
		{name: "unrelated terminal state is unobservable", makeFixture: unrelatedProbeFixture, wantErr: core.ErrContextObservation},
		{name: "typed nil terminal error is unobservable", makeFixture: typedNilTerminalProbeFixture, wantErr: core.ErrContextObservation},
		{name: "noncomparable terminal error is rejected without identity traversal", makeFixture: nonComparableTerminalProbeFixture, wantErr: core.ErrContextObservation},
		{name: "panicking Err is contained", makeFixture: panickingErrProbeFixture, wantErr: core.ErrContextObservation},
		{name: "custom identity method is not consulted", makeFixture: panickingIdentityProbeFixture, wantErr: core.ErrContextObservation},
		{name: "cyclic custom error is rejected without traversal", makeFixture: cyclicErrorProbeFixture, wantErr: core.ErrContextObservation},
		{name: "typed nil whose Err panics is contained", makeFixture: nilPanickingContextFixture, wantErr: core.ErrContextObservation},
		{name: "closed Done cannot invent a terminal Err", makeFixture: activeAfterDoneProbeFixture, wantState: contextstate.StateNone},
		{name: "cancelled Err needs no other method", makeFixture: errOnlyCancelledContextFixture, wantState: contextstate.StateCancelled, wantErr: context.Canceled, wantExact: true},
		{name: "expired Err needs no other method", makeFixture: errOnlyDeadlineContextFixture, wantState: contextstate.StateDeadlineExceeded, wantErr: context.DeadlineExceeded, wantExact: true},
		{name: "nil panic is contained", makeFixture: nilPanicProbeFixture, wantErr: core.ErrContextObservation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			t.Run("Validate", func(t *testing.T) {
				t.Parallel()
				fixture := tc.makeFixture()
				if fixture.err != nil {
					t.Fatalf("context fixture failed: %v", fixture.err)
				}
				if fixture.cleanup != nil {
					defer fixture.cleanup()
				}
				got := contextstate.Validate(fixture.ctx)
				if !errors.Is(got, tc.wantErr) || tc.wantExact && got != tc.wantErr {
					t.Fatalf("Validate()=%v (%T); want %v with exact=%t", got, got, tc.wantErr, tc.wantExact)
				}
				if fixture.probe != nil && fixture.probe.errCalls != 1 {
					t.Fatalf("Err calls=%d; want one", fixture.probe.errCalls)
				}
			})
			operations := []struct {
				name      string
				call      func(context.Context) (contextstate.State, error)
				afterDone bool
			}{
				{name: "Observe", call: contextstate.Observe},
				{name: "ObserveAfterDone", call: contextstate.ObserveAfterDone, afterDone: true},
			}
			for _, operation := range operations {
				t.Run(operation.name, func(t *testing.T) {
					t.Parallel()
					fixture := tc.makeFixture()
					if fixture.err != nil {
						t.Fatalf("context fixture failed: %v", fixture.err)
					}
					if fixture.cleanup != nil {
						defer fixture.cleanup()
					}
					wantState, wantErr := tc.wantState, tc.wantErr
					if tc.wantExact {
						wantErr = nil
					}
					if operation.afterDone && wantState == contextstate.StateNone {
						wantState, wantErr = contextstate.State(0), core.ErrContextObservation
					}
					got, err := operation.call(fixture.ctx)
					if got != wantState || !errors.Is(err, wantErr) {
						t.Fatalf("observation=%v, %v; want %v, %v", got, err, wantState, wantErr)
					}
					if fixture.probe != nil && fixture.probe.errCalls != 1 {
						t.Fatalf("Err calls=%d; want one", fixture.probe.errCalls)
					}
				})
			}
		})
	}
}

func nilContextFixture() contextFixture {
	return contextFixture{}
}

func backgroundContextFixture() contextFixture {
	return contextFixture{ctx: context.Background()}
}

func todoContextFixture() contextFixture {
	return contextFixture{ctx: context.TODO()}
}

func valueContextFixture() contextFixture {
	return contextFixture{
		ctx: context.WithValue(
			context.Background(),
			testContextValueKey,
			testContextValue,
		),
	}
}

func activeContextFixture() contextFixture {
	ctx, cancel := context.WithCancel(context.Background())
	return contextFixture{ctx: ctx, cleanup: cancel}
}

func futureDeadlineContextFixture() contextFixture {
	ctx, cancel, err := temporal.WithDeadline(temporal.DeadlineRequest{
		Parent: context.Background(), Deadline: temporal.InstantFromNanoseconds(math.MaxInt64),
	})
	return contextFixture{ctx: ctx, cleanup: cancel, err: err}
}

func detachedContextFixture() contextFixture {
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	return contextFixture{ctx: context.WithoutCancel(parent)}
}

func nilSafeContextFixture() contextFixture {
	var ctx *nilSafeContext
	return contextFixture{ctx: ctx}
}

func activeProbeFixture() contextFixture {
	probe := &contextProbe{Context: context.Background()}
	return contextFixture{ctx: probe, probe: probe}
}

func errOnlyActiveContextFixture() contextFixture {
	return contextFixture{
		// The nil embedded Context makes accidental Value access panic too.
		ctx: &errOnlyContext{},
	}
}

func errOnlyCancelledContextFixture() contextFixture {
	return contextFixture{
		ctx: &errOnlyContext{
			terminal: context.Canceled,
		},
	}
}

func cancelledContextFixture() contextFixture {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return contextFixture{ctx: ctx}
}

func cancellationCauseContextFixture() contextFixture {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(core.ErrPrimitiveContract)
	return contextFixture{ctx: ctx}
}

func deadlineContextFixture() contextFixture {
	ctx, cancel, err := temporal.WithDeadline(temporal.DeadlineRequest{
		Parent: context.Background(), Deadline: temporal.InstantFromNanoseconds(0),
	})
	return contextFixture{ctx: ctx, cleanup: cancel, err: err}
}

func wrappedCancellationProbeFixture() contextFixture {
	return terminalProbeFixture(singleWrappedError{child: context.Canceled})
}

func hostileCancellationProbeFixture() contextFixture {
	return terminalProbeFixture(hostileCancellationError{})
}

func wrappedDeadlineProbeFixture() contextFixture {
	return terminalProbeFixture(singleWrappedError{child: context.DeadlineExceeded})
}

func contradictoryProbeFixture() contextFixture {
	return terminalProbeFixture(
		errors.Join(context.DeadlineExceeded, context.Canceled),
	)
}

func unrelatedProbeFixture() contextFixture {
	return terminalProbeFixture(core.ErrPrimitiveContract)
}

func typedNilTerminalProbeFixture() contextFixture {
	var terminal *nilReceiverError
	return terminalProbeFixture(terminal)
}

func nonComparableTerminalProbeFixture() contextFixture {
	return terminalProbeFixture(nonComparableTerminalError{})
}

func panickingErrProbeFixture() contextFixture {
	probe := &contextProbe{
		Context:     context.Background(),
		errBehavior: contextErrPanics,
	}
	return contextFixture{ctx: probe, probe: probe}
}

func panickingIdentityProbeFixture() contextFixture {
	return terminalProbeFixture(panickingIsError{})
}

func cyclicErrorProbeFixture() contextFixture {
	return terminalProbeFixture(selfCycleCause())
}

func selfCycleCause() error {
	cycle := &cycleError{}
	cycle.child = cycle
	return cycle
}

func activeAfterDoneProbeFixture() contextFixture {
	done := make(chan struct{})
	close(done)
	probe := &contextProbe{Context: context.Background(), done: done}
	return contextFixture{ctx: probe, probe: probe}
}

func nilPanickingContextFixture() contextFixture {
	var ctx *contextProbe
	return contextFixture{ctx: ctx}
}

func terminalProbeFixture(terminal error) contextFixture {
	done := make(chan struct{})
	close(done)
	probe := &contextProbe{
		Context:  context.Background(),
		terminal: terminal,
		done:     done,
	}
	return contextFixture{ctx: probe, probe: probe}
}

func errOnlyDeadlineContextFixture() contextFixture {
	return contextFixture{ctx: &errOnlyContext{terminal: context.DeadlineExceeded}}
}

func nilPanicProbeFixture() contextFixture {
	probe := &contextProbe{errBehavior: contextErrPanicsNil}
	return contextFixture{ctx: probe, probe: probe}
}

func cancellationDeadlineCauseFixture() contextFixture {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(context.DeadlineExceeded)
	return contextFixture{ctx: ctx}
}
