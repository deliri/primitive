//go:build windows

package filestore

import (
	"errors"

	"golang.org/x/sys/windows"

	"github.com/deliri/primitive/v2026/core"
)

// observeSharing probes with a zero-share read open: success proves no
// exclusive holder at that moment, and the two contention refusals prove one.
// Every other outcome is a failed observation with the native cause kept.
func observeSharing(path core.AbsolutePath) (Sharing, error) {
	name, err := windows.UTF16PtrFromString(path.String())
	if err != nil {
		return SharingUnknown, sourceError(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err == nil {
		if closeErr := windows.CloseHandle(handle); closeErr != nil {
			return SharingUnknown, sourceError(closeErr)
		}
		return SharingAvailable, nil
	}
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return SharingHeld, nil
	}
	return SharingUnknown, sourceError(err)
}
