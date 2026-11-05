//go:build !linux

package hostfacts

import (
	"context"
	"github.com/deliri/primitive/v2026/core"
)

func reclaimIdleWorkloadMemoryCache(ctx context.Context, _ core.ByteLength) (WorkloadMemoryHeadroom, error) {
	return ObserveWorkloadMemoryScopeParent(ctx)
}
