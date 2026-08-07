//go:build windows

package process

import (
	"errors"

	"golang.org/x/sys/windows"

	"github.com/deliri/primitive/v2026/core"
)

// observedLiveness asks the kernel by opening the process for the least
// query access and reading whether it still runs.
//
// An open refused for permissions is an alive answer: the kernel only
// protects a process that exists. An invalid-parameter refusal is the gone
// answer, because it is how OpenProcess names an identity no process
// carries. Every other failure is a failed observation.
func observedLiveness(identity ProcessIdentity) (Liveness, error) {
	pid, err := identity.Int()
	if err != nil {
		return LivenessUnknown, err
	}
	handle, openErr := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		uint32(pid), // #nosec G115 -- ProcessIdentity admits only positive pid-domain values.
	)
	if openErr != nil {
		if errors.Is(openErr, windows.ERROR_ACCESS_DENIED) {
			return LivenessAlive, nil
		}
		if errors.Is(openErr, windows.ERROR_INVALID_PARAMETER) {
			return LivenessGone, nil
		}
		return LivenessUnknown, errors.Join(core.ErrProcessObservation, openErr)
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	// STATUS_PENDING is the value the console API names STILL_ACTIVE: the
	// exit code a process reports for as long as it has not exited.
	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return LivenessUnknown, errors.Join(core.ErrProcessObservation, err)
	}
	if exitCode == uint32(windows.STATUS_PENDING) {
		return LivenessAlive, nil
	}
	return LivenessGone, nil
}
