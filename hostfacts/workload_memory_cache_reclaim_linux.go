//go:build linux

package hostfacts

import (
	"context"
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func reclaimIdleWorkloadMemoryCache(ctx context.Context, maximum core.ByteLength) (WorkloadMemoryHeadroom, error) {
	budget, err := temporal.DurationFromSeconds(resourceCommandWaitSeconds)
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	bounded, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: budget})
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	defer cancel()
	return reclaimObservedScopeCache(bounded, maximum)
}

func reclaimObservedScopeCache(ctx context.Context, maximum core.ByteLength) (result WorkloadMemoryHeadroom, resultErr error) {
	before, err := ObserveWorkloadMemoryScopeParent(ctx)
	if err != nil {
		return result, err
	}
	if before.binding.state != WorkloadMemoryLimitLimited || before.binding.source != WorkloadMemoryLimitSourceCgroupV2 {
		return before, nil
	}
	root, err := openObservedCacheScope(ctx, before)
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, filestore.CloseRoot(filestore.RootHandleRequest{Root: root}))
	}()
	_, reclaimErr := reclaimEmptyCgroupCache(ctx, root, maximum.Uint64())
	// Reuse the complete native ancestry observer, even after partial reclaim.
	// No requested amount is turned into an admission credit.
	after, observeErr := ObserveWorkloadMemoryScopeParent(ctx)
	if observeErr != nil {
		return result, errors.Join(reclaimErr, observeErr)
	}
	if !sameCacheScopeBinding(before, after) {
		return result, errors.Join(reclaimErr, core.ErrHostFactsObservation)
	}
	return after, reclaimErr
}

func openObservedCacheScope(ctx context.Context, before WorkloadMemoryHeadroom) (*os.Root, error) {
	directory, err := before.binding.path.Parent()
	if err != nil {
		return nil, err
	}
	root, err := filestore.OpenRoot(ctx, directory)
	if err != nil {
		return nil, err
	}
	current, observeErr := ObserveWorkloadMemoryScopeParent(ctx)
	identityErr := filestore.ValidateRootIdentity(root, directory)
	if observeErr == nil && identityErr == nil && sameCacheScopeBinding(before, current) {
		return root, nil
	}
	return nil, errors.Join(core.ErrHostFactsObservation, observeErr, identityErr, filestore.CloseRoot(filestore.RootHandleRequest{Root: root}))
}

func sameCacheScopeBinding(before, after WorkloadMemoryHeadroom) bool {
	return before.binding.path == after.binding.path && before.binding.limit == after.binding.limit && before.binding.source == after.binding.source && before.binding.state == after.binding.state
}
