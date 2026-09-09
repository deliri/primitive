package currency

import (
	"errors"
	"math"
	"strconv"
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

func TestDecimalUnsignedOverflowPreservesStandardLibraryFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		raw       string
		code      Code
		wantRange bool
	}{
		{name: "unsigned ceiling plus one keeps Go range error", code: CodeJPY, raw: "18446744073709551616", wantRange: true},
		{name: "negative unsigned overflow keeps Go range error", code: CodeJPY, raw: "-18446744073709551616", wantRange: true},
		{name: "minor-unit padding overflows Go unsigned conversion", code: CodeCLF, raw: "1844674407370956", wantRange: true},
		{name: "signed currency bound does not fabricate Go unsigned failure", code: CodeJPY, raw: "9223372036854775808"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tc.code, tc.raw)
			var native *strconv.NumError
			if got != (Amount{}) || !errors.Is(err, core.ErrCurrencyOverflow) || !errors.Is(err, core.ErrNumericOverflow) || errors.Is(err, strconv.ErrRange) != tc.wantRange || errors.As(err, &native) != tc.wantRange {
				t.Fatalf("decimal overflow = (%v,%v,native=%v), want zero/currency overflow/native range=%t", got, err, native, tc.wantRange)
			}
		})
	}
}

// This tests the production converter directly because decimalDigits currently
// excludes syntax failures. Grammar changes must not turn syntax into overflow.
func TestDecimalMagnitudeClassifiesNativeFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		digits string
		want   uint64
		cause  error
	}{
		{name: "empty digits are syntax", digits: "", cause: strconv.ErrSyntax},
		{name: "embedded nondigit is syntax", digits: "12x3", cause: strconv.ErrSyntax},
		{name: "minus cannot enter magnitude", digits: "-1", cause: strconv.ErrSyntax},
		{name: "unsigned maximum plus one is range", digits: "18446744073709551616", cause: strconv.ErrRange},
		{name: "zero is retained", digits: "0"},
		{name: "unsigned maximum stays available to signed bound", digits: "18446744073709551615", want: math.MaxUint64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseDecimalMagnitude(tc.digits)
			if got != tc.want || !errors.Is(err, tc.cause) {
				t.Fatalf("magnitude = (%d, %v), want (%d, %v)", got, err, tc.want, tc.cause)
			}
			wantOverflow := errors.Is(tc.cause, strconv.ErrRange)
			if errors.Is(err, core.ErrCurrencyOverflow) != wantOverflow || errors.Is(err, core.ErrNumericOverflow) != wantOverflow {
				t.Fatalf("magnitude error = %v, want overflow identity only on range=%t", err, wantOverflow)
			}
			if errors.Is(err, core.ErrCurrencyDecimal) != errors.Is(tc.cause, strconv.ErrSyntax) {
				t.Fatalf("magnitude error = %v, want decimal identity only on syntax", err)
			}
			var native *strconv.NumError
			if errors.As(err, &native) != (tc.cause != nil) {
				t.Fatalf("magnitude error = %v, want native error presence=%t", err, tc.cause != nil)
			}
			if tc.cause != nil && (native.Num != tc.digits || !errors.Is(native.Err, tc.cause)) {
				t.Fatalf("magnitude error = %v, want exact native input/cause", err)
			}
		})
	}
}
