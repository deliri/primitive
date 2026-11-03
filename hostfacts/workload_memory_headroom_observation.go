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
	if err := validateWorkloadMemoryHeadroomMount(membership, mount); err != nil {
		return WorkloadMemoryHeadroom{}, err
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
		var candidate WorkloadMemoryHeadroom
		candidate, err = readWorkloadMemoryHeadroomLevel(ctx, directory, limit, membership.source)
		if err != nil {
			return WorkloadMemoryHeadroom{}, err
		}
		result = tighterWorkloadMemoryHeadroom(result, candidate)
		if directory == mount.mountPoint {
			return validateWorkloadMemoryHeadroom(result)
		}
		parent, parentErr := directory.Parent()
		if parentErr != nil {
			return WorkloadMemoryHeadroom{}, parentErr
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
		if err := filestore.CloseRoot(filestore.RootHandleRequest{Root: location.Root}); err != nil {
			result = WorkloadMemoryHeadroom{}
			resultErr = errors.Join(resultErr, err)
		}
	}()
	return readPinnedWorkloadMemoryHeadroom(ctx, location, WorkloadMemoryLimit{path: path, source: source})
}

func readPinnedWorkloadMemoryHeadroom(ctx context.Context, location filestore.Location, binding WorkloadMemoryLimit) (WorkloadMemoryHeadroom, error) {
	level, err := readCgroupLevelAt(ctx, location, binding.source)
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	switch level.state {
	case cgroupLevelLimitAbsent:
		unavailable, unavailableErr := unavailableWorkloadMemoryLimit()
		return WorkloadMemoryHeadroom{binding: unavailable}, unavailableErr
	case cgroupLevelLimitUnlimited:
		binding.state = WorkloadMemoryLimitUnlimited
		return validateWorkloadMemoryHeadroom(WorkloadMemoryHeadroom{binding: binding})
	case cgroupLevelLimitFinite:
		current, chargeErr := readWorkloadMemoryChargeAt(ctx, location, binding.source)
		if chargeErr != nil {
			return WorkloadMemoryHeadroom{}, chargeErr
		}
		after, afterErr := readCgroupLevelAt(ctx, location, binding.source)
		if afterErr != nil {
			return WorkloadMemoryHeadroom{}, afterErr
		}
		if after != level {
			return WorkloadMemoryHeadroom{}, core.ErrHostFactsObservation
		}
		binding.state = WorkloadMemoryLimitLimited
		return finiteWorkloadMemoryHeadroom(binding, level.value, current)
	default:
		return WorkloadMemoryHeadroom{}, core.ErrHostFactsObservation
	}
}

func validateWorkloadMemoryHeadroomMount(membership cgroupMembership, mount cgroupMount) error {
	if err := errors.Join(membership.Validate(), mount.Validate()); err != nil {
		return err
	}
	if membership.source != mount.source {
		return core.ErrHostFactsObservation
	}
	return nil
}

func tighterWorkloadMemoryHeadroom(current, candidate WorkloadMemoryHeadroom) WorkloadMemoryHeadroom {
	if candidate.binding.state == WorkloadMemoryLimitLimited {
		if current.binding.state != WorkloadMemoryLimitLimited || candidate.remaining.Uint64() < current.remaining.Uint64() {
			return candidate
		}
	}
	if current.binding.state == WorkloadMemoryLimitUnavailable && candidate.binding.state == WorkloadMemoryLimitUnlimited {
		return candidate
	}
	return current
}

func readWorkloadMemoryChargeAt(ctx context.Context, location filestore.Location, source WorkloadMemoryLimitSource) (core.ByteLength, error) {
	name := cgroupV2CurrentName
	if source == WorkloadMemoryLimitSourceCgroupV1 {
		name = cgroupV1CurrentName
	}
	relative, err := core.ParseRelativePath(name)
	if err != nil {
		return core.ByteLength{}, err
	}
	file, err := filestore.OpenRead(ctx, filestore.ReadHandleRequest{Location: filestore.Location{Root: location.Root, Path: relative}})
	if err != nil {
		return core.ByteLength{}, err
	}
	raw, err := readVirtualFile(ctx, file, limitValueMaximumBytes)
	if err != nil {
		return core.ByteLength{}, err
	}
	return parseWorkloadMemoryCharge(raw)
}

func finiteWorkloadMemoryHeadroom(binding WorkloadMemoryLimit, value uint64, current core.ByteLength) (WorkloadMemoryHeadroom, error) {
	limit, err := core.NewByteLength(value)
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	remaining := uint64(0)
	if current.Uint64() < value {
		remaining = value - current.Uint64()
	}
	available, err := core.NewByteLength(remaining)
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	binding.limit = limit
	return validateWorkloadMemoryHeadroom(WorkloadMemoryHeadroom{binding: binding, current: current, remaining: available})
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
