//go:build unix

package process

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

// applyContainment projects the validated containment onto the one command
// this package is about to start. Group isolation is a configuration os/exec
// carries to the kernel at fork time; nothing here calls the kernel.
func applyContainment(command *exec.Cmd, containment Containment) error {
	if containment.Isolation == IsolationGroup {
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	return nil
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
	if delivery.containment.Isolation != IsolationGroup {
		return delivery.process.Signal(value)
	}
	pid, err := delivery.identity.Int()
	if err != nil {
		return err
	}
	return unix.Kill(-pid, value)
}
