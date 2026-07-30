//go:build darwin

package hostfacts

import "golang.org/x/sys/unix"

func deviceIdentity(stat unix.Stat_t) uint64 {
	return uint64(uint32(stat.Dev)) // #nosec G115 -- dev_t is used as an opaque bit pattern.
}
