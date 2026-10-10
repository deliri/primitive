//go:build unix && !linux

package hostfacts

import (
	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/sys/unix"
)

func nativeTerminalColumns(fd uintptr) (uint16, error) {
	window, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	if err != nil {
		return 0, err
	}
	if window == nil {
		return 0, core.ErrHostFactsObservation
	}
	return window.Col, nil
}
