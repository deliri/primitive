//go:build unix

package hostfacts

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"

	"github.com/deliri/primitive/v2026/core"
)

// observedTerminalGeometry interrogates the descriptor with the TIOCGWINSZ
// ioctl, the one question a POSIX terminal answers about its own geometry.
//
// The descriptor is reached through SyscallConn rather than Fd because Fd has
// a side effect: it switches the file into blocking mode for the rest of its
// life. An observation must not change the thing it observes, and the
// control callback guards the descriptor against concurrent close for
// exactly the duration of the ioctl.
//
// A descriptor that is not a terminal is an observation, not a failure: the
// kernel names that case with a specific errno family, and "you are piped"
// is exactly what a renderer asking for a width needs to know. Every other
// errno is a failure, because the descriptor may well be a terminal the
// caller could not interrogate, and recording it as detached would be an
// observation the caller was never allowed to make.
func observedTerminalGeometry(file *os.File) (TerminalGeometry, error) {
	conn, err := file.SyscallConn()
	if err != nil {
		return TerminalGeometry{}, fail(OperationTerminalGeometry, core.ErrHostFactsObservation, err)
	}
	var window *unix.Winsize
	var ioctlErr error
	controlErr := conn.Control(func(fd uintptr) {
		window, ioctlErr = unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	})
	if controlErr != nil {
		return TerminalGeometry{}, fail(OperationTerminalGeometry, core.ErrHostFactsObservation, controlErr)
	}
	if ioctlErr != nil {
		if errnoSaysNotATerminal(ioctlErr) {
			return newDetachedTerminalGeometry()
		}
		return TerminalGeometry{}, fail(OperationTerminalGeometry, core.ErrHostFactsObservation, ioctlErr)
	}
	if window == nil {
		return TerminalGeometry{}, fail(OperationTerminalGeometry, core.ErrHostFactsObservation,
			errors.New("winsize ioctl answered without a window"))
	}
	if window.Col == 0 {
		return newTerminalWithoutGeometry()
	}
	return newAttachedTerminalGeometry(TerminalColumns(window.Col))
}

// errnoSaysNotATerminal names the errno family kernels use to answer "this
// descriptor has no terminal behind it": ENOTTY from every POSIX system,
// ENODEV and ENXIO from BSD device layers, and EOPNOTSUPP from descriptors
// whose driver refuses terminal control entirely.
func errnoSaysNotATerminal(err error) bool {
	return errors.Is(err, unix.ENOTTY) ||
		errors.Is(err, unix.ENODEV) ||
		errors.Is(err, unix.ENXIO) ||
		errors.Is(err, unix.EOPNOTSUPP)
}
