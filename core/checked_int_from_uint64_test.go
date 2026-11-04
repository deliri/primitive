package core

import (
	"errors"
	"math"
	"strconv"
	"testing"
)

func TestCheckedIntFromUint64PreservesNativeWidth(t *testing.T) {
	t.Parallel()
	t.Logf("native int width=%d", strconv.IntSize)
	for _, row := range []struct {
		name    string
		value   uint64
		want    int
		wantErr error
	}{
		{name: "zero accepted count", value: 0, want: 0},
		{name: "partial write count", value: 3, want: 3},
		{name: "one below native maximum", value: uint64(math.MaxInt) - 1, want: math.MaxInt - 1},
		{name: "exact native maximum", value: uint64(math.MaxInt), want: math.MaxInt},
		{name: "one above native maximum", value: uint64(math.MaxInt) + 1, wantErr: ErrNumericOverflow},
		{name: "unsigned maximum", value: math.MaxUint64, wantErr: ErrNumericOverflow},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := CheckedIntFromUint64(row.value)
			if !errors.Is(err, row.wantErr) || got != row.want {
				t.Fatalf("native projection(%d) got=%d/%v, want=%d/%v", row.value, got, err, row.want, row.wantErr)
			}
		})
	}
}
