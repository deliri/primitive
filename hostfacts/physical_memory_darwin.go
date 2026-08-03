//go:build darwin

package hostfacts

import "golang.org/x/sys/unix"

func observePhysicalMemory() (uint64, error) {
	return unix.SysctlUint64("hw.memsize")
}
