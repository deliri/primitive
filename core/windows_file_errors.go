package core

import "syscall"

// These native error identities match Go's internal/syscall/windows constants.
// syscall exposes Errno but does not export these two Windows file refusals.
// Keeping their identity here lets OS boundaries preserve errors.Is without an
// external syscall implementation or copied numbers in consuming packages.
// The named Windows identities are available on every build; native execution
// and interpretation remain in the Windows implementation of the OS boundary.
const (
	WindowsFileSharingViolation syscall.Errno = 32
	WindowsFileLockViolation    syscall.Errno = 33
)
