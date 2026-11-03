package hostfacts

import (
	"context"

	"github.com/deliri/primitive/v2026/contextstate"
)

// ObserveWorkloadMemoryScopeParent observes the current native ancestry of
// app.slice, the default parent used by systemd-run --user --scope without a
// --slice override (including process's capped command). It queries the user
// manager's actual ControlGroup and rechecks it after the bounded read. It
// neither creates a scope nor reserves capacity for a future command.
//
// An absent target, inaccessible namespace or absent memory interface is
// unavailable, never an invented unlimited limit. Unsupported platforms return
// Unsupported. A caller must reobserve before admission and retain other jobs'
// full reservations independently of this changing charge observation.
func ObserveWorkloadMemoryScopeParent(ctx context.Context) (WorkloadMemoryHeadroom, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	return observeWorkloadMemoryScopeParent(ctx)
}
