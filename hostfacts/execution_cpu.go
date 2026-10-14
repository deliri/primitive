package hostfacts

import (
	"github.com/deliri/primitive/v2026/core"
	"runtime"
)

// GoCPUConcurrency is Go's current execution budget. It may change when Go
// updates container quota observations or an owner changes GOMAXPROCS. It is
// distinct from the process-start logical CPU count and from worker policy.
type GoCPUConcurrency struct{ count int }

func (c GoCPUConcurrency) Validate() error {
	if c.count <= 0 {
		return core.ErrHostFactsObservation
	}
	return nil
}

func (c GoCPUConcurrency) Int() int { return c.count }

// ObserveGoCPUConcurrency queries Go without changing the process budget.
// Go owns affinity/quota observation, minimums, rounding and explicit overrides.
// Consumers choose their own worker budget from this execution fact.
// https://pkg.go.dev/runtime#GOMAXPROCS
func ObserveGoCPUConcurrency() (GoCPUConcurrency, error) {
	observation := GoCPUConcurrency{count: runtime.GOMAXPROCS(0)}
	if err := observation.Validate(); err != nil {
		return GoCPUConcurrency{}, fail(OperationGoCPUConcurrency, core.ErrHostFactsObservation, err)
	}
	return observation, nil
}
