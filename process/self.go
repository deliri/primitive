package process

import "os"

// Self reports the calling process's own identity, the number a lock file or
// diagnostic record names as its writer. It is an observation of this
// process, never a capability over another one: nothing here can signal,
// wait on, or supervise the identity it returns, and ownership decisions
// stay with the advisory mechanism that guards the record, because process
// identifiers are reused.
func Self() (ProcessIdentity, error) {
	return newProcessIdentity(os.Getpid())
}
