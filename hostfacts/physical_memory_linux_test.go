//go:build linux

package hostfacts

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestPhysicalMemoryBytesBoundaries pins both the scaling result and the
// stable identity of each refusal. Asserting only "some error" would let the zero-unit
// guard be deleted: without it a zero unit divides by zero in the overflow
// guard, and any replacement that reports overflow instead would hide a
// malformed kernel report behind a numeric-range diagnostic.
func TestPhysicalMemoryBytesBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		total   uint64
		unit    uint64
		want    uint64
	}{
		{name: "zero unit is rejected as malformed OS evidence", total: 1, unit: 0, wantErr: core.ErrHostFactsObservation},
		{name: "zero unit is rejected even with a zero total", total: 0, unit: 0, wantErr: core.ErrHostFactsObservation},
		{name: "zero unit is rejected at the maximum total", total: math.MaxUint64, unit: 0, wantErr: core.ErrHostFactsObservation},
		{name: "zero total remains a caller-owned absent observation", total: 0, unit: 1},
		{name: "unit scale preserves the reported total", total: 1, unit: 1, want: 1},
		{name: "unscaled maximum total is admitted", total: math.MaxUint64, unit: 1, want: math.MaxUint64},
		{name: "typical kernel page unit scales exactly", total: 1 << 20, unit: 4096, want: (1 << 20) * 4096},
		{name: "scaled total at the last representable multiple is admitted", total: math.MaxUint64 / 3, unit: 3, want: math.MaxUint64 / 3 * 3},
		{name: "scaled total one multiple past the maximum is rejected", total: math.MaxUint64/3 + 1, unit: 3, wantErr: core.ErrHostFactsObservation},
		{name: "maximum total under a scaling unit is rejected", total: math.MaxUint64, unit: 2, wantErr: core.ErrHostFactsObservation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := physicalMemoryBytes(tc.total, tc.unit)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("physicalMemoryBytes(%d, %d) error = %v, want %v", tc.total, tc.unit, gotErr, tc.wantErr)
				}
				if got != 0 {
					t.Fatalf("physicalMemoryBytes(%d, %d) = %d on refusal, want 0", tc.total, tc.unit, got)
				}
				return
			}
			if gotErr != nil || got != tc.want {
				t.Fatalf("physicalMemoryBytes(%d, %d) = (%d, %v), want (%d, nil)", tc.total, tc.unit, got, gotErr, tc.want)
			}
		})
	}
}
