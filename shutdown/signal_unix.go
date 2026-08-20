//go:build !windows

package shutdown

import (
	"os"
	"syscall"
)

func operatingSystemSignals(set SignalSet) []os.Signal {
	if set == SignalSetInteractive {
		return []os.Signal{os.Interrupt}
	}
	if set == SignalSetStandard {
		return []os.Signal{os.Interrupt, syscall.SIGTERM}
	}
	if set == SignalSetTerminalLifecycle {
		return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
	}
	return nil
}

func classifyOperatingSystemSignal(observed os.Signal) SignalKind {
	switch observed {
	case os.Interrupt:
		return SignalKindInterrupt
	case syscall.SIGTERM:
		return SignalKindTerminate
	case syscall.SIGHUP:
		return SignalKindHangup
	default:
		return SignalKindUnknown
	}
}
