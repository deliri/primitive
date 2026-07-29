package currency

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestEveryAdmittedCodeHasACompleteDefinitionRow walks the closed domain
// itself rather than a hand-listed table.
//
// The definition table is an array indexed by Code, so a code inserted into the
// middle of the iota block shifts every later index and silently leaves one
// admitted code with the zero definition row: the empty token and a zero
// exponent. Appending at the end is a compile error because the array literal
// shrinks, but inserting in the middle is not. A hand-listed matrix keeps
// passing in that state because it never names the new code.
//
// This is the same failure class Core gates in its Go identifier domains, where
// the projection also answers the empty string outside the closed domain.
func TestEveryAdmittedCodeHasACompleteDefinitionRow(t *testing.T) {
	t.Parallel()

	var tokens [codeLimit]string
	for code := CodeUSD; code < codeLimit; code++ {
		if gotErr := code.Validate(); gotErr != nil {
			t.Fatalf("Code(%d).Validate() error = %v, want nil for an admitted code", code, gotErr)
		}

		token := code.String()
		if token == "" {
			t.Fatalf("Code(%d).String() = empty, want a canonical token; its definition row is missing", code)
		}
		for prior := CodeUSD; prior < code; prior++ {
			if token == tokens[prior] {
				t.Fatalf("Code(%d).String() = %q, which duplicates Code(%d)", code, token, prior)
			}
		}
		tokens[code] = token

		gotParsed, gotParseErr := ParseCode(token)
		if gotParseErr != nil || gotParsed != code {
			t.Fatalf("ParseCode(%q) = (%v, %v), want (%v, nil)", token, gotParsed, gotParseErr, code)
		}

		gotDigits, gotDigitsErr := code.FractionDigits()
		if gotDigitsErr != nil {
			t.Fatalf("Code(%d).FractionDigits() error = %v, want nil", code, gotDigitsErr)
		}
		switch gotDigits {
		case MinorUnitDigitsZero, MinorUnitDigitsTwo, MinorUnitDigitsThree, MinorUnitDigitsFour:
		default:
			t.Fatalf("Code(%d).FractionDigits() = %d, want an admitted minor-unit exponent", code, gotDigits)
		}
	}
}

// TestEmptyTokenNeverResolvesToAnAdmittedCode is the runtime half of the gate
// in ParseCode. String answers the empty token for every value outside the
// closed domain and for any admitted code whose definition row went missing,
// so the empty token must never select a currency.
func TestEmptyTokenNeverResolvesToAnAdmittedCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		token string
	}{
		{name: "empty token"},
		{name: "unknown code projection", token: CodeUnknown.String()},
		{name: "limit projection", token: codeLimit.String()},
		{name: "maximum backing value projection", token: Code(math.MaxUint8).String()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseCode(tc.token)
			if got != CodeUnknown || !errors.Is(gotErr, core.ErrCurrencyContract) {
				t.Fatalf(
					"ParseCode(%q) = (%v, %v), want (%v, %v)",
					tc.token,
					got,
					gotErr,
					CodeUnknown,
					core.ErrCurrencyContract,
				)
			}
		})
	}
}

// TestDecimalMaximumBytesIsTheObservedCeilingAcrossTheClosedDomain derives the
// bound instead of trusting the literal.
//
// DecimalMaximumBytes gates external decimal input, and Decimal is the
// projection that must survive that gate. If the two ever disagree, an amount
// this package produced could not be re-parsed by this package. The bound is
// also exponent-sensitive: padding makes the widest projection grow with the
// exponent, so admitting a currency with a larger exponent silently invalidates
// a hand-written literal.
func TestDecimalMaximumBytesIsTheObservedCeilingAcrossTheClosedDomain(t *testing.T) {
	t.Parallel()

	extremes := []struct {
		name       string
		minorUnits int64
	}{
		{name: "int64 minimum", minorUnits: math.MinInt64},
		{name: "one above int64 minimum", minorUnits: math.MinInt64 + 1},
		{name: "negative one", minorUnits: -1},
		{name: "zero", minorUnits: 0},
		{name: "one", minorUnits: 1},
		{name: "one below int64 maximum", minorUnits: math.MaxInt64 - 1},
		{name: "int64 maximum", minorUnits: math.MaxInt64},
	}

	widest := 0
	for code := CodeUSD; code < codeLimit; code++ {
		for _, tc := range extremes {
			amount, gotNewErr := New(code, tc.minorUnits)
			if gotNewErr != nil {
				t.Fatalf("New(%v, %d) error = %v, want nil", code, tc.minorUnits, gotNewErr)
			}
			decimal, gotDecimalErr := amount.Decimal()
			if gotDecimalErr != nil {
				t.Fatalf("New(%v, %d).Decimal() error = %v, want nil", code, tc.minorUnits, gotDecimalErr)
			}
			if len(decimal) > DecimalMaximumBytes {
				t.Fatalf(
					"New(%v, %d).Decimal() = %q (%d bytes), want at most DecimalMaximumBytes = %d",
					code,
					tc.minorUnits,
					decimal,
					len(decimal),
					DecimalMaximumBytes,
				)
			}
			widest = max(widest, len(decimal))

			gotRoundTrip, gotParseErr := Parse(code, decimal)
			if gotParseErr != nil || gotRoundTrip != amount {
				t.Fatalf(
					"Parse(%v, %q) = (%v, %v), want (%v, nil)",
					code,
					decimal,
					gotRoundTrip,
					gotParseErr,
					amount,
				)
			}
		}
	}

	if widest != DecimalMaximumBytes {
		t.Fatalf(
			"widest Decimal() projection across the closed domain = %d bytes, want DecimalMaximumBytes = %d; the bound is no longer exact",
			widest,
			DecimalMaximumBytes,
		)
	}
}

// TestEveryAdmittedOrderHasADistinctToken applies the same closure rule to the
// comparison domain, whose token table is indexed the same way.
func TestEveryAdmittedOrderHasADistinctToken(t *testing.T) {
	t.Parallel()

	var tokens [orderLimit]string
	for order := OrderLess; order < orderLimit; order++ {
		if gotErr := order.Validate(); gotErr != nil {
			t.Fatalf("Order(%d).Validate() error = %v, want nil for an admitted order", order, gotErr)
		}
		token := order.String()
		if token == "" {
			t.Fatalf("Order(%d).String() = empty, want a comparison token; its table row is missing", order)
		}
		for prior := OrderLess; prior < order; prior++ {
			if token == tokens[prior] {
				t.Fatalf("Order(%d).String() = %q, which duplicates Order(%d)", order, token, prior)
			}
		}
		tokens[order] = token
	}

	outside := []struct {
		name  string
		order Order
	}{
		{name: "unknown order", order: OrderUnknown},
		{name: "limit order", order: orderLimit},
		{name: "maximum backing value", order: Order(math.MaxUint8)},
	}
	for _, tc := range outside {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.order.String(); got != "" {
				t.Fatalf("Order(%d).String() = %q, want empty outside the closed domain", tc.order, got)
			}
			if gotErr := tc.order.Validate(); !errors.Is(gotErr, core.ErrCurrencyContract) {
				t.Fatalf("Order(%d).Validate() error = %v, want %v", tc.order, gotErr, core.ErrCurrencyContract)
			}
		})
	}
}
