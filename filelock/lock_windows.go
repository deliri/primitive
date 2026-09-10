//go:build windows

package filelock

import (
	"errors"
	"golang.org/x/sys/windows"
	"runtime"
)

// The whole native 64-bit byte range, independent of the current file size.
const lockRegionLength = ^uint32(0)

// Windows documents this event-handle bit as suppressing completion-port
// notifications for this operation; Go retains ownership of its IOCP.
const lockPrivateEvent = windows.Handle(1)

func acquire(fd uintptr, exclusivity Exclusivity, patience Patience) (bool, error) {
	flags, err := lockFlags(exclusivity, patience)
	if err != nil {
		return false, err
	}
	handle := windows.Handle(fd)
	contended := false
	err = windowsLockOperation(handle, func(overlapped *windows.Overlapped) error {
		nativeErr := windows.LockFileEx(handle, flags, 0, lockRegionLength, lockRegionLength, overlapped)
		if patience == Immediate && errors.Is(nativeErr, windows.ERROR_LOCK_VIOLATION) {
			contended = true
			return nil
		}
		return nativeErr
	})
	if err != nil {
		return false, err
	}
	return !contended, nil
}
func release(fd uintptr) error {
	handle := windows.Handle(fd)
	return windowsLockOperation(handle, func(overlapped *windows.Overlapped) error {
		return windows.UnlockFileEx(handle, 0, lockRegionLength, lockRegionLength, overlapped)
	})
}

// Wait for native completion before releasing the Go descriptor reference or
// the OVERLAPPED storage. No worker, retry policy or private IO runtime is added.
func windowsLockOperation(handle windows.Handle, operation func(*windows.Overlapped) error) (resultErr error) {
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, windows.CloseHandle(event)) }()
	overlapped := windows.Overlapped{HEvent: event | lockPrivateEvent}
	var pin runtime.Pinner
	pin.Pin(&overlapped)
	defer pin.Unpin()
	resultErr = operation(&overlapped)
	if errors.Is(resultErr, windows.ERROR_IO_PENDING) {
		var transferred uint32
		resultErr = windows.GetOverlappedResult(handle, &overlapped, &transferred, true)
	}
	return resultErr
}

func lockFlags(exclusivity Exclusivity, patience Patience) (uint32, error) {
	var flags uint32
	switch exclusivity {
	case Exclusive:
		flags = windows.LOCKFILE_EXCLUSIVE_LOCK
	case Shared:
		flags = 0
	default:
		return 0, exclusivity.Validate()
	}
	switch patience {
	case Immediate:
		return flags | windows.LOCKFILE_FAIL_IMMEDIATELY, nil
	case Blocking:
		return flags, nil
	default:
		return 0, patience.Validate()
	}
}
