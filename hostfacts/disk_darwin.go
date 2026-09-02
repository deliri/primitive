//go:build darwin

package hostfacts

import (
	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/sys/unix"
)

func (r *platformRoot) diskCapacity() (DiskCapacity, error) {
	file, err := r.directory.File()
	if err != nil {
		return DiskCapacity{}, err
	}
	var stat unix.Statfs_t
	if err := unix.Fstatfs(int(file.Fd()), &stat); err != nil {
		return DiskCapacity{}, err
	}
	if stat.Bsize <= 0 {
		return DiskCapacity{}, core.ErrHostFactsObservation
	}
	blockBytes := uint64(stat.Bsize) // #nosec G115 -- positive value checked above.
	available, err := blocksToBytes(stat.Bavail, blockBytes)
	if err != nil {
		return DiskCapacity{}, err
	}
	total, err := blocksToBytes(stat.Blocks, blockBytes)
	if err != nil {
		return DiskCapacity{}, err
	}
	return newDiskCapacity(available, total)
}
