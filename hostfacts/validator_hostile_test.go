package hostfacts

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestOwnerValidatorsRejectImpossibleCrossingShapes(t *testing.T) {
	t.Parallel()

	capacity, err := newDiskCapacity(1, 2)
	if err != nil {
		t.Fatalf("newDiskCapacity(1, 2) error = %v, want nil", err)
	}
	contradictoryDisk := DiskAssessment{
		capacity: capacity,
		policy: DiskPressurePolicy{
			FreeSpaceFloor: core.NewByteLength(1),
		},
		state: DiskPressureHealthy,
	}
	if gotErr := contradictoryDisk.Validate(); !errors.Is(gotErr, core.ErrHostFactsContract) {
		t.Fatalf("contradictory DiskAssessment.Validate() error = %v, want %v", gotErr, core.ErrHostFactsContract)
	}

	percent, err := NewPercent(50)
	if err != nil {
		t.Fatalf("NewPercent(50) error = %v, want nil", err)
	}
	snapshot, err := newGoMemorySnapshot(50, 0, 100)
	if err != nil {
		t.Fatalf("newGoMemorySnapshot(50, 0, 100) error = %v, want nil", err)
	}
	trigger, err := (GoMemoryPressurePolicy{TriggerPercent: percent}).TriggerBytes(snapshot.LimitBytes())
	if err != nil {
		t.Fatalf("GoMemoryPressurePolicy.TriggerBytes() error = %v, want nil", err)
	}
	contradictoryMemory := GoMemoryAssessment{
		snapshot: snapshot,
		trigger:  trigger,
		policy:   GoMemoryPressurePolicy{TriggerPercent: percent},
		state:    MemoryPressureHealthy,
	}
	if gotErr := contradictoryMemory.Validate(); !errors.Is(gotErr, core.ErrHostFactsContract) {
		t.Fatalf("contradictory GoMemoryAssessment.Validate() error = %v, want %v", gotErr, core.ErrHostFactsContract)
	}

	wrongInterface, err := core.ParseAbsolutePath("/sys/fs/cgroup/memory.limit_in_bytes")
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(wrong interface) error = %v, want nil", err)
	}
	contradictoryWorkload := WorkloadMemoryLimit{
		path: wrongInterface, limit: core.NewByteLength(1),
		state: WorkloadMemoryLimitLimited, source: WorkloadMemoryLimitSourceCgroupV2,
	}
	if gotErr := contradictoryWorkload.Validate(); !errors.Is(gotErr, core.ErrHostFactsContract) {
		t.Fatalf("contradictory WorkloadMemoryLimit.Validate() error = %v, want %v", gotErr, core.ErrHostFactsContract)
	}

	for _, examined := range []uint64{0, uint64(len(GoOOMPlainBanner) - 1)} {
		impossibleEvidence := GoOOMBannerEvidence{
			examined: core.NewByteLength(examined),
			state:    GoOOMBannerPresent,
		}
		if gotErr := impossibleEvidence.Validate(); !errors.Is(gotErr, core.ErrHostFactsEvidence) {
			t.Fatalf("present GoOOMBannerEvidence(%d bytes).Validate() error = %v, want %v", examined, gotErr, core.ErrHostFactsEvidence)
		}
	}
}
