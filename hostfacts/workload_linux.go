//go:build linux

package hostfacts

import (
	"context"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

const (
	procSelfCgroupValue = "/proc/self/cgroup"
	procSelfMountsValue = "/proc/self/mountinfo"
)

func procSelfCgroupPath() (core.AbsolutePath, error) {
	return core.ParseAbsolutePath(procSelfCgroupValue)
}

func procSelfMountsPath() (core.AbsolutePath, error) {
	return core.ParseAbsolutePath(procSelfMountsValue)
}

func observeEffectiveWorkloadMemoryLimit(ctx context.Context) (WorkloadMemoryLimit, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return WorkloadMemoryLimit{}, err
	}
	procPath, err := procSelfCgroupPath()
	if err != nil {
		return WorkloadMemoryLimit{}, err
	}
	membership, found, err := observeCgroupMembership(ctx, procPath)
	if err != nil {
		return WorkloadMemoryLimit{}, fail(OperationCgroupMembership, core.ErrHostFactsObservation, err)
	}
	if !found {
		return unavailableWorkloadMemoryLimit()
	}
	mount, err := observeCgroupMount(ctx, membership)
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

func observeCgroupMount(ctx context.Context, membership cgroupMembership) (cgroupMount, error) {
	mountsPath, err := procSelfMountsPath()
	if err != nil {
		return cgroupMount{}, err
	}
	return selectCgroupMount(ctx, mountsPath, membership)
}
