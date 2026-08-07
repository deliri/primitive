//go:build unix

package process

import (
	"os"
	"syscall"
)

// observedTerminationSignal reads the ending signal out of the wait status
// the standard library already returned. This is a type assertion on a
// reaped observation, not a call: nothing here asks the kernel for anything.
func observedTerminationSignal(state *os.ProcessState) (SignalNumber, bool) {
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return 0, false
	}
	signal := SignalNumber(status.Signal())
	if signal.Validate() != nil {
		return 0, false
	}
	return signal, true
}
