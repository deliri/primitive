package temporal_test

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// A bounded math/big oracle pressures both limbs independently of math/bits.
func FuzzAggregateDurationArithmetic(f *testing.F) {
	for _, seed := range []struct{ high, low, addHigh, addLow, multiplier uint64 }{
		{0, 0, 0, 0, 0},
		{0, 1, 0, 1, 1},
		{0, math.MaxUint64, 0, 1, 2},
		{1, 0, 0, math.MaxUint64, math.MaxUint64},
		{math.MaxUint64, math.MaxUint64, 0, 1, 2},
		{math.MaxUint64, math.MaxUint64, 0, 0, 1},
		{math.MaxUint64, math.MaxUint64, 0, 0, 0},
	} {
		f.Add(seed.high, seed.low, seed.addHigh, seed.addLow, seed.multiplier)
	}
	f.Fuzz(func(t *testing.T, high, low, addHigh, addLow, multiplier uint64) {
		left := new(big.Int).Lsh(new(big.Int).SetUint64(high), 64)
		left.Or(left, new(big.Int).SetUint64(low))
		right := new(big.Int).Lsh(new(big.Int).SetUint64(addHigh), 64)
		right.Or(right, new(big.Int).SetUint64(addLow))
		input, inputErr := temporal.ParseAggregateDuration(left.String())
		addend, addendErr := temporal.ParseAggregateDuration(right.String())
		if inputErr != nil || addendErr != nil {
			t.Fatalf("bounded operands = (%v,%v), want nil", inputErr, addendErr)
		}
		sum, sumErr := input.Add(addend)
		product, productErr := input.Multiply(multiplier)
		wantSum := new(big.Int).Add(left, right)
		wantProduct := new(big.Int).Mul(left, new(big.Int).SetUint64(multiplier))
		for _, result := range []struct {
			name   string
			got    temporal.AggregateDuration
			gotErr error
			want   *big.Int
		}{
			{"sum", sum, sumErr, wantSum},
			{"product", product, productErr, wantProduct},
		} {
			if result.want.BitLen() > 128 {
				if result.got != (temporal.AggregateDuration{}) || !errors.Is(result.gotErr, core.ErrTemporalOverflow) || !errors.Is(result.gotErr, core.ErrNumericOverflow) {
					t.Fatalf("%s overflow = (%v,%v), want zero typed overflow", result.name, result.got, result.gotErr)
				}
				continue
			}
			if result.gotErr != nil || result.got.Validate() != nil || result.got.Decimal() != result.want.String() {
				t.Fatalf("%s = (%q,%v), want %q", result.name, result.got.Decimal(), result.gotErr, result.want.String())
			}
		}
		wantComparison := core.ComparisonEqual
		if comparison := left.Cmp(right); comparison < 0 {
			wantComparison = core.ComparisonLess
		} else if comparison > 0 {
			wantComparison = core.ComparisonGreater
		}
		if got := input.Compare(addend); got != wantComparison {
			t.Fatalf("comparison = %v, want %v", got, wantComparison)
		}
		narrow, narrowErr := input.Duration()
		if left.IsInt64() {
			if narrowErr != nil || narrow.Nanoseconds() != left.Int64() {
				t.Fatalf("narrowing = (%v,%v), want %v", narrow, narrowErr, left)
			}
		} else if narrow != (temporal.Duration{}) || !errors.Is(narrowErr, core.ErrTemporalOverflow) {
			t.Fatalf("narrowing overflow = (%v,%v), want zero typed overflow", narrow, narrowErr)
		}
	})
}
