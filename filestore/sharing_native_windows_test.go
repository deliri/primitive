//go:build windows

package filestore_test

import (
	"errors"
	"syscall"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Native control performs the actual Windows probe, without Primitive wrappers.
// Named Windows tables independently pin Available/Held/refusal classification.
func nativeSharingProbe(path string) (filestore.Sharing, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return filestore.SharingUnknown, err
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err == nil {
		return filestore.SharingAvailable, syscall.CloseHandle(handle)
	}
	if errors.Is(err, core.WindowsFileSharingViolation) || errors.Is(err, core.WindowsFileLockViolation) {
		return filestore.SharingHeld, nil
	}
	return filestore.SharingUnknown, err
}
