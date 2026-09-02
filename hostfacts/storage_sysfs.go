package hostfacts

import (
	"io/fs"
	"path"
	"strings"
)

const (
	// sysFilesystemRootText confines kernel storage observations to sysfs.
	sysFilesystemRootText = "/sys"
	// sysDevBlockDirectoryText is the kernel's own index from device number
	// to block device, the one mapping that needs no mount-table text or
	// device-name heuristic to resolve.
	sysDevBlockDirectoryText = sysFilesystemRootText + "/dev/block"
)

func resolveSysfsDeviceDirectory(target string) (string, error) {
	if target == "" {
		return "", fs.ErrInvalid
	}
	resolved := target
	if !path.IsAbs(resolved) {
		resolved = path.Join(sysDevBlockDirectoryText, resolved)
	}
	resolved = path.Clean(resolved)
	if !strings.HasPrefix(resolved, sysFilesystemRootText+"/") {
		return "", fs.ErrInvalid
	}
	return resolved, nil
}

func resolveSysfsDeviceParent(deviceDirectory string) (string, error) {
	return resolveSysfsDeviceDirectory(path.Dir(deviceDirectory))
}
