//go:build unix

package process

import (
	"errors"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

// applyContainment projects the validated containment onto the one command
// this package is about to start. Group isolation is a configuration os/exec
// carries to the kernel at fork time; nothing here calls the kernel.
func applyContainment(command *exec.Cmd, containment Containment) error {
	switch containment.Isolation {
	case IsolationDirect:
		return nil
	case IsolationGroup:
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		return nil
	default:
		return contractError(isolationOutsideDomainDiagnostic)
	}
}

// cancelSignalValue names the exact kernel signal each admitted cancel
// signal delivers on a POSIX host.
func cancelSignalValue(signal CancelSignal) (unix.Signal, error) {
	switch signal {
	case CancelSignalKill:
		return unix.SIGKILL, nil
	case CancelSignalQuit:
		return unix.SIGQUIT, nil
	case CancelSignalInterrupt:
		return unix.SIGINT, nil
	case CancelSignalTerminate:
		return unix.SIGTERM, nil
	default:
		return 0, contractError("cancel signal is outside the admitted domain")
	}
}

// sweepGroup delivers one final hard stop to the whole group the child led.
// ESRCH proves the group is already gone and EPERM proves this process may no
// longer address it; neither can be repaired by retrying, so both are
// successful terminal outcomes rather than failures.
func sweepGroup(identity ProcessIdentity) error {
	pid, err := identity.Int()
	if err != nil {
		return err
	}
	killErr := unix.Kill(-pid, unix.SIGKILL)
	if killErr == nil || errors.Is(killErr, unix.ESRCH) || errors.Is(killErr, unix.EPERM) {
		return nil
	}
	return killErr
}

// deliverSignal addresses one admitted signal to the direct child or, under
// group isolation, to the whole process group the child leads.
//
// The direct address goes through the held os.Process, whose platform handle
// cannot name a recycled process. The group address has no handle form: the
// negative identifier is the kernel's own spelling of "the group led by this
// leader", and it stays correct exactly as long as any member still runs.
func deliverSignal(delivery signalDelivery) error {
	value, err := cancelSignalValue(delivery.signal)
	if err != nil {
		return err
	}
	switch delivery.containment.Isolation {
	case IsolationDirect:
		return delivery.process.Signal(value)
	case IsolationGroup:
		pid, pidErr := delivery.identity.Int()
		if pidErr != nil {
			return pidErr
		}
		return unix.Kill(-pid, value)
	default:
		return contractError(isolationOutsideDomainDiagnostic)
	}
}
