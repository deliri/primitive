//go:build linux

package hostfacts

import (
	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/sys/unix"
)

func (r *platformRoot) diskCapacity() (DiskCapacity, error) {
	var stat unix.Statfs_t
	if err := unix.Fstatfs(r.fd, &stat); err != nil {
		return DiskCapacity{}, err
	}
	fragmentBytes := stat.Frsize
	if fragmentBytes <= 0 {
		fragmentBytes = stat.Bsize
	}
	if fragmentBytes <= 0 {
		return DiskCapacity{}, core.ErrHostFactsObservation
	}
	unit := uint64(fragmentBytes) // #nosec G115 -- positive value checked above.
	available, err := blocksToBytes(stat.Bavail, unit)
	if err != nil {
		return DiskCapacity{}, err
	}
	total, err := blocksToBytes(stat.Blocks, unit)
	if err != nil {
		return DiskCapacity{}, err
	}
	return newDiskCapacity(available, total)
}
