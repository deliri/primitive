//go:build darwin

package filestore

import (
	"path/filepath"
	"strconv"
)

func openRootDescriptorPath(descriptor int) string {
	return filepath.Join("/dev/fd", strconv.Itoa(descriptor))
}
