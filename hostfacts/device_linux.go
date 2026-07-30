//go:build linux

package hostfacts

import "golang.org/x/sys/unix"

func deviceIdentity(stat unix.Stat_t) uint64 {
	return uint64(stat.Dev)
}
