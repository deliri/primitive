//go:build windows

package core

import "syscall"

// These native error identities match Go's internal/syscall/windows constants.
// syscall exposes Errno but does not export these two Windows file refusals.
// Keeping their identity here lets OS boundaries preserve errors.Is without an
// external syscall implementation or copied numbers in consuming packages.
const (
	WindowsFileSharingViolation syscall.Errno = 32
	WindowsFileLockViolation    syscall.Errno = 33
)
