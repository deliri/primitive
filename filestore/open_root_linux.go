//go:build linux

package filestore

import (
	"path/filepath"
	"strconv"
)

func openRootDescriptorPath(descriptor int) string {
	return filepath.Join("/proc/self/fd", strconv.Itoa(descriptor))
}
