//go:build !linux

package hostfacts

import (
	"context"
	"testing"
)

func TestPublicWorkloadObservationReturnsValidatedUnsupportedFact(t *testing.T) {
	t.Parallel()

	got, gotErr := ObserveEffectiveWorkloadMemoryLimit(context.Background())
	if gotErr != nil || got.State() != WorkloadMemoryLimitUnsupported ||
		got.Source() != WorkloadMemoryLimitSourceNone || got.Validate() != nil {
		t.Fatalf("ObserveEffectiveWorkloadMemoryLimit() = (%v, %v), want valid unsupported fact", got, gotErr)
	}
}
