//go:build !unix

package process

import "os"

// observedTerminationSignal reports no signal where the platform names none.
// The unreported answer is honest: Result.TerminationSignal refuses it, so a
// caller never records a fabricated signal number.
func observedTerminationSignal(_ *os.ProcessState) (SignalNumber, bool) {
	return 0, false
}
