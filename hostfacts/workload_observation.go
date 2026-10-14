package hostfacts

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// This is the native production observation with explicit proc-file paths.
// Tests supply owned native files; no fabricated observation callback exists.
func observeWorkloadMemoryAt(ctx context.Context, procPath, mountsPath core.AbsolutePath) (WorkloadMemoryLimit, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return WorkloadMemoryLimit{}, err
	}
	if err := errors.Join(procPath.Validate(), mountsPath.Validate()); err != nil {
		return WorkloadMemoryLimit{}, err
	}
	membership, found, err := observeCgroupMembership(ctx, procPath)
	if err != nil {
		return WorkloadMemoryLimit{}, fail(OperationCgroupMembership, core.ErrHostFactsObservation, err)
	}
	if !found {
		return unavailableWorkloadMemoryLimit()
	}
	mount, err := selectCgroupMount(ctx, mountsPath, membership)
	if errors.Is(err, core.ErrCgroupMountMissing) {
		return unavailableWorkloadMemoryLimit()
	}
	if err != nil {
		return WorkloadMemoryLimit{}, fail(OperationCgroupMount, core.ErrHostFactsObservation, err)
	}
	result, err := foldCgroupLimits(ctx, membership, mount)
	if err != nil {
		return WorkloadMemoryLimit{}, fail(OperationCgroupLimit, core.ErrHostFactsObservation, err)
	}
	if err := recheckCgroupMembership(ctx, procPath, membership); err != nil {
		return WorkloadMemoryLimit{}, fail(OperationCgroupMembership, core.ErrHostFactsObservation, err)
	}
	return result, result.Validate()
}

func recheckCgroupMembership(ctx context.Context, procPath core.AbsolutePath, membership cgroupMembership) error {
	after, found, err := observeCgroupMembership(ctx, procPath)
	if err != nil {
		return err
	}
	if !found {
		return core.ErrCgroupMembershipDisappeared
	}
	if after != membership {
		return core.ErrCgroupMembershipChanged
	}
	return nil
}

func selectCgroupMount(ctx context.Context, mountsPath core.AbsolutePath, membership cgroupMembership) (cgroupMount, error) {
	var selection cgroupMountSelection
	err := scanVirtualLines(ctx, virtualFileRequest{Path: mountsPath, MaximumBytes: virtualFileMaximumBytes}, func(line []byte) error {
		mount, matches, parseErr := parseMountInfoLine(line, membership.source)
		if parseErr != nil {
			return parseErr
		}
		if matches {
			return selection.consider(mount, membership)
		}
		return nil
	})
	if err != nil {
		return cgroupMount{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	if selection.selected.source == WorkloadMemoryLimitSourceUnknown {
		return cgroupMount{}, core.ErrCgroupMountMissing
	}
	return selection.selected, selection.selected.Validate()
}
