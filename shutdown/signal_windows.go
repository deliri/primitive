//go:build windows

package shutdown

import "os"

func operatingSystemSignals(set SignalSet) []os.Signal {
	if !set.IsValid() {
		return nil
	}
	return []os.Signal{os.Interrupt}
}

func classifyOperatingSystemSignal(observed os.Signal) SignalKind {
	if observed == os.Interrupt {
		return SignalKindInterrupt
	}
	return SignalKindUnknown
}
