//go:build windows

package process

import (
	"errors"
	"fmt"
	"syscall"

	"github.com/deliri/primitive/v2026/core"
)

// observedLiveness probes the process handle with Go's native wait API.
// An exit code cannot prove liveness: a terminated child can have exit code
// STILL_ACTIVE. A zero timeout observes the handle without blocking.
func observedLiveness(identity ProcessIdentity) (liveness Liveness, resultErr error) {
	pid, err := identity.Int()
	if err != nil {
		return LivenessUnknown, err
	}
	handle, openErr := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if openErr != nil {
		return livenessFromOpenError(openErr)
	}
	defer func() {
		if err := syscall.CloseHandle(handle); err != nil {
			liveness = LivenessUnknown
			resultErr = errors.Join(core.ErrProcessObservation, resultErr, err)
		}
	}()
	status, waitErr := syscall.WaitForSingleObject(handle, 0)
	return livenessFromWait(status, waitErr)
}

func livenessFromOpenError(err error) (Liveness, error) {
	if errors.Is(err, syscall.ERROR_ACCESS_DENIED) {
		return LivenessAlive, nil
	}
	if errors.Is(err, core.WindowsProcessInvalidParameter) {
		return LivenessGone, nil
	}
	return LivenessUnknown, errors.Join(core.ErrProcessObservation, err)
}

func livenessFromWait(status uint32, err error) (Liveness, error) {
	if err != nil {
		return LivenessUnknown, errors.Join(core.ErrProcessObservation, err)
	}
	switch status {
	case syscall.WAIT_OBJECT_0:
		return LivenessGone, nil
	case syscall.WAIT_TIMEOUT:
		return LivenessAlive, nil
	default:
		return LivenessUnknown, fmt.Errorf("process wait returned status %d: %w", status, core.ErrProcessObservation)
	}
}
