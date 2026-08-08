//go:build unix

package process

import (
	"errors"

	"golang.org/x/sys/unix"

	"github.com/deliri/primitive/v2026/core"
)

// observedLiveness asks the kernel with the null signal, the POSIX spelling
// of "does this identity name a process" that delivers nothing.
//
// The x/sys escape is taken deliberately over os.FindProcess plus Signal(0),
// which the standard library arguably reaches: since Go 1.23 FindProcess may
// acquire a pidfd, turning it into a handle probe whose semantics differ by
// kernel version, while this door's contract is a name probe over exactly the
// number a durable record holds. The package forbids os.FindProcess for the
// same reason, so the leaf states the layering call here rather than leaving
// a reviewer to reconstruct it.
//
// A permission refusal is an alive answer, not a failure: the kernel refuses
// to let this caller signal the process, which it only does for a process
// that exists. Every other errno is a failed observation.
func observedLiveness(identity ProcessIdentity) (Liveness, error) {
	pid, err := identity.Int()
	if err != nil {
		return LivenessUnknown, err
	}
	probeErr := unix.Kill(pid, 0)
	if probeErr == nil || errors.Is(probeErr, unix.EPERM) {
		return LivenessAlive, nil
	}
	if errors.Is(probeErr, unix.ESRCH) {
		return LivenessGone, nil
	}
	return LivenessUnknown, errors.Join(core.ErrProcessObservation, probeErr)
}
