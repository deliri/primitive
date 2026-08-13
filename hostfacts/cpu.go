package hostfacts

import (
	"errors"
	"runtime"

	"github.com/deliri/primitive/v2026/core"
)

// LogicalCPUCount is the Go runtime's immutable observation of the logical
// CPUs available to the current process. It is a host fact, not a worker or
// scheduling policy.
type LogicalCPUCount struct {
	count int
}

// Validate rejects an absent logical-CPU observation.
func (c LogicalCPUCount) Validate() error {
	if c.count < 1 {
		return errors.Join(core.ErrHostFactsContract, errors.New("logical CPU count is absent"))
	}
	return nil
}

// Int returns the observed logical-CPU count.
func (c LogicalCPUCount) Int() int {
	return c.count
}

// ObserveLogicalCPUCount reports the logical CPU count exposed by Go's
// runtime. Consumers retain ownership of any worker-budget policy derived
// from this observation.
func ObserveLogicalCPUCount() (LogicalCPUCount, error) {
	count := LogicalCPUCount{count: runtime.NumCPU()}
	if err := count.Validate(); err != nil {
		return LogicalCPUCount{}, fail(OperationLogicalCPUCount, core.ErrHostFactsObservation, err)
	}
	return count, nil
}
