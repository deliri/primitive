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
		{name: "wrapped cancellation is normalized to exact sentinel", makeFixture: wrappedCancellationProbeFixture, wantErr: context.Canceled, wantExact: true},
		{name: "hostile custom cancellation match is normalized without escape", makeFixture: hostileCancellationProbeFixture, wantErr: context.Canceled, wantExact: true},
		{name: "wrapped deadline is normalized to exact sentinel", makeFixture: wrappedDeadlineProbeFixture, wantErr: context.DeadlineExceeded, wantExact: true},
		{name: "contradictory terminal state gives cancellation precedence", makeFixture: contradictoryProbeFixture, wantErr: context.Canceled, wantExact: true},
		{name: "unrelated terminal state is unobservable", makeFixture: unrelatedProbeFixture, wantErr: core.ErrContextObservation},
		{name: "typed nil terminal error is unobservable", makeFixture: typedNilTerminalProbeFixture, wantErr: core.ErrContextObservation},
		{name: "panicking Err is contained", makeFixture: panickingErrProbeFixture, wantErr: core.ErrContextObservation},
		{name: "panicking identity is contained", makeFixture: panickingIdentityProbeFixture, wantErr: core.ErrContextObservation},
		{name: "cyclic terminal graph is bounded", makeFixture: cyclicErrorProbeFixture, wantErr: core.ErrContextObservation},
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

func TestObserveAfterDonePublicTerminalMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		makeFixture func() contextFixture
		wantErr     error
		name        string
		wantState   contextstate.State
	}{
		{name: "nil interface is rejected", makeFixture: nilContextFixture, wantState: contextstate.StateUnknown, wantErr: core.ErrNilContext},
		{name: "standard cancellation is observed", makeFixture: cancelledContextFixture, wantState: contextstate.StateCancelled},
		{name: "cancellation cause is observed", makeFixture: cancellationCauseContextFixture, wantState: contextstate.StateCancelled},
		{name: "expired deadline is observed", makeFixture: deadlineContextFixture, wantState: contextstate.StateDeadlineExceeded},
		{name: "post-Done observation reads Err without rereading Done", makeFixture: errOnlyCancelledContextFixture, wantState: contextstate.StateCancelled},
		{name: "wrapped cancellation is normalized", makeFixture: wrappedCancellationProbeFixture, wantState: contextstate.StateCancelled},
		{name: "hostile custom cancellation match is normalized without escape", makeFixture: hostileCancellationProbeFixture, wantState: contextstate.StateCancelled},
		{name: "wrapped deadline is normalized", makeFixture: wrappedDeadlineProbeFixture, wantState: contextstate.StateDeadlineExceeded},
		{name: "contradictory terminal state gives cancellation precedence", makeFixture: contradictoryProbeFixture, wantState: contextstate.StateCancelled},
		{name: "closed Done with active state is unobservable", makeFixture: activeAfterDoneProbeFixture, wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "background used after Done is unobservable", makeFixture: backgroundContextFixture, wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "safe typed nil used after Done is unobservable", makeFixture: nilSafeContextFixture, wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "unrelated terminal state is unobservable", makeFixture: unrelatedProbeFixture, wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "typed nil terminal error is unobservable", makeFixture: typedNilTerminalProbeFixture, wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "panicking Err is contained", makeFixture: panickingErrProbeFixture, wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "panicking identity is contained", makeFixture: panickingIdentityProbeFixture, wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "cyclic terminal graph is bounded", makeFixture: cyclicErrorProbeFixture, wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "typed nil whose Err panics is contained", makeFixture: nilPanickingContextFixture, wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
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
