package currency_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/currency"
)

func TestDecimalBoundaryAndNormalizationMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr   error
		name      string
		raw       string
		want      string
		wantMinor int64
		code      currency.Code
	}{
		{name: "JPY zero", code: currency.CodeJPY, raw: "0", want: "0"},
		{name: "JPY maximum", code: currency.CodeJPY, raw: "9223372036854775807", want: "9223372036854775807", wantMinor: math.MaxInt64},
		{name: "JPY minimum", code: currency.CodeJPY, raw: "-9223372036854775808", want: "-9223372036854775808", wantMinor: math.MinInt64},
		{name: "CAD integer pads fraction", code: currency.CodeCAD, raw: "12", want: "12.00", wantMinor: 1200},
		{name: "CAD one fraction digit pads", code: currency.CodeCAD, raw: "12.3", want: "12.30", wantMinor: 1230},
		{name: "CAD exact fraction", code: currency.CodeCAD, raw: "12.34", want: "12.34", wantMinor: 1234},
		{name: "CAD leading zeros normalize", code: currency.CodeCAD, raw: "00012.30", want: "12.30", wantMinor: 1230},
		{name: "CAD negative subunit", code: currency.CodeCAD, raw: "-0.01", want: "-0.01", wantMinor: -1},
		{name: "CAD smallest positive integer", code: currency.CodeCAD, raw: "1", want: "1.00", wantMinor: 100},
		{name: "CAD smallest negative integer", code: currency.CodeCAD, raw: "-1", want: "-1.00", wantMinor: -100},
		{name: "CAD exact one below byte bound zeros normalize", code: currency.CodeCAD, raw: strings.Repeat("0", currency.DecimalMaximumBytes-1), want: "0.00"},
		{name: "CAD exact byte bound zeros normalize", code: currency.CodeCAD, raw: strings.Repeat("0", currency.DecimalMaximumBytes), want: "0.00"},
		{name: "BHD one fraction digit pads", code: currency.CodeBHD, raw: "12.3", want: "12.300", wantMinor: 12300},
		{name: "BHD two fraction digits pad", code: currency.CodeBHD, raw: "12.34", want: "12.340", wantMinor: 12340},
		{name: "BHD three fraction digits", code: currency.CodeBHD, raw: "12.345", want: "12.345", wantMinor: 12345},
		{name: "CLF one fraction digit pads", code: currency.CodeCLF, raw: "12.3", want: "12.3000", wantMinor: 123000},
		{name: "CLF two fraction digits pad", code: currency.CodeCLF, raw: "12.34", want: "12.3400", wantMinor: 123400},
		{name: "CLF three fraction digits pad", code: currency.CodeCLF, raw: "12.345", want: "12.3450", wantMinor: 123450},
		{name: "CLF four fraction digits", code: currency.CodeCLF, raw: "12.3456", want: "12.3456", wantMinor: 123456},
		{name: "CLF maximum", code: currency.CodeCLF, raw: "922337203685477.5807", want: "922337203685477.5807", wantMinor: math.MaxInt64},
		{name: "CLF minimum", code: currency.CodeCLF, raw: "-922337203685477.5808", want: "-922337203685477.5808", wantMinor: math.MinInt64},
		{name: "empty rejected", code: currency.CodeCAD, wantErr: core.ErrCurrencyDecimal},
		{name: "unknown currency rejected before decimal", code: currency.CodeUnknown, raw: "1.00", wantErr: core.ErrCurrencyContract},
		{name: "future currency rejected before decimal", code: currency.CodeCLF + 1, raw: "1.00", wantErr: core.ErrCurrencyContract},
		{name: "plus rejected", code: currency.CodeCAD, raw: "+1.00", wantErr: core.ErrCurrencyDecimal},
		{name: "minus without magnitude rejected", code: currency.CodeCAD, raw: "-", wantErr: core.ErrCurrencyDecimal},
		{name: "double minus rejected", code: currency.CodeCAD, raw: "--1.00", wantErr: core.ErrCurrencyDecimal},
		{name: "negative zero rejected", code: currency.CodeCAD, raw: "-0", wantErr: core.ErrCurrencyDecimal},
		{name: "negative fractional zero rejected", code: currency.CodeCAD, raw: "-0.00", wantErr: core.ErrCurrencyDecimal},
		{name: "bare decimal rejected", code: currency.CodeCAD, raw: ".01", wantErr: core.ErrCurrencyDecimal},
		{name: "empty fraction rejected", code: currency.CodeCAD, raw: "1.", wantErr: core.ErrCurrencyDecimal},
		{name: "too many fraction digits rejected", code: currency.CodeCAD, raw: "1.001", wantErr: core.ErrCurrencyDecimal},
		{name: "one above BHD fraction bound rejected", code: currency.CodeBHD, raw: "1.0001", wantErr: core.ErrCurrencyDecimal},
		{name: "one above CLF fraction bound rejected", code: currency.CodeCLF, raw: "1.00001", wantErr: core.ErrCurrencyDecimal},
		{name: "fraction rejected for JPY", code: currency.CodeJPY, raw: "1.0", wantErr: core.ErrCurrencyDecimal},
		{name: "second decimal point rejected", code: currency.CodeCAD, raw: "1.0.0", wantErr: core.ErrCurrencyDecimal},
		{name: "non-ASCII digit rejected", code: currency.CodeCAD, raw: "١.٠٠", wantErr: core.ErrCurrencyDecimal},
		{name: "leading whitespace rejected", code: currency.CodeCAD, raw: " 1.00", wantErr: core.ErrCurrencyDecimal},
		{name: "trailing whitespace rejected", code: currency.CodeCAD, raw: "1.00 ", wantErr: core.ErrCurrencyDecimal},
		{name: "embedded whitespace rejected", code: currency.CodeCAD, raw: "1 .00", wantErr: core.ErrCurrencyDecimal},
		{name: "newline rejected", code: currency.CodeCAD, raw: "1.00\n", wantErr: core.ErrCurrencyDecimal},
		{name: "exponent notation rejected", code: currency.CodeCAD, raw: "1e2", wantErr: core.ErrCurrencyDecimal},
		{name: "underscore separator rejected", code: currency.CodeCAD, raw: "1_000.00", wantErr: core.ErrCurrencyDecimal},
		{name: "comma separator rejected", code: currency.CodeCAD, raw: "1,000.00", wantErr: core.ErrCurrencyDecimal},
		{name: "embedded NUL rejected", code: currency.CodeCAD, raw: "1\x00.00", wantErr: core.ErrCurrencyDecimal},
		{name: "one byte above maximum rejected", code: currency.CodeCAD, raw: strings.Repeat("0", currency.DecimalMaximumBytes+1), wantErr: core.ErrCurrencyDecimal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := currency.Parse(tc.code, tc.raw)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) ||
					!errors.Is(gotErr, core.ErrCurrencyContract) ||
					!errors.Is(gotErr, core.ErrPrimitiveContract) {
					t.Fatalf(
						"currency.Parse(%v, %q) error = %v, want %v/%v/%v",
						tc.code,
						tc.raw,
						gotErr,
						tc.wantErr,
						core.ErrCurrencyContract,
						core.ErrPrimitiveContract,
					)
				}
				if got != (currency.Amount{}) {
					t.Fatalf("currency.Parse(%v, %q) = %v, want zero amount", tc.code, tc.raw, got)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("currency.Parse(%v, %q) error = %v, want nil", tc.code, tc.raw, gotErr)
			}
			gotMinor, gotMinorErr := got.MinorUnits()
			if gotMinorErr != nil || gotMinor != tc.wantMinor {
				t.Fatalf("Amount.MinorUnits() = (%d, %v), want (%d, nil)", gotMinor, gotMinorErr, tc.wantMinor)
			}
			gotDecimal, gotDecimalErr := got.Decimal()
			if gotDecimalErr != nil || gotDecimal != tc.want {
				t.Fatalf("Amount.Decimal() = (%q, %v), want (%q, nil)", gotDecimal, gotDecimalErr, tc.want)
			}
		})
	}
}

// TestDecimalInt64DomainOverflowCarriesTheNumericOverflowCallerDecision
// ratchets one caller question against one answer. Add and Subtract already
// report core.ErrNumericOverflow when a result leaves the int64 minor-unit
// domain, so a decimal that leaves the same domain must report it too. Two
// identities for one question forces callers to string-match or to ask twice,
// and the accumulator, the positive bound, and the negative bound are three
// separate places that can drift apart.
func TestDecimalInt64DomainOverflowCarriesTheNumericOverflowCallerDecision(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		raw     string
		want    int64
		code    currency.Code
	}{
		{name: "two-digit exponent exact maximum", code: currency.CodeUSD, raw: "92233720368547758.07", want: math.MaxInt64},
		{name: "two-digit exponent one above maximum", code: currency.CodeUSD, raw: "92233720368547758.08", wantErr: core.ErrNumericOverflow},
		{name: "two-digit exponent exact minimum", code: currency.CodeUSD, raw: "-92233720368547758.08", want: math.MinInt64},
		{name: "two-digit exponent one below minimum", code: currency.CodeUSD, raw: "-92233720368547758.09", wantErr: core.ErrNumericOverflow},
		{name: "zero exponent exact maximum", code: currency.CodeJPY, raw: "9223372036854775807", want: math.MaxInt64},
		{name: "zero exponent one above maximum", code: currency.CodeJPY, raw: "9223372036854775808", wantErr: core.ErrNumericOverflow},
		{name: "zero exponent exact minimum", code: currency.CodeJPY, raw: "-9223372036854775808", want: math.MinInt64},
		{name: "zero exponent one below minimum", code: currency.CodeJPY, raw: "-9223372036854775809", wantErr: core.ErrNumericOverflow},
		{name: "four-digit exponent exact maximum", code: currency.CodeCLF, raw: "922337203685477.5807", want: math.MaxInt64},
		{name: "four-digit exponent one above maximum", code: currency.CodeCLF, raw: "922337203685477.5808", wantErr: core.ErrNumericOverflow},
		{name: "four-digit exponent exact minimum", code: currency.CodeCLF, raw: "-922337203685477.5808", want: math.MinInt64},
		{name: "four-digit exponent one below minimum", code: currency.CodeCLF, raw: "-922337203685477.5809", wantErr: core.ErrNumericOverflow},
		{name: "magnitude beyond unsigned accumulation", code: currency.CodeJPY, raw: "99999999999999999999", wantErr: core.ErrNumericOverflow},
		{name: "negative magnitude beyond unsigned accumulation", code: currency.CodeJPY, raw: "-99999999999999999999", wantErr: core.ErrNumericOverflow},
		{name: "exponent padding pushes magnitude beyond unsigned accumulation", code: currency.CodeCLF, raw: "9999999999999999", wantErr: core.ErrNumericOverflow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := currency.Parse(tc.code, tc.raw)
			if tc.wantErr == nil {
				gotMinor, gotMinorErr := got.MinorUnits()
				if gotErr != nil || gotMinorErr != nil || gotMinor != tc.want {
					t.Fatalf(
						"currency.Parse(%v, %q) minor units = (%d, %v/%v), want (%d, nil)",
						tc.code, tc.raw, gotMinor, gotErr, gotMinorErr, tc.want,
					)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) ||
				!errors.Is(gotErr, core.ErrCurrencyOverflow) ||
				!errors.Is(gotErr, core.ErrCurrencyContract) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf(
					"currency.Parse(%v, %q) error = %v, want %v/%v/%v/%v",
					tc.code, tc.raw, gotErr, tc.wantErr, core.ErrCurrencyOverflow,
					core.ErrCurrencyContract, core.ErrPrimitiveContract,
				)
			}
			if got != (currency.Amount{}) {
				t.Fatalf("currency.Parse(%v, %q) = %v, want zero amount", tc.code, tc.raw, got)
			}
		})
	}
}

// TestDecimalRejectionsOutsideTheInt64DomainStayDistinctFromOverflow keeps the
// negative-zero rejection from being absorbed into the overflow identity once
// the two paths are separated.
func TestDecimalRejectionsOutsideTheInt64DomainStayDistinctFromOverflow(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"-0", "-0.00", "-0.0000"} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			_, gotErr := currency.Parse(currency.CodeCLF, raw)
			if !errors.Is(gotErr, core.ErrCurrencyDecimal) ||
				errors.Is(gotErr, core.ErrNumericOverflow) {
				t.Fatalf(
					"currency.Parse(CLF, %q) error = %v, want %v and not %v",
					raw, gotErr, core.ErrCurrencyDecimal, core.ErrNumericOverflow,
				)
			}
		})
	}
}

func TestAmountJSONUsesClosedExactProjection(t *testing.T) {
	t.Parallel()

	value := mustAmount(t, currency.CodeCAD, math.MinInt64)
	wire, gotMarshalErr := json.Marshal(value)
	if gotMarshalErr != nil {
		t.Fatalf("json.Marshal(Amount) error = %v, want nil", gotMarshalErr)
	}
	direct, gotDirectErr := value.MarshalJSON()
	if gotDirectErr != nil {
		t.Fatalf("Amount.MarshalJSON() error = %v, want nil", gotDirectErr)
	}
	if string(wire) != string(direct) {
		t.Fatalf("json.Marshal(Amount) = %q, want direct fixed point %q", wire, direct)
	}

	wantWire := fmt.Appendf(nil,
		`{"%s":%q,"%s":%q}`,
		currency.JSONFieldCurrency,
		currency.CodeTokenCAD,
		currency.JSONFieldMinorUnits,
		"-9223372036854775808",
	)
	if string(wire) != string(wantWire) {
		t.Fatalf("json.Marshal(Amount) = %s, want %s", wire, wantWire)
	}
	if len(wire) != currency.AmountCanonicalJSONMaximumBytes {
		t.Fatalf(
			"maximum canonical Amount JSON bytes = %d, want %d",
			len(wire),
			currency.AmountCanonicalJSONMaximumBytes,
		)
	}

	var got currency.Amount
	gotUnmarshalErr := json.Unmarshal(wire, &got)
	if gotUnmarshalErr != nil || got != value {
		t.Fatalf("json.Unmarshal(Amount) = (%v, %v), want (%v, nil)", got, gotUnmarshalErr, value)
	}
	whitespaceCases := []struct {
		name  string
		count int
	}{
		{name: "one byte below document slack", count: currency.AmountJSONDocumentSlackBytes - 1},
		{name: "exact document slack", count: currency.AmountJSONDocumentSlackBytes},
	}
	for _, tc := range whitespaceCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			document := append(append([]byte(nil), wire...), []byte(strings.Repeat(" ", tc.count))...)
			var gotWhitespace currency.Amount
			gotWhitespaceErr := json.Unmarshal(document, &gotWhitespace)
			if gotWhitespaceErr != nil || gotWhitespace != value {
				t.Fatalf(
					"json.Unmarshal(Amount with %d whitespace bytes) = (%v, %v), want (%v, nil)",
					tc.count,
					gotWhitespace,
					gotWhitespaceErr,
					value,
				)
			}
		})
	}

	indented, gotIndentErr := json.MarshalIndent(struct {
		Amount currency.Amount `json:"amount"`
	}{Amount: value}, "", "  ")
	if gotIndentErr != nil {
		t.Fatalf("json.MarshalIndent(Amount carrier) error = %v, want nil", gotIndentErr)
	}
	var gotCarrier struct {
		Amount currency.Amount `json:"amount"`
	}
	gotCarrierErr := json.Unmarshal(indented, &gotCarrier)
	if gotCarrierErr != nil || gotCarrier.Amount != value {
		t.Fatalf("json.Unmarshal(indented Amount carrier) = (%v, %v), want (%v, nil)", gotCarrier.Amount, gotCarrierErr, value)
	}
}

func TestAmountJSONValidSemanticMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		data      []byte
		wantCode  currency.Code
		wantMinor int64
	}{
		{name: "USD zero", data: amountJSONFixture(currency.CodeTokenUSD, "0"), wantCode: currency.CodeUSD},
		{name: "EUR positive one", data: amountJSONFixture(currency.CodeTokenEUR, "1"), wantCode: currency.CodeEUR, wantMinor: 1},
		{name: "GBP negative one", data: amountJSONFixture(currency.CodeTokenGBP, "-1"), wantCode: currency.CodeGBP, wantMinor: -1},
		{name: "CAD maximum", data: amountJSONFixture(currency.CodeTokenCAD, "9223372036854775807"), wantCode: currency.CodeCAD, wantMinor: math.MaxInt64},
		{name: "AUD minimum", data: amountJSONFixture(currency.CodeTokenAUD, "-9223372036854775808"), wantCode: currency.CodeAUD, wantMinor: math.MinInt64},
		{name: "JPY maximum", data: amountJSONFixture(currency.CodeTokenJPY, "9223372036854775807"), wantCode: currency.CodeJPY, wantMinor: math.MaxInt64},
		{name: "CHF ordinary positive", data: amountJSONFixture(currency.CodeTokenCHF, "125"), wantCode: currency.CodeCHF, wantMinor: 125},
		{name: "NZD ordinary negative", data: amountJSONFixture(currency.CodeTokenNZD, "-125"), wantCode: currency.CodeNZD, wantMinor: -125},
		{name: "SGD one below maximum", data: amountJSONFixture(currency.CodeTokenSGD, "9223372036854775806"), wantCode: currency.CodeSGD, wantMinor: math.MaxInt64 - 1},
		{name: "HKD one above minimum", data: amountJSONFixture(currency.CodeTokenHKD, "-9223372036854775807"), wantCode: currency.CodeHKD, wantMinor: math.MinInt64 + 1},
		{name: "BHD arbitrary signed extent", data: amountJSONFixture(currency.CodeTokenBHD, "-123456789"), wantCode: currency.CodeBHD, wantMinor: -123456789},
		{name: "CLF arbitrary signed extent", data: amountJSONFixture(currency.CodeTokenCLF, "123456789"), wantCode: currency.CodeCLF, wantMinor: 123456789},
		{
			name: "field order is semantically irrelevant",
			data: fmt.Appendf(nil,
				`{"%s":%q,"%s":%q}`,
				currency.JSONFieldMinorUnits,
				"73",
				currency.JSONFieldCurrency,
				currency.CodeTokenCAD,
			),
			wantCode:  currency.CodeCAD,
			wantMinor: 73,
		},
		{
			name: "equivalent JSON string escapes normalize",
			data: fmt.Appendf(nil,
				`{"%s":"C\u0041D","%s":"\u0037\u0033"}`,
				currency.JSONFieldCurrency,
				currency.JSONFieldMinorUnits,
			),
			wantCode:  currency.CodeCAD,
			wantMinor: 73,
		},
		{
			name: "leading trailing and structural whitespace normalize",
			data: fmt.Appendf(nil,
				" \n{\t%q : %q,\r%q : %q }\n",
				currency.JSONFieldCurrency,
				currency.CodeTokenCAD,
				currency.JSONFieldMinorUnits,
				"73",
			),
			wantCode:  currency.CodeCAD,
			wantMinor: 73,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got currency.Amount
			gotErr := got.UnmarshalJSON(tc.data)
			if gotErr != nil {
				t.Fatalf("Amount.UnmarshalJSON(%q) error = %v, want nil", tc.data, gotErr)
			}
			gotCode, gotCodeErr := got.Code()
			gotMinor, gotMinorErr := got.MinorUnits()
			if gotCodeErr != nil || gotMinorErr != nil ||
				gotCode != tc.wantCode || gotMinor != tc.wantMinor {
				t.Fatalf(
					"decoded Amount projection = (code:%v/%v minor:%d/%v), want (%v, %d)",
					gotCode,
					gotCodeErr,
					gotMinor,
					gotMinorErr,
					tc.wantCode,
					tc.wantMinor,
				)
			}
			canonical, gotMarshalErr := json.Marshal(got)
			if gotMarshalErr != nil || !json.Valid(canonical) ||
				len(canonical) > currency.AmountCanonicalJSONMaximumBytes {
				t.Fatalf(
					"json.Marshal(decoded Amount) = (%q, %v), want valid JSON within %d bytes",
					canonical,
					gotMarshalErr,
					currency.AmountCanonicalJSONMaximumBytes,
				)
			}
			var gotRoundTrip currency.Amount
			gotRoundTripErr := json.Unmarshal(canonical, &gotRoundTrip)
			if gotRoundTripErr != nil || gotRoundTrip != got {
				t.Fatalf(
					"json.Unmarshal(canonical Amount) = (%v, %v), want (%v, nil)",
					gotRoundTrip,
					gotRoundTripErr,
					got,
				)
			}
		})
	}
}

func TestAmountJSONHostileMatrixPreservesReceiver(t *testing.T) {
	t.Parallel()

	before := mustAmount(t, currency.CodeCAD, 125)
	canonical, gotCanonicalErr := json.Marshal(before)
	if gotCanonicalErr != nil {
		t.Fatalf("json.Marshal(Amount) error = %v, want nil", gotCanonicalErr)
	}
	maximumCanonical, gotMaximumCanonicalErr := json.Marshal(
		mustAmount(t, currency.CodeCAD, math.MinInt64),
	)
	if gotMaximumCanonicalErr != nil {
		t.Fatalf("json.Marshal(maximum Amount) error = %v, want nil", gotMaximumCanonicalErr)
	}
	cases := []struct {
		wantErr error
		name    string
		data    []byte
	}{
		{name: "empty rejected", wantErr: core.ErrJSONContract},
		{name: "null rejected", data: []byte("null"), wantErr: core.ErrJSONContract},
		{name: "boolean rejected", data: []byte("true"), wantErr: core.ErrJSONContract},
		{name: "number rejected", data: []byte("1"), wantErr: core.ErrJSONContract},
		{name: "string rejected", data: []byte(`"amount"`), wantErr: core.ErrJSONContract},
		{name: "array rejected", data: []byte("[]"), wantErr: core.ErrJSONContract},
		{name: "empty object rejected", data: []byte("{}"), wantErr: core.ErrCurrencyContract},
		{name: "opening object is truncated", data: []byte("{"), wantErr: core.ErrJSONContract},
		{name: "currency string is truncated", data: fmt.Appendf(nil,
			`{"%s":"CAD`,
			currency.JSONFieldCurrency,
		), wantErr: core.ErrJSONContract},
		{name: "minor unit string is truncated", data: fmt.Appendf(nil,
			`{"%s":%q,"%s":"125`,
			currency.JSONFieldCurrency,
			currency.CodeTokenCAD,
			currency.JSONFieldMinorUnits,
		), wantErr: core.ErrJSONContract},
		{name: "unknown field rejected", data: fmt.Appendf(nil,
			`{"%s":%q,"%s":%q,"unknown":true}`,
			currency.JSONFieldCurrency,
			currency.CodeTokenCAD,
			currency.JSONFieldMinorUnits,
			"125",
		), wantErr: core.ErrJSONContract},
		{name: "duplicate field rejected", data: fmt.Appendf(nil,
			`{"%s":%q,"%s":%q,"%s":%q}`,
			currency.JSONFieldCurrency,
			currency.CodeTokenCAD,
			currency.JSONFieldCurrency,
			currency.CodeTokenUSD,
			currency.JSONFieldMinorUnits,
			"125",
		), wantErr: core.ErrJSONContract},
		{name: "duplicate minor units rejected", data: fmt.Appendf(nil,
			`{"%s":%q,"%s":%q,"%s":%q}`,
			currency.JSONFieldCurrency,
			currency.CodeTokenCAD,
			currency.JSONFieldMinorUnits,
			"125",
			currency.JSONFieldMinorUnits,
			"126",
		), wantErr: core.ErrJSONContract},
		{name: "case variant duplicate rejected", data: fmt.Appendf(nil,
			`{"%s":%q,"Currency":%q,"%s":%q}`,
			currency.JSONFieldCurrency,
			currency.CodeTokenCAD,
			currency.CodeTokenUSD,
			currency.JSONFieldMinorUnits,
			"125",
		), wantErr: core.ErrJSONContract},
		{name: "case variant field rejected without canonical spelling", data: fmt.Appendf(nil,
			`{"%s":%q,"%s":%q}`,
			strings.ToUpper(currency.JSONFieldCurrency),
			currency.CodeTokenCAD,
			currency.JSONFieldMinorUnits,
			"125",
		), wantErr: core.ErrJSONContract},
		{name: "missing currency rejected", data: fmt.Appendf(nil,
			`{"%s":%q}`,
			currency.JSONFieldMinorUnits,
			"125",
		), wantErr: core.ErrCurrencyContract},
		{name: "missing minor units rejected", data: fmt.Appendf(nil,
			`{"%s":%q}`,
			currency.JSONFieldCurrency,
			currency.CodeTokenCAD,
		), wantErr: core.ErrCurrencyDecimal},
		{name: "currency number rejected", data: fmt.Appendf(nil,
			`{"%s":1,"%s":%q}`,
			currency.JSONFieldCurrency,
			currency.JSONFieldMinorUnits,
			"125",
		), wantErr: core.ErrCurrencyContract},
		{name: "numeric minor units rejected", data: fmt.Appendf(nil,
			`{"%s":%q,"%s":125}`,
			currency.JSONFieldCurrency,
			currency.CodeTokenCAD,
			currency.JSONFieldMinorUnits,
		), wantErr: core.ErrCurrencyDecimal},
		{name: "null currency rejected", data: fmt.Appendf(nil,
			`{"%s":null,"%s":%q}`,
			currency.JSONFieldCurrency,
			currency.JSONFieldMinorUnits,
			"125",
		), wantErr: core.ErrCurrencyContract},
		{name: "null minor units rejected", data: fmt.Appendf(nil,
			`{"%s":%q,"%s":null}`,
			currency.JSONFieldCurrency,
			currency.CodeTokenCAD,
			currency.JSONFieldMinorUnits,
		), wantErr: core.ErrCurrencyDecimal},
		{name: "lowercase currency rejected", data: amountJSONFixture("cad", "125"), wantErr: core.ErrCurrencyContract},
		{name: "unknown currency rejected", data: amountJSONFixture("ZZZ", "125"), wantErr: core.ErrCurrencyContract},
		{name: "noncanonical integer rejected", data: fmt.Appendf(nil,
			`{"%s":%q,"%s":%q}`,
			currency.JSONFieldCurrency,
			currency.CodeTokenCAD,
			currency.JSONFieldMinorUnits,
			"0125",
		), wantErr: core.ErrCurrencyDecimal},
		{name: "explicit plus integer rejected", data: amountJSONFixture(currency.CodeTokenCAD, "+125"), wantErr: core.ErrCurrencyDecimal},
		{name: "negative zero integer rejected", data: amountJSONFixture(currency.CodeTokenCAD, "-0"), wantErr: core.ErrCurrencyDecimal},
		{name: "positive overflow rejected", data: amountJSONFixture(currency.CodeTokenCAD, "9223372036854775808"), wantErr: core.ErrCurrencyDecimal},
		{name: "negative overflow rejected", data: amountJSONFixture(currency.CodeTokenCAD, "-9223372036854775809"), wantErr: core.ErrCurrencyDecimal},
		{name: "trailing comma rejected", data: fmt.Appendf(nil,
			`{"%s":%q,"%s":%q,}`,
			currency.JSONFieldCurrency,
			currency.CodeTokenCAD,
			currency.JSONFieldMinorUnits,
			"125",
		), wantErr: core.ErrJSONContract},
		{name: "nested currency object rejected", data: fmt.Appendf(nil,
			`{"%s":{},"%s":%q}`,
			currency.JSONFieldCurrency,
			currency.JSONFieldMinorUnits,
			"125",
		), wantErr: core.ErrJSONContract},
		{name: "unpaired surrogate rejected", data: fmt.Appendf(nil,
			`{"%s":"\uD800","%s":%q}`,
			currency.JSONFieldCurrency,
			currency.JSONFieldMinorUnits,
			"125",
		), wantErr: core.ErrJSONContract},
		{name: "invalid UTF-8 rejected", data: []byte{'{', '"', 0xff, '"', '}'}, wantErr: core.ErrJSONContract},
		{name: "trailing document rejected", data: append(append([]byte(nil), canonical...), canonical...), wantErr: core.ErrJSONContract},
		{name: "one byte above document bound rejected", data: append(append([]byte(nil), maximumCanonical...), []byte(strings.Repeat(" ", currency.AmountJSONDocumentSlackBytes+1))...), wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := before
			gotErr := got.UnmarshalJSON(tc.data)
			if !errors.Is(gotErr, tc.wantErr) ||
				!errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrCurrencyContract) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf(
					"Amount.UnmarshalJSON(%q) error = %v, want %v/%v/%v/%v",
					tc.data,
					gotErr,
					tc.wantErr,
					core.ErrJSONContract,
					core.ErrCurrencyContract,
					core.ErrPrimitiveContract,
				)
			}
			if got != before {
				t.Fatalf("rejected JSON mutated receiver: got %v, want %v", got, before)
			}
		})
	}

	var nilAmount *currency.Amount
	if gotErr := nilAmount.UnmarshalJSON(canonical); !errors.Is(gotErr, core.ErrJSONContract) {
		t.Fatalf("nil Amount.UnmarshalJSON() error = %v, want %v", gotErr, core.ErrJSONContract)
	}
}

func amountJSONFixture(code, minorUnits string) []byte {
	return fmt.Appendf(nil,
		`{"%s":%q,"%s":%q}`,
		currency.JSONFieldCurrency,
		code,
		currency.JSONFieldMinorUnits,
		minorUnits,
	)
}
