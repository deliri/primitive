//go:build !unix && !windows

package process

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// observedLiveness refuses on hosts with no process interrogation mechanism,
// so a caller is never told a counterpart is gone on a platform where nobody
// looked.
func observedLiveness(_ ProcessIdentity) (Liveness, error) {
	return LivenessUnknown, errors.Join(
		core.ErrProcessUnsupported,
		errors.New("this host interrogates no process identities"),
	)
}
