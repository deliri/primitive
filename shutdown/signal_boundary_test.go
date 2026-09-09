package shutdown

import (
	"context"
	"errors"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type hostileWatchParent struct {
	context.Context
	panicDone bool
}

func (p hostileWatchParent) Done() <-chan struct{} {
	if p.panicDone {
		panic(core.ErrContextObservation)
	}
	return p.Context.Done()
}

// witness:waiver doctrine/code_form/return_interface -- implements the exact context.Context.Value signature to attack Go cancellation propagation.
func (p hostileWatchParent) Value(key any) any {
	if !p.panicDone {
		panic(core.ErrContextObservation)
	}
	return p.Context.Value(key)
}

func TestWatchContextConstructionRefusesHostileParent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		panicDone bool
	}{
		{name: "panicking Done refuses context construction", panicDone: true},
		{name: "panicking Value refuses propagation lookup"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parent, cancel := context.WithCancel(t.Context())
			defer cancel()
			request := WatchRequest{Parent: hostileWatchParent{Context: parent, panicDone: tc.panicDone}, Policy: defaultSignalPolicy(), Set: SignalSetStandard}
			events := make(chan os.Signal)
			var got *Controller
			var err error
			var panicked bool
			func() {
				defer func() {
					if recover() != nil {
						panicked = true
					}
				}()
				got, err = watchSource(request, signalSource{events: events, release: func() {}})
			}()
			if got != nil {
				if closeErr := got.Close(); closeErr != nil {
					t.Errorf("unexpected controller Close = %v, want nil", closeErr)
				}
			}
			if panicked || got != nil || !errors.Is(err, core.ErrShutdownContract) || !errors.Is(err, core.ErrContextObservation) {
				t.Fatalf("context construction = (panic=%t,controller=%v,error=%v), want false/nil/shutdown+observation", panicked, got, err)
			}
		})
	}
}

type signalEnd uint8

const (
	signalEndClose signalEnd = iota + 1
	signalEndParent
	signalEndSource
	signalEndSecond
	signalEndGrace
)

func TestShutdownSignalObservationLayerTriadOwnsEveryExit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		first      bool
		end        signalEnd
		second     SecondSignalAction
		grace      GraceExpiryAction
		wantCause  error
		wantReason EscalationReason
	}{
		{name: "neutral close before any signal", end: signalEndClose, second: SecondSignalRelease, grace: GraceExpiryDisabled, wantCause: context.Canceled},
		{name: "parent refusal before any signal", end: signalEndParent, second: SecondSignalEscalate, grace: GraceExpiryDisabled, wantCause: errHostileCleanup},
		{name: "source loss refuses rather than authenticating absence", end: signalEndSource, second: SecondSignalRelease, grace: GraceExpiryDisabled, wantCause: core.ErrShutdownSignalSource},
		{name: "close after first preserves authentic cause without escalation", first: true, end: signalEndClose, second: SecondSignalEscalate, grace: GraceExpiryDisabled, wantCause: core.ErrShutdownSignalReceived},
		{name: "parent cancellation after first preserves the observed signal", first: true, end: signalEndParent, second: SecondSignalEscalate, grace: GraceExpiryEscalate, wantCause: core.ErrShutdownSignalReceived},
		{name: "source closure after first fabricates no second signal", first: true, end: signalEndSource, second: SecondSignalEscalate, grace: GraceExpiryDisabled, wantCause: core.ErrShutdownSignalReceived},
		{name: "second signal publishes exactly one fact before grace expiry", first: true, end: signalEndSecond, second: SecondSignalEscalate, grace: GraceExpiryEscalate, wantCause: core.ErrShutdownSignalReceived, wantReason: EscalationSecondSignal},
		{name: "grace expires while second signal observation stays active", first: true, end: signalEndGrace, second: SecondSignalEscalate, grace: GraceExpiryEscalate, wantCause: core.ErrShutdownSignalReceived, wantReason: EscalationGraceExpired},
		{name: "grace expires after the subscription is released", first: true, end: signalEndGrace, second: SecondSignalRelease, grace: GraceExpiryEscalate, wantCause: core.ErrShutdownSignalReceived, wantReason: EscalationGraceExpired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				parent, cancel := context.WithCancelCause(t.Context())
				defer cancel(context.Canceled)
				policy := SignalPolicy{SecondSignal: tc.second, GraceExpiry: tc.grace}
				if tc.grace == GraceExpiryEscalate {
					policy.GracePeriod = durationForTest(t, time.Second)
				}
				events := make(chan os.Signal, signalTransitionCapacity)
				releases := make(chan struct{}, 1)
				c := watchSourceRequestForTest(t, WatchRequest{Parent: parent, Policy: policy, Set: SignalSetStandard}, events, releases)
				events <- unknownSignal{}
				synctest.Wait()
				if context.Cause(c.Context()) != nil || len(releases) != 0 {
					t.Fatalf("unknown signal = (cause=%v,releases=%d), want nil/0", context.Cause(c.Context()), len(releases))
				}
				if tc.first {
					events <- firstPlatformSignal()
					waitContext(c.Context(), t)
					synctest.Wait()
					wantReleases := 0
					if tc.second == SecondSignalRelease {
						wantReleases = 1
					}
					if len(releases) != wantReleases {
						t.Fatalf("first signal releases = %d, want %d", len(releases), wantReleases)
					}
					var cause SignalCause
					if !errors.As(context.Cause(c.Context()), &cause) || cause.Kind() != SignalKindInterrupt || cause.Validate() != nil {
						t.Fatalf("first cause = %v, want authentic interrupt", context.Cause(c.Context()))
					}
				}
				switch tc.end {
				case signalEndClose:
					if err := c.Close(); err != nil {
						t.Fatalf("Close = %v, want nil", err)
					}
				case signalEndParent:
					cancel(errHostileCleanup)
				case signalEndSource:
					close(events)
				case signalEndSecond:
					events <- unknownSignal{}
					events <- firstPlatformSignal()
				case signalEndGrace:
				default:
					t.Fatalf("test exit = %d, want named exit", tc.end)
				}
				waitController(t, c)
				if !errors.Is(context.Cause(c.Context()), tc.wantCause) || len(releases) != 1 {
					t.Fatalf("exit = (%v,%d releases), want %v/1", context.Cause(c.Context()), len(releases), tc.wantCause)
				}
				got, open := <-c.Escalated()
				wantOpen := tc.wantReason != EscalationReasonUnknown
				if open != wantOpen {
					t.Fatalf("escalation presence = %t, want %t", open, wantOpen)
				}
				if wantOpen {
					wantTrigger := SignalKindUnknown
					if tc.wantReason == EscalationSecondSignal {
						wantTrigger = SignalKindInterrupt
					}
					if got.Validate() != nil || got.Reason() != tc.wantReason || got.FirstSignal() != SignalKindInterrupt || got.TriggerSignal() != wantTrigger {
						t.Fatalf("escalation = %+v, want %v/interrupt/%v", got, tc.wantReason, wantTrigger)
					}
				} else if got != (Escalation{}) {
					t.Fatalf("absent escalation = %+v, want zero", got)
				}
				if extra, open := <-c.Escalated(); open || extra != (Escalation{}) {
					t.Fatalf("extra escalation = (%+v,%t), want zero/closed", extra, open)
				}
				if err := c.Close(); err != nil || len(releases) != 1 {
					t.Fatalf("repeated Close = (%v,%d releases), want nil/1", err, len(releases))
				}
			})
		})
	}
}

func FuzzWatchPolicyAndSourceClosure(f *testing.F) {
	for _, second := range []SecondSignalAction{SecondSignalRelease, SecondSignalEscalate} {
		for _, grace := range []GraceExpiryAction{GraceExpiryDisabled, GraceExpiryEscalate} {
			period := int64(0)
			if grace == GraceExpiryEscalate {
				period = 1
			}
			f.Add(uint8(second), uint8(grace), uint8(SignalSetStandard), period)
		}
	}
	f.Add(uint8(0), uint8(0), uint8(0), int64(0))
	f.Fuzz(func(t *testing.T, second, grace, set uint8, period int64) {
		duration, err := temporal.DurationFromNanoseconds(period)
		if period < 0 {
			if !errors.Is(err, core.ErrTemporalContract) {
				t.Fatalf("duration = %v, want temporal contract", err)
			}
			return
		}
		request := WatchRequest{Parent: t.Context(), Set: SignalSet(set), Policy: SignalPolicy{SecondSignal: SecondSignalAction(second), GraceExpiry: GraceExpiryAction(grace), GracePeriod: duration}}
		wantValid := (second == uint8(SecondSignalRelease) || second == uint8(SecondSignalEscalate)) && (grace == uint8(GraceExpiryDisabled) || grace == uint8(GraceExpiryEscalate)) && (set == uint8(SignalSetInteractive) || set == uint8(SignalSetStandard) || set == uint8(SignalSetTerminalLifecycle)) && ((grace == uint8(GraceExpiryEscalate)) == (period > 0))
		events := make(chan os.Signal)
		close(events)
		released := false
		got, err := watchSource(request, signalSource{events: events, release: func() { released = true }})
		if !wantValid {
			if !errors.Is(err, core.ErrShutdownContract) || got != nil || released {
				t.Fatalf("rejected Watch source = (%v,%v,%t), want nil/contract/not owned", got, err, released)
			}
			return
		}
		if err != nil || got == nil {
			t.Fatalf("admitted Watch source = (%v,%v), want owned/nil", got, err)
		}
		waitController(t, got)
		if err := got.Close(); err != nil {
			t.Fatalf("Close = %v, want nil", err)
		}
		if !released || !errors.Is(context.Cause(got.Context()), core.ErrShutdownSignalSource) {
			t.Fatalf("source loss = (%t,%v), want released/source error", released, context.Cause(got.Context()))
		}
		if e, open := <-got.Escalated(); open || e != (Escalation{}) {
			t.Fatalf("source loss escalation = (%+v,%t), want zero/closed", e, open)
		}
	})
}
