package core

import (
	"errors"
	"math"
	"testing"
)

func TestImageDimensionsPreserveNativePixelDomain(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		value   ImageDimensions
		wantErr error
	}{
		{name: "absent dimensions refuse", wantErr: ErrPrimitiveContract},
		{name: "absent width refuses", value: ImageDimensions{Height: 1}, wantErr: ErrPrimitiveContract},
		{name: "absent height refuses", value: ImageDimensions{Width: 1}, wantErr: ErrPrimitiveContract},
		{name: "one pixel is meaningful", value: ImageDimensions{Width: 1, Height: 1}},
		{name: "axes preserve distinct observations", value: ImageDimensions{Width: 1122, Height: 1402}},
		{name: "native maximum is no product ceiling", value: ImageDimensions{Width: math.MaxUint64, Height: math.MaxUint64}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.value.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("dimensions %+v validation = %v, want %v", tc.value, err, tc.wantErr)
			}
		})
	}
}

func FuzzPixelDimensionPreservesNativeRepresentation(f *testing.F) {
	for _, value := range []uint64{0, 1, 2, math.MaxUint64} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value uint64) {
		got, err := NewPixelDimension(value)
		if value == 0 {
			if !errors.Is(err, ErrPrimitiveContract) || got != 0 {
				t.Fatalf("absent measurement = %d/%v, want zero and contract refusal", got, err)
			}
			return
		}
		if err != nil || uint64(got) != value || got.Validate() != nil {
			t.Fatalf("pixel measurement = %d/%v, want exact native value %d", got, err, value)
		}
	})
}
