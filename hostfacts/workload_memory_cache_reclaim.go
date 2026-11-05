package hostfacts

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ReclaimIdleWorkloadMemoryCache requests at most maximum bytes of file-cache
// reclamation from one currently empty cgroup below the actual prospective
// scope's binding ancestor. It never changes limits, moves or signals processes,
// writes global drop_caches, or promises that requested bytes became available.
// Only a group with enough inactive file cache and no anonymous, shared, dirty
// or writeback memory is eligible. Native membership can change concurrently;
// emptiness is observed again immediately before the cache-only request.
//
// The returned headroom is a fresh native observation, including after a partial
// reclaim error when possible. EAGAIN and other native causes remain available
// through errors.Is/As. Callers must retain other reservations and reobserve all
// admission facts; this operation does not reserve any reclaimed capacity.
// Unsupported platforms and unavailable non-v2 bindings do not reclaim.
func ReclaimIdleWorkloadMemoryCache(ctx context.Context, maximum core.ByteLength) (result WorkloadMemoryHeadroom, resultErr error) {
	defer func() {
		if resultErr != nil && ctx != nil {
			resultErr = errors.Join(resultErr, context.Cause(ctx))
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), maximum.Validate()); err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	if maximum.Uint64() == 0 {
		return WorkloadMemoryHeadroom{}, core.ErrHostFactsContract
	}
	return reclaimIdleWorkloadMemoryCache(ctx, maximum)
}
