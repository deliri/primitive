//go:build !windows

package shutdown

import (
	"os"
	"syscall"
)

func operatingSystemSignals(set SignalSet) []os.Signal {
	switch set {
	case SignalSetInteractive:
		return []os.Signal{os.Interrupt}
	case SignalSetStandard:
		return []os.Signal{os.Interrupt, syscall.SIGTERM}
	case SignalSetTerminalLifecycle:
		return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
	default:
		return nil
	}
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
