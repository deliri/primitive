package shutdown

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type unknownSignal struct{}

func (unknownSignal) Signal()        {}
func (unknownSignal) String() string { return "unknown" }

func TestSignalPolicyCrossProductAndWatchIngress(t *testing.T) {
	t.Parallel()

	zero := durationForTest(t, 0)
	one := durationForTest(t, time.Nanosecond)
	for second := SecondSignalAction(0); second <= secondSignalActionLimit; second++ {
		for grace := GraceExpiryAction(0); grace <= graceExpiryActionLimit; grace++ {
			for _, period := range []struct {
				name  string
				value temporal.Duration
			}{
				{name: "zero", value: zero},
				{name: "one", value: one},
			} {
				policy := SignalPolicy{
					SecondSignal: second,
					GraceExpiry:  grace,
					GracePeriod:  period.value,
				}
				wantValid := second.IsValid() && grace.IsValid() &&
					((grace == GraceExpiryEscalate) == !period.value.IsZero())
				gotErr := policy.Validate()
				if (gotErr == nil) != wantValid {
					t.Fatalf("policy second:%d grace:%d period:%s error:%v, want valid:%t",
						second, grace, period.name, gotErr, wantValid)
				}
			}
		}
	}

	valid := WatchRequest{
		Parent: t.Context(),
		Policy: SignalPolicy{
			SecondSignal: SecondSignalRelease,
			GraceExpiry:  GraceExpiryDisabled,
		},
		Set: SignalSetStandard,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid request error = %v", err)
	}
	terminal, cancel := context.WithCancel(t.Context())
	cancel()
	rejected := []struct {
		name           string
		wantDiagnostic diagnostic
		request        WatchRequest
	}{
		{
			name:           "wholly unset request names the parent",
			request:        WatchRequest{},
			wantDiagnostic: diagnosticParentContext,
		},
		{
			name:           "unknown signal set names the set",
			request:        WatchRequest{Parent: t.Context(), Policy: valid.Policy, Set: SignalSetUnknown},
			wantDiagnostic: diagnosticSignalSetUnsupported,
		},
		{
			name:           "signal set at the private limit names the set",
			request:        WatchRequest{Parent: t.Context(), Policy: valid.Policy, Set: signalSetLimit},
			wantDiagnostic: diagnosticSignalSetUnsupported,
		},
		{
			name:           "terminal parent names the parent",
			request:        WatchRequest{Parent: terminal, Policy: valid.Policy, Set: SignalSetStandard},
			wantDiagnostic: diagnosticParentContext,
		},
		{
			name:           "unset policy names the grace expiry action",
			request:        WatchRequest{Parent: t.Context(), Set: SignalSetStandard},
			wantDiagnostic: diagnosticGraceExpiryUnsupported,
		},
		{
			name: "grace period without escalation names the coupling",
			request: WatchRequest{Parent: t.Context(), Set: SignalSetStandard, Policy: SignalPolicy{
				SecondSignal: SecondSignalRelease,
				GraceExpiry:  GraceExpiryDisabled,
				GracePeriod:  one,
			}},
			wantDiagnostic: diagnosticGracePeriodCoupling,
		},
		{
			name: "escalating grace expiry without a period names the coupling",
			request: WatchRequest{Parent: t.Context(), Set: SignalSetStandard, Policy: SignalPolicy{
				SecondSignal: SecondSignalRelease,
				GraceExpiry:  GraceExpiryEscalate,
				GracePeriod:  zero,
			}},
			wantDiagnostic: diagnosticGracePeriodCoupling,
		},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			requireRejection(t, tc.request.Validate(), core.ErrShutdownContract, tc.wantDiagnostic)
			controller, err := Watch(tc.request)
			requireRejection(t, err, core.ErrShutdownContract, tc.wantDiagnostic)
			if controller != nil {
				t.Fatalf("Watch(%s) controller = %v, want nil on rejection", tc.name, controller)
			}
		})
	}

	t.Run("a valid set that projects no platform signal names the projection", func(t *testing.T) {
		t.Parallel()
		requireRejection(t, watchSourceRejection(WatchRequest{
			Parent: t.Context(), Policy: valid.Policy, Set: SignalSetStandard,
		}), core.ErrShutdownContract, diagnosticSignalSourceIncomplete)
	})
}

// watchSourceRejection drives the owned constructor with an incomplete source,
// which Watch itself can never build. It is a direct unit ratchet on the
// constructor gate, not a signal-delivery proof.
func watchSourceRejection(request WatchRequest) error {
	_, err := watchSource(request, signalSource{})
	return err
}

func TestControllerFirstSignalAuthenticatesCauseAndReleases(t *testing.T) {
	t.Parallel()

	events := make(chan os.Signal, 2)
	released := make(chan struct{}, 1)
	controller := watchSourceForTest(t, events, released, SignalPolicy{
		SecondSignal: SecondSignalRelease,
		GraceExpiry:  GraceExpiryDisabled,
	})
	events <- unknownSignal{}
	events <- firstPlatformSignal()
	waitController(t, controller)
	var cause SignalCause
	gotCause := context.Cause(controller.Context())
	if !errors.As(gotCause, &cause) ||
		!errors.Is(gotCause, core.ErrShutdownSignalReceived) ||
		cause.Validate() != nil || cause.Kind() != SignalKindInterrupt {
		t.Fatalf("controller cause = %v typed:%+v, want authentic interrupt",
			gotCause, cause)
	}
	if len(released) != 1 {
		t.Fatalf("release count = %d, want 1", len(released))
	}
	if _, open := <-controller.Escalated(); open {
		t.Fatal("Escalated channel open = true, want false")
	}
	if err := controller.Close(); err != nil || len(released) != 1 {
		t.Fatalf("Close(after completion) = %v releases:%d, want nil/1",
			err, len(released))
	}
}

func TestControllerSecondSignalPublishesAfterRelease(t *testing.T) {
	t.Parallel()

	events := make(chan os.Signal, 3)
	released := make(chan struct{}, 1)
	controller := watchSourceForTest(t, events, released, SignalPolicy{
		SecondSignal: SecondSignalEscalate,
		GraceExpiry:  GraceExpiryDisabled,
	})
	events <- firstPlatformSignal()
	waitContext(controller.Context(), t)
	if len(released) != 0 {
		t.Fatalf("release count after first = %d, want 0", len(released))
	}
	events <- unknownSignal{}
	events <- secondPlatformSignal()
	escalation := receiveEscalation(t, controller)
	if len(released) != 1 {
		t.Fatalf("release count before escalation observation = %d, want 1", len(released))
	}
	if escalation.Validate() != nil ||
		escalation.Reason() != EscalationSecondSignal ||
		escalation.FirstSignal() != SignalKindInterrupt ||
		!escalation.TriggerSignal().IsValid() {
		t.Fatalf("escalation = reason:%s first:%s trigger:%s validation:%v",
			escalation.Reason(), escalation.FirstSignal(),
			escalation.TriggerSignal(), escalation.Validate())
	}
	waitController(t, controller)
}

func TestControllerGraceExpiryUsesRealGoTimer(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		events := make(chan os.Signal, 1)
		released := make(chan struct{}, 1)
		controller := watchSourceForTest(t, events, released, SignalPolicy{
			SecondSignal: SecondSignalRelease,
			GraceExpiry:  GraceExpiryEscalate,
			GracePeriod:  durationForTest(t, 7*time.Minute),
		})
		events <- firstPlatformSignal()
		escalation, open := <-controller.Escalated()
		if !open {
			t.Fatalf("Escalated channel open = %t, want true", open)
		}
		if escalation.Validate() != nil ||
			escalation.Reason() != EscalationGraceExpired ||
			escalation.TriggerSignal() != SignalKindUnknown ||
			len(released) != 1 {
			t.Fatalf("grace escalation = %+v validation:%v releases:%d",
				escalation, escalation.Validate(), len(released))
		}
		waitController(t, controller)
	})
}

func TestControllerSourceClosureParentCancellationAndConcurrentClose(t *testing.T) {
	t.Parallel()

	t.Run("source closes before first signal", func(t *testing.T) {
		t.Parallel()
		events := make(chan os.Signal)
		close(events)
		released := make(chan struct{}, 1)
		controller := watchSourceForTest(t, events, released, defaultSignalPolicy())
		waitController(t, controller)
		if !errors.Is(context.Cause(controller.Context()), core.ErrShutdownSignalSource) ||
			len(released) != 1 {
			t.Fatalf("closed source = cause:%v releases:%d",
				context.Cause(controller.Context()), len(released))
		}
	})

	t.Run("parent cancellation joins after first signal", func(t *testing.T) {
		t.Parallel()
		parent, cancel := context.WithCancel(t.Context())
		events := make(chan os.Signal, 1)
		released := make(chan struct{}, 1)
		controller := watchSourceRequestForTest(t, WatchRequest{
			Parent: parent,
			Policy: SignalPolicy{
				SecondSignal: SecondSignalEscalate,
				GraceExpiry:  GraceExpiryDisabled,
			},
			Set: SignalSetStandard,
		}, events, released)
		events <- firstPlatformSignal()
		waitContext(controller.Context(), t)
		cancel()
		waitController(t, controller)
		if len(released) != 1 {
			t.Fatalf("release count = %d, want 1", len(released))
		}
	})

	t.Run("concurrent close is idempotent and joined", func(t *testing.T) {
		t.Parallel()
		events := make(chan os.Signal)
		released := make(chan struct{}, 1)
		controller := watchSourceForTest(t, events, released, defaultSignalPolicy())
		var group sync.WaitGroup
		for range 64 {
			group.Go(func() {
				if err := controller.Close(); err != nil {
					t.Errorf("Close() error = %v", err)
				}
			})
		}
		group.Wait()
		if len(released) != 1 ||
			!errors.Is(context.Cause(controller.Context()), context.Canceled) {
			t.Fatalf("concurrent Close = releases:%d cause:%v",
				len(released), context.Cause(controller.Context()))
		}
	})
}

func TestEscalationAndTypedNilImpossibleStates(t *testing.T) {
	t.Parallel()

	invalid := []Escalation{
		{},
		{valid: true, reason: EscalationSecondSignal, first: SignalKindUnknown, trigger: SignalKindInterrupt},
		{valid: true, reason: EscalationSecondSignal, first: SignalKindInterrupt, trigger: SignalKindUnknown},
		{valid: true, reason: EscalationGraceExpired, first: SignalKindInterrupt, trigger: SignalKindTerminate},
	}
	for index, escalation := range invalid {
		if err := escalation.Validate(); !errors.Is(err, core.ErrShutdownContract) {
			t.Fatalf("invalid escalation %d error = %v, want contract", index, err)
		}
	}
	if err := (SignalCause{kind: SignalKindInterrupt}).Validate(); !errors.Is(err, core.ErrShutdownContract) {
		t.Fatalf("forged SignalCause error = %v, want contract", err)
	}
	var controller *Controller
	if controller.Context() != nil || controller.Done() != nil ||
		controller.Escalated() != nil ||
		!errors.Is(controller.Close(), core.ErrShutdownContract) {
		t.Fatalf("typed nil Controller = context:%v done:%v escalated:%v close:%v",
			controller.Context(), controller.Done(), controller.Escalated(),
			controller.Close())
	}
}

func defaultSignalPolicy() SignalPolicy {
	return SignalPolicy{
		SecondSignal: SecondSignalRelease,
		GraceExpiry:  GraceExpiryDisabled,
	}
}

func watchSourceForTest(
	t *testing.T,
	events <-chan os.Signal,
	released chan<- struct{},
	policy SignalPolicy,
) *Controller {
	t.Helper()
	return watchSourceRequestForTest(t, WatchRequest{
		Parent: t.Context(), Policy: policy, Set: SignalSetStandard,
	}, events, released)
}

func watchSourceRequestForTest(
	t *testing.T,
	request WatchRequest,
	events <-chan os.Signal,
	released chan<- struct{},
) *Controller {
	t.Helper()
	controller, err := watchSource(request, signalSource{
		events:  events,
		release: func() { released <- struct{}{} },
	})
	if err != nil {
		t.Fatalf("watchSource() error = %v", err)
	}
	return controller
}

func receiveEscalation(t *testing.T, controller *Controller) Escalation {
	t.Helper()
	select {
	case escalation, open := <-controller.Escalated():
		if !open {
			t.Fatalf("Escalated channel open = %t, want true", open)
		}
		return escalation
	case <-time.After(10 * time.Second):
		t.Fatalf("Escalated channel produced value = false after %s, want true",
			10*time.Second)
		return Escalation{}
	}
}

func waitContext(ctx context.Context, t *testing.T) {
	t.Helper()
	if ctx == nil {
		t.Fatalf("controller context = %v, want non-nil", ctx)
	}
	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
		t.Fatalf("controller context terminated = false after %s, want true",
			10*time.Second)
	}
}

func waitController(t *testing.T, controller *Controller) {
	t.Helper()
	select {
	case <-controller.Done():
	case <-time.After(10 * time.Second):
		t.Fatalf("controller joined = false after %s, want true", 10*time.Second)
	}
}
