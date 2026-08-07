//go:build !unix && !windows

package process

import (
	"errors"
	"os/exec"

	"github.com/deliri/primitive/v2026/core"
)

// applyContainment refuses everything beyond the direct silent stop on hosts
// with no process-group or signal vocabulary, so a caller is never told a
// containment exists that nobody can enforce.
func applyContainment(_ *exec.Cmd, containment Containment) error {
	if containment.Isolation == IsolationGroup || containment.CancelSignal != CancelSignalKill {
		return errors.Join(
			core.ErrProcessUnsupported,
			errors.New("this host delivers no containment beyond killing the direct child"),
		)
	}
	return nil
}

// deliverSignal refuses on hosts with no signal vocabulary.
func deliverSignal(_ signalDelivery) error {
	return errors.Join(
		core.ErrProcessUnsupported,
		errors.New("this host delivers no signals"),
	)
}
