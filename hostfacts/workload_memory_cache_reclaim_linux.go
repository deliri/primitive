//go:build linux

package hostfacts

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func reclaimIdleWorkloadMemoryCache(ctx context.Context, maximum core.ByteLength) (result WorkloadMemoryHeadroom, resultErr error) {
	budget, err := temporal.DurationFromSeconds(resourceCommandWaitSeconds)
	if err != nil {
		return result, err
	}
	bounded, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: budget})
	if err != nil {
		return result, err
	}
	defer cancel()
	data, err := readWorkloadMemoryScopeParent(bounded)
	if err != nil {
		return result, err
	}
	proc, err := procSelfCgroupPath()
	if err != nil {
		return result, err
	}
	mounts, err := procSelfMountsPath()
	if err != nil {
		return result, err
	}
	files := workloadMemoryScopeParentFiles{membership: proc, mounts: mounts}
	selection, present, err := selectWorkloadMemoryScopeParent(bounded, files, data)
	if err != nil {
		return result, err
	}
	if !present {
		return unavailableWorkloadMemoryHeadroom()
	}
	before, err := foldWorkloadMemoryHeadroom(bounded, selection.prospective, selection.mount)
	if err != nil {
		return result, err
	}
	if before.binding.state != WorkloadMemoryLimitLimited || before.binding.source != WorkloadMemoryLimitSourceCgroupV2 {
		return before, nil
	}
	directory, err := before.binding.path.Parent()
	if err != nil {
		return result, err
	}
	root, err := filestore.OpenRoot(bounded, directory)
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, filestore.CloseRoot(filestore.RootHandleRequest{Root: root}))
	}()
	// Do not use a stale caller path or a second namespace to authorize the write.
	if err := recheckCgroupMembership(bounded, files.membership, selection.current); err != nil {
		return result, err
	}
	afterTarget, err := readWorkloadMemoryScopeParent(bounded)
	if err != nil {
		return result, err
	}
	if err := sameWorkloadMemoryScopeParent(data, afterTarget); err != nil {
		return result, err
	}
	_, reclaimErr := reclaimEmptyCgroupCache(bounded, root, maximum.Uint64())
	// Read the same binding even on EAGAIN; requested bytes are never a credit.
	after, observeErr := foldWorkloadMemoryHeadroom(bounded, selection.prospective, selection.mount)
	identityErr := recheckCgroupMembership(bounded, files.membership, selection.current)
	afterTarget, targetErr := readWorkloadMemoryScopeParent(bounded)
	if targetErr == nil {
		targetErr = sameWorkloadMemoryScopeParent(data, afterTarget)
	}
	if err := errors.Join(observeErr, identityErr, targetErr, contextstate.Validate(bounded)); err != nil {
		return result, errors.Join(reclaimErr, err)
	}
	if after.binding.path != before.binding.path || after.binding.limit != before.binding.limit || after.binding.source != before.binding.source {
		return result, errors.Join(reclaimErr, core.ErrHostFactsObservation)
	}
	return after, reclaimErr
}
