//go:build linux

package hostfacts

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const workloadMemoryUserScopeParent = "app.slice"

func observeWorkloadMemoryScopeParent(ctx context.Context) (WorkloadMemoryHeadroom, error) {
	budget, err := temporal.DurationFromSeconds(resourceCommandWaitSeconds)
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	bounded, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: budget})
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	defer cancel()
	before, err := readWorkloadMemoryScopeParent(bounded)
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	proc, err := procSelfCgroupPath()
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	mounts, err := procSelfMountsPath()
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	result, err := observeWorkloadMemoryScopeParentAt(bounded, workloadMemoryScopeParentFiles{membership: proc, mounts: mounts}, before)
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	after, err := readWorkloadMemoryScopeParent(bounded)
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	if err := sameWorkloadMemoryScopeParent(before, after); err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	if err := contextstate.Validate(bounded); err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	return result, result.Validate()
}

func readWorkloadMemoryScopeParent(ctx context.Context) ([]byte, error) {
	data, err := resourceCommand(ctx, "systemctl", "--user", "show", "--property=ControlGroup", "--value", "--", workloadMemoryUserScopeParent)
	if err != nil {
		return nil, errors.Join(core.ErrHostFactsObservation, err)
	}
	return data, nil
}
