package currency

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestDecimalRejectionReportsTheRuleThatActuallyFired pins the second tier of
// the decimal rejection contract. Every rejection already carries
// core.ErrCurrencyDecimal, so the identity alone cannot tell a reviewer which
// rule rejected the input. That is exactly how a rejection can be attributed to
// the wrong rule and stay green: a discarded inner error is replaced by an
// unrelated message while the sentinel keeps matching.
//
// The want field is a package constant, not a repeated literal, so the
// diagnostic has one compiler-visible home shared by production and this table.
func TestDecimalRejectionReportsTheRuleThatActuallyFired(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want decimalRejection
		code Code
	}{
		{name: "empty input is a byte-length rejection", code: CodeCAD, raw: "", want: decimalRejectionLength},
		{name: "one byte above the bound is a byte-length rejection", code: CodeCAD, raw: strings.Repeat("0", DecimalMaximumBytes+1), want: decimalRejectionLength},
		{name: "explicit plus sign is a sign rejection", code: CodeCAD, raw: "+1.00", want: decimalRejectionSign},
		{name: "lone minus is a sign rejection", code: CodeCAD, raw: "-", want: decimalRejectionSign},
		{name: "double minus is a whole-unit rejection", code: CodeCAD, raw: "--1.00", want: decimalRejectionWhole},
		{name: "bare separator has no whole units", code: CodeCAD, raw: ".01", want: decimalRejectionWhole},
		{name: "negative bare separator has no whole units", code: CodeCAD, raw: "-.01", want: decimalRejectionWhole},
		{name: "non-digit whole units are a whole-unit rejection", code: CodeCAD, raw: "1a.00", want: decimalRejectionWhole},
		{name: "whitespace in whole units is a whole-unit rejection", code: CodeCAD, raw: "1 .00", want: decimalRejectionWhole},
		{name: "empty fraction is a fraction rejection", code: CodeCAD, raw: "1.", want: decimalRejectionFraction},
		{name: "fraction beyond the exponent is a fraction rejection", code: CodeCAD, raw: "1.001", want: decimalRejectionFraction},
		{name: "any fraction for a zero-exponent currency is a fraction rejection", code: CodeJPY, raw: "1.0", want: decimalRejectionFraction},
		{name: "non-digit fraction is a fraction rejection", code: CodeCAD, raw: "1.0a", want: decimalRejectionFraction},
		{name: "negative zero is rejected after valid digit accumulation", code: CodeCAD, raw: "-0.00", want: decimalRejectionNegativeZero},

		// A second separator is a fraction-side fact: strings.Cut keeps the
		// whole units intact and leaves the surplus separator inside the
		// fraction, where the exponent bound and the digit rule reject it.
		// Attributing this to the whole units would be a false report about
		// whole units that parsed correctly.
		{name: "second separator within the exponent bound is a fraction rejection", code: CodeCLF, raw: "1.0.0", want: decimalRejectionFraction},
		{name: "second separator beyond the exponent bound is a fraction rejection", code: CodeCAD, raw: "1.0.0", want: decimalRejectionFraction},
		{name: "trailing separator after a full fraction is a fraction rejection", code: CodeCAD, raw: "1.00.", want: decimalRejectionFraction},
		{name: "leading separator inside the fraction is a fraction rejection", code: CodeCLF, raw: "1..0", want: decimalRejectionFraction},
		{name: "separator-only fraction is a fraction rejection", code: CodeCLF, raw: "1..", want: decimalRejectionFraction},
		{name: "three separators are a fraction rejection", code: CodeCLF, raw: "1.0.0.", want: decimalRejectionFraction},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := Parse(tc.code, tc.raw)
			if !errors.Is(gotErr, core.ErrCurrencyDecimal) {
				t.Fatalf("Parse(%v, %q) error = %v, want %v", tc.code, tc.raw, gotErr, core.ErrCurrencyDecimal)
			}
			if got != (Amount{}) {
				t.Fatalf("Parse(%v, %q) = %v, want zero amount", tc.code, tc.raw, got)
			}
			var gotReason decimalRejection
			if !errors.As(gotErr, &gotReason) || gotReason != tc.want {
				t.Fatalf(
					"Parse(%v, %q) rejection = %v, want typed rule %v",
					tc.code,
					tc.raw,
					gotReason,
					tc.want,
				)
			}
		})
	}
}
