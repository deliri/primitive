//go:build windows

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

	for _, set := range []SignalSet{
		SignalSetInteractive, SignalSetStandard, SignalSetTerminalLifecycle,
	} {
		got := operatingSystemSignals(set)
		if len(got) != 1 || got[0] != os.Interrupt {
			t.Fatalf("operatingSystemSignals(%s) = %v, want interrupt", set, got)
		}
	}
	if got := operatingSystemSignals(SignalSetUnknown); got != nil {
		t.Fatalf("operatingSystemSignals(unknown) = %v, want nil", got)
	}
}

func firstPlatformSignal() os.Signal  { return os.Interrupt }
func secondPlatformSignal() os.Signal { return os.Interrupt }

func FuzzNativeSignalObservation(f *testing.F) {
	for _, raw := range []int64{0, int64(syscall.SIGINT), -1} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw int64) {
		native := syscall.Signal(raw)
		events := make(chan os.Signal, 1)
		events <- native
		close(events)
		released := make(chan struct{}, 1)
		c := watchSourceForTest(t, events, released, defaultSignalPolicy())
		waitController(t, c)
		var cause SignalCause
		got := context.Cause(c.Context())
		if native != os.Interrupt {
			if !errors.Is(got, core.ErrShutdownSignalSource) || errors.As(got, &cause) {
				t.Fatalf("unsupported native value %d = %v, want source refusal", raw, got)
			}
		} else if !errors.As(got, &cause) || cause.Kind() != SignalKindInterrupt || cause.Validate() != nil || !errors.Is(got, core.ErrShutdownSignalReceived) {
			t.Fatalf("native value %d = (%v,%v), want authentic interrupt", raw, got, cause.Kind())
		}
		if len(released) != 1 {
			t.Fatalf("release count = %d, want one", len(released))
		}
		if e, open := <-c.Escalated(); open || e != (Escalation{}) {
			t.Fatalf("first-only escalation = (%+v,%t), want zero/closed", e, open)
		}
	})
}
