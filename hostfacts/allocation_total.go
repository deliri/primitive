package hostfacts

import (
	"context"
	"errors"
	"runtime"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ObserveGoAllocationTotal observes runtime.MemStats.TotalAlloc: cumulative
// bytes allocated in the current Go process, including subsequently reclaimed
// objects. It does not collect garbage. Callers own sample comparison and
// budgets; concurrent allocations remain part of the process observation.
func ObserveGoAllocationTotal(ctx context.Context) (core.ByteLength, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return core.ByteLength{}, err
	}
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	if err := contextstate.Validate(ctx); err != nil {
		return core.ByteLength{}, err
	}
	total, err := core.NewByteLength(stats.TotalAlloc)
	if err != nil {
		return core.ByteLength{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return total, nil
}
