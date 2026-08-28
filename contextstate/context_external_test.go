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

type observationOperation func(context.Context) (contextstate.State, error)

type observationCase struct {
	makeFixture contextFixtureFactory
	name        string
	operation   observationOperation
	wantErr     error
	wantState   contextstate.State
}

type contextFixtureFactory func() contextFixture

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

func TestContextstateObservationLayerTriadPreservesOnlyExactStandardTerminalFacts(t *testing.T) {
	t.Parallel()

	t.Run("positive exact standard terminal facts cross both public observation doors", func(t *testing.T) {
		t.Parallel()

		cases := []observationCase{
			{name: "Observe admits exact cancellation", makeFixture: cancelledContextFixture, operation: contextstate.Observe, wantState: contextstate.StateCancelled},
			{name: "Observe admits cancellation whose cause remains owner-private", makeFixture: cancellationCauseContextFixture, operation: contextstate.Observe, wantState: contextstate.StateCancelled},
			{name: "Observe admits exact deadline expiration", makeFixture: deadlineContextFixture, operation: contextstate.Observe, wantState: contextstate.StateDeadlineExceeded},
			{name: "ObserveAfterDone admits exact cancellation", makeFixture: cancelledContextFixture, operation: contextstate.ObserveAfterDone, wantState: contextstate.StateCancelled},
			{name: "ObserveAfterDone admits cancellation whose cause remains owner-private", makeFixture: cancellationCauseContextFixture, operation: contextstate.ObserveAfterDone, wantState: contextstate.StateCancelled},
			{name: "ObserveAfterDone admits exact deadline expiration", makeFixture: deadlineContextFixture, operation: contextstate.ObserveAfterDone, wantState: contextstate.StateDeadlineExceeded},
			{name: "ObserveAfterDone reads Err without rereading Done", makeFixture: errOnlyCancelledContextFixture, operation: contextstate.ObserveAfterDone, wantState: contextstate.StateCancelled},
		}
		runObservationCases(t, cases)
	})

	t.Run("negative malformed context implementations cannot forge a terminal fact", func(t *testing.T) {
		t.Parallel()

		cases := []observationCase{
			{name: "Observe rejects a nil interface", makeFixture: nilContextFixture, operation: contextstate.Observe, wantErr: core.ErrNilContext},
			{name: "ObserveAfterDone rejects a nil interface", makeFixture: nilContextFixture, operation: contextstate.ObserveAfterDone, wantErr: core.ErrNilContext},
			{name: "wrapped cancellation is not the exact Context sentinel", makeFixture: wrappedCancellationProbeFixture, operation: contextstate.Observe, wantErr: core.ErrContextObservation},
			{name: "custom cancellation matching cannot replace sentinel identity", makeFixture: hostileCancellationProbeFixture, operation: contextstate.ObserveAfterDone, wantErr: core.ErrContextObservation},
			{name: "wrapped deadline is not the exact Context sentinel", makeFixture: wrappedDeadlineProbeFixture, operation: contextstate.Observe, wantErr: core.ErrContextObservation},
			{name: "joined terminal states are contradictory", makeFixture: contradictoryProbeFixture, operation: contextstate.ObserveAfterDone, wantErr: core.ErrContextObservation},
			{name: "unrelated terminal error has no observable state", makeFixture: unrelatedProbeFixture, operation: contextstate.Observe, wantErr: core.ErrContextObservation},
			{name: "typed nil terminal error has no observable state", makeFixture: typedNilTerminalProbeFixture, operation: contextstate.ObserveAfterDone, wantErr: core.ErrContextObservation},
			{name: "noncomparable terminal error is rejected without identity traversal", makeFixture: nonComparableTerminalProbeFixture, operation: contextstate.Observe, wantErr: core.ErrContextObservation},
			{name: "panicking Err is contained", makeFixture: panickingErrProbeFixture, operation: contextstate.ObserveAfterDone, wantErr: core.ErrContextObservation},
			{name: "custom Is method is never consulted", makeFixture: panickingIdentityProbeFixture, operation: contextstate.Observe, wantErr: core.ErrContextObservation},
			{name: "cyclic error is rejected without traversal", makeFixture: cyclicErrorProbeFixture, operation: contextstate.ObserveAfterDone, wantErr: core.ErrContextObservation},
			{name: "typed nil whose Err panics is contained", makeFixture: nilPanickingContextFixture, operation: contextstate.Observe, wantErr: core.ErrContextObservation},
		}
		runObservationCases(t, cases)
	})

	t.Run("neutral active contexts stay active without an invented terminal fact", func(t *testing.T) {
		t.Parallel()

		cases := []observationCase{
			{name: "Observe reports background as active", makeFixture: backgroundContextFixture, operation: contextstate.Observe, wantState: contextstate.StateNone},
			{name: "Observe reports TODO as active", makeFixture: todoContextFixture, operation: contextstate.Observe, wantState: contextstate.StateNone},
			{name: "Observe ignores an unrelated context value", makeFixture: valueContextFixture, operation: contextstate.Observe, wantState: contextstate.StateNone},
			{name: "Observe reports an uncancelled context as active", makeFixture: activeContextFixture, operation: contextstate.Observe, wantState: contextstate.StateNone},
			{name: "Observe does not infer a future deadline terminal state", makeFixture: futureDeadlineContextFixture, operation: contextstate.Observe, wantState: contextstate.StateNone},
			{name: "Observe honors a detached active context", makeFixture: detachedContextFixture, operation: contextstate.Observe, wantState: contextstate.StateNone},
			{name: "Observe admits a nil-safe typed nil reporting no terminal error", makeFixture: nilSafeContextFixture, operation: contextstate.Observe, wantState: contextstate.StateNone},
			{name: "Observe calls an active custom context Err exactly once", makeFixture: activeProbeFixture, operation: contextstate.Observe, wantState: contextstate.StateNone},
			{name: "ObserveAfterDone refuses an active state after a closed Done", makeFixture: activeAfterDoneProbeFixture, operation: contextstate.ObserveAfterDone, wantErr: core.ErrContextObservation},
			{name: "ObserveAfterDone refuses background without inventing state", makeFixture: backgroundContextFixture, operation: contextstate.ObserveAfterDone, wantErr: core.ErrContextObservation},
			{name: "ObserveAfterDone refuses a nil-safe typed nil without inventing state", makeFixture: nilSafeContextFixture, operation: contextstate.ObserveAfterDone, wantErr: core.ErrContextObservation},
		}
		runObservationCases(t, cases)
	})
}

func runObservationCases(t *testing.T, cases []observationCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture := tc.makeFixture()
			if fixture.cleanup != nil {
				defer fixture.cleanup()
			}
			gotState, gotErr := tc.operation(fixture.ctx)
			if gotState != tc.wantState {
				t.Fatalf("observation state = %v, want %v", gotState, tc.wantState)
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("observation error = %v, want %v", gotErr, tc.wantErr)
			}
			if fixture.probe != nil && fixture.probe.errCalls != 1 {
				t.Fatalf("Context.Err() calls = %d, want 1", fixture.probe.errCalls)
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
