//go:build !windows

package shutdown

import (
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
