//go:build windows

package filestore

import (
	"errors"

	"syscall"

	"github.com/deliri/primitive/v2026/core"
)

// observeSharing probes with a zero-share read open: success proves no
// exclusive holder at that moment, and the two contention refusals prove one.
// Every other outcome is a failed observation with the native cause kept.
func observeSharing(path core.AbsolutePath) (Sharing, error) {
	name, err := syscall.UTF16PtrFromString(path.String())
	if err != nil {
		return SharingUnknown, sourceError(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err == nil {
		if closeErr := syscall.CloseHandle(handle); closeErr != nil {
			return SharingUnknown, sourceError(closeErr)
		}
		return SharingAvailable, nil
	}
	if errors.Is(err, core.WindowsFileSharingViolation) || errors.Is(err, core.WindowsFileLockViolation) {
		return SharingHeld, nil
	}
	return SharingUnknown, sourceError(err)
}
