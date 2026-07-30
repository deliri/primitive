//go:build linux

package hostfacts

import (
	"context"
	"errors"
	"os"
	"sync"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

const (
	procSelfCgroupValue    = "/proc/self/cgroup"
	procSelfMountsValue    = "/proc/self/mountinfo"
	procCgroupMaximumBytes = 64 << 10
)

var (
	procSelfCgroupPath = sync.OnceValues(func() (core.AbsolutePath, error) {
		return core.ParseAbsolutePath(procSelfCgroupValue)
	})
	procSelfMountsPath = sync.OnceValues(func() (core.AbsolutePath, error) {
		return core.ParseAbsolutePath(procSelfMountsValue)
	})
)

func observeEffectiveWorkloadMemoryLimit(ctx context.Context) (WorkloadMemoryLimit, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return WorkloadMemoryLimit{}, err
	}
	membership, found, err := observeCgroupMembership(ctx)
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
	after, afterFound, err := observeCgroupMembership(ctx)
	if err != nil || !afterFound || after != membership {
		return WorkloadMemoryLimit{}, fail(OperationCgroupMembership, core.ErrHostFactsObservation, err)
	}
	return result, result.Validate()
}

func observeCgroupMembership(ctx context.Context) (cgroupMembership, bool, error) {
	procPath, err := procSelfCgroupPath()
	if err != nil {
		return cgroupMembership{}, false, err
	}
	var v2, v1 cgroupMembership
	v2Count, v1Count := 0, 0
	err = scanVirtualLines(ctx, virtualFileRequest{Path: procPath, MaximumBytes: procCgroupMaximumBytes}, func(line []byte) error {
		membership, parseErr := parseCgroupMembershipLine(line)
		if parseErr != nil {
			return parseErr
		}
		switch membership.source {
		case WorkloadMemoryLimitSourceCgroupV2:
			v2, v2Count = membership, v2Count+1
		case WorkloadMemoryLimitSourceCgroupV1:
			v1, v1Count = membership, v1Count+1
		}
		return nil
	})
	if err != nil || v2Count > 1 || v1Count > 1 {
		return cgroupMembership{}, false, errors.Join(core.ErrHostFactsObservation, err)
	}
	if v2Count == 1 {
		return v2, true, v2.Validate()
	}
	if v1Count == 1 {
		return v1, true, v1.Validate()
	}
	return cgroupMembership{}, false, nil
}

func observeCgroupMount(ctx context.Context, membership cgroupMembership) (cgroupMount, error) {
	mountsPath, err := procSelfMountsPath()
	if err != nil {
		return cgroupMount{}, err
	}
	var selection cgroupMountSelection
	err = scanVirtualLines(ctx, virtualFileRequest{Path: mountsPath, MaximumBytes: virtualFileMaximumBytes}, func(line []byte) error {
		mount, matches, parseErr := parseMountInfoLine(line, membership.source)
		if parseErr != nil {
			return parseErr
		}
		if matches {
			return selection.consider(mount, membership)
		}
		return nil
	})
	if err != nil || selection.count != 1 {
		return cgroupMount{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return selection.selected, selection.selected.Validate()
}

func scanVirtualLines(
	ctx context.Context,
	request virtualFileRequest,
	visit func([]byte) error,
) error {
	if err := request.Validate(); err != nil {
		return err
	}
	closed := false
	file, err := os.Open(request.Path.String())
	if err != nil {
		return err
	}
	defer func() {
		if !closed {
			_ = file.Close()
		}
	}()
	scanErr := (boundedLineScan{
		reader: file, maximum: request.MaximumBytes, visit: visit,
	}).run(ctx)
	closeErr := file.Close()
	closed = true
	return errors.Join(scanErr, closeErr)
}
