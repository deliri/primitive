package hostfacts

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestCgroupNamespaceScopeExhaustsClosedDomain(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		scope := cgroupMembershipScope(raw)
		got := scope.Validate()
		if raw < 2 {
			if got != nil || !scope.IsValid() || scope.String() == core.UnknownEnumDiagnostic {
				t.Fatalf("scope %d = %v/%q, want valid diagnostic", raw, got, scope.String())
			}
		} else if !errors.Is(got, core.ErrHostFactsObservation) || scope.IsValid() || scope.String() != core.UnknownEnumDiagnostic {
			t.Fatalf("future scope %d = %v/%q, want typed refusal", raw, got, scope.String())
		}
	}
	for _, membership := range []cgroupMembership{
		{path: "/job", source: WorkloadMemoryLimitSourceCgroupV2, scope: cgroupMembershipOutside},
		{path: "/../..", source: WorkloadMemoryLimitSourceCgroupV2, scope: cgroupMembershipInside},
		{path: "/../a/../b", source: WorkloadMemoryLimitSourceCgroupV2, scope: cgroupMembershipOutside},
	} {
		if err := membership.Validate(); !errors.Is(err, core.ErrHostFactsObservation) {
			t.Fatalf("contradictory membership = %+v/%v, want refusal", membership, err)
		}
	}
}
