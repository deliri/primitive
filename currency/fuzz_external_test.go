package currency_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/currency"
)

func FuzzDecimalParserAgainstStandardGrammarAndBigRationalOracle(f *testing.F) {
	grammar := decimalOracleGrammar()
	seeds := []struct {
		raw      string
		selector uint8
	}{
		{selector: uint8(currency.CodeJPY - currency.CodeUSD), raw: "0"},
		{selector: uint8(currency.CodeJPY - currency.CodeUSD), raw: "-9223372036854775808"},
		{selector: uint8(currency.CodeJPY - currency.CodeUSD), raw: "9223372036854775807"},
		{selector: uint8(currency.CodeCAD - currency.CodeUSD), raw: "00012.3"},
		{selector: uint8(currency.CodeCAD - currency.CodeUSD), raw: "-0.01"},
		{selector: uint8(currency.CodeCAD - currency.CodeUSD), raw: "92233720368547758.07"},
		{selector: uint8(currency.CodeCAD - currency.CodeUSD), raw: "-92233720368547758.08"},
		{selector: uint8(currency.CodeBHD - currency.CodeUSD), raw: "12.345"},
		{selector: uint8(currency.CodeCLF - currency.CodeUSD), raw: "922337203685477.5807"},
		{selector: uint8(currency.CodeCLF - currency.CodeUSD), raw: "-922337203685477.5808"},
		{selector: uint8(currency.CodeCAD - currency.CodeUSD), raw: "-0.00"},
		{selector: uint8(currency.CodeCAD - currency.CodeUSD), raw: "1.001"},
		{selector: uint8(currency.CodeJPY - currency.CodeUSD), raw: "1.0"},
		{selector: uint8(currency.CodeCLF - currency.CodeUSD), raw: "1.00001"},
		{selector: uint8(currency.CodeCAD - currency.CodeUSD), raw: ""},
		{selector: uint8(currency.CodeCAD - currency.CodeUSD), raw: "+"},
		{selector: uint8(currency.CodeCAD - currency.CodeUSD), raw: ".01"},
		{selector: uint8(currency.CodeCAD - currency.CodeUSD), raw: "1."},
		{selector: uint8(currency.CodeCAD - currency.CodeUSD), raw: "1e2"},
		{selector: uint8(currency.CodeCAD - currency.CodeUSD), raw: " 1.00"},
		{selector: uint8(currency.CodeCAD - currency.CodeUSD), raw: "92233720368547758.08"},
		{selector: uint8(currency.CodeCAD - currency.CodeUSD), raw: "-92233720368547758.09"},
	}
	for _, seed := range seeds {
		f.Add(seed.selector, seed.raw)
	}
	f.Fuzz(func(t *testing.T, selector uint8, raw string) {
		code := currency.Code(
			uint8(currency.CodeUSD) +
				selector%uint8(currency.CodeCLF-currency.CodeUSD+1),
		)
		wantMinor, wantAccept := oracleDecimal(grammar, code, raw)
		got, gotErr := currency.Parse(code, raw)
		proveFuzzDecimalResult(
			t,
			code,
			raw,
			got,
			gotErr,
			wantMinor,
			wantAccept,
		)
	})
}

func FuzzAmountJSONAgainstStandardTokenStreamOracle(f *testing.F) {
	exactMaximum := append(
		bytes.Repeat([]byte{' '}, currency.AmountJSONDocumentSlackBytes),
		[]byte(`{"currency":"BHD","minor_units":"-9223372036854775808"}`)...,
	)
	seeds := [][]byte{
		[]byte(`{"currency":"USD","minor_units":"0"}`),
		[]byte(`{"minor_units":"-1","currency":"CAD"}`),
		[]byte(" \n\t{\"currency\":\"JPY\",\"minor_units\":\"9223372036854775807\"}\r "),
		[]byte(`{"currency":"BHD","minor_units":"-9223372036854775808"}`),
		[]byte(`{"currency":"CLF","minor_units":"9223372036854775807"}`),
		[]byte(`{"currency":"usd","minor_units":"1"}`),
		[]byte(`{"currency":"USD","minor_units":"\u0031"}`),
		[]byte(`{"\u0063urrency":"USD","minor_units":"1"}`),
		[]byte(`{"currency":"USD","minor_units":"-0"}`),
		[]byte(`{"currency":"USD","minor_units":"01"}`),
		[]byte(`{"currency":"USD","minor_units":"9223372036854775808"}`),
		[]byte(`{"currency":"USD","minor_units":"-9223372036854775809"}`),
		[]byte(`{"currency":"USD","minor_units":"1","extra":true}`),
		[]byte(`{"currency":"USD","currency":"CAD","minor_units":"1"}`),
		[]byte(`{"currency":"USD","Currency":"CAD","minor_units":"1"}`),
		[]byte(`{"CURRENCY":"USD","minor_units":"1"}`),
		[]byte(`{"currency":"USD"}`),
		[]byte(`{"minor_units":"1"}`),
		[]byte(`{"currency":1,"minor_units":"1"}`),
		[]byte(`{"currency":"USD","minor_units":1}`),
		[]byte(`{"currency":"USD","minor_units":"1",}`),
		[]byte(`{"currency":"USD","minor_units":"1"`),
		[]byte(`null`),
		[]byte(`[]`),
		[]byte(`{}`),
		{0xff},
		exactMaximum,
		append(bytes.Clone(exactMaximum), ' '),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		wantCode, wantMinor, wantAccept := oracleAmountJSON(data)
		before := mustAmount(t, currency.CodeCLF, math.MinInt64)
		got := before
		gotErr := got.UnmarshalJSON(data)
		proveFuzzAmountJSONResult(
			t,
			data,
			before,
			got,
			gotErr,
			wantCode,
			wantMinor,
			wantAccept,
		)
	})
}

type decimalOraclePatterns struct {
	zero  *regexp.Regexp
	two   *regexp.Regexp
	three *regexp.Regexp
	four  *regexp.Regexp
}

func decimalOracleGrammar() decimalOraclePatterns {
	return decimalOraclePatterns{
		zero:  regexp.MustCompile(`^-?[0-9]+$`),
		two:   regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]{1,2})?$`),
		three: regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]{1,3})?$`),
		four:  regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]{1,4})?$`),
	}
}

func oracleDecimal(
	grammar decimalOraclePatterns,
	code currency.Code,
	raw string,
) (int64, bool) {
	exponent, admitted := oracleFractionDigits(code)
	pattern := grammar.forExponent(exponent)
	if !admitted || pattern == nil || raw == "" ||
		len(raw) > currency.DecimalMaximumBytes || !pattern.MatchString(raw) {
		return 0, false
	}
	value, ok := new(big.Rat).SetString(raw)
	if !ok || strings.HasPrefix(raw, "-") && value.Sign() == 0 {
		return 0, false
	}
	scale := new(big.Int).Exp(
		big.NewInt(10),
		big.NewInt(int64(exponent)),
		nil,
	)
	value.Mul(value, new(big.Rat).SetInt(scale))
	if !value.IsInt() || !value.Num().IsInt64() {
		return 0, false
	}
	return value.Num().Int64(), true
}

func (p decimalOraclePatterns) forExponent(exponent uint8) *regexp.Regexp {
	switch exponent {
	case currency.MinorUnitDigitsZero:
		return p.zero
	case currency.MinorUnitDigitsTwo:
		return p.two
	case currency.MinorUnitDigitsThree:
		return p.three
	case currency.MinorUnitDigitsFour:
		return p.four
	default:
		return nil
	}
}

func oracleFractionDigits(code currency.Code) (uint8, bool) {
	switch code {
	case currency.CodeJPY:
		return currency.MinorUnitDigitsZero, true
	case currency.CodeUSD, currency.CodeEUR, currency.CodeGBP,
		currency.CodeCAD, currency.CodeAUD, currency.CodeCHF,
		currency.CodeNZD, currency.CodeSGD, currency.CodeHKD:
		return currency.MinorUnitDigitsTwo, true
	case currency.CodeBHD:
		return currency.MinorUnitDigitsThree, true
	case currency.CodeCLF:
		return currency.MinorUnitDigitsFour, true
	default:
		return 0, false
	}
}

type amountJSONOracleState struct {
	minor    int64
	code     currency.Code
	codeSet  bool
	minorSet bool
}

func oracleAmountJSON(data []byte) (currency.Code, int64, bool) {
	if len(data) == 0 || len(data) > currency.AmountJSONMaximumBytes ||
		!utf8.Valid(data) {
		return currency.CodeUnknown, 0, false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	open, err := decoder.Token()
	if err != nil || open != json.Delim('{') {
		return currency.CodeUnknown, 0, false
	}
	state, ok := oracleAmountJSONFields(decoder)
	if !ok || !state.codeSet || !state.minorSet {
		return currency.CodeUnknown, 0, false
	}
	closeObject, err := decoder.Token()
	if err != nil || closeObject != json.Delim('}') {
		return currency.CodeUnknown, 0, false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return currency.CodeUnknown, 0, false
	}
	return state.code, state.minor, true
}

func oracleAmountJSONFields(
	decoder *json.Decoder,
) (amountJSONOracleState, bool) {
	var state amountJSONOracleState
	for decoder.More() {
		field, err := decoder.Token()
		if err != nil {
			return amountJSONOracleState{}, false
		}
		fieldName, ok := field.(string)
		if !ok {
			return amountJSONOracleState{}, false
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return amountJSONOracleState{}, false
		}
		state, ok = oracleAmountJSONField(state, fieldName, raw)
		if !ok {
			return amountJSONOracleState{}, false
		}
	}
	return state, true
}

func oracleAmountJSONField(
	state amountJSONOracleState,
	fieldName string,
	raw json.RawMessage,
) (amountJSONOracleState, bool) {
	var token string
	if err := json.Unmarshal(raw, &token); err != nil {
		return amountJSONOracleState{}, false
	}
	switch fieldName {
	case currency.JSONFieldCurrency:
		if state.codeSet {
			return amountJSONOracleState{}, false
		}
		state.code, state.codeSet = oracleCode(token)
		return state, state.codeSet
	case currency.JSONFieldMinorUnits:
		if state.minorSet {
			return amountJSONOracleState{}, false
		}
		value, err := strconv.ParseInt(token, 10, 64)
		if err != nil || strconv.FormatInt(value, 10) != token {
			return amountJSONOracleState{}, false
		}
		state.minor, state.minorSet = value, true
		return state, true
	default:
		return amountJSONOracleState{}, false
	}
}

func proveFuzzDecimalResult(
	t *testing.T,
	code currency.Code,
	raw string,
	got currency.Amount,
	gotErr error,
	wantMinor int64,
	wantAccept bool,
) {
	t.Helper()

	if !wantAccept {
		proveFuzzDecimalRejection(t, code, raw, got, gotErr)
		return
	}
	proveFuzzDecimalAcceptance(t, code, raw, got, gotErr, wantMinor)
}

func proveFuzzDecimalRejection(
	t *testing.T,
	code currency.Code,
	raw string,
	got currency.Amount,
	gotErr error,
) {
	t.Helper()

	if got != (currency.Amount{}) ||
		!errors.Is(gotErr, core.ErrCurrencyContract) ||
		!errors.Is(gotErr, core.ErrPrimitiveContract) {
		t.Fatalf(
			"currency.Parse(%v, %q) = (%v, %v), want zero and %v/%v",
			code,
			raw,
			got,
			gotErr,
			core.ErrCurrencyContract,
			core.ErrPrimitiveContract,
		)
	}
}

func proveFuzzDecimalAcceptance(
	t *testing.T,
	code currency.Code,
	raw string,
	got currency.Amount,
	gotErr error,
	wantMinor int64,
) {
	t.Helper()

	if gotErr != nil {
		t.Fatalf("currency.Parse(%v, %q) error = %v, want nil", code, raw, gotErr)
	}
	gotCode, gotCodeErr := got.Code()
	gotMinor, gotMinorErr := got.MinorUnits()
	if gotCodeErr != nil || gotMinorErr != nil ||
		gotCode != code || gotMinor != wantMinor {
		t.Fatalf(
			"parsed Amount projection = (code:%v/%v minor:%d/%v), want (%v, %d)",
			gotCode,
			gotCodeErr,
			gotMinor,
			gotMinorErr,
			code,
			wantMinor,
		)
	}
	canonical, gotCanonicalErr := got.Decimal()
	if gotCanonicalErr != nil {
		t.Fatalf("Amount.Decimal() error = %v, want nil", gotCanonicalErr)
	}
	gotRoundTrip, gotRoundTripErr := currency.Parse(code, canonical)
	if gotRoundTripErr != nil || gotRoundTrip != got {
		t.Fatalf(
			"currency.Parse(%v, Amount.Decimal()) = (%v, %v), want (%v, nil)",
			code,
			gotRoundTrip,
			gotRoundTripErr,
			got,
		)
	}
}

func proveFuzzAmountJSONResult(
	t *testing.T,
	data []byte,
	before currency.Amount,
	got currency.Amount,
	gotErr error,
	wantCode currency.Code,
	wantMinor int64,
	wantAccept bool,
) {
	t.Helper()

	if !wantAccept {
		proveFuzzAmountJSONRejection(t, data, before, got, gotErr)
		return
	}
	proveFuzzAmountJSONAcceptance(
		t,
		data,
		got,
		gotErr,
		wantCode,
		wantMinor,
	)
}

func proveFuzzAmountJSONRejection(
	t *testing.T,
	data []byte,
	before currency.Amount,
	got currency.Amount,
	gotErr error,
) {
	t.Helper()

	if !errors.Is(gotErr, core.ErrJSONContract) ||
		!errors.Is(gotErr, core.ErrCurrencyContract) ||
		!errors.Is(gotErr, core.ErrPrimitiveContract) ||
		got != before {
		t.Fatalf(
			"Amount.UnmarshalJSON(%q) = (%v, %v), want preserved receiver and %v/%v/%v",
			data,
			got,
			gotErr,
			core.ErrJSONContract,
			core.ErrCurrencyContract,
			core.ErrPrimitiveContract,
		)
	}
}

func proveFuzzAmountJSONAcceptance(
	t *testing.T,
	data []byte,
	got currency.Amount,
	gotErr error,
	wantCode currency.Code,
	wantMinor int64,
) {
	t.Helper()

	if gotErr != nil {
		t.Fatalf("Amount.UnmarshalJSON(%q) error = %v, want nil", data, gotErr)
	}
	gotCode, gotCodeErr := got.Code()
	gotMinor, gotMinorErr := got.MinorUnits()
	if gotCodeErr != nil || gotMinorErr != nil ||
		gotCode != wantCode || gotMinor != wantMinor {
		t.Fatalf(
			"decoded Amount projection = (code:%v/%v minor:%d/%v), want (%v, %d)",
			gotCode,
			gotCodeErr,
			gotMinor,
			gotMinorErr,
			wantCode,
			wantMinor,
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
}

func oracleCode(token string) (currency.Code, bool) {
	switch token {
	case currency.CodeTokenUSD:
		return currency.CodeUSD, true
	case currency.CodeTokenEUR:
		return currency.CodeEUR, true
	case currency.CodeTokenGBP:
		return currency.CodeGBP, true
	case currency.CodeTokenCAD:
		return currency.CodeCAD, true
	case currency.CodeTokenAUD:
		return currency.CodeAUD, true
	case currency.CodeTokenJPY:
		return currency.CodeJPY, true
	case currency.CodeTokenCHF:
		return currency.CodeCHF, true
	case currency.CodeTokenNZD:
		return currency.CodeNZD, true
	case currency.CodeTokenSGD:
		return currency.CodeSGD, true
	case currency.CodeTokenHKD:
		return currency.CodeHKD, true
	case currency.CodeTokenBHD:
		return currency.CodeBHD, true
	case currency.CodeTokenCLF:
		return currency.CodeCLF, true
	default:
		return currency.CodeUnknown, false
	}
}
