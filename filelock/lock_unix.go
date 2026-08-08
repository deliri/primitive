//go:build unix

package filelock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
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
func acquire(file *os.File, exclusivity Exclusivity, patience Patience) (bool, error) {
	flags, err := lockFlags(exclusivity, patience)
	if err != nil {
		return false, err
	}
	for {
		flockErr := unix.Flock(int(file.Fd()), flags)
		if flockErr == nil {
			return true, nil
		}
		if errors.Is(flockErr, unix.EINTR) {
			continue
		}
		if patience == Immediate &&
			(errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN)) {
			return false, nil
		}
		return false, err
	}
}

func lockFlags(exclusivity Exclusivity, patience Patience) (int, error) {
	flags := 0
	switch exclusivity {
	case Exclusive:
		flags = unix.LOCK_EX
	case Shared:
		flags = unix.LOCK_SH
	default:
		return 0, exclusivity.Validate()
	}
	switch patience {
	case Immediate:
		return flags | unix.LOCK_NB, nil
	case Blocking:
		return flags, nil
	default:
		return 0, patience.Validate()
	}
}

func release(file *os.File) error {
	for {
		err := unix.Flock(int(file.Fd()), unix.LOCK_UN)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		return err
	}
}
