//go:build windows

package hostfacts

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"

	"github.com/deliri/primitive/v2026/core"
)

// observedTerminalGeometry interrogates the handle with
// GetConsoleScreenBufferInfo, the console API's geometry question.
//
// The handle is reached through SyscallConn for the same reason the POSIX
// leaf uses it: the callback guards the handle against concurrent close for
// exactly the duration of the call.
//
// A handle that is not a console answers with ERROR_INVALID_HANDLE, and a
// redirected standard handle answers with ERROR_INVALID_FUNCTION; both mean
// "you are not attached to a console" and are the detached observation. Any
// other failure is reported, because the handle may be a console the caller
// could not interrogate.
func observedTerminalGeometry(file *os.File) (TerminalGeometry, error) {
	conn, err := file.SyscallConn()
	if err != nil {
		return TerminalGeometry{}, fail(OperationTerminalGeometry, core.ErrHostFactsObservation, err)
	}
	var info windows.ConsoleScreenBufferInfo
	var queryErr error
	controlErr := conn.Control(func(fd uintptr) {
		queryErr = windows.GetConsoleScreenBufferInfo(windows.Handle(fd), &info)
	})
	if controlErr != nil {
		return TerminalGeometry{}, fail(OperationTerminalGeometry, core.ErrHostFactsObservation, controlErr)
	}
	if queryErr != nil {
		if errors.Is(queryErr, windows.ERROR_INVALID_HANDLE) || errors.Is(queryErr, windows.ERROR_INVALID_FUNCTION) {
			return newDetachedTerminalGeometry()
		}
		return TerminalGeometry{}, fail(OperationTerminalGeometry, core.ErrHostFactsObservation, queryErr)
	}
	columns := int32(info.Window.Right) - int32(info.Window.Left) + 1
	if columns <= 0 || columns > int32(^TerminalColumns(0)) {
		// The console answered, so attachment was positively observed; only
		// the reported window is unusable as a width.
		return newTerminalWithoutGeometry()
	}
	return newAttachedTerminalGeometry(TerminalColumns(columns))
}
