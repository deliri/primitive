package contextstate_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

type contextValueKey uint8
type contextValue uint8

const testContextValueKey contextValueKey = iota
const testContextValue contextValue = iota
const futureDeadlineYear = 9999
const unknownState contextstate.State = 0

type contextErrBehavior uint8

const (
	contextErrReturns contextErrBehavior = iota
	contextErrPanics
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
}

func TestValidatePublicIngressMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		makeFixture func() contextFixture
		wantErr     error
		name        string
		wantExact   bool
	}{
		{name: "nil interface is rejected", makeFixture: nilContextFixture, wantErr: core.ErrNilContext},
		{name: "background is usable now", makeFixture: backgroundContextFixture},
		{name: "TODO is usable now", makeFixture: todoContextFixture},
		{name: "value context is usable now", makeFixture: valueContextFixture},
		{name: "active cancellable context is usable now", makeFixture: activeContextFixture},
		{name: "future deadline remains usable until Err reports terminal", makeFixture: futureDeadlineContextFixture},
		{name: "without cancel masks cancelled parent", makeFixture: detachedContextFixture},
		{name: "nil-safe typed nil is undetectable and admitted", makeFixture: nilSafeContextFixture},
		{name: "custom active implementation is usable now", makeFixture: activeProbeFixture},
		{name: "Err is the sole context method observed at ingress", makeFixture: errOnlyActiveContextFixture},
		{name: "standard cancellation returns exact sentinel", makeFixture: cancelledContextFixture, wantErr: context.Canceled, wantExact: true},
		{name: "cancellation cause returns exact sentinel", makeFixture: cancellationCauseContextFixture, wantErr: context.Canceled, wantExact: true},
		{name: "expired deadline returns exact sentinel", makeFixture: deadlineContextFixture, wantErr: context.DeadlineExceeded, wantExact: true},
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture := tc.makeFixture()
			if fixture.cleanup != nil {
				defer fixture.cleanup()
			}
			gotErr := contextstate.Validate(fixture.ctx)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantExact && gotErr != tc.wantErr {
				t.Fatalf(
					"Validate() error identity = %v (%T at %p), "+
						"want the exact sentinel %v (%T at %p)",
					gotErr,
					gotErr,
					gotErr,
					tc.wantErr,
					tc.wantErr,
					tc.wantErr,
				)
			}
			if fixture.probe != nil && fixture.probe.errCalls != 1 {
				t.Fatalf(
					"Validate() Context.Err() calls = %d, want 1",
					fixture.probe.errCalls,
				)
			}
		})
	}
}

func TestContextstateProcessBoundaryLayerTriadObserveAfterDoneTerminalMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		makeFixture func() contextFixture
		wantErr     error
		name        string
		wantState   contextstate.State
	}{
		{name: "nil interface is rejected", makeFixture: nilContextFixture, wantState: unknownState, wantErr: core.ErrNilContext},
		{name: "standard cancellation is observed", makeFixture: cancelledContextFixture, wantState: contextstate.StateCancelled},
		{name: "cancellation cause is observed", makeFixture: cancellationCauseContextFixture, wantState: contextstate.StateCancelled},
		{name: "expired deadline is observed", makeFixture: deadlineContextFixture, wantState: contextstate.StateDeadlineExceeded},
		{name: "post-Done observation reads Err without rereading Done", makeFixture: errOnlyCancelledContextFixture, wantState: contextstate.StateCancelled},
		{name: "wrapped cancellation violates the Context Err contract", makeFixture: wrappedCancellationProbeFixture, wantState: unknownState, wantErr: core.ErrContextObservation},
		{name: "custom cancellation matcher cannot replace the exact sentinel", makeFixture: hostileCancellationProbeFixture, wantState: unknownState, wantErr: core.ErrContextObservation},
		{name: "wrapped deadline violates the Context Err contract", makeFixture: wrappedDeadlineProbeFixture, wantState: unknownState, wantErr: core.ErrContextObservation},
		{name: "joined terminal state violates the Context Err contract", makeFixture: contradictoryProbeFixture, wantState: unknownState, wantErr: core.ErrContextObservation},
		{name: "closed Done with active state is unobservable", makeFixture: activeAfterDoneProbeFixture, wantState: unknownState, wantErr: core.ErrContextObservation},
		{name: "background used after Done is unobservable", makeFixture: backgroundContextFixture, wantState: unknownState, wantErr: core.ErrContextObservation},
		{name: "safe typed nil used after Done is unobservable", makeFixture: nilSafeContextFixture, wantState: unknownState, wantErr: core.ErrContextObservation},
		{name: "unrelated terminal state is unobservable", makeFixture: unrelatedProbeFixture, wantState: unknownState, wantErr: core.ErrContextObservation},
		{name: "typed nil terminal error is unobservable", makeFixture: typedNilTerminalProbeFixture, wantState: unknownState, wantErr: core.ErrContextObservation},
		{name: "noncomparable terminal error is rejected without identity traversal", makeFixture: nonComparableTerminalProbeFixture, wantState: unknownState, wantErr: core.ErrContextObservation},
		{name: "panicking Err is contained", makeFixture: panickingErrProbeFixture, wantState: unknownState, wantErr: core.ErrContextObservation},
		{name: "custom Is method is not consulted", makeFixture: panickingIdentityProbeFixture, wantState: unknownState, wantErr: core.ErrContextObservation},
		{name: "cyclic custom error is rejected without traversal", makeFixture: cyclicErrorProbeFixture, wantState: unknownState, wantErr: core.ErrContextObservation},
		{name: "typed nil whose Err panics is contained", makeFixture: nilPanickingContextFixture, wantState: unknownState, wantErr: core.ErrContextObservation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture := tc.makeFixture()
			if fixture.cleanup != nil {
				defer fixture.cleanup()
			}
			gotState, gotErr := contextstate.ObserveAfterDone(fixture.ctx)
			if gotState != tc.wantState {
				t.Fatalf("ObserveAfterDone() state = %v, want %v", gotState, tc.wantState)
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"ObserveAfterDone() error = %v, want %v",
					gotErr,
					tc.wantErr,
				)
			}
			if fixture.probe != nil && fixture.probe.errCalls != 1 {
				t.Fatalf(
					"ObserveAfterDone() Context.Err() calls = %d, want 1",
					fixture.probe.errCalls,
				)
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
	deadline := time.Date(
		futureDeadlineYear,
		time.December,
		31,
		0,
		0,
		0,
		0,
		time.UTC,
	)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	return contextFixture{ctx: ctx, cleanup: cancel}
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
		ctx: &errOnlyContext{Context: context.Background()},
	}
}

func errOnlyCancelledContextFixture() contextFixture {
	return contextFixture{
		ctx: &errOnlyContext{
			Context:  context.Background(),
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
	ctx, cancel := context.WithDeadline(context.Background(), time.Time{})
	return contextFixture{ctx: ctx, cleanup: cancel}
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
