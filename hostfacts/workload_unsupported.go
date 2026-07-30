//go:build !linux

package hostfacts

import (
	"context"

	"github.com/deliri/primitive/v2026/contextstate"
)

func observeEffectiveWorkloadMemoryLimit(ctx context.Context) (WorkloadMemoryLimit, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return WorkloadMemoryLimit{}, err
	}
	return unsupportedWorkloadMemoryLimit()
}
