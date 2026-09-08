//go:build unix

package process

import "github.com/deliri/primitive/v2026/core"

func unixProcessID(identity ProcessIdentity) (int, error) {
	if uint32(identity) > core.ProcessPOSIXIdentityMaximum {
		return 0, contractError("identity exceeds the POSIX process ID domain")
	}
	return identity.Int()
}
