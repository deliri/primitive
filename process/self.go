package process

import "os"

// Self reports the calling process's own identity, the number a lock file or
// diagnostic record names as its writer. It is an observation of this
// process, never a capability over another one: nothing here can signal,
// wait on, or supervise the identity it returns, and ownership decisions
// stay with the advisory mechanism that guards the record, because process
// identifiers are reused.
func Self() (ProcessIdentity, error) {
	identity := ProcessIdentity(os.Getpid()) // #nosec G115 -- the platform reports the calling process's identifier inside the platform pid domain.
	if err := identity.Validate(); err != nil {
		return 0, err
	}
	return identity, nil
}
