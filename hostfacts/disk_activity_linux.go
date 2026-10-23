//go:build linux

package hostfacts

import (
	"context"
	"errors"
	"io/fs"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func observeDiskActivity(ctx context.Context, request DiskActivityRequest) (out DiskActivity, err error) {
	root, err := openRoot(ctx, request.Directory)
	if err != nil {
		return out, err
	}
	defer func() {
		err = errors.Join(err, root.close())
		if err != nil {
			out = DiskActivity{}
		}
	}()
	device, err := activityDevice(ctx, root.dev)
	if err != nil {
		return out, err
	}
	start, err := temporal.Observe()
	if err != nil {
		return out, err
	}
	before, err := readDiskActivity(ctx, device)
	if err != nil {
		return out, err
	}
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx, Duration: request.Interval}); err != nil {
		return out, err
	}
	after, err := readDiskActivity(ctx, device)
	if err != nil {
		return out, err
	}
	end, err := temporal.Observe()
	if err != nil {
		return out, err
	}
	window, err := end.Since(start)
	if err != nil {
		return out, err
	}
	return before.delta(device, after, window)
}

// Resolve only a single backing device. A multi-device RAID/striped layout
// cannot honestly be represented by one utilization percentage.
func activityDevice(ctx context.Context, number uint64) (core.AbsolutePath, error) {
	node, err := core.ParseAbsolutePath(sysDevBlockDirectoryText + "/" + strconv.FormatUint(uint64(linuxDeviceMajor(number)), 10) + ":" + strconv.FormatUint(uint64(linuxDeviceMinor(number)), 10))
	if err != nil {
		return core.AbsolutePath{}, err
	}
	location, err := filestore.OpenParent(ctx, node)
	if err != nil {
		return core.AbsolutePath{}, err
	}
	target, readErr := filestore.ReadSymbolicLink(ctx, location)
	err = errors.Join(readErr, location.Root.Close())
	if errors.Is(err, fs.ErrNotExist) {
		return core.AbsolutePath{}, core.ErrHostFactsUnsupported
	}
	if err != nil {
		return core.AbsolutePath{}, err
	}
	resolved, err := resolveSysfsDeviceDirectory(target.String())
	if err != nil {
		return core.AbsolutePath{}, err
	}
	device, err := core.ParseAbsolutePath(resolved)
	if err != nil {
		return core.AbsolutePath{}, err
	}
	for {
		// A partition's counters omit activity on sibling partitions. Ascend
		// the kernel's partition marker before sampling the entire drive.
		device, err = activityWholeDevice(ctx, device)
		if err != nil {
			return core.AbsolutePath{}, err
		}
		slaves, resolveErr := device.ResolveText("slaves")
		if resolveErr != nil {
			return core.AbsolutePath{}, resolveErr
		}
		root, openErr := filestore.OpenRoot(ctx, slaves)
		if errors.Is(openErr, fs.ErrNotExist) {
			return device, nil
		}
		if openErr != nil {
			return core.AbsolutePath{}, openErr
		}
		dot, dotErr := core.ParseRelativePath(".")
		if dotErr != nil {
			return core.AbsolutePath{}, errors.Join(dotErr, root.Close())
		}
		var child core.AbsolutePath
		count := 0
		err = filestore.Walk(ctx, filestore.WalkRequest{Location: filestore.Location{Root: root, Path: dot}, Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			count++
			if count > 1 {
				return filestore.WalkSkipDirectory, core.ErrHostFactsUnsupported
			}
			link, e := filestore.ReadSymbolicLink(ctx, filestore.Location{Root: root, Path: entry.Path})
			if e != nil {
				return filestore.WalkSkipDirectory, e
			}
			child, e = slaves.ResolveText(link.String())
			return filestore.WalkSkipDirectory, e
		}})
		err = errors.Join(err, root.Close())
		if err != nil {
			return core.AbsolutePath{}, err
		}
		if count == 0 {
			return device, nil
		}
		if child == device {
			return core.AbsolutePath{}, core.ErrHostFactsObservation
		}
		device = child
	}
}

func activityWholeDevice(ctx context.Context, device core.AbsolutePath) (core.AbsolutePath, error) {
	root, err := filestore.OpenRoot(ctx, device)
	if err != nil {
		return core.AbsolutePath{}, err
	}
	partition, err := core.ParseRelativePath("partition")
	if err != nil {
		return core.AbsolutePath{}, errors.Join(err, root.Close())
	}
	entry, inspectErr := filestore.InspectEntry(ctx, filestore.Location{Root: root, Path: partition})
	closeErr := root.Close()
	if errors.Is(inspectErr, fs.ErrNotExist) {
		if closeErr != nil {
			return core.AbsolutePath{}, closeErr
		}
		return device, nil
	}
	err = errors.Join(inspectErr, closeErr)
	if err != nil {
		return core.AbsolutePath{}, err
	}
	kind, err := entry.Kind()
	if err != nil {
		return core.AbsolutePath{}, err
	}
	if kind != filestore.PathKindRegularFile {
		return core.AbsolutePath{}, core.ErrHostFactsObservation
	}
	parent, err := resolveSysfsDeviceParent(device.String())
	if err != nil {
		return core.AbsolutePath{}, err
	}
	return core.ParseAbsolutePath(parent)
}

func readDiskActivity(ctx context.Context, device core.AbsolutePath) (out diskActivityCounters, err error) {
	path, err := device.ResolveText("stat")
	if err != nil {
		return out, err
	}
	location, err := filestore.OpenParent(ctx, path)
	if err != nil {
		return out, err
	}
	defer func() {
		err = errors.Join(err, location.Root.Close())
		if err != nil {
			out = diskActivityCounters{}
		}
	}()
	file, err := filestore.OpenRead(ctx, filestore.ReadHandleRequest{Location: location})
	if err != nil {
		return out, err
	}
	defer func() {
		err = errors.Join(err, file.Close())
		if err != nil {
			out = diskActivityCounters{}
		}
	}()
	return parseDiskActivity(ctx, file)
}
