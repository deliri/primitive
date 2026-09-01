//go:build android || linux

package filestore

import (
	"path/filepath"
	"strconv"
)

func openRootDescriptorPath(descriptor int) string {
	return filepath.Join("/proc/self/fd", strconv.Itoa(descriptor))
}

// Opening /proc/self/fd/N duplicates N, so the acquisition descriptor remains
// separately owned here and must be closed after os.Root has its own handle.
func closeRootDescriptorAfterOpen() bool { return true }
