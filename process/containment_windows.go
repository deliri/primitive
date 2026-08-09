//go:build windows

package process

import (
	"errors"
	"os/exec"

	"github.com/deliri/primitive/v2026/core"
)

// windowsCancelKillOnlyDiagnostic is the one spelling of the refusal Windows
// answers for every cancel signal it cannot deliver, shared by the
// containment gate and the delivery leaf.
const windowsCancelKillOnlyDiagnostic = "windows delivers no cancel signal other than kill"

// applyContainment projects the validated containment onto the one command
// this package is about to start. Windows has no POSIX signal vocabulary, so
// only the silent hard stop is deliverable here; a request naming any other
// cancel signal is refused before the child exists rather than silently
// downgraded after it does.
func applyContainment(_ *exec.Cmd, containment Containment) error {
	if containment.CancelSignal != CancelSignalKill {
		return errors.Join(
			core.ErrProcessUnsupported,
			errors.New(windowsCancelKillOnlyDiagnostic),
		)
	}
	switch containment.Isolation {
	case IsolationDirect:
		return nil
	case IsolationGroup:
		// A new process group can be created here, but nothing in this
		// package can deliver a cancellation to it: deliverSignal below only
		// ever kills the direct child, because Windows has no group signal.
		// Accepting the request would tell a supervisor that cancellation
		// reaches the whole tree when it reaches one process, so the request
		// is refused before the child exists rather than silently downgraded
		// after it does. Whole-tree containment on Windows is a job object
		// and a separate decision.
		return errors.Join(
			core.ErrProcessUnsupported,
			errors.New("windows delivers no group cancellation"),
		)
	default:
		return contractError(isolationOutsideDomainDiagnostic)
	}
}

// sweepGroup refuses: windows admits no group containment, so no execution
// holding a group to sweep can exist here, and the refusal keeps the door's
// shape identical across hosts.
func sweepGroup(_ ProcessIdentity) error {
	return errors.Join(
		core.ErrProcessUnsupported,
		errors.New("windows delivers no group sweep"),
	)
}

// deliverSignal addresses the one admitted stop to the direct child through
// its held handle. Windows offers no group signal; descendant policy stays
// with the caller, who can run the documented taskkill tool through this
// same package.
func deliverSignal(delivery signalDelivery) error {
	if delivery.signal != CancelSignalKill {
		return errors.Join(
			core.ErrProcessUnsupported,
			errors.New(windowsCancelKillOnlyDiagnostic),
		)
	}
	return delivery.process.Kill()
}
