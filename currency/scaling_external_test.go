package currency_test

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/currency"
)

// The independent oracle uses arbitrary precision and signed Euclidean floor;
// production uses fixed-width magnitude arithmetic and math/bits.
func rateOracle(minor int64, rate uint64, policy currency.Rounding) *big.Int {
	product := new(big.Int).Mul(big.NewInt(minor), new(big.Int).SetUint64(rate))
	denominator := big.NewInt(10000)
	if policy == currency.RoundingDown {
		return new(big.Int).Div(product, denominator)
	}
	negative := product.Sign() < 0
	product.Abs(product)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(product, denominator, remainder)
	comparison := new(big.Int).Lsh(remainder, 1).Cmp(denominator)
	if comparison > 0 || comparison == 0 && (policy == currency.RoundingHalfUp || quotient.Bit(0) == 1) {
		quotient.Add(quotient, big.NewInt(1))
	}
	if negative {
		quotient.Neg(quotient)
	}
	return quotient
}

func TestAmountScalingMatchesIndependentExactBoundaryMatrix(t *testing.T) {
	t.Parallel()
	// These inputs cover zero, signed endpoints, exact divisibility, odd/even
	// ties and each side of ties; rates cover neutral, subunit, 100%, growth,
	// uint64 multiply overflow and a 128-bit intermediate with a valid quotient.
	minors := []int64{math.MinInt64, math.MinInt64 + 1, -10001, -10000, -3, -2, -1, 0, 1, 2, 3, 9999, 10000, 10001, math.MaxInt64 - 1, math.MaxInt64}
	rates := []uint64{0, 1, 4999, 5000, 5001, 9999, 10000, 10001, 20000, math.MaxUint64}
	for _, minor := range minors {
		for _, rawRate := range rates {
			for _, policy := range []currency.Rounding{currency.RoundingDown, currency.RoundingHalfUp, currency.RoundingHalfEven} {
				t.Run(fmt.Sprintf("minor-%d-rate-%d-round-%s", minor, rawRate, policy), func(t *testing.T) {
					t.Parallel()
					amount := mustAmount(t, currency.CodeCAD, minor)
					rate, err := currency.NewBasisPoints(rawRate)
					if err != nil {
						t.Fatalf("rate construction = %v, want nil", err)
					}
					got, err := amount.ApplyBasisPoints(rate, policy)
					want := rateOracle(minor, rawRate, policy)
					if !want.IsInt64() {
						if got != (currency.Amount{}) || !errors.Is(err, core.ErrCurrencyOverflow) || errors.Is(err, core.ErrCurrencyContract) || errors.Is(err, core.ErrCurrencyDecimal) || errors.Is(err, core.ErrNumericOverflow) {
							t.Fatalf("rate result = (%v,%v), want zero/currency overflow for %v", got, err, want)
						}
						return
					}
					if err != nil || got != mustAmount(t, currency.CodeCAD, want.Int64()) {
						t.Fatalf("rate result = (%v,%v), want exact %v", got, err, want)
					}
				})
			}
		}
	}
	for _, minor := range minors {
		for _, count := range []uint64{0, 1, 2, 3, math.MaxInt64, 1 << 63, math.MaxUint64} {
			t.Run(fmt.Sprintf("multiply-%d-count-%d", minor, count), func(t *testing.T) {
				t.Parallel()
				quantity, err := currency.NewQuantity(count)
				if err != nil {
					t.Fatalf("quantity = %v, want nil", err)
				}
				got, err := mustAmount(t, currency.CodeCAD, minor).MultiplyQuantity(quantity)
				want := new(big.Int).Mul(big.NewInt(minor), new(big.Int).SetUint64(count))
				if !want.IsInt64() {
					if got != (currency.Amount{}) || !errors.Is(err, core.ErrCurrencyOverflow) || errors.Is(err, core.ErrNumericOverflow) || errors.Is(err, core.ErrCurrencyContract) {
						t.Fatalf("quantity product = (%v,%v), want zero/currency overflow", got, err)
					}
					return
				}
				if err != nil || got != mustAmount(t, currency.CodeCAD, want.Int64()) {
					t.Fatalf("quantity product = (%v,%v), want %v", got, err, want)
				}
			})
		}
	}
}

func TestCurrencyControlEnumsExhaustClosedDomains(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		t.Run(fmt.Sprintf("closed-policy-byte-%d", raw), func(t *testing.T) {
			t.Parallel()
			rounding, sign := currency.Rounding(raw), currency.Sign(raw)
			want := raw >= 1 && raw <= 3
			if rounding.IsValid() != want || sign.IsValid() != want || (rounding.Validate() == nil) != want || (sign.Validate() == nil) != want || (rounding.String() != "") != want || (sign.String() != "") != want {
				t.Fatalf("enum byte %d = (%v,%v), want admitted=%t", raw, rounding, sign, want)
			}
			if !want && (!errors.Is(rounding.Validate(), core.ErrCurrencyContract) || !errors.Is(sign.Validate(), core.ErrCurrencyContract)) {
				t.Fatalf("enum refusal = (%v,%v), want currency contract", rounding.Validate(), sign.Validate())
			}
			rounding.OffWireEnum()
			sign.OffWireEnum()
		})
	}
}

func TestCurrencyArithmeticAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	amount := mustAmount(t, currency.CodeUSD, 5)
	zero, err := currency.NewQuantity(0)
	if err != nil {
		t.Fatalf("quantity = %v, want nil", err)
	}
	zeroRate, err := currency.NewBasisPoints(0)
	if err != nil {
		t.Fatalf("rate = %v, want nil", err)
	}
	failures := []struct {
		name string
		call func() (currency.Amount, error)
	}{
		{name: "unconstructed amount multiply", call: func() (currency.Amount, error) { return (currency.Amount{}).MultiplyQuantity(zero) }},
		{name: "unconstructed quantity", call: func() (currency.Amount, error) { return amount.MultiplyQuantity(currency.Quantity{}) }},
		{name: "unconstructed amount rate", call: func() (currency.Amount, error) {
			return (currency.Amount{}).ApplyBasisPoints(zeroRate, currency.RoundingDown)
		}},
		{name: "unconstructed rate", call: func() (currency.Amount, error) {
			return amount.ApplyBasisPoints(currency.BasisPoints{}, currency.RoundingDown)
		}},
		{name: "unselected rounding", call: func() (currency.Amount, error) { return amount.ApplyBasisPoints(zeroRate, currency.RoundingUnknown) }},
		{name: "future rounding", call: func() (currency.Amount, error) { return amount.ApplyBasisPoints(zeroRate, currency.Rounding(255)) }},
		{name: "unconstructed negate", call: func() (currency.Amount, error) { return (currency.Amount{}).Negate() }},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.call()
			if got != (currency.Amount{}) || !errors.Is(err, core.ErrCurrencyContract) || errors.Is(err, core.ErrCurrencyOverflow) {
				t.Fatalf("arithmetic admission = (%v,%v), want zero/contract without overflow", got, err)
			}
		})
	}
	got, err := amount.MultiplyQuantity(zero)
	if err != nil || got != mustAmount(t, currency.CodeUSD, 0) {
		t.Fatalf("zero count = (%v,%v), want admitted currency zero", got, err)
	}
	got, err = amount.ApplyBasisPoints(zeroRate, currency.RoundingHalfEven)
	if err != nil || got != mustAmount(t, currency.CodeUSD, 0) {
		t.Fatalf("zero rate = (%v,%v), want currency zero", got, err)
	}
}

func TestAmountSignAndNegationConserveExactValue(t *testing.T) {
	t.Parallel()
	for _, minor := range []int64{math.MinInt64, math.MinInt64 + 1, -1, 0, 1, math.MaxInt64 - 1, math.MaxInt64} {
		t.Run(fmt.Sprintf("signed-boundary-%d", minor), func(t *testing.T) {
			t.Parallel()
			amount := mustAmount(t, currency.CodeUSD, minor)
			gotSign, err := amount.Sign()
			wantSign := currency.SignZero
			if minor < 0 {
				wantSign = currency.SignNegative
			}
			if minor > 0 {
				wantSign = currency.SignPositive
			}
			if err != nil || gotSign != wantSign {
				t.Fatalf("sign = (%v,%v), want %v", gotSign, err, wantSign)
			}
			got, err := amount.Negate()
			if minor == math.MinInt64 {
				if got != (currency.Amount{}) || !errors.Is(err, core.ErrCurrencyOverflow) {
					t.Fatalf("minimum negate = (%v,%v), want zero/overflow", got, err)
				}
				return
			}
			if err != nil || got != mustAmount(t, currency.CodeUSD, -minor) {
				t.Fatalf("inverse = (%v,%v), want %d", got, err, -minor)
			}
			restored, err := got.Negate()
			if err != nil || restored != amount {
				t.Fatalf("double inverse = (%v,%v), want original %v", restored, err, amount)
			}
		})
	}
	if sign, err := (currency.Amount{}).Sign(); sign != currency.SignUnknown || !errors.Is(err, core.ErrCurrencyContract) {
		t.Fatalf("unconstructed sign = (%v,%v), want unknown/contract", sign, err)
	}
}

func TestEqualAllocationStreamsAndConservesTotals(t *testing.T) {
	t.Parallel()
	for _, minor := range []int64{math.MinInt64, math.MinInt64 + 1, -10001, -3, -1, 0, 1, 3, 10001, math.MaxInt64} {
		for _, count := range []uint64{1, 2, 3, 32767, 32768, 32769, 262151} {
			t.Run(fmt.Sprintf("total-%d-shares-%d", minor, count), func(t *testing.T) {
				t.Parallel()
				amount := mustAmount(t, currency.CodeCAD, minor)
				shares, err := currency.NewQuantity(count)
				if err != nil {
					t.Fatalf("share count = %v, want nil", err)
				}
				sequence, err := amount.Allocate(shares)
				if err != nil {
					t.Fatalf("allocate = %v, want nil", err)
				}
				for traversal := range 2 {
					var sum big.Int
					var seen uint64
					var minimum, maximum int64
					for part := range sequence {
						value, err := part.MinorUnits()
						code, codeErr := part.Code()
						if err != nil || codeErr != nil || code != currency.CodeCAD {
							t.Fatalf("allocated part = (%v,%v,%v), want admitted same currency", part, err, codeErr)
						}
						if seen == 0 || value < minimum {
							minimum = value
						}
						if seen == 0 || value > maximum {
							maximum = value
						}
						seen++
						sum.Add(&sum, big.NewInt(value))
					}
					if seen != count || sum.Cmp(big.NewInt(minor)) != 0 || new(big.Int).Sub(big.NewInt(maximum), big.NewInt(minimum)).Cmp(big.NewInt(1)) > 0 {
						t.Fatalf("traversal %d = (%d,%v,span %d..%d), want count %d/exact %d/span <=1", traversal, seen, &sum, minimum, maximum, count, minor)
					}
				}
			})
		}
	}
	amount := mustAmount(t, currency.CodeUSD, math.MinInt64)
	huge, err := currency.NewQuantity(math.MaxUint64)
	if err != nil {
		t.Fatalf("huge quantity = %v, want nil", err)
	}
	sequence, err := amount.Allocate(huge)
	if err != nil {
		t.Fatalf("huge allocate = %v, want nil", err)
	}
	seen := 0
	for part := range sequence {
		if part != mustAmount(t, currency.CodeUSD, -1) {
			t.Fatalf("first huge share = %v, want -1", part)
		}
		seen++
		break
	}
	if seen != 1 {
		t.Fatalf("early stop = %d effects, want one", seen)
	}
	for _, shares := range []currency.Quantity{{}, mustQuantity(t, 0)} {
		seq, err := amount.Allocate(shares)
		if seq != nil || !errors.Is(err, core.ErrCurrencyContract) {
			t.Fatalf("empty allocation = (%v,%v), want nil/contract", seq, err)
		}
	}
	if seq, err := (currency.Amount{}).Allocate(mustQuantity(t, 1)); seq != nil || !errors.Is(err, core.ErrCurrencyContract) {
		t.Fatalf("invalid amount allocation = (%v,%v), want nil/contract", seq, err)
	}
}

func mustQuantity(t *testing.T, count uint64) currency.Quantity {
	t.Helper()
	got, err := currency.NewQuantity(count)
	if err != nil {
		t.Fatalf("quantity construction = %v, want nil", err)
	}
	return got
}

func TestProviderCodeASCIIAdmissionAndProjection(t *testing.T) {
	t.Parallel()
	for code := currency.CodeUSD; code <= currency.CodeCLF; code++ {
		token := code.String()
		for mask := range 8 {
			input := []byte(token)
			for index := range 3 {
				if mask&(1<<index) != 0 {
					input[index] += 'a' - 'A'
				}
			}
			t.Run(fmt.Sprintf("currency-%s-ascii-mask-%d", token, mask), func(t *testing.T) {
				t.Parallel()
				got, err := currency.ParseProviderCode(string(input))
				if err != nil || got != code {
					t.Fatalf("provider parse %q = (%v,%v), want %v", input, got, err, code)
				}
				wire, err := got.ProviderCode()
				if err != nil || wire != strings.ToLower(token) {
					t.Fatalf("provider projection = (%q,%v), want lowercase %s", wire, err, token)
				}
			})
		}
	}
	for _, input := range []string{"", "US", "USDD", " usd", "usd ", "u\x00d", "u\xffd", "u1d", "uſd", "ZZZ", "u$d", "\nusd"} {
		t.Run(fmt.Sprintf("foreign-provider-token-%q", input), func(t *testing.T) {
			t.Parallel()
			got, err := currency.ParseProviderCode(input)
			if got != currency.CodeUnknown || !errors.Is(err, core.ErrCurrencyContract) {
				t.Fatalf("provider parse %q = (%v,%v), want unknown/contract", input, got, err)
			}
		})
	}
	for _, code := range []currency.Code{currency.CodeUnknown, currency.CodeCLF + 1, currency.Code(255)} {
		if wire, err := code.ProviderCode(); wire != "" || !errors.Is(err, core.ErrCurrencyContract) {
			t.Fatalf("invalid provider projection = (%q,%v), want empty/contract", wire, err)
		}
	}
}

func FuzzProviderCodeSemanticClosure(f *testing.F) {
	for code := currency.CodeUSD; code <= currency.CodeCLF; code++ {
		wire, err := code.ProviderCode()
		if err != nil {
			f.Fatalf("provider seed = %v, want nil", err)
		}
		f.Add(wire)
	}
	for _, wire := range []string{"", "US", "USDX", "u\x00d", "uſd", " usd", "ZZZ"} {
		f.Add(wire)
	}
	f.Fuzz(func(t *testing.T, wire string) {
		var want currency.Code
		admitted := false
		ascii := len(wire) == 3
		if ascii {
			want, admitted = oracleCode(strings.ToUpper(wire))
		}
		for index := range len(wire) {
			value := wire[index]
			ascii = ascii && (value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z')
		}
		admitted = admitted && ascii
		got, err := currency.ParseProviderCode(wire)
		if !admitted {
			if got != currency.CodeUnknown || !errors.Is(err, core.ErrCurrencyContract) {
				t.Fatalf("provider refusal = (%v,%v), want unknown/contract", got, err)
			}
			return
		}
		if err != nil || got != want || got.Validate() != nil {
			t.Fatalf("provider admission = (%v,%v), want %v", got, err, want)
		}
		canonical, err := got.ProviderCode()
		roundTrip, parseErr := currency.ParseProviderCode(canonical)
		if err != nil || parseErr != nil || roundTrip != got || canonical != strings.ToLower(wire) {
			t.Fatalf("provider closure = (%q,%v,%v,%v), want exact normalized currency", canonical, roundTrip, err, parseErr)
		}
	})
}

func FuzzScalingAgainstBigIntegerOracle(f *testing.F) {
	for _, seed := range []struct {
		minor    int64
		factor   uint64
		rounding uint8
	}{{0, 0, uint8(currency.RoundingDown)}, {math.MinInt64, 1, uint8(currency.RoundingDown)}, {math.MaxInt64, 10000, uint8(currency.RoundingHalfEven)}, {3, 5000, uint8(currency.RoundingHalfUp)}, {-3, 5000, uint8(currency.RoundingHalfEven)}, {1, math.MaxUint64, uint8(currency.RoundingDown)}, {1, 1, 255}} {
		amount, err := currency.New(currency.CodeUSD, seed.minor)
		if err != nil || amount.Validate() != nil {
			f.Fatalf("scaling seed amount = (%v,%v), want admitted", amount, err)
		}
		minor, err := amount.MinorUnits()
		if err != nil {
			f.Fatalf("seed projection = %v, want nil", err)
		}
		f.Add(minor, seed.factor, seed.rounding)
	}
	f.Fuzz(func(t *testing.T, minor int64, factor uint64, rawPolicy uint8) {
		amount := mustAmount(t, currency.CodeUSD, minor)
		quantity, err := currency.NewQuantity(factor)
		if err != nil || quantity.Validate() != nil {
			t.Fatalf("quantity constructor = (%v,%v), want admitted", quantity, err)
		}
		rate, err := currency.NewBasisPoints(factor)
		if err != nil || rate.Validate() != nil {
			t.Fatalf("rate constructor = (%v,%v), want admitted", rate, err)
		}
		got, err := amount.MultiplyQuantity(quantity)
		want := new(big.Int).Mul(big.NewInt(minor), new(big.Int).SetUint64(factor))
		proveScaleOracle(t, got, err, want)
		policy := currency.Rounding(rawPolicy)
		got, err = amount.ApplyBasisPoints(rate, policy)
		if !policy.IsValid() {
			if got != (currency.Amount{}) || !errors.Is(err, core.ErrCurrencyContract) || errors.Is(err, core.ErrCurrencyOverflow) {
				t.Fatalf("policy refusal = (%v,%v), want zero/contract", got, err)
			}
			return
		}
		proveScaleOracle(t, got, err, rateOracle(minor, factor, policy))
	})
}

func proveScaleOracle(t *testing.T, got currency.Amount, err error, want *big.Int) {
	t.Helper()
	if !want.IsInt64() {
		if got != (currency.Amount{}) || !errors.Is(err, core.ErrCurrencyOverflow) || errors.Is(err, core.ErrCurrencyDecimal) || errors.Is(err, core.ErrCurrencyContract) || errors.Is(err, core.ErrNumericOverflow) {
			t.Fatalf("scaling refusal = (%v,%v), want zero/currency overflow", got, err)
		}
		return
	}
	if err != nil || got != mustAmount(t, currency.CodeUSD, want.Int64()) {
		t.Fatalf("scaling admission = (%v,%v), want exact %v", got, err, want)
	}
}
