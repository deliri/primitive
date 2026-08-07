//go:build windows

package process

import (
	"errors"
	"os/exec"
	"syscall"

	"github.com/deliri/primitive/v2026/core"
)

// applyContainment projects the validated containment onto the one command
// this package is about to start. Windows has no POSIX signal vocabulary, so
// only the silent hard stop is deliverable here; a request naming any other
// cancel signal is refused before the child exists rather than silently
// downgraded after it does.
func applyContainment(command *exec.Cmd, containment Containment) error {
	if containment.CancelSignal != CancelSignalKill {
		return errors.Join(
			core.ErrProcessUnsupported,
			errors.New("windows delivers no cancel signal other than kill"),
		)
	}
	if containment.Isolation == IsolationGroup {
		command.SysProcAttr = &syscall.SysProcAttr{
			CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		}
	}
	return nil
}

// deliverSignal addresses the one admitted stop to the direct child through
// its held handle. Windows offers no group signal; descendant policy stays
// with the caller, who can run the documented taskkill tool through this
// same package.
func deliverSignal(delivery signalDelivery) error {
	if delivery.signal != CancelSignalKill {
		return errors.Join(
			core.ErrProcessUnsupported,
			errors.New("windows delivers no cancel signal other than kill"),
		)
	}
	return delivery.process.Kill()
}
