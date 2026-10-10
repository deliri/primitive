//go:build linux

package hostfacts

import (
	"syscall"
	"unsafe"
)

func nativeTerminalColumns(fd uintptr) (uint16, error) {
	var window terminalWindow
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&window)))
	if errno != 0 {
		return 0, errno
	}
	return window.Columns, nil
}
