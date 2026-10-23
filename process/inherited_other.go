//go:build !unix

package process

import (
	"errors"
	"os/exec"

	"github.com/deliri/primitive/v2026/core"
)

// inheritedUnsupportedDiagnostic names why a lent stream is refused here.
const inheritedUnsupportedDiagnostic = "this host passes no additional descriptors to a child"

// applyInherited refuses any lent stream on hosts that cannot pass one, so a
// caller is never told a descriptor exists that the child cannot see.
func applyInherited(_ *exec.Cmd, streams []InheritedStream) error {
	if len(streams) != 0 {
		return errors.Join(core.ErrProcessUnsupported, errors.New(inheritedUnsupportedDiagnostic))
	}
	return nil
}

// openLent refuses on hosts that cannot inherit descriptors.
func openLent(_, _ uintptr) (Lent, error) {
	return Lent{}, errors.Join(core.ErrProcessUnsupported, errors.New(inheritedUnsupportedDiagnostic))
}
