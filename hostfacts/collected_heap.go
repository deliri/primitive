package hostfacts

import (
	"context"
	"errors"
	"runtime"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ObserveCollectedGoHeap requests a synchronous Go garbage collection and
// observes runtime.MemStats.HeapAlloc afterward. The caller owns the reason
// for collection and any memory budget. This is a process-wide observation;
// other allocations can change the heap while the caller evaluates it.
// Go owns collection scheduling. Cancellation is checked before collection
// and after observation; it cannot interrupt the runtime's collection itself.
func ObserveCollectedGoHeap(ctx context.Context) (core.ByteLength, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return core.ByteLength{}, err
	}
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	if err := contextstate.Validate(ctx); err != nil {
		return core.ByteLength{}, err
	}
	heap, err := core.NewByteLength(stats.HeapAlloc)
	if err != nil {
		return core.ByteLength{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return heap, nil
}
