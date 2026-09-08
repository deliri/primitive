package core

import "syscall"

// Process exit observations preserve Go's signaled marker and the complete
// Windows unsigned exit-code range in one portable signed representation.
const (
	ProcessExitCodeSignaled int64 = -1
	ProcessExitCodeSuccess  int64 = 0
	ProcessExitCodeMaximum  int64 = 1<<32 - 1
)

// WindowsProcessInvalidParameter matches Go's syscall _ERROR_INVALID_PARAMETER.
// The standard library keeps this native identity private; Process needs it
// to distinguish an absent process from a refused observation.
const WindowsProcessInvalidParameter syscall.Errno = 87

// ProcessPOSIXIdentityMaximum prevents a portable unsigned process identity
// from narrowing into a negative pid_t (a process-group or all-process selector).
const ProcessPOSIXIdentityMaximum uint32 = 1<<31 - 1
