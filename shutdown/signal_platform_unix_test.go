//go:build !windows

package shutdown

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"os"
	"syscall"
	"testing"
)

func TestPlatformSignalProjectionIsExact(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		want []os.Signal
		set  SignalSet
	}{
		{name: "interactive", set: SignalSetInteractive, want: []os.Signal{os.Interrupt}},
		{name: "standard", set: SignalSetStandard, want: []os.Signal{os.Interrupt, syscall.SIGTERM}},
		{name: "terminal lifecycle", set: SignalSetTerminalLifecycle, want: []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}},
		{name: "unknown", set: SignalSetUnknown},
		{name: "future", set: signalSetLimit},
	}
	for _, tc := range cases {
		got := operatingSystemSignals(tc.set)
		if len(got) != len(tc.want) {
			t.Fatalf("%s signals = %v, want %v", tc.name, got, tc.want)
		}
		for index := range got {
			if got[index] != tc.want[index] {
				t.Fatalf("%s signal[%d] = %v, want %v", tc.name, index, got[index], tc.want[index])
			}
		}
	}
	classifications := []struct {
		signal os.Signal
		want   SignalKind
	}{
		{signal: os.Interrupt, want: SignalKindInterrupt},
		{signal: syscall.SIGTERM, want: SignalKindTerminate},
		{signal: syscall.SIGHUP, want: SignalKindHangup},
		{signal: syscall.SIGUSR1, want: SignalKindUnknown},
		{signal: nil, want: SignalKindUnknown},
	}
	for _, tc := range classifications {
		if got := classifyOperatingSystemSignal(tc.signal); got != tc.want {
			t.Fatalf("classifyOperatingSystemSignal(%v) = %s, want %s",
				tc.signal, got, tc.want)
		}
	}
}

func firstPlatformSignal() os.Signal  { return os.Interrupt }
func secondPlatformSignal() os.Signal { return syscall.SIGTERM }

// Native signal values cross the OS adapter as integers. The oracle maps
// admitted native values to exact typed observations and proves that all other
// values end in source refusal, never an authenticated signal.
func FuzzNativeSignalObservation(f *testing.F) {
	for _, native := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR1, 0, -1} {
		f.Add(int64(native))
	}
	f.Fuzz(func(t *testing.T, raw int64) {
		native := syscall.Signal(raw)
		want := SignalKindUnknown
		for _, pair := range []struct {
			native syscall.Signal
			kind   SignalKind
		}{
			{syscall.SIGINT, SignalKindInterrupt}, {syscall.SIGTERM, SignalKindTerminate}, {syscall.SIGHUP, SignalKindHangup},
		} {
			if native == pair.native {
				want = pair.kind
			}
		}
		events := make(chan os.Signal, 1)
		events <- native
		close(events)
		released := make(chan struct{}, 1)
		c := watchSourceForTest(t, events, released, defaultSignalPolicy())
		waitController(t, c)
		var cause SignalCause
		got := context.Cause(c.Context())
		if want == SignalKindUnknown {
			if !errors.Is(got, core.ErrShutdownSignalSource) || errors.As(got, &cause) {
				t.Fatalf("unsupported native value %d = %v, want source refusal", raw, got)
			}
		} else if !errors.As(got, &cause) || cause.Kind() != want || cause.Validate() != nil || !errors.Is(got, core.ErrShutdownSignalReceived) {
			t.Fatalf("native value %d = (%v,%v), want authentic %v", raw, got, cause.Kind(), want)
		}
		if len(released) != 1 {
			t.Fatalf("release count = %d, want one", len(released))
		}
		if e, open := <-c.Escalated(); open || e != (Escalation{}) {
			t.Fatalf("first-only escalation = (%+v,%t), want zero/closed", e, open)
		}
	})
}
