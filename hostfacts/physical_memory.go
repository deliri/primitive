package hostfacts

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// PhysicalMemory is the operating system's current total physical-memory fact.
// It is immutable after observation and excludes runtime and workload limits,
// which have separate owners in this package.
type PhysicalMemory struct {
	total core.ByteLength
}

// Validate rejects an absent or numerically invalid physical-memory fact.
func (m PhysicalMemory) Validate() error {
	if err := m.total.Validate(); err != nil {
		return errors.Join(core.ErrHostFactsContract, err)
	}
	if m.total.Uint64() == 0 {
		return errors.Join(core.ErrHostFactsContract, errors.New("physical memory total is absent"))
	}
	return nil
}

// TotalBytes returns the observed total physical-memory extent.
func (m PhysicalMemory) TotalBytes() core.ByteLength {
	return m.total
}

// ObservePhysicalMemory observes total physical memory through the operating
// system interface owned by the current platform.
func ObservePhysicalMemory(ctx context.Context) (PhysicalMemory, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return PhysicalMemory{}, err
	}
	total, err := observePhysicalMemory()
	if err != nil {
		return PhysicalMemory{}, fail(OperationPhysicalMemory, core.ErrHostFactsObservation, err)
	}
	if total == 0 {
		return PhysicalMemory{}, fail(
			OperationPhysicalMemory,
			core.ErrHostFactsObservation,
			errors.New("operating system reported zero physical memory"),
		)
	}
	length, err := core.NewByteLength(total)
	if err != nil {
		return PhysicalMemory{}, fail(OperationPhysicalMemory, core.ErrHostFactsObservation, err)
	}
	memory := PhysicalMemory{total: length}
	if err := memory.Validate(); err != nil {
		return PhysicalMemory{}, err
	}
	return memory, nil
}
