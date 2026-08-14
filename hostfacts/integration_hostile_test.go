package hostfacts

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAssessDiskRealHeldRootLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive real directory returns caller capacity and disabled policy", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		got, gotErr := AssessDisk(context.Background(), DiskAssessmentRequest{
			Directory: mustAbsolutePathForHostfactsTest(t, root),
			Policy:    DiskPressurePolicy{},
		})
		if gotErr != nil {
			t.Fatalf("AssessDisk(%s) error = %v, want nil", root, gotErr)
		}
		total, totalErr := got.Capacity().TotalBytes().Uint64()
		if totalErr != nil || total == 0 || got.Capacity().AvailableBytes().Uint64() > total ||
			got.State() != DiskPressureDisabled || got.Validate() != nil {
			t.Fatalf("AssessDisk(%s) = %v total=%d/%v, want valid disabled caller capacity", root, got, total, totalErr)
		}
	})

	t.Run("negative missing root preserves native not-exist identity", func(t *testing.T) {
		t.Parallel()

		root := filepath.Join(t.TempDir(), "absent")
		got, gotErr := AssessDisk(context.Background(), DiskAssessmentRequest{
			Directory: mustAbsolutePathForHostfactsTest(t, root),
		})
		var failure Failure
		if got != (DiskAssessment{}) ||
			!errors.Is(gotErr, core.ErrHostFactsObservation) ||
			!errors.Is(gotErr, os.ErrNotExist) ||
			!errors.As(gotErr, &failure) ||
			failure.Operation != OperationOpenRoot {
			t.Fatalf("AssessDisk(missing) = (%v, %v), want zero typed open-root observation preserving not-exist", got, gotErr)
		}
	})

	t.Run("negative symlink root is contract failure without observation identity", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		target := filepath.Join(parent, "target")
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatalf("os.Mkdir(%s) error = %v", target, err)
		}
		link := filepath.Join(parent, "root-link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("os.Symlink(%s) error = %v", link, err)
		}
		got, gotErr := AssessDisk(context.Background(), DiskAssessmentRequest{
			Directory: mustAbsolutePathForHostfactsTest(t, link),
		})
		if got != (DiskAssessment{}) ||
			!errors.Is(gotErr, core.ErrHostFactsContract) ||
			errors.Is(gotErr, core.ErrHostFactsObservation) {
			t.Fatalf(
				"AssessDisk(symlink root) = (%v, %v), want zero contract failure without observation identity",
				got,
				gotErr,
			)
		}
	})

	t.Run("neutral zero floor does not turn an observation into pressure", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		got, gotErr := AssessDisk(context.Background(), DiskAssessmentRequest{
			Directory: mustAbsolutePathForHostfactsTest(t, root),
			Policy:    DiskPressurePolicy{FreeSpaceFloor: mustByteLength(t, 0)},
		})
		if gotErr != nil || got.State() != DiskPressureDisabled ||
			errors.Is(gotErr, core.ErrHostFactsPressure) {
			t.Fatalf("AssessDisk(zero floor) = (%v, %v), want disabled without pressure", got, gotErr)
		}
	})

	// The floor is derived from the device rather than saturated, because a
	// floor at or above total capacity is now refused as a contradiction: it
	// could never be satisfied on this device, so it proves nothing about the
	// reached path. One byte below total is the largest admissible floor and is
	// necessarily above whatever is available on a disk holding this checkout.
	t.Run("reached floor returns the complete validated assessment", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		observed, observeErr := AssessDisk(context.Background(), DiskAssessmentRequest{
			Directory: mustAbsolutePathForHostfactsTest(t, root),
			Policy:    DiskPressurePolicy{},
		})
		if observeErr != nil {
			t.Fatalf("AssessDisk(zero floor) error = %v, want nil", observeErr)
		}
		total, totalErr := observed.Capacity().TotalBytes().Uint64()
		if totalErr != nil {
			t.Fatalf("TotalBytes().Uint64() error = %v, want nil", totalErr)
		}
		if total == 0 {
			t.Fatalf("observed total capacity = 0, want a real device")
		}
		got, gotErr := AssessDisk(context.Background(), DiskAssessmentRequest{
			Directory: mustAbsolutePathForHostfactsTest(t, root),
			Policy: DiskPressurePolicy{
				FreeSpaceFloor: mustByteLength(t, total-1),
			},
		})
		if !errors.Is(gotErr, core.ErrDiskFloorReached) ||
			errors.Is(gotErr, core.ErrHostFactsContract) ||
			got.State() != DiskPressureReached ||
			got.Validate() != nil {
			t.Fatalf("AssessDisk(maximum floor) = (%v, %v), want complete reached assessment outside contract axis", got, gotErr)
		}
	})
}

func TestGoMemoryAssessmentThresholdAndRuntimePath(t *testing.T) {
	t.Parallel()

	percent, err := NewPercent(50)
	if err != nil {
		t.Fatalf("NewPercent(50) error = %v, want nil", err)
	}
	policy := GoMemoryPressurePolicy{TriggerPercent: percent}
	cases := []struct {
		name        string
		system      uint64
		released    uint64
		limit       int64
		wantState   MemoryPressureState
		wantErr     core.ErrorIdentity
		wantFailure bool
	}{
		{name: "one below trigger is healthy", system: 49, released: 0, limit: 100, wantState: MemoryPressureHealthy},
		{name: "exact trigger is reached", system: 50, released: 0, limit: 100, wantState: MemoryPressureReached, wantErr: core.ErrMemoryLimitReached, wantFailure: true},
		{name: "one above trigger is reached", system: 51, released: 0, limit: 100, wantState: MemoryPressureReached, wantErr: core.ErrMemoryLimitReached, wantFailure: true},
		{name: "released heap lowers managed bytes below trigger", system: 100, released: 51, limit: 100, wantState: MemoryPressureHealthy},
		{name: "released heap leaves exact trigger", system: 100, released: 50, limit: 100, wantState: MemoryPressureReached, wantErr: core.ErrMemoryLimitReached, wantFailure: true},
		{name: "canonical maximum limit is disabled", system: 1, released: 0, limit: math.MaxInt64, wantState: MemoryPressureDisabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			snapshot, snapshotErr := newGoMemorySnapshot(tc.system, tc.released, tc.limit)
			if snapshotErr != nil {
				t.Fatalf("newGoMemorySnapshot() error = %v, want nil", snapshotErr)
			}
			got, gotErr := assessGoMemorySnapshot(snapshot, policy)
			if tc.wantFailure && !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("assessGoMemorySnapshot() error = %v, want %v", gotErr, tc.wantErr)
			}
			if !tc.wantFailure && gotErr != nil {
				t.Fatalf("assessGoMemorySnapshot() error = %v, want nil", gotErr)
			}
			if got.State() != tc.wantState || got.Validate() != nil {
				t.Fatalf("assessGoMemorySnapshot() = %v, want valid state %v", got, tc.wantState)
			}
			system, systemErr := got.Snapshot().SystemBytes().Uint64()
			limit, limitErr := got.Snapshot().LimitBytes().Uint64()
			wantManaged := tc.system - tc.released
			wantTrigger, triggerErr := policy.TriggerBytes(got.Snapshot().LimitBytes())
			if systemErr != nil || limitErr != nil || triggerErr != nil ||
				system != tc.system ||
				got.Snapshot().HeapReleasedBytes().Uint64() != tc.released ||
				got.Snapshot().ManagedBytes().Uint64() != wantManaged ||
				limit != uint64(tc.limit) ||
				got.TriggerBytes() != wantTrigger ||
				got.Policy() != policy {
				t.Fatalf(
					"assessment public projection = system %d/%v released %d managed %d limit %d/%v trigger %v/%v policy %v, want source snapshot and policy",
					system,
					systemErr,
					got.Snapshot().HeapReleasedBytes().Uint64(),
					got.Snapshot().ManagedBytes().Uint64(),
					limit,
					limitErr,
					got.TriggerBytes(),
					triggerErr,
					got.Policy(),
				)
			}
		})
	}

	runtimePercent, err := NewPercent(100)
	if err != nil {
		t.Fatalf("NewPercent(100) error = %v, want nil", err)
	}
	runtimeAssessment, runtimeErr := AssessGoMemory(GoMemoryAssessmentRequest{
		Policy: GoMemoryPressurePolicy{TriggerPercent: runtimePercent},
	})
	if runtimeErr != nil {
		t.Fatalf("AssessGoMemory(real runtime) error = %v, want nil", runtimeErr)
	}
	if runtimeAssessment.Validate() != nil {
		t.Fatalf("AssessGoMemory(real runtime).Validate() error = %v, want nil", runtimeAssessment.Validate())
	}
}

func TestUnsupportedWorkloadObservationIsAValidNeutralFact(t *testing.T) {
	t.Parallel()

	got, gotErr := unsupportedWorkloadMemoryLimit()
	if gotErr != nil || got.State() != WorkloadMemoryLimitUnsupported ||
		got.Source() != WorkloadMemoryLimitSourceNone || got.Validate() != nil {
		t.Fatalf("unsupportedWorkloadMemoryLimit() = (%v, %v), want valid unsupported fact", got, gotErr)
	}
	if _, present := got.LimitBytes(); present {
		t.Fatalf("unsupportedWorkloadMemoryLimit().LimitBytes() present = true, want false")
	}
	if _, present := got.InterfacePath(); present {
		t.Fatalf("unsupportedWorkloadMemoryLimit().InterfacePath() present = true, want false")
	}
}

func TestHostFactsErrorAxesAndTypedFailure(t *testing.T) {
	t.Parallel()

	native := errors.New("native observation failed")
	got := fail(OperationDiskCapacity, core.ErrHostFactsObservation, native)
	var failure Failure
	if !errors.As(got, &failure) ||
		!errors.Is(got, core.ErrHostFacts) ||
		!errors.Is(got, core.ErrHostFactsObservation) ||
		!errors.Is(got, native) ||
		errors.Is(got, core.ErrHostFactsContract) ||
		failure.Validate() != nil {
		t.Fatalf("typed observation failure = %v, want valid observation/native axes and no contract axis", got)
	}
	identityOnly := Failure{
		Operation: OperationDiskCapacity,
		Identity:  core.ErrHostFactsObservation,
	}
	unwrapped := identityOnly.Unwrap()
	if identityOnly.Validate() != nil ||
		!errors.Is(identityOnly, core.ErrHostFactsObservation) ||
		len(unwrapped) != 1 || unwrapped[0] != core.ErrHostFactsObservation {
		t.Fatalf(
			"identity-only Failure unwrap = %v, want one non-nil stable identity",
			unwrapped,
		)
	}
	invalid := []struct {
		wantErr error
		failure Failure
	}{
		{failure: Failure{}, wantErr: core.ErrHostFactsContract},
		{failure: Failure{Operation: OperationUnknown, Identity: core.ErrHostFactsObservation}, wantErr: core.ErrHostFactsContract},
		{failure: Failure{Operation: OperationDiskCapacity, Identity: core.ErrUnknown}, wantErr: core.ErrPrimitiveContract},
		{failure: Failure{Operation: OperationDiskCapacity, Identity: core.ErrExchangeContract}, wantErr: core.ErrHostFactsContract},
	}
	for _, candidate := range invalid {
		if gotErr := candidate.failure.Validate(); !errors.Is(gotErr, candidate.wantErr) {
			t.Fatalf("Failure%+v.Validate() error = %v, want errors.Is %v", candidate.failure, gotErr, candidate.wantErr)
		}
	}
}

func TestOperationDiagnosticStringExhaustsTheClosedDomain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		want      string
		operation Operation
	}{
		{name: "unknown operation", operation: OperationUnknown, want: "unknown"},
		{name: "open root", operation: OperationOpenRoot, want: "open root"},
		{name: "disk capacity", operation: OperationDiskCapacity, want: "disk capacity"},
		{name: "go memory", operation: OperationGoMemory, want: "go memory"},
		{name: "physical memory", operation: OperationPhysicalMemory, want: "physical memory"},
		{name: "logical CPU count", operation: OperationLogicalCPUCount, want: "logical CPU count"},
		{name: "cgroup membership", operation: OperationCgroupMembership, want: "cgroup membership"},
		{name: "cgroup mount", operation: OperationCgroupMount, want: "cgroup mount"},
		{name: "cgroup limit", operation: OperationCgroupLimit, want: "cgroup limit"},
		{name: "tree walk", operation: OperationTreeWalk, want: "tree walk"},
		{name: "go OOM banner", operation: OperationGoOOMBanner, want: "go OOM banner"},
		{name: "future operation", operation: Operation(255), want: "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.operation.String(); got != tc.want {
				t.Fatalf("Operation(%d).String() = %q, want %q", tc.operation, got, tc.want)
			}
		})
	}
}
