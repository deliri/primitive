package hostfacts

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestDiskAssessmentValidatorClosesCapacityPolicyAndState(t *testing.T) {
	t.Parallel()

	oneOfTwo := mustDiskCapacityForHostfactsTest(t, 1, 2)
	twoOfThree := mustDiskCapacityForHostfactsTest(t, 2, 3)
	threeOfThree := mustDiskCapacityForHostfactsTest(t, 3, 3)
	disabled := DiskPressurePolicy{FreeSpaceFloor: mustByteLength(t, 0)}
	oneByteFloor := DiskPressurePolicy{FreeSpaceFloor: mustByteLength(t, 1)}
	cases := []struct {
		wantErr    error
		name       string
		assessment DiskAssessment
	}{
		{name: "zero floor seals disabled state", assessment: DiskAssessment{capacity: oneOfTwo, policy: disabled, state: DiskPressureDisabled}},
		{name: "availability at floor seals reached state", assessment: DiskAssessment{capacity: oneOfTwo, policy: oneByteFloor, state: DiskPressureReached}},
		{name: "availability above floor seals healthy state", assessment: DiskAssessment{capacity: twoOfThree, policy: oneByteFloor, state: DiskPressureHealthy}},
		{name: "zero assessment has no observed capacity", assessment: DiskAssessment{}, wantErr: core.ErrHostFactsObservation},
		{name: "unknown state is outside the closed domain", assessment: DiskAssessment{capacity: oneOfTwo, policy: disabled, state: DiskPressureUnknown}, wantErr: core.ErrHostFactsContract},
		{name: "future state is outside the closed domain", assessment: DiskAssessment{capacity: oneOfTwo, policy: disabled, state: DiskPressureState(math.MaxUint8)}, wantErr: core.ErrHostFactsContract},
		{name: "disabled policy cannot claim healthy", assessment: DiskAssessment{capacity: oneOfTwo, policy: disabled, state: DiskPressureHealthy}, wantErr: core.ErrHostFactsContract},
		{name: "disabled policy cannot claim reached", assessment: DiskAssessment{capacity: oneOfTwo, policy: disabled, state: DiskPressureReached}, wantErr: core.ErrHostFactsContract},
		{name: "availability at floor cannot claim healthy", assessment: DiskAssessment{capacity: oneOfTwo, policy: oneByteFloor, state: DiskPressureHealthy}, wantErr: core.ErrHostFactsContract},
		{name: "availability at floor cannot claim disabled", assessment: DiskAssessment{capacity: oneOfTwo, policy: oneByteFloor, state: DiskPressureDisabled}, wantErr: core.ErrHostFactsContract},
		{name: "availability above floor cannot claim reached", assessment: DiskAssessment{capacity: twoOfThree, policy: oneByteFloor, state: DiskPressureReached}, wantErr: core.ErrHostFactsContract},
		{name: "availability above floor cannot claim disabled", assessment: DiskAssessment{capacity: twoOfThree, policy: oneByteFloor, state: DiskPressureDisabled}, wantErr: core.ErrHostFactsContract},
		{name: "available bytes above total are not an observation", assessment: DiskAssessment{capacity: DiskCapacity{available: threeOfThree.available, total: oneOfTwo.total}, policy: disabled, state: DiskPressureDisabled}, wantErr: core.ErrHostFactsObservation},
		{name: "unset total is not an observation", assessment: DiskAssessment{capacity: DiskCapacity{available: mustByteLength(t, 0)}, policy: disabled, state: DiskPressureDisabled}, wantErr: core.ErrHostFactsObservation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.assessment.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("DiskAssessment%+v.Validate() error = %v, want errors.Is(..., %v)", tc.assessment, gotErr, tc.wantErr)
			}
		})
	}
}

func TestGoMemoryAssessmentValidatorClosesSnapshotTriggerPolicyAndState(t *testing.T) {
	t.Parallel()

	percent50, err := NewPercent(50)
	if err != nil {
		t.Fatalf("NewPercent(50) error = %v, want nil", err)
	}
	policy := GoMemoryPressurePolicy{TriggerPercent: percent50}
	snapshot, err := newGoMemorySnapshot(50, 0, 100)
	if err != nil {
		t.Fatalf("newGoMemorySnapshot(50, 0, 100) error = %v, want nil", err)
	}
	trigger, err := policy.TriggerBytes(snapshot.LimitBytes())
	if err != nil {
		t.Fatalf("GoMemoryPressurePolicy.TriggerBytes() error = %v, want nil", err)
	}
	percent49, err := NewPercent(49)
	if err != nil {
		t.Fatalf("NewPercent(49) error = %v, want nil", err)
	}
	wrongTrigger, err := (GoMemoryPressurePolicy{TriggerPercent: percent49}).TriggerBytes(snapshot.LimitBytes())
	if err != nil {
		t.Fatalf("GoMemoryPressurePolicy(49).TriggerBytes() error = %v, want nil", err)
	}
	overSignedLimit, err := core.NewByteCount(math.MaxInt64 + 1)
	if err != nil {
		t.Fatalf("core.NewByteCount(MaxInt64+1) error = %v, want nil", err)
	}
	cases := []struct {
		wantErr    error
		name       string
		assessment GoMemoryAssessment
	}{
		{name: "exact trigger seals reached state", assessment: GoMemoryAssessment{snapshot: snapshot, trigger: trigger, policy: policy, state: MemoryPressureReached}},
		{name: "zero assessment has no runtime observation", assessment: GoMemoryAssessment{}, wantErr: core.ErrHostFactsObservation},
		{name: "unknown state is outside the closed domain", assessment: GoMemoryAssessment{snapshot: snapshot, trigger: trigger, policy: policy, state: MemoryPressureUnknown}, wantErr: core.ErrHostFactsContract},
		{name: "future state is outside the closed domain", assessment: GoMemoryAssessment{snapshot: snapshot, trigger: trigger, policy: policy, state: MemoryPressureState(math.MaxUint8)}, wantErr: core.ErrHostFactsContract},
		{name: "exact trigger cannot claim healthy", assessment: GoMemoryAssessment{snapshot: snapshot, trigger: trigger, policy: policy, state: MemoryPressureHealthy}, wantErr: core.ErrHostFactsContract},
		{name: "exact trigger cannot claim disabled", assessment: GoMemoryAssessment{snapshot: snapshot, trigger: trigger, policy: policy, state: MemoryPressureDisabled}, wantErr: core.ErrHostFactsContract},
		{name: "zero trigger is not a projected byte count", assessment: GoMemoryAssessment{snapshot: snapshot, policy: policy, state: MemoryPressureReached}, wantErr: core.ErrPrimitiveContract},
		{name: "trigger from another percentage is contradictory", assessment: GoMemoryAssessment{snapshot: snapshot, trigger: wrongTrigger, policy: policy, state: MemoryPressureReached}, wantErr: core.ErrHostFactsContract},
		{name: "zero snapshot system is not an observation", assessment: GoMemoryAssessment{snapshot: GoMemorySnapshot{limit: snapshot.limit}, trigger: trigger, policy: policy, state: MemoryPressureReached}, wantErr: core.ErrHostFactsObservation},
		{name: "released bytes above system are contradictory", assessment: GoMemoryAssessment{snapshot: GoMemorySnapshot{system: snapshot.system, heapReleased: mustByteLength(t, 51), managed: mustByteLength(t, 0), limit: snapshot.limit}, trigger: trigger, policy: policy, state: MemoryPressureReached}, wantErr: core.ErrHostFactsObservation},
		{name: "managed bytes must equal system minus released", assessment: GoMemoryAssessment{snapshot: GoMemorySnapshot{system: snapshot.system, heapReleased: snapshot.heapReleased, managed: mustByteLength(t, 49), limit: snapshot.limit}, trigger: trigger, policy: policy, state: MemoryPressureReached}, wantErr: core.ErrHostFactsObservation},
		{name: "unset runtime limit is not an observation", assessment: GoMemoryAssessment{snapshot: GoMemorySnapshot{system: snapshot.system, heapReleased: snapshot.heapReleased, managed: snapshot.managed}, trigger: trigger, policy: policy, state: MemoryPressureReached}, wantErr: core.ErrHostFactsObservation},
		{name: "runtime limit above signed domain is not observable", assessment: GoMemoryAssessment{snapshot: GoMemorySnapshot{system: snapshot.system, heapReleased: snapshot.heapReleased, managed: snapshot.managed, limit: overSignedLimit}, trigger: trigger, policy: policy, state: MemoryPressureReached}, wantErr: core.ErrHostFactsObservation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.assessment.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("GoMemoryAssessment%+v.Validate() error = %v, want errors.Is(..., %v)", tc.assessment, gotErr, tc.wantErr)
			}
		})
	}
}

func TestWorkloadMemoryLimitValidatorClosesStateSourcePathAndExtent(t *testing.T) {
	t.Parallel()

	v2Path := mustAbsolutePathForHostfactsTest(t, "/sys/fs/cgroup/memory.max")
	v1Path := mustAbsolutePathForHostfactsTest(t, "/sys/fs/cgroup/memory/memory.limit_in_bytes")
	wrongPath := mustAbsolutePathForHostfactsTest(t, "/sys/fs/cgroup/not-a-limit")
	oneByte := mustByteLength(t, 1)
	cases := []struct {
		wantErr error
		name    string
		limit   WorkloadMemoryLimit
	}{
		{name: "unsupported carries no source path or extent", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitUnsupported, source: WorkloadMemoryLimitSourceNone}},
		{name: "unavailable carries no source path or extent", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitUnavailable, source: WorkloadMemoryLimitSourceNone}},
		{name: "v2 unlimited carries its exact interface", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitUnlimited, source: WorkloadMemoryLimitSourceCgroupV2, path: v2Path}},
		{name: "v1 unlimited carries its exact interface", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitUnlimited, source: WorkloadMemoryLimitSourceCgroupV1, path: v1Path}},
		{name: "v2 finite zero is exact evidence", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitLimited, source: WorkloadMemoryLimitSourceCgroupV2, path: v2Path}},
		{name: "v1 finite positive extent is exact evidence", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitLimited, source: WorkloadMemoryLimitSourceCgroupV1, path: v1Path, limit: oneByte}},
		{name: "zero value has unknown state and source", limit: WorkloadMemoryLimit{}, wantErr: core.ErrHostFactsContract},
		{name: "future state is outside the closed domain", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitState(math.MaxUint8), source: WorkloadMemoryLimitSourceNone}, wantErr: core.ErrHostFactsContract},
		{name: "future source is outside the closed domain", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitUnavailable, source: WorkloadMemoryLimitSource(math.MaxUint8)}, wantErr: core.ErrHostFactsContract},
		{name: "unsupported cannot carry a kernel source", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitUnsupported, source: WorkloadMemoryLimitSourceCgroupV2}, wantErr: core.ErrHostFactsContract},
		{name: "unsupported cannot carry an interface path", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitUnsupported, source: WorkloadMemoryLimitSourceNone, path: v2Path}, wantErr: core.ErrHostFactsContract},
		{name: "unsupported cannot carry an extent", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitUnsupported, source: WorkloadMemoryLimitSourceNone, limit: oneByte}, wantErr: core.ErrHostFactsContract},
		{name: "unavailable cannot carry a kernel source", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitUnavailable, source: WorkloadMemoryLimitSourceCgroupV1}, wantErr: core.ErrHostFactsContract},
		{name: "unavailable cannot carry an interface path", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitUnavailable, source: WorkloadMemoryLimitSourceNone, path: v1Path}, wantErr: core.ErrHostFactsContract},
		{name: "unavailable cannot carry an extent", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitUnavailable, source: WorkloadMemoryLimitSourceNone, limit: oneByte}, wantErr: core.ErrHostFactsContract},
		{name: "unlimited requires a kernel source", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitUnlimited, source: WorkloadMemoryLimitSourceNone, path: v2Path}, wantErr: core.ErrHostFactsContract},
		{name: "unlimited requires an interface path", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitUnlimited, source: WorkloadMemoryLimitSourceCgroupV2}, wantErr: core.ErrHostFactsContract},
		{name: "unlimited cannot carry a finite extent", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitUnlimited, source: WorkloadMemoryLimitSourceCgroupV2, path: v2Path, limit: oneByte}, wantErr: core.ErrHostFactsContract},
		{name: "limited requires a kernel source", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitLimited, source: WorkloadMemoryLimitSourceNone, path: v2Path}, wantErr: core.ErrHostFactsContract},
		{name: "limited requires an interface path", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitLimited, source: WorkloadMemoryLimitSourceCgroupV2, limit: oneByte}, wantErr: core.ErrHostFactsContract},
		{name: "v2 source refuses the v1 interface name", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitLimited, source: WorkloadMemoryLimitSourceCgroupV2, path: v1Path, limit: oneByte}, wantErr: core.ErrHostFactsContract},
		{name: "v1 source refuses the v2 interface name", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitLimited, source: WorkloadMemoryLimitSourceCgroupV1, path: v2Path, limit: oneByte}, wantErr: core.ErrHostFactsContract},
		{name: "v2 source refuses an unrelated interface name", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitLimited, source: WorkloadMemoryLimitSourceCgroupV2, path: wrongPath, limit: oneByte}, wantErr: core.ErrHostFactsContract},
		{name: "v1 source refuses an unrelated interface name", limit: WorkloadMemoryLimit{state: WorkloadMemoryLimitUnlimited, source: WorkloadMemoryLimitSourceCgroupV1, path: wrongPath}, wantErr: core.ErrHostFactsContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.limit.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("WorkloadMemoryLimit%+v.Validate() error = %v, want errors.Is(..., %v)", tc.limit, gotErr, tc.wantErr)
			}
		})
	}
}

func TestGoOOMBannerEvidenceValidatorAdmitsOnlyClassifierProvenance(t *testing.T) {
	t.Parallel()

	minimumPresence := uint64(len(GoOOMPlainBanner))
	cases := []struct {
		wantErr error
		name    string
		extent  uint64
		state   GoOOMBannerState
	}{
		{name: "zero-byte absence is classifier provenance", extent: 0, state: GoOOMBannerAbsent},
		{name: "one-byte absence is classifier provenance", extent: 1, state: GoOOMBannerAbsent},
		{name: "one below shortest banner remains absence", extent: minimumPresence - 1, state: GoOOMBannerAbsent},
		{name: "shortest banner extent admits absence", extent: minimumPresence, state: GoOOMBannerAbsent},
		{name: "shortest banner extent admits presence", extent: minimumPresence, state: GoOOMBannerPresent},
		{name: "maximum classifier extent admits absence", extent: GoOOMMaximumEvidenceBytes, state: GoOOMBannerAbsent},
		{name: "maximum classifier extent admits presence", extent: GoOOMMaximumEvidenceBytes, state: GoOOMBannerPresent},
		{name: "unknown state is never classifier evidence", extent: 0, state: GoOOMBannerUnknown, wantErr: core.ErrHostFactsEvidence},
		{name: "future state is never classifier evidence", extent: 0, state: GoOOMBannerState(math.MaxUint8), wantErr: core.ErrHostFactsEvidence},
		{name: "zero bytes cannot contain the shortest banner", extent: 0, state: GoOOMBannerPresent, wantErr: core.ErrHostFactsEvidence},
		{name: "one byte cannot contain the shortest banner", extent: 1, state: GoOOMBannerPresent, wantErr: core.ErrHostFactsEvidence},
		{name: "two below shortest banner cannot prove presence", extent: minimumPresence - 2, state: GoOOMBannerPresent, wantErr: core.ErrHostFactsEvidence},
		{name: "one below shortest banner cannot prove presence", extent: minimumPresence - 1, state: GoOOMBannerPresent, wantErr: core.ErrHostFactsEvidence},
		{name: "one over classifier extent cannot be evidence", extent: GoOOMMaximumEvidenceBytes + 1, state: GoOOMBannerAbsent, wantErr: core.ErrHostFactsEvidence},
		{name: "two over classifier extent cannot be evidence", extent: GoOOMMaximumEvidenceBytes + 2, state: GoOOMBannerPresent, wantErr: core.ErrHostFactsEvidence},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			evidence := GoOOMBannerEvidence{examined: mustByteLength(t, tc.extent), state: tc.state}
			gotErr := evidence.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("GoOOMBannerEvidence%+v.Validate() error = %v, want errors.Is(..., %v)", evidence, gotErr, tc.wantErr)
			}
		})
	}
}

func mustDiskCapacityForHostfactsTest(t testing.TB, available, total uint64) DiskCapacity {
	t.Helper()

	capacity, err := newDiskCapacity(available, total)
	if err != nil {
		t.Fatalf("newDiskCapacity(%d, %d) error = %v, want nil", available, total, err)
	}
	return capacity
}
