package hostfacts

import (
	"context"
	"errors"
	"strconv"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

const cgroupV2CurrentName = "memory.current"
const cgroupV1CurrentName = "memory.usage_in_bytes"

func observeWorkloadMemoryHeadroomAt(ctx context.Context, procPath, mountsPath core.AbsolutePath) (WorkloadMemoryHeadroom, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	if err := errors.Join(procPath.Validate(), mountsPath.Validate()); err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	membership, found, err := observeCgroupMembership(ctx, procPath)
	if err != nil {
		return WorkloadMemoryHeadroom{}, fail(OperationCgroupMembership, core.ErrHostFactsObservation, err)
	}
	if !found {
		binding, e := unavailableWorkloadMemoryLimit()
		return WorkloadMemoryHeadroom{binding: binding}, e
	}
	mount, err := selectCgroupMount(ctx, mountsPath, membership)
	if errors.Is(err, core.ErrCgroupMountMissing) {
		binding, e := unavailableWorkloadMemoryLimit()
		return WorkloadMemoryHeadroom{binding: binding}, e
	}
	if err != nil {
		return WorkloadMemoryHeadroom{}, fail(OperationCgroupMount, core.ErrHostFactsObservation, err)
	}
	result, err := foldWorkloadMemoryHeadroom(ctx, membership, mount)
	if err != nil {
		return WorkloadMemoryHeadroom{}, fail(OperationCgroupLimit, core.ErrHostFactsObservation, err)
	}
	if err := recheckCgroupMembership(ctx, procPath, membership); err != nil {
		return WorkloadMemoryHeadroom{}, fail(OperationCgroupMembership, core.ErrHostFactsObservation, err)
	}
	return validateWorkloadMemoryHeadroom(result)
}

func foldWorkloadMemoryHeadroom(ctx context.Context, membership cgroupMembership, mount cgroupMount) (WorkloadMemoryHeadroom, error) {
	if err := errors.Join(membership.Validate(), mount.Validate()); err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	if membership.source != mount.source {
		return WorkloadMemoryHeadroom{}, core.ErrHostFactsObservation
	}
	directory, err := resolveCgroupDirectory(membership.path, mount)
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	limit, err := limitComponent(membership.source)
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	binding, err := unavailableWorkloadMemoryLimit()
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	result := WorkloadMemoryHeadroom{binding: binding}
	for {
		candidate, err := readWorkloadMemoryHeadroomLevel(ctx, directory, limit, membership.source)
		if err != nil {
			return WorkloadMemoryHeadroom{}, err
		}
		if candidate.binding.state == WorkloadMemoryLimitLimited && (result.binding.state != WorkloadMemoryLimitLimited || candidate.remaining.Uint64() < result.remaining.Uint64()) || result.binding.state == WorkloadMemoryLimitUnavailable && candidate.binding.state == WorkloadMemoryLimitUnlimited {
			result = candidate
		}
		if directory == mount.mountPoint {
			return validateWorkloadMemoryHeadroom(result)
		}
		parent, err := directory.Parent()
		if err != nil {
			return WorkloadMemoryHeadroom{}, err
		}
		if parent == directory {
			return WorkloadMemoryHeadroom{}, core.ErrCgroupContainment
		}
		directory = parent
	}
}

// Both limit reads and the charge read use one pinned native ancestor root.
// A changed limit refuses instead of pairing different limit generations.
func readWorkloadMemoryHeadroomLevel(ctx context.Context, directory core.AbsolutePath, component core.PathComponent, source WorkloadMemoryLimitSource) (result WorkloadMemoryHeadroom, resultErr error) {
	path, err := directory.Join(component)
	if err != nil {
		return result, err
	}
	location, err := filestore.OpenParent(ctx, path)
	if err != nil {
		return result, err
	}
	defer func() {
		if err := location.Root.Close(); err != nil {
			result = WorkloadMemoryHeadroom{}
			resultErr = errors.Join(resultErr, err)
		}
	}()
	level, err := readCgroupLevelAt(ctx, location, source)
	if err != nil {
		return result, err
	}
	switch level.state {
	case cgroupLevelLimitAbsent:
		binding, e := unavailableWorkloadMemoryLimit()
		return WorkloadMemoryHeadroom{binding: binding}, e
	case cgroupLevelLimitUnlimited:
		return validateWorkloadMemoryHeadroom(WorkloadMemoryHeadroom{binding: WorkloadMemoryLimit{path: path, state: WorkloadMemoryLimitUnlimited, source: source}})
	case cgroupLevelLimitFinite:
		name := cgroupV2CurrentName
		if source == WorkloadMemoryLimitSourceCgroupV1 {
			name = cgroupV1CurrentName
		}
		relative, err := core.ParseRelativePath(name)
		if err != nil {
			return result, err
		}
		file, err := filestore.OpenRead(ctx, filestore.ReadHandleRequest{Location: filestore.Location{Root: location.Root, Path: relative}})
		if err != nil {
			return result, err
		}
		raw, err := readVirtualFile(ctx, file, limitValueMaximumBytes)
		if err != nil {
			return result, err
		}
		current, err := parseWorkloadMemoryCharge(raw)
		if err != nil {
			return result, err
		}
		after, err := readCgroupLevelAt(ctx, location, source)
		if err != nil {
			return result, err
		}
		if after != level {
			return result, core.ErrHostFactsObservation
		}
		limit, err := core.NewByteLength(level.value)
		if err != nil {
			return result, err
		}
		remaining := uint64(0)
		if current.Uint64() < level.value {
			remaining = level.value - current.Uint64()
		}
		available, err := core.NewByteLength(remaining)
		if err != nil {
			return result, err
		}
		return validateWorkloadMemoryHeadroom(WorkloadMemoryHeadroom{binding: WorkloadMemoryLimit{path: path, limit: limit, state: WorkloadMemoryLimitLimited, source: source}, current: current, remaining: available})
	default:
		return result, core.ErrHostFactsObservation
	}
}

func parseWorkloadMemoryCharge(data []byte) (core.ByteLength, error) {
	if len(data) > limitValueMaximumBytes {
		return core.ByteLength{}, core.ErrHostFactsObservation
	}
	token, err := canonicalVirtualValueToken(data)
	if err != nil {
		return core.ByteLength{}, err
	}
	value, err := strconv.ParseUint(token, 10, 64)
	if err != nil || strconv.FormatUint(value, 10) != token {
		return core.ByteLength{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	bytes, err := core.NewByteLength(value)
	if err != nil {
		return core.ByteLength{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return bytes, nil
}
