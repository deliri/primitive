package standard

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestMachineRateConstructorsRejectZeroAndPreserveUnitBounds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		value   uint64
		wantErr bool
	}{
		{name: "zero is outside every positive machine rate domain", value: 0, wantErr: true},
		{name: "one is the minimum positive machine rate", value: 1},
		{name: "maximum uint64 remains an exact machine rate", value: math.MaxUint64},
	}

	t.Run("IOPS retains operation units", func(t *testing.T) {
		t.Parallel()
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got, gotErr := NewMachineIOPS(tc.value)
				if (gotErr != nil) != tc.wantErr || (!tc.wantErr && uint64(got) != tc.value) {
					t.Fatalf("NewMachineIOPS(%d) = (%d, %v), want exact value with rejection %t", tc.value, got, gotErr, tc.wantErr)
				}
				if tc.wantErr && !errors.Is(gotErr, core.ErrStandardContract) {
					t.Fatalf("NewMachineIOPS(%d) error = %v, want errors.Is(..., %v)", tc.value, gotErr, core.ErrStandardContract)
				}
			})
		}
	})

	t.Run("byte throughput retains byte units", func(t *testing.T) {
		t.Parallel()
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got, gotErr := NewMachineBytesPerSecond(tc.value)
				if (gotErr != nil) != tc.wantErr || (!tc.wantErr && uint64(got) != tc.value) {
					t.Fatalf("NewMachineBytesPerSecond(%d) = (%d, %v), want exact value with rejection %t", tc.value, got, gotErr, tc.wantErr)
				}
				if tc.wantErr && !errors.Is(gotErr, core.ErrStandardContract) {
					t.Fatalf("NewMachineBytesPerSecond(%d) error = %v, want errors.Is(..., %v)", tc.value, gotErr, core.ErrStandardContract)
				}
			})
		}
	})

	t.Run("bit throughput retains bit units", func(t *testing.T) {
		t.Parallel()
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got, gotErr := NewMachineBitsPerSecond(tc.value)
				if (gotErr != nil) != tc.wantErr || (!tc.wantErr && uint64(got) != tc.value) {
					t.Fatalf("NewMachineBitsPerSecond(%d) = (%d, %v), want exact value with rejection %t", tc.value, got, gotErr, tc.wantErr)
				}
				if tc.wantErr && !errors.Is(gotErr, core.ErrStandardContract) {
					t.Fatalf("NewMachineBitsPerSecond(%d) error = %v, want errors.Is(..., %v)", tc.value, gotErr, core.ErrStandardContract)
				}
			})
		}
	})
}
