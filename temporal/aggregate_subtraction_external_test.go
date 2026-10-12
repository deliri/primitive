package temporal_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestAggregateDurationSubtractionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, left, right, want string
		wantErr                 error
	}{
		{name: "empty time remains neutral", left: "0", right: "0", want: "0"},
		{name: "minimum positive time is removed exactly", left: "1", right: "1", want: "0"},
		{name: "removing nothing preserves maximum", left: maximumUint128Decimal, right: "0", want: maximumUint128Decimal},
		{name: "low limb borrows one from high", left: uint64HighLimbDecimal, right: "1", want: maximumUint64Decimal},
		{name: "borrow preserves nonzero high limb", left: "36893488147419103232", right: "1", want: "36893488147419103231"},
		{name: "two limb removal leaves low limb", left: "18446744073709551617", right: uint64HighLimbDecimal, want: "1"},
		{name: "whole maximum is removed exactly", left: maximumUint128Decimal, right: maximumUint128Decimal, want: "0"},
		{name: "low limb underflow cannot wrap", left: "0", right: "1", wantErr: core.ErrTemporalOverflow},
		{name: "high limb underflow cannot wrap", left: maximumUint64Decimal, right: uint64HighLimbDecimal, wantErr: core.ErrTemporalOverflow},
		{name: "borrow out of the high limb cannot wrap", left: uint64HighLimbDecimal, right: "18446744073709551617", wantErr: core.ErrTemporalOverflow},
		{name: "maximum removal exceeds a small receiver", left: "1", right: maximumUint128Decimal, wantErr: core.ErrTemporalOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			left, err := temporal.ParseAggregateDuration(tc.left)
			if err != nil {
				t.Fatal(err)
			}
			right, err := temporal.ParseAggregateDuration(tc.right)
			if err != nil {
				t.Fatal(err)
			}
			got, err := left.Subtract(right)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Subtract() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !got.IsZero() || !errors.Is(err, core.ErrNumericOverflow) || !errors.Is(err, core.ErrTemporalContract) {
					t.Fatalf("underflow = (%v, %v), want zero and numeric/temporal refusal", got, err)
				}
				return
			}
			restored, addErr := got.Add(right)
			if got.Decimal() != tc.want || addErr != nil || restored != left {
				t.Fatalf("difference and restoration = (%s, %v, %v), want (%s, %v, nil)", got.Decimal(), restored, addErr, tc.want, left)
			}
		})
	}
}

// The four uint64 limbs span the entire typed domain. big.Int is independent
// of Temporal's fixed-limb implementation and checks borrow plus refusal.
func FuzzAggregateDurationSubtractionConservation(f *testing.F) {
	for _, limbs := range [][4]uint64{{0, 0, 0, 0}, {0, 0, 0, 1}, {1, 0, 0, 1}, {1, 0, 1, 1}, {2, 0, 1, ^uint64(0)}, {^uint64(0), ^uint64(0), ^uint64(0), ^uint64(0)}, {^uint64(0), 0, ^uint64(0), 1}} {
		f.Add(limbs[0], limbs[1], limbs[2], limbs[3])
	}
	f.Fuzz(func(t *testing.T, ah, al, bh, bl uint64) {
		leftValue := new(big.Int).Lsh(new(big.Int).SetUint64(ah), 64)
		leftValue.Add(leftValue, new(big.Int).SetUint64(al))
		rightValue := new(big.Int).Lsh(new(big.Int).SetUint64(bh), 64)
		rightValue.Add(rightValue, new(big.Int).SetUint64(bl))
		left, err := temporal.ParseAggregateDuration(leftValue.String())
		if err != nil {
			t.Fatal(err)
		}
		right, err := temporal.ParseAggregateDuration(rightValue.String())
		if err != nil {
			t.Fatal(err)
		}
		want := new(big.Int).Sub(leftValue, rightValue)
		got, err := left.Subtract(right)
		if want.Sign() < 0 {
			if !errors.Is(err, core.ErrTemporalOverflow) || !errors.Is(err, core.ErrNumericOverflow) || !got.IsZero() {
				t.Fatalf("Subtract() = (%v, %v), want zero and typed underflow", got, err)
			}
			return
		}
		restored, restoreErr := got.Add(right)
		if err != nil || got.Decimal() != want.String() || got.Validate() != nil || restoreErr != nil || restored != left {
			t.Fatalf("Subtract() = (%s, %v), restoration = (%v, %v), want (%s, nil) and original", got.Decimal(), err, restored, restoreErr, want.String())
		}
	})
}
