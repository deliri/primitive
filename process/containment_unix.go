//go:build unix

package process

import (
	"errors"
	"os/exec"
	"syscall"

	"github.com/deliri/primitive/v2026/core"
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
func cancelSignalValue(signal CancelSignal) (syscall.Signal, error) {
	switch signal {
	case CancelSignalKill:
		return syscall.SIGKILL, nil
	case CancelSignalQuit:
		return syscall.SIGQUIT, nil
	case CancelSignalInterrupt:
		return syscall.SIGINT, nil
	case CancelSignalTerminate:
		return syscall.SIGTERM, nil
	default:
		return 0, contractError(cancelSignalOutsideDomainDiagnostic)
	}
}

// sweepGroup delivers one final hard stop to the whole group the child led.
// ESRCH proves the group is already gone. Every other failure, including
// permission denial, remains a failed effect with its native identity.
func sweepGroup(identity ProcessIdentity) error {
	pid, err := unixProcessID(identity)
	if err != nil {
		return err
	}
	return groupSweepError(syscall.Kill(-pid, syscall.SIGKILL))
}

func groupSweepError(killErr error) error {
	if killErr == nil || errors.Is(killErr, syscall.ESRCH) {
		return nil
	}
	return killErr
}

func observedGroupLiveness(identity ProcessIdentity) (Liveness, error) {
	pid, err := unixProcessID(identity)
	if err != nil {
		return LivenessUnknown, err
	}
	return groupProbeLiveness(syscall.Kill(-pid, 0))
}

func groupProbeLiveness(probeErr error) (Liveness, error) {
	if probeErr == nil || errors.Is(probeErr, syscall.EPERM) {
		return LivenessAlive, nil
	}
	if errors.Is(probeErr, syscall.ESRCH) {
		return LivenessGone, nil
	}
	return LivenessUnknown, errors.Join(core.ErrProcessObservation, probeErr)
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
		pid, pidErr := unixProcessID(delivery.identity)
		if pidErr != nil {
			return pidErr
		}
		return syscall.Kill(-pid, value)
	default:
		return contractError(isolationOutsideDomainDiagnostic)
	}
}
