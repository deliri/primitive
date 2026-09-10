//go:build darwin || linux

package filelock

import (
	"errors"
	"syscall"
)

// acquire performs the one real locking effect on Unix.
//
// EWOULDBLOCK and EAGAIN are the same errno on most Unix systems, but not
// universally, so both are treated as contention. Everything else — EBADF,
// ENOTSUP, ENOLCK, EIO — is a real failure and surfaces immediately. Products
// that classified only EWOULDBLOCK turned an unsupported filesystem into a
// silent timeout whose cause was gone by the time anyone read the log.
//
// EINTR is retried rather than reported. A signal arriving mid-call says
// nothing about who holds the lock, and making every caller loop over that is
// how the same retry ends up written three times.
func acquire(fd uintptr, exclusivity Exclusivity, patience Patience) (bool, error) {
	flags, err := lockFlags(exclusivity, patience)
	if err != nil {
		return false, err
	}
	for {
		flockErr := syscall.Flock(int(fd), flags)
		if flockErr == nil {
			return true, nil
		}
		if errors.Is(flockErr, syscall.EINTR) {
			continue
		}
		if patience == Immediate &&
			(errors.Is(flockErr, syscall.EWOULDBLOCK) || errors.Is(flockErr, syscall.EAGAIN)) {
			return false, nil
		}
		return false, flockErr
	}
}

func lockFlags(exclusivity Exclusivity, patience Patience) (int, error) {
	flags := 0
	switch exclusivity {
	case Exclusive:
		flags = syscall.LOCK_EX
	case Shared:
		flags = syscall.LOCK_SH
	default:
		return 0, exclusivity.Validate()
	}
	switch patience {
	case Immediate:
		return flags | syscall.LOCK_NB, nil
	case Blocking:
		return flags, nil
	default:
		return 0, patience.Validate()
	}
}

func release(fd uintptr) error {
	for {
		err := syscall.Flock(int(fd), syscall.LOCK_UN)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		return err
	}
}
