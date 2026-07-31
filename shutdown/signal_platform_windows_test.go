//go:build windows

package shutdown

import (
	"os"
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
