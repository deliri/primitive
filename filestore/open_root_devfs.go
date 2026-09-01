//go:build aix || darwin || dragonfly || freebsd || illumos || ios || netbsd || openbsd || solaris

package filestore

import (
	"path/filepath"
	"strconv"
)

func openRootDescriptorPath(descriptor int) string {
	return filepath.Join("/dev/fd", strconv.Itoa(descriptor))
}

// Opening /dev/fd/N duplicates the descriptor for os.Root. The acquisition
// descriptor remains independently owned here and must be closed.
func closeRootDescriptorAfterOpen() bool { return true }
