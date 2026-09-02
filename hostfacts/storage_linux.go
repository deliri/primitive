//go:build linux

package hostfacts

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"golang.org/x/sys/unix"
)

const (
	// rotationalInterfaceRelativeText names the queue interface below a
	// block device's own sysfs directory.
	rotationalInterfaceRelativeText = "queue/rotational"
)

// observeDiskRotation reads the rotational flag of the block device backing
// one held directory. The device identity comes from the same held
// capability every other disk observation opens.
func observeDiskRotation(ctx context.Context, directory core.AbsolutePath) (DiskRotation, error) {
	root, err := openRoot(ctx, directory)
	if err != nil {
		return DiskRotationUnknown, failRootOpen(OperationOpenRoot, core.ErrHostFactsObservation, err)
	}
	device := root.dev
	if err := root.close(); err != nil {
		return DiskRotationUnknown, fail(OperationDiskRotation, core.ErrHostFactsObservation, err)
	}
	return rotationForDevice(ctx, device)
}

// rotationForDevice resolves one device number through the kernel index. A
// device the index does not name is not a block device at all, an overlay,
// network, or otherwise synthetic filesystem, and that is the unavailable
// observation rather than a failure.
func rotationForDevice(ctx context.Context, device uint64) (DiskRotation, error) {
	node := sysDevBlockDirectoryText + "/" +
		strconv.FormatUint(uint64(unix.Major(device)), 10) + ":" +
		strconv.FormatUint(uint64(unix.Minor(device)), 10)
	nodePath, err := core.ParseAbsolutePath(node)
	if err != nil {
		return DiskRotationUnknown, fail(OperationDiskRotation, core.ErrHostFactsObservation, err)
	}
	location, err := filestore.OpenParent(ctx, nodePath)
	if err != nil {
		return DiskRotationUnknown, fail(OperationDiskRotation, core.ErrHostFactsObservation, err)
	}
	target, readErr := filestore.ReadSymbolicLink(ctx, location)
	err = errors.Join(readErr, location.Root.Close())
	if errors.Is(err, fs.ErrNotExist) {
		return DiskRotationUnavailable, nil
	}
	if err != nil {
		return DiskRotationUnknown, fail(OperationDiskRotation, core.ErrHostFactsObservation, err)
	}
	resolved, err := resolveSysfsDeviceDirectory(target.String())
	if err != nil {
		return DiskRotationUnknown, fail(OperationDiskRotation, core.ErrHostFactsObservation, err)
	}
	return rotationAtDeviceDirectory(ctx, resolved)
}

// rotationAtDeviceDirectory reads the flag beside the resolved device, then
// beside its parent: a whole disk carries its own queue interface and a
// partition sits one level below the disk that does. A device with neither
// exposes no rotation fact to observe.
func rotationAtDeviceDirectory(ctx context.Context, deviceDirectory string) (DiskRotation, error) {
	rotation, err := readRotationalFlag(ctx, deviceDirectory)
	if errors.Is(err, fs.ErrNotExist) {
		parent, parentErr := resolveSysfsDeviceParent(deviceDirectory)
		if parentErr != nil {
			return DiskRotationUnknown, fail(OperationDiskRotation, core.ErrHostFactsObservation, parentErr)
		}
		rotation, err = readRotationalFlag(ctx, parent)
	}
	if errors.Is(err, fs.ErrNotExist) {
		return DiskRotationUnavailable, nil
	}
	if err != nil {
		return DiskRotationUnknown, fail(OperationDiskRotation, core.ErrHostFactsObservation, err)
	}
	return rotation, nil
}

// readRotationalFlag reads and classifies one queue interface. Absence
// flows to the caller undecorated so the disk-or-partition fallback can
// tell "no interface here" from every other refusal.
func readRotationalFlag(ctx context.Context, deviceDirectory string) (DiskRotation, error) {
	interfacePath, err := core.ParseAbsolutePath(filepath.Join(deviceDirectory, rotationalInterfaceRelativeText))
	if err != nil {
		return DiskRotationUnknown, errors.Join(core.ErrHostFactsObservation, err)
	}
	data, err := readVirtualValue(ctx, virtualFileRequest{
		Path: interfacePath, MaximumBytes: rotationalFlagMaximumBytes,
	})
	if err != nil {
		return DiskRotationUnknown, err
	}
	return classifyRotationalFlag(data)
}
