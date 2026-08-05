//go:build windows

package filelock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// lockRegionLength covers the whole file regardless of its size. Windows locks
// byte ranges rather than files, so the maximum range is how a range lock is
// spelled as a file lock.
const lockRegionLength = ^uint32(0)

// acquire performs the one real locking effect on Windows.
//
// ERROR_LOCK_VIOLATION is what LockFileEx returns instead of blocking when
// LOCKFILE_FAIL_IMMEDIATELY is set and another process holds the range. It is
// contention, not failure, and is the exact counterpart of EWOULDBLOCK on Unix.
func acquire(file *os.File, exclusivity Exclusivity, patience Patience) (bool, error) {
	flags, err := lockFlags(exclusivity, patience)
	if err != nil {
		return false, err
	}
	overlapped := new(windows.Overlapped)
	err = windows.LockFileEx(
		windows.Handle(file.Fd()),
		flags,
		0,
		lockRegionLength,
		lockRegionLength,
		overlapped,
	)
	if err == nil {
		return true, nil
	}
	if patience == Immediate && errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return false, nil
	}
	return false, err
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

func release(file *os.File) error {
	overlapped := new(windows.Overlapped)
	return windows.UnlockFileEx(
		windows.Handle(file.Fd()),
		0,
		lockRegionLength,
		lockRegionLength,
		overlapped,
	)
}
