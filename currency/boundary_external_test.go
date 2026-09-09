package currency_test

import (
	"bytes"
	"errors"
	"math"
	"math/big"
	"slices"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/currency"
)

// The complete byte domain replaces sampled unknown enums. Each iteration
// attacks the same nominal value at every public admission/projection door.
func TestCurrencyNominalDomainExhaustsUnderlyingByte(t *testing.T) {
	t.Parallel()
	for raw := 0; raw <= math.MaxUint8; raw++ {
		code := currency.Code(raw)
		t.Run("code-"+strconv.Itoa(raw), func(t *testing.T) {
			t.Parallel()
			wantDigits, valid := oracleFractionDigits(code)
			wantErr := error(core.ErrCurrencyContract)
			if valid {
				wantErr = nil
			}
			if err := code.Validate(); !errors.Is(err, wantErr) || code.IsValid() != valid {
				t.Fatalf("code %d admission = (%v,%t), want (%v,%t)", code, err, code.IsValid(), wantErr, valid)
			}
			digits, err := code.FractionDigits()
			if !errors.Is(err, wantErr) || digits != wantDigits {
				t.Fatalf("code %d digits = (%d,%v), want (%d,%v)", code, digits, err, wantDigits, wantErr)
			}
			amount, err := currency.New(code, math.MinInt64)
			if !errors.Is(err, wantErr) {
				t.Fatalf("New code %d = %v, want %v", code, err, wantErr)
			}
			if valid {
				gotCode, ce := amount.Code()
				minor, me := amount.MinorUnits()
				if ce != nil || me != nil || gotCode != code || minor != math.MinInt64 {
					t.Fatalf("New projection = (%v,%d,%v,%v), want exact code/minimum/nil/nil", gotCode, minor, ce, me)
				}
				return
			}
			if amount != (currency.Amount{}) || code.String() != "" {
				t.Fatalf("invalid code projection = (%v,%q), want zero/empty", amount, code.String())
			}
			parsed, err := currency.Parse(code, "")
			if parsed != (currency.Amount{}) || !errors.Is(err, core.ErrCurrencyContract) || errors.Is(err, core.ErrCurrencyDecimal) {
				t.Fatalf("invalid-code Parse = (%v,%v), want zero/code refusal before decimal", parsed, err)
			}
			wire, err := code.MarshalJSON()
			if wire != nil || !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrCurrencyContract) {
				t.Fatalf("invalid-code JSON = (%q,%v), want nil/JSON and currency refusal", wire, err)
			}
		})
	}
}

func TestCurrencyArithmeticLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                string
		leftCode, rightCode currency.Code
		left, right         int64
		wantErr             error
	}{
		{name: "positive opposite extrema retain exact signed arithmetic", leftCode: currency.CodeCAD, rightCode: currency.CodeCAD, left: math.MaxInt64, right: math.MinInt64},
		{name: "neutral zero cannot alter either arithmetic operand", leftCode: currency.CodeCLF, rightCode: currency.CodeCLF, left: 0, right: 0},
		{name: "currency mismatch outranks positive overflow", leftCode: currency.CodeCAD, rightCode: currency.CodeUSD, left: math.MaxInt64, right: 1, wantErr: core.ErrCurrencyMismatch},
		{name: "currency mismatch outranks negative overflow", leftCode: currency.CodeUSD, rightCode: currency.CodeCAD, left: math.MinInt64, right: -1, wantErr: core.ErrCurrencyMismatch},
		{name: "unconstructed operand outranks mismatch and arithmetic", leftCode: currency.CodeUnknown, rightCode: currency.CodeCAD, left: math.MaxInt64, right: math.MinInt64, wantErr: core.ErrCurrencyContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			left, leftErr := currency.New(tc.leftCode, tc.left)
			right, rightErr := currency.New(tc.rightCode, tc.right)
			if tc.leftCode != currency.CodeUnknown && leftErr != nil || rightErr != nil {
				t.Fatalf("fixtures = (%v,%v), want declared valid operands", leftErr, rightErr)
			}
			if tc.leftCode == currency.CodeUnknown && (!errors.Is(leftErr, core.ErrCurrencyContract) || left != (currency.Amount{})) {
				t.Fatalf("invalid producer = (%v,%v), want zero/contract", left, leftErr)
			}
			for _, op := range []struct {
				name   string
				apply  func(currency.Amount, currency.Amount) (currency.Amount, error)
				oracle *big.Int
			}{
				{name: "add", apply: currency.Amount.Add, oracle: new(big.Int).Add(big.NewInt(tc.left), big.NewInt(tc.right))},
				{name: "subtract", apply: currency.Amount.Subtract, oracle: new(big.Int).Sub(big.NewInt(tc.left), big.NewInt(tc.right))},
			} {
				got, err := op.apply(left, right)
				wantErr := tc.wantErr
				if wantErr == nil && !op.oracle.IsInt64() {
					wantErr = core.ErrCurrencyOverflow
				}
				if wantErr != nil {
					if got != (currency.Amount{}) || !errors.Is(err, wantErr) || (tc.wantErr != nil && errors.Is(err, core.ErrCurrencyOverflow)) {
						t.Fatalf("%s refusal = (%v,%v), want zero/%v with admission precedence", op.name, got, err, wantErr)
					}
					continue
				}
				want := mustAmount(t, tc.leftCode, op.oracle.Int64())
				if err != nil || got != want {
					t.Fatalf("%s = (%v,%v), want (%v,nil)", op.name, got, err, want)
				}
			}
		})
	}
}

func TestCurrencyJSONProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		minor     int64
		duplicate bool
	}{
		{name: "positive minimum survives canonical projection", minor: math.MinInt64},
		{name: "negative duplicate member cannot overwrite a retained amount", minor: 1, duplicate: true},
		{name: "neutral zero remains a present exact amount"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			value := mustAmount(t, currency.CodeCLF, tc.minor)
			wire, err := value.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON = %v, want nil", err)
			}
			if tc.duplicate {
				// Repeat the real encoded member set: do not synthesize a second DTO.
				mutated := append(bytes.Clone(wire[:len(wire)-1]), ',')
				mutated = append(mutated, wire[1:]...)
				if bytes.Equal(mutated, wire) {
					t.Fatal("member duplication changed = false, want true")
				}
				wire = mutated
			}
			before := mustAmount(t, currency.CodeJPY, math.MaxInt64)
			got := before
			err = got.UnmarshalJSON(wire)
			if tc.duplicate {
				if got != before || !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrCurrencyContract) {
					t.Fatalf("duplicated wire = (%v,%v), want preserved/typed refusal", got, err)
				}
				return
			}
			if err != nil || got != value || got.Validate() != nil {
				t.Fatalf("projection = (%v,%v), want (%v,nil)", got, err, value)
			}
			second, err := got.MarshalJSON()
			if err != nil || !bytes.Equal(second, wire) {
				t.Fatalf("second projection = (%q,%v), want (%q,nil)", second, err, wire)
			}
		})
	}
}

// The typed constructor was the missing ingress: the existing decimal fuzzer
// deliberately folds its selector into valid codes and cannot prove this door.
func FuzzAmountNominalArithmetic(f *testing.F) {
	for _, seed := range []struct {
		code        currency.Code
		left, right int64
	}{
		{code: currency.CodeJPY},
		{code: currency.CodeCAD, left: math.MaxInt64, right: 1},
		{code: currency.CodeCLF, left: math.MinInt64, right: -1},
		{code: currency.CodeBHD, left: math.MaxInt64, right: math.MinInt64},
		{code: currency.CodeUSD, left: math.MinInt64, right: math.MinInt64},
	} {
		left, le := currency.New(seed.code, seed.left)
		right, re := currency.New(seed.code, seed.right)
		if le != nil || re != nil || left.Validate() != nil || right.Validate() != nil {
			f.Fatalf("seed construction = (%v,%v), want valid pair", le, re)
		}
		f.Add(uint8(seed.code), seed.left, uint8(seed.code), seed.right)
	}
	f.Add(uint8(currency.CodeUnknown), int64(1), uint8(currency.CodeCAD), int64(0))
	f.Add(uint8(currency.CodeCAD), int64(1), uint8(math.MaxUint8), int64(0))
	f.Add(uint8(currency.CodeCAD), int64(math.MaxInt64), uint8(currency.CodeUSD), int64(1))
	f.Fuzz(func(t *testing.T, leftCode uint8, leftMinor int64, rightCode uint8, rightMinor int64) {
		left, le := currency.New(currency.Code(leftCode), leftMinor)
		right, re := currency.New(currency.Code(rightCode), rightMinor)
		for _, input := range []struct {
			code  currency.Code
			minor int64
			got   currency.Amount
			err   error
		}{
			{currency.Code(leftCode), leftMinor, left, le}, {currency.Code(rightCode), rightMinor, right, re},
		} {
			_, valid := oracleFractionDigits(input.code)
			if !valid {
				if input.got != (currency.Amount{}) || !errors.Is(input.err, core.ErrCurrencyContract) {
					t.Fatalf("invalid nominal input = (%v,%v), want zero/contract", input.got, input.err)
				}
				continue
			}
			gc, ce := input.got.Code()
			gm, me := input.got.MinorUnits()
			if input.err != nil || ce != nil || me != nil || gc != input.code || gm != input.minor || input.got.Validate() != nil {
				t.Fatalf("nominal input = (%v,%d,%v), want (%v,%d,nil)", gc, gm, input.err, input.code, input.minor)
			}
		}
		var admission error
		if le != nil || re != nil {
			admission = core.ErrCurrencyContract
		} else if leftCode != rightCode {
			admission = core.ErrCurrencyMismatch
		}
		for _, op := range []struct {
			name   string
			apply  func(currency.Amount, currency.Amount) (currency.Amount, error)
			oracle *big.Int
		}{
			{"add", currency.Amount.Add, new(big.Int).Add(big.NewInt(leftMinor), big.NewInt(rightMinor))},
			{"subtract", currency.Amount.Subtract, new(big.Int).Sub(big.NewInt(leftMinor), big.NewInt(rightMinor))},
		} {
			got, err := op.apply(left, right)
			wantErr := admission
			if wantErr == nil && !op.oracle.IsInt64() {
				wantErr = core.ErrCurrencyOverflow
			}
			if wantErr != nil {
				if got != (currency.Amount{}) || !errors.Is(err, wantErr) || (admission != nil && errors.Is(err, core.ErrCurrencyOverflow)) {
					t.Fatalf("%s = (%v,%v), want zero/%v", op.name, got, err, wantErr)
				}
				continue
			}
			want := mustAmount(t, currency.Code(leftCode), op.oracle.Int64())
			if err != nil || got != want {
				t.Fatalf("%s = (%v,%v), want (%v,nil)", op.name, got, err, want)
			}
		}
		got, err := left.Compare(right)
		want := core.ComparisonUnknown
		if admission == nil {
			switch big.NewInt(leftMinor).Cmp(big.NewInt(rightMinor)) {
			case -1:
				want = core.ComparisonLess
			case 0:
				want = core.ComparisonEqual
			case 1:
				want = core.ComparisonGreater
			}
		}
		if !errors.Is(err, admission) || got != want {
			t.Fatalf("Compare = (%v,%v), want (%v,%v)", got, err, want, admission)
		}
	})
}

func TestDecimalFormattingMatchesRationalScaleAtEveryDigitWidth(t *testing.T) {
	t.Parallel()
	values := []int64{math.MinInt64, math.MaxInt64, 0}
	for power := int64(1); power <= math.MaxInt64/10; power *= 10 {
		for _, delta := range []int64{-1, 0, 1} {
			value := power + delta
			values = append(values, value, -value)
		}
	}
	// Include the last int64-safe power without overflowing the generator.
	for _, value := range []int64{1_000_000_000_000_000_000 - 1, 1_000_000_000_000_000_000, 1_000_000_000_000_000_000 + 1} {
		values = append(values, value, -value)
	}
	slices.Sort(values)
	values = slices.Compact(values)
	for _, code := range []currency.Code{currency.CodeJPY, currency.CodeCAD, currency.CodeBHD, currency.CodeCLF} {
		t.Run(code.String(), func(t *testing.T) {
			t.Parallel()
			digits, valid := oracleFractionDigits(code)
			if !valid {
				t.Fatalf("fixture code = %v, want admitted scale", code)
			}
			scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil)
			for _, minor := range values {
				t.Run(strconv.FormatInt(minor, 10), func(t *testing.T) {
					t.Parallel()
					amount := mustAmount(t, code, minor)
					want := new(big.Rat).SetFrac(big.NewInt(minor), scale).FloatString(int(digits))
					got, err := amount.Decimal()
					if err != nil || got != want || len(got) > currency.DecimalMaximumBytes {
						t.Fatalf("Decimal = (%q,%v), want (%q,nil) within byte ceiling", got, err, want)
					}
					roundTrip, err := currency.Parse(code, got)
					if err != nil || roundTrip != amount {
						t.Fatalf("Parse decimal = (%v,%v), want (%v,nil)", roundTrip, err, amount)
					}
				})
			}
		})
	}
}
