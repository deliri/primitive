package hostfacts

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// WorkloadMemoryHeadroom is the least remaining charge capacity across the
// current cgroup membership's visible ancestors. The binding limit/current
// come from the same native directory. This is an observation, not a lease,
// future capacity promise, host-memory fact or process-RSS estimate.
type WorkloadMemoryHeadroom struct {
	binding   WorkloadMemoryLimit
	current   core.ByteLength
	remaining core.ByteLength
}

func (v WorkloadMemoryHeadroom) Validate() error {
	if err := errors.Join(v.binding.Validate(), v.current.Validate(), v.remaining.Validate()); err != nil {
		return err
	}
	if v.binding.state != WorkloadMemoryLimitLimited {
		if v.current.Uint64() != 0 || v.remaining.Uint64() != 0 {
			return core.ErrHostFactsContract
		}
		return nil
	}
	limit, current := v.binding.limit.Uint64(), v.current.Uint64()
	want := uint64(0)
	if current < limit {
		want = limit - current
	}
	if v.remaining.Uint64() != want {
		return core.ErrHostFactsContract
	}
	return nil
}

// BindingLimit names the ancestor that supplied the least remaining capacity;
// it need not be the ancestor with the smallest absolute hard limit.
func (v WorkloadMemoryHeadroom) BindingLimit() WorkloadMemoryLimit { return v.binding }

// CurrentBytes returns the binding ancestor's native charge, not process RSS.
func (v WorkloadMemoryHeadroom) CurrentBytes() (core.ByteLength, bool) {
	return v.current, v.binding.state == WorkloadMemoryLimitLimited
}

// RemainingBytes returns saturating limit-current. A present zero means the
// finite binding ancestor has no remaining capacity, including over-limit use.
func (v WorkloadMemoryHeadroom) RemainingBytes() (core.ByteLength, bool) {
	return v.remaining, v.binding.state == WorkloadMemoryLimitLimited
}

// ObserveWorkloadMemoryHeadroom reads the current native cgroup ancestry on
// Linux. Unsupported, unavailable and unlimited remain distinct observations.
// Context, malformed counters, missing finite charge files and membership
// changes refuse without a partial capacity fact. Callers own admission policy
// and must reobserve before committing new work.
func ObserveWorkloadMemoryHeadroom(ctx context.Context) (WorkloadMemoryHeadroom, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	return observeWorkloadMemoryHeadroom(ctx)
}

func validateWorkloadMemoryHeadroom(v WorkloadMemoryHeadroom) (WorkloadMemoryHeadroom, error) {
	if err := v.Validate(); err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	return v, nil
}

var _ core.Validatable = WorkloadMemoryHeadroom{}
