package hostfacts

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestDiskCapacityAndPolicyPressureSignedSizeDomain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		available   uint64
		total       uint64
		floor       uint64
		wantState   DiskPressureState
		wantErr     core.ErrorIdentity
		wantFailure bool
	}{
		{name: "zero floor disables at empty availability", available: 0, total: 1, floor: 0, wantState: DiskPressureDisabled},
		{name: "zero floor disables at full availability", available: 1, total: 1, floor: 0, wantState: DiskPressureDisabled},
		{name: "one byte above floor is healthy", available: 2, total: 2, floor: 1, wantState: DiskPressureHealthy},
		{name: "availability equal to floor is reached", available: 1, total: 2, floor: 1, wantState: DiskPressureReached, wantErr: core.ErrDiskFloorReached, wantFailure: true},
		{name: "availability below floor is reached", available: 1, total: 3, floor: 2, wantState: DiskPressureReached, wantErr: core.ErrDiskFloorReached, wantFailure: true},
		{name: "zero availability reaches positive floor", available: 0, total: 1, floor: 1, wantState: DiskPressureReached, wantErr: core.ErrDiskFloorReached, wantFailure: true},
		{name: "maximum signed capacity remains healthy", available: math.MaxInt64, total: math.MaxInt64, floor: math.MaxInt64 - 1, wantState: DiskPressureHealthy},
		{name: "large total does not narrow small availability", available: 7, total: math.MaxUint64, floor: 6, wantState: DiskPressureHealthy},
		{name: "large total preserves equality boundary", available: 7, total: math.MaxUint64, floor: 7, wantState: DiskPressureReached, wantErr: core.ErrDiskFloorReached, wantFailure: true},
		{name: "small exact healthy adjacency", available: 101, total: 1000, floor: 100, wantState: DiskPressureHealthy},
		{name: "small exact reached adjacency", available: 100, total: 1000, floor: 100, wantState: DiskPressureReached, wantErr: core.ErrDiskFloorReached, wantFailure: true},
		{name: "small below reached adjacency", available: 99, total: 1000, floor: 100, wantState: DiskPressureReached, wantErr: core.ErrDiskFloorReached, wantFailure: true},
		{name: "disabled ignores maximum total", available: 0, total: math.MaxUint64, floor: 0, wantState: DiskPressureDisabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			capacity, gotCapacityErr := newDiskCapacity(tc.available, tc.total)
			if gotCapacityErr != nil {
				t.Fatalf("newDiskCapacity(%d, %d) error = %v, want nil", tc.available, tc.total, gotCapacityErr)
			}
			got, gotErr := assessDiskCapacity(
				capacity,
				DiskPressurePolicy{FreeSpaceFloor: mustByteLength(t, tc.floor)},
			)
			if tc.wantFailure && !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("assessDiskCapacity() error = %v, want %v", gotErr, tc.wantErr)
			}
			if !tc.wantFailure && gotErr != nil {
				t.Fatalf("assessDiskCapacity() error = %v, want nil", gotErr)
			}
			if got.State() != tc.wantState {
				t.Fatalf("assessDiskCapacity().State() = %v, want %v", got.State(), tc.wantState)
			}
			if got.Policy().FreeSpaceFloor.Uint64() != tc.floor {
				t.Fatalf("assessDiskCapacity().Policy().FreeSpaceFloor = %d, want %d", got.Policy().FreeSpaceFloor.Uint64(), tc.floor)
			}
			if got.Validate() != nil {
				t.Fatalf("assessDiskCapacity().Validate() error = %v, want nil", got.Validate())
			}
		})
	}
}

func TestDiskCapacityRejectsImpossibleShapesAndMultiplicationOverflow(t *testing.T) {
	t.Parallel()

	capacityCases := []struct {
		name      string
		available uint64
		total     uint64
	}{
		{name: "zero total with zero available is impossible", available: 0, total: 0},
		{name: "zero total with one available is impossible", available: 1, total: 0},
		{name: "available exceeds total by one", available: 2, total: 1},
		{name: "maximum available exceeds small total", available: math.MaxUint64, total: 1},
		{name: "maximum available exceeds signed total", available: math.MaxUint64, total: math.MaxInt64},
		{name: "first extent outside Go signed size domain", available: math.MaxInt64 + 1, total: math.MaxUint64},
		{name: "maximum extent outside Go signed size domain", available: math.MaxUint64, total: math.MaxUint64},
	}
	for _, tc := range capacityCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := newDiskCapacity(tc.available, tc.total)
			if got != (DiskCapacity{}) || !errors.Is(gotErr, core.ErrHostFactsObservation) {
				t.Fatalf("newDiskCapacity(%d, %d) = (%v, %v), want (zero, %v)", tc.available, tc.total, got, gotErr, core.ErrHostFactsObservation)
			}
			var failure Failure
			if errors.As(gotErr, &failure) {
				t.Fatalf("newDiskCapacity(%d, %d) error = %v, want leaf error without operation wrapper", tc.available, tc.total, gotErr)
			}
		})
	}

	multiplyCases := []struct {
		name        string
		blocks      uint64
		blockBytes  uint64
		want        uint64
		wantErr     core.ErrorIdentity
		wantFailure bool
	}{
		{name: "zero blocks is exact", blocks: 0, blockBytes: math.MaxUint64, want: 0},
		{name: "zero block size is exact", blocks: math.MaxUint64, blockBytes: 0, want: 0},
		{name: "one times maximum is exact", blocks: 1, blockBytes: math.MaxUint64, want: math.MaxUint64},
		{name: "maximum times one is exact", blocks: math.MaxUint64, blockBytes: 1, want: math.MaxUint64},
		{name: "signed boundary product is exact", blocks: math.MaxInt64, blockBytes: 1, want: math.MaxInt64},
		{name: "first unsigned product is exact", blocks: math.MaxInt64 + 1, blockBytes: 1, want: math.MaxInt64 + 1},
		{name: "two times half range overflows", blocks: 2, blockBytes: math.MaxInt64 + 1, wantErr: core.ErrNumericOverflow, wantFailure: true},
		{name: "maximum times two overflows", blocks: math.MaxUint64, blockBytes: 2, wantErr: core.ErrNumericOverflow, wantFailure: true},
		{name: "large square overflows", blocks: 1 << 32, blockBytes: 1 << 32, wantErr: core.ErrNumericOverflow, wantFailure: true},
		{name: "largest square below range is exact", blocks: math.MaxUint32, blockBytes: math.MaxUint32, want: uint64(math.MaxUint32) * uint64(math.MaxUint32)},
	}
	for _, tc := range multiplyCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := blocksToBytes(tc.blocks, tc.blockBytes)
			if tc.wantFailure {
				if got != 0 || !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("blocksToBytes(%d, %d) = (%d, %v), want (0, %v)", tc.blocks, tc.blockBytes, got, gotErr, tc.wantErr)
				}
				return
			}
			if got != tc.want || gotErr != nil {
				t.Fatalf("blocksToBytes(%d, %d) = (%d, %v), want (%d, nil)", tc.blocks, tc.blockBytes, got, gotErr, tc.want)
			}
		})
	}
}

func TestGoMemoryPercentageProjectionMatchesIndependentIntegerOracle(t *testing.T) {
	t.Parallel()

	values := []uint64{
		1, 2, 3, 7, 10, 63, 64, 99, 100, 101,
		255, 256, 1023, 1024, 1 << 20, math.MaxInt32,
		math.MaxUint32, math.MaxInt64, math.MaxInt64 + 1, math.MaxUint64,
	}
	percents := []uint8{1, 2, 3, 10, 33, 50, 67, 90, 99, 100}
	for _, value := range values {
		for _, percentValue := range percents {
			percent, percentErr := NewPercent(percentValue)
			if percentErr != nil {
				t.Fatalf("NewPercent(%d) error = %v, want nil", percentValue, percentErr)
			}
			limit, limitErr := core.NewByteCount(value)
			if limitErr != nil {
				t.Fatalf("core.NewByteCount(%d) error = %v, want nil", value, limitErr)
			}
			got, gotErr := (GoMemoryPressurePolicy{TriggerPercent: percent}).TriggerBytes(limit)
			if gotErr != nil {
				t.Fatalf("TriggerBytes(%d at %d%%) error = %v, want nil", value, percentValue, gotErr)
			}
			gotValue, _ := got.Uint64()
			want := ceilingPercentOracle(value, percentValue)
			if gotValue != want {
				t.Fatalf("TriggerBytes(%d at %d%%) = %d, want %d", value, percentValue, gotValue, want)
			}
		}
	}
}

func TestGoMemoryPolicyAndSnapshotRejectBoundaryContradictions(t *testing.T) {
	t.Parallel()

	for _, value := range []uint8{0, 101, 127, 128, 200, 254, 255} {
		t.Run("out of range percentage "+new(big.Int).SetUint64(uint64(value)).String(), func(t *testing.T) {
			t.Parallel()

			got, gotErr := NewPercent(value)
			if got != (Percent{}) || !errors.Is(gotErr, core.ErrHostFactsContract) {
				t.Fatalf("NewPercent(%d) = (%v, %v), want (zero, %v)", value, got, gotErr, core.ErrHostFactsContract)
			}
		})
	}

	snapshotCases := []struct {
		name         string
		system       uint64
		heapReleased uint64
		limit        int64
	}{
		{name: "zero system is not an observation", system: 0, heapReleased: 0, limit: 1},
		{name: "released exceeds system by one", system: 1, heapReleased: 2, limit: 1},
		{name: "maximum release exceeds small system", system: 1, heapReleased: math.MaxUint64, limit: 1},
		{name: "zero limit is rejected", system: 1, heapReleased: 0, limit: 0},
		{name: "negative limit is rejected", system: 1, heapReleased: 0, limit: -1},
		{name: "minimum negative limit is rejected", system: 1, heapReleased: 0, limit: math.MinInt64},
	}
	for _, tc := range snapshotCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := newGoMemorySnapshot(tc.system, tc.heapReleased, tc.limit)
			if got != (GoMemorySnapshot{}) || !errors.Is(gotErr, core.ErrHostFactsObservation) {
				t.Fatalf("newGoMemorySnapshot() = (%v, %v), want (zero, %v)", got, gotErr, core.ErrHostFactsObservation)
			}
			var failure Failure
			if errors.As(gotErr, &failure) {
				t.Fatalf("newGoMemorySnapshot() error = %v, want leaf error without operation wrapper", gotErr)
			}
		})
	}
}

func ceilingPercentOracle(value uint64, percent uint8) uint64 {
	numerator := new(big.Int).Mul(
		new(big.Int).SetUint64(value),
		new(big.Int).SetUint64(uint64(percent)),
	)
	numerator.Add(numerator, new(big.Int).SetUint64(percentDenominator-1))
	numerator.Div(numerator, new(big.Int).SetUint64(percentDenominator))
	return numerator.Uint64()
}
