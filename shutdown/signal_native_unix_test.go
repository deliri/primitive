//go:build !windows

package shutdown

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

// This is the native signal adapter under test. The extra standard-library
// subscription prevents a failed assertion from restoring a fatal default
// while an injected signal is in flight. Stop owns its final delivery join.
func TestWatchNativeUnixLifecycle(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessSignal, Scope: core.TestIsolationScopePackageProcess})
	for _, tc := range []struct {
		name   string
		set    SignalSet
		native syscall.Signal
		want   SignalKind
		second bool
	}{
		{name: "interactive interrupt cancels with exact native identity", set: SignalSetInteractive, native: syscall.SIGINT, want: SignalKindInterrupt},
		{name: "standard terminate cancels with exact native identity", set: SignalSetStandard, native: syscall.SIGTERM, want: SignalKindTerminate},
		{name: "terminal hangup cancels with exact native identity", set: SignalSetTerminalLifecycle, native: syscall.SIGHUP, want: SignalKindHangup},
		{name: "second interrupt remains an interrupt escalation", set: SignalSetInteractive, native: syscall.SIGINT, want: SignalKindInterrupt, second: true},
		{name: "second terminate remains a terminate escalation", set: SignalSetStandard, native: syscall.SIGTERM, want: SignalKindTerminate, second: true},
		{name: "second hangup remains a hangup escalation", set: SignalSetTerminalLifecycle, native: syscall.SIGHUP, want: SignalKindHangup, second: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessSignal, Scope: core.TestIsolationScopePackageProcess})
			guard := make(chan os.Signal, signalTransitionCapacity)
			signal.Notify(guard, tc.native)
			defer signal.Stop(guard)
			policy := defaultSignalPolicy()
			if tc.second {
				policy.SecondSignal = SecondSignalEscalate
			}
			controller, err := Watch(WatchRequest{Parent: t.Context(), Policy: policy, Set: tc.set})
			if err != nil {
				t.Fatalf("Watch = %v, want nil", err)
			}
			defer func() {
				if err := controller.Close(); err != nil {
					t.Errorf("Close = %v, want nil", err)
				}
			}()
			if err := syscall.Kill(os.Getpid(), tc.native); err != nil {
				t.Fatalf("native signal = %v, want nil", err)
			}
			waitContext(controller.Context(), t)
			var cause SignalCause
			got := context.Cause(controller.Context())
			if !errors.As(got, &cause) || cause.Validate() != nil || cause.Kind() != tc.want || !errors.Is(got, core.ErrShutdownSignalReceived) {
				t.Fatalf("first signal = (%v,%v), want authentic %v", got, cause.Kind(), tc.want)
			}
			if tc.second {
				if err := syscall.Kill(os.Getpid(), tc.native); err != nil {
					t.Fatalf("second native signal = %v, want nil", err)
				}
				escalation := receiveEscalation(t, controller)
				if escalation.Validate() != nil || escalation.Reason() != EscalationSecondSignal || escalation.FirstSignal() != tc.want || escalation.TriggerSignal() != tc.want {
					t.Fatalf("native escalation = %+v, want second %v/%v", escalation, tc.want, tc.want)
				}
			}
			waitController(t, controller)
			if got, open := <-controller.Escalated(); open || got != (Escalation{}) {
				t.Fatalf("extra escalation = (%+v,%t), want zero/closed", got, open)
			}
		})
	}
}

func TestWatchRejectsHostileParentAtPublicBoundary(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessSignal, Scope: core.TestIsolationScopePackageProcess})
	for _, tc := range []struct {
		name      string
		panicDone bool
	}{
		{name: "Done panic unwinds real subscription", panicDone: true},
		{name: "Value panic unwinds real subscription"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessSignal, Scope: core.TestIsolationScopePackageProcess})
			got, err := Watch(WatchRequest{Parent: hostileWatchParent{Context: t.Context(), panicDone: tc.panicDone}, Policy: defaultSignalPolicy(), Set: SignalSetStandard})
			if got != nil {
				if closeErr := got.Close(); closeErr != nil {
					t.Errorf("unexpected controller Close = %v, want nil", closeErr)
				}
			}
			if got != nil || !errors.Is(err, core.ErrShutdownContract) || !errors.Is(err, core.ErrContextObservation) {
				t.Fatalf("Watch = (%v,%v), want nil/shutdown+observation", got, err)
			}
		})
	}
}
