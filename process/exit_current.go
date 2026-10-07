package process

import (
	"os"

	"github.com/deliri/primitive/v2026/core"
)

// ExitStatus is the caller's intended normal termination status. Its zero value
// is success. Every status in the portable OS byte domain is admitted; this is
// distinct from ExitCode's observed signal marker and native Windows results.
type ExitStatus int

const (
	ExitSuccess ExitStatus = 0
	ExitFailure ExitStatus = 1
	ExitUsage   ExitStatus = 2
)

// Validate refuses values the portable normal-exit representation cannot carry.
func (status ExitStatus) Validate() error {
	if status < 0 || status > core.ProcessExitStatusMaximum {
		return contractError("exit status is outside the portable normal-exit domain")
	}
	return nil
}

// ExitCurrent immediately terminates the calling process with the validated
// status through Go's os.Exit. It does not run deferred functions, close caller
// resources, flush streams, or select product success policy. The caller must
// finish required cleanup before invoking it. A valid invocation never returns;
// an invalid status returns ErrProcessContract without terminating the process.
func ExitCurrent(status ExitStatus) error {
	if err := status.Validate(); err != nil {
		return err
	}
	os.Exit(int(status))
	return nil // Go's os.Exit cannot return after admitting the status.
}
