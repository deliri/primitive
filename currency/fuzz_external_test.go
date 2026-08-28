package currency_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"io"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"encoding/json/jsontext"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/currency"
)

func FuzzDecimalParserAgainstStandardGrammarAndBigRationalOracle(f *testing.F) {
	grammar := decimalOracleGrammar()
	typedSeeds := []struct {
		code       currency.Code
		minorUnits int64
	}{
		{code: currency.CodeJPY},
		{code: currency.CodeJPY, minorUnits: math.MinInt64},
		{code: currency.CodeJPY, minorUnits: math.MaxInt64},
		{code: currency.CodeCAD, minorUnits: -1},
		{code: currency.CodeCAD, minorUnits: math.MaxInt64},
		{code: currency.CodeCAD, minorUnits: math.MinInt64},
		{code: currency.CodeBHD, minorUnits: 12345},
		{code: currency.CodeCLF, minorUnits: math.MaxInt64},
		{code: currency.CodeCLF, minorUnits: math.MinInt64},
	}
	for _, seed := range typedSeeds {
		amount, gotNewErr := currency.New(seed.code, seed.minorUnits)
		if gotNewErr != nil {
			f.Fatalf("currency.New(%v, %d) seed error = %v, want nil", seed.code, seed.minorUnits, gotNewErr)
		}
		if gotValidateErr := amount.Validate(); gotValidateErr != nil {
			f.Fatalf("Amount.Validate(seed) error = %v, want nil", gotValidateErr)
		}
		raw, gotDecimalErr := amount.Decimal()
		if gotDecimalErr != nil {
			f.Fatalf("Amount.Decimal(seed) error = %v, want nil", gotDecimalErr)
		}
		f.Add(uint8(seed.code-currency.CodeUSD), raw)
	}
	hostileSeeds := []struct {
		raw      string
		selector uint8
	}{
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
	for _, seed := range hostileSeeds {
		f.Add(seed.selector, seed.raw)
	}
	f.Fuzz(func(t *testing.T, selector uint8, raw string) {
		code := currency.Code(
			uint8(currency.CodeUSD) +
				selector%uint8(currency.CodeCLF-currency.CodeUSD+1),
		)
		wantMinor, wantErr := oracleDecimal(grammar, code, raw)
		got, gotErr := currency.Parse(code, raw)
		proveFuzzDecimalResult(
			t,
			code,
			raw,
			got,
			gotErr,
			wantMinor,
			wantErr,
		)
	})
}

func FuzzParseCodeAgainstClosedCurrencyDomain(f *testing.F) {
	codes := []currency.Code{
		currency.CodeUSD,
		currency.CodeEUR,
		currency.CodeGBP,
		currency.CodeCAD,
		currency.CodeAUD,
		currency.CodeJPY,
		currency.CodeCHF,
		currency.CodeNZD,
		currency.CodeSGD,
		currency.CodeHKD,
		currency.CodeBHD,
		currency.CodeCLF,
	}
	for _, code := range codes {
		if gotErr := code.Validate(); gotErr != nil {
			f.Fatalf("Code(%d).Validate(seed) error = %v, want nil", code, gotErr)
		}
		f.Add(code.String())
	}
	for _, hostile := range []string{
		"",
		"usd",
		" USD",
		"USD ",
		"US",
		"USDX",
		"U\x00D",
		"U\xffD",
		"ZZZ",
		"\nUSD",
	} {
		f.Add(hostile)
	}
	f.Fuzz(func(t *testing.T, token string) {
		wantCode, wantAccept := oracleCode(token)
		gotCode, gotErr := currency.ParseCode(token)
		if !wantAccept {
			if gotCode != currency.CodeUnknown ||
				!errors.Is(gotErr, core.ErrCurrencyContract) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf(
					"currency.ParseCode(%q) = (%v, %v), want (%v, %v/%v)",
					token,
					gotCode,
					gotErr,
					currency.CodeUnknown,
					core.ErrCurrencyContract,
					core.ErrPrimitiveContract,
				)
			}
			return
		}
		if gotErr != nil || gotCode != wantCode {
			t.Fatalf(
				"currency.ParseCode(%q) = (%v, %v), want (%v, nil)",
				token,
				gotCode,
				gotErr,
				wantCode,
			)
		}
		if gotValidateErr := gotCode.Validate(); gotValidateErr != nil {
			t.Fatalf("accepted Code.Validate() error = %v, want nil", gotValidateErr)
		}
		canonical := gotCode.String()
		gotRoundTrip, gotRoundTripErr := currency.ParseCode(canonical)
		if gotRoundTripErr != nil || gotRoundTrip != gotCode {
			t.Fatalf(
				"currency.ParseCode(Code.String()) = (%v, %v), want (%v, nil)",
				gotRoundTrip,
				gotRoundTripErr,
				gotCode,
			)
		}
		if gotSecond := gotRoundTrip.String(); gotSecond != canonical {
			t.Fatalf("second Code.String() = %q, want %q", gotSecond, canonical)
		}
	})
}

func FuzzCodeJSONAgainstIndependentStringTokenOracle(f *testing.F) {
	codes := []currency.Code{
		currency.CodeUSD,
		currency.CodeEUR,
		currency.CodeGBP,
		currency.CodeCAD,
		currency.CodeAUD,
		currency.CodeJPY,
		currency.CodeCHF,
		currency.CodeNZD,
		currency.CodeSGD,
		currency.CodeHKD,
		currency.CodeBHD,
		currency.CodeCLF,
	}
	for _, code := range codes {
		if gotErr := code.Validate(); gotErr != nil {
			f.Fatalf("Code(%d).Validate(seed) error = %v, want nil", code, gotErr)
		}
		canonical, gotErr := code.MarshalJSON()
		if gotErr != nil {
			f.Fatalf("Code(%d).MarshalJSON(seed) error = %v, want nil", code, gotErr)
		}
		f.Add(canonical)
	}
	canonical, gotCanonicalErr := currency.CodeCAD.MarshalJSON()
	if gotCanonicalErr != nil {
		f.Fatalf("CodeCAD.MarshalJSON(seed) error = %v, want nil", gotCanonicalErr)
	}
	padding := currency.CodeJSONMaximumBytes - len(canonical)
	f.Add(append(bytes.Repeat([]byte{' '}, padding), canonical...))
	f.Add(append(append([]byte(nil), canonical...), bytes.Repeat([]byte{' '}, padding)...))
	f.Add(append(append([]byte(nil), canonical...), bytes.Repeat([]byte{' '}, padding+1)...))
	for _, hostile := range [][]byte{
		nil,
		[]byte("null"),
		[]byte("true"),
		[]byte("1"),
		[]byte("[]"),
		[]byte("{}"),
		[]byte(`"cad"`),
		[]byte(`"ZZZ"`),
		[]byte(`"CAD`),
		[]byte{0xff},
		append(append([]byte(nil), canonical...), canonical...),
	} {
		f.Add(hostile)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		wantCode, wantAccept := oracleCodeJSON(data)

		var gotFresh currency.Code
		gotFreshErr := gotFresh.UnmarshalJSON(data)
		gotPopulated := currency.CodeCLF
		gotPopulatedErr := gotPopulated.UnmarshalJSON(data)

		proveFuzzCodeJSONResult(
			t,
			data,
			gotFresh,
			gotFreshErr,
			gotPopulated,
			gotPopulatedErr,
			wantCode,
			wantAccept,
		)
	})
}

func FuzzAmountJSONAgainstStandardTokenStreamOracle(f *testing.F) {
	typedSeeds := []struct {
		code       currency.Code
		minorUnits int64
	}{
		{code: currency.CodeUSD},
		{code: currency.CodeCAD, minorUnits: -1},
		{code: currency.CodeJPY, minorUnits: math.MaxInt64},
		{code: currency.CodeBHD, minorUnits: math.MinInt64},
		{code: currency.CodeCLF, minorUnits: math.MaxInt64},
	}
	var maximumCanonical []byte
	for _, seed := range typedSeeds {
		amount, gotNewErr := currency.New(seed.code, seed.minorUnits)
		if gotNewErr != nil {
			f.Fatalf("currency.New(%v, %d) seed error = %v, want nil", seed.code, seed.minorUnits, gotNewErr)
		}
		if gotValidateErr := amount.Validate(); gotValidateErr != nil {
			f.Fatalf("Amount.Validate(seed) error = %v, want nil", gotValidateErr)
		}
		canonical, gotMarshalErr := amount.MarshalJSON()
		if gotMarshalErr != nil {
			f.Fatalf("Amount.MarshalJSON(seed) error = %v, want nil", gotMarshalErr)
		}
		f.Add(canonical)
		if seed.code == currency.CodeBHD && seed.minorUnits == math.MinInt64 {
			maximumCanonical = canonical
		}
	}
	if len(maximumCanonical) != currency.AmountCanonicalJSONMaximumBytes {
		f.Fatalf(
			"maximum canonical Amount seed bytes = %d, want %d",
			len(maximumCanonical),
			currency.AmountCanonicalJSONMaximumBytes,
		)
	}
	exactMaximum := append(
		bytes.Repeat([]byte{' '}, currency.AmountJSONDocumentSlackBytes),
		maximumCanonical...,
	)
	f.Add(exactMaximum)
	seeds := [][]byte{
		[]byte(`{"currency":"usd","minor_units":"1"}`),
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
		append(bytes.Clone(exactMaximum), ' '),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		wantCode, wantMinor, wantAccept := oracleAmountJSON(data)
		before := mustAmount(t, currency.CodeCLF, math.MinInt64)
		var gotFresh currency.Amount
		gotFreshErr := gotFresh.UnmarshalJSON(data)
		gotPopulated := before
		gotPopulatedErr := gotPopulated.UnmarshalJSON(data)
		proveFuzzAmountJSONResult(
			t,
			data,
			before,
			gotFresh,
			gotFreshErr,
			gotPopulated,
			gotPopulatedErr,
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
) (int64, error) {
	exponent, admitted := oracleFractionDigits(code)
	pattern := grammar.forExponent(exponent)
	if !admitted || pattern == nil || raw == "" ||
		len(raw) > currency.DecimalMaximumBytes || !pattern.MatchString(raw) {
		return 0, core.ErrCurrencyDecimal
	}
	value, ok := new(big.Rat).SetString(raw)
	if !ok || strings.HasPrefix(raw, "-") && value.Sign() == 0 {
		return 0, core.ErrCurrencyDecimal
	}
	scale := new(big.Int).Exp(
		big.NewInt(10),
		big.NewInt(int64(exponent)),
		nil,
	)
	value.Mul(value, new(big.Rat).SetInt(scale))
	if !value.IsInt() {
		return 0, core.ErrCurrencyDecimal
	}
	if !value.Num().IsInt64() {
		return 0, core.ErrCurrencyOverflow
	}
	return value.Num().Int64(), nil
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

func oracleCodeJSON(data []byte) (currency.Code, bool) {
	if len(data) == 0 || len(data) > currency.CodeJSONMaximumBytes ||
		!utf8.Valid(data) {
		return currency.CodeUnknown, false
	}
	decoder := jsontext.NewDecoder(bytes.NewReader(data))
	token, err := decoder.ReadToken()
	if err != nil || token.Kind() != jsontext.KindString {
		return currency.CodeUnknown, false
	}
	value := token.String()
	if _, err := decoder.ReadToken(); !errors.Is(err, io.EOF) {
		return currency.CodeUnknown, false
	}
	return oracleCode(value)
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
	decoder := jsontext.NewDecoder(bytes.NewReader(data))
	open, err := decoder.ReadToken()
	if err != nil || open.Kind() != jsontext.KindBeginObject {
		return currency.CodeUnknown, 0, false
	}
	state, ok := oracleAmountJSONFields(decoder)
	if !ok || !state.codeSet || !state.minorSet {
		return currency.CodeUnknown, 0, false
	}
	closeObject, err := decoder.ReadToken()
	if err != nil || closeObject.Kind() != jsontext.KindEndObject {
		return currency.CodeUnknown, 0, false
	}
	if _, err := decoder.ReadToken(); !errors.Is(err, io.EOF) {
		return currency.CodeUnknown, 0, false
	}
	return state.code, state.minor, true
}

func oracleAmountJSONFields(
	decoder *jsontext.Decoder,
) (amountJSONOracleState, bool) {
	var state amountJSONOracleState
	for decoder.PeekKind() != jsontext.KindEndObject {
		field, err := decoder.ReadToken()
		if err != nil {
			return amountJSONOracleState{}, false
		}
		if field.Kind() != jsontext.KindString {
			return amountJSONOracleState{}, false
		}
		fieldName := field.String()
		raw, err := decoder.ReadValue()
		if err != nil {
			return amountJSONOracleState{}, false
		}
		next, ok := oracleAmountJSONField(state, fieldName, raw)
		if !ok {
			return amountJSONOracleState{}, false
		}
		state = next
	}
	return state, true
}

func oracleAmountJSONField(
	state amountJSONOracleState,
	fieldName string,
	raw jsontext.Value,
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
	wantErr error,
) {
	t.Helper()

	if wantErr != nil {
		proveFuzzDecimalRejection(t, code, raw, got, gotErr, wantErr)
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
	wantErr error,
) {
	t.Helper()

	if got != (currency.Amount{}) ||
		!errors.Is(gotErr, wantErr) ||
		!errors.Is(gotErr, core.ErrCurrencyContract) ||
		!errors.Is(gotErr, core.ErrPrimitiveContract) {
		t.Fatalf(
			"currency.Parse(%v, %q) = (%v, %v), want zero and %v/%v/%v",
			code,
			raw,
			got,
			gotErr,
			wantErr,
			core.ErrCurrencyContract,
			core.ErrPrimitiveContract,
		)
	}
	if errors.Is(wantErr, core.ErrCurrencyOverflow) {
		if !errors.Is(gotErr, core.ErrNumericOverflow) {
			t.Fatalf("currency.Parse(%v, %q) error = %v, want %v", code, raw, gotErr, core.ErrNumericOverflow)
		}
		return
	}
	if errors.Is(gotErr, core.ErrCurrencyOverflow) ||
		errors.Is(gotErr, core.ErrNumericOverflow) {
		t.Fatalf(
			"currency.Parse(%v, %q) error = %v, want decimal rejection without overflow identity",
			code,
			raw,
			gotErr,
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
	if gotValidateErr := got.Validate(); gotValidateErr != nil {
		t.Fatalf("currency.Parse(%v, %q).Validate() error = %v, want nil", code, raw, gotValidateErr)
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
	gotSecond, gotSecondErr := gotRoundTrip.Decimal()
	if gotSecondErr != nil || gotSecond != canonical {
		t.Fatalf(
			"second Amount.Decimal() = (%q, %v), want (%q, nil)",
			gotSecond,
			gotSecondErr,
			canonical,
		)
	}
}

func proveFuzzCodeJSONResult(
	t *testing.T,
	data []byte,
	gotFresh currency.Code,
	gotFreshErr error,
	gotPopulated currency.Code,
	gotPopulatedErr error,
	wantCode currency.Code,
	wantAccept bool,
) {
	t.Helper()

	if !wantAccept {
		if gotFresh != currency.CodeUnknown || gotPopulated != currency.CodeCLF ||
			!errors.Is(gotFreshErr, core.ErrJSONContract) ||
			!errors.Is(gotFreshErr, core.ErrCurrencyContract) ||
			!errors.Is(gotFreshErr, core.ErrPrimitiveContract) ||
			!errors.Is(gotPopulatedErr, core.ErrJSONContract) ||
			!errors.Is(gotPopulatedErr, core.ErrCurrencyContract) ||
			!errors.Is(gotPopulatedErr, core.ErrPrimitiveContract) {
			t.Fatalf(
				"Code.UnmarshalJSON(%q) fresh/populated = (%v/%v, %v/%v), want zero/preserved and %v/%v/%v",
				data,
				gotFresh,
				gotFreshErr,
				gotPopulated,
				gotPopulatedErr,
				core.ErrJSONContract,
				core.ErrCurrencyContract,
				core.ErrPrimitiveContract,
			)
		}
		return
	}
	if gotFreshErr != nil || gotPopulatedErr != nil ||
		gotFresh != wantCode || gotPopulated != wantCode {
		t.Fatalf(
			"Code.UnmarshalJSON(%q) fresh/populated = (%v/%v, %v/%v), want (%v/nil, %v/nil)",
			data,
			gotFresh,
			gotFreshErr,
			gotPopulated,
			gotPopulatedErr,
			wantCode,
			wantCode,
		)
	}
	if gotValidateErr := gotFresh.Validate(); gotValidateErr != nil {
		t.Fatalf("accepted Code.Validate() error = %v, want nil", gotValidateErr)
	}
	canonical, gotMarshalErr := gotFresh.MarshalJSON()
	if gotMarshalErr != nil || len(canonical) > currency.CodeJSONMaximumBytes {
		t.Fatalf(
			"Code.MarshalJSON(accepted) = (%q, %v), want at most %d bytes and nil",
			canonical,
			gotMarshalErr,
			currency.CodeJSONMaximumBytes,
		)
	}
	var gotRoundTrip currency.Code
	gotRoundTripErr := gotRoundTrip.UnmarshalJSON(canonical)
	if gotRoundTripErr != nil || gotRoundTrip != gotFresh {
		t.Fatalf(
			"Code.UnmarshalJSON(canonical) = (%v, %v), want (%v, nil)",
			gotRoundTrip,
			gotRoundTripErr,
			gotFresh,
		)
	}
	second, gotSecondErr := gotRoundTrip.MarshalJSON()
	if gotSecondErr != nil || !bytes.Equal(second, canonical) {
		t.Fatalf(
			"second Code.MarshalJSON() = (%q, %v), want (%q, nil)",
			second,
			gotSecondErr,
			canonical,
		)
	}
}

func proveFuzzAmountJSONResult(
	t *testing.T,
	data []byte,
	before currency.Amount,
	gotFresh currency.Amount,
	gotFreshErr error,
	gotPopulated currency.Amount,
	gotPopulatedErr error,
	wantCode currency.Code,
	wantMinor int64,
	wantAccept bool,
) {
	t.Helper()

	if !wantAccept {
		proveFuzzAmountJSONRejection(
			t,
			data,
			before,
			gotFresh,
			gotFreshErr,
			gotPopulated,
			gotPopulatedErr,
		)
		return
	}
	proveFuzzAmountJSONAcceptance(
		t,
		data,
		gotFresh,
		gotFreshErr,
		gotPopulated,
		gotPopulatedErr,
		wantCode,
		wantMinor,
	)
}

func proveFuzzAmountJSONRejection(
	t *testing.T,
	data []byte,
	before currency.Amount,
	gotFresh currency.Amount,
	gotFreshErr error,
	gotPopulated currency.Amount,
	gotPopulatedErr error,
) {
	t.Helper()

	if !errors.Is(gotFreshErr, core.ErrJSONContract) ||
		!errors.Is(gotFreshErr, core.ErrCurrencyContract) ||
		!errors.Is(gotFreshErr, core.ErrPrimitiveContract) ||
		!errors.Is(gotPopulatedErr, core.ErrJSONContract) ||
		!errors.Is(gotPopulatedErr, core.ErrCurrencyContract) ||
		!errors.Is(gotPopulatedErr, core.ErrPrimitiveContract) ||
		gotFresh != (currency.Amount{}) || gotPopulated != before {
		t.Fatalf(
			"Amount.UnmarshalJSON(%q) fresh/populated = (%v/%v, %v/%v), want zero/preserved and %v/%v/%v",
			data,
			gotFresh,
			gotFreshErr,
			gotPopulated,
			gotPopulatedErr,
			core.ErrJSONContract,
			core.ErrCurrencyContract,
			core.ErrPrimitiveContract,
		)
	}
}

func proveFuzzAmountJSONAcceptance(
	t *testing.T,
	data []byte,
	gotFresh currency.Amount,
	gotFreshErr error,
	gotPopulated currency.Amount,
	gotPopulatedErr error,
	wantCode currency.Code,
	wantMinor int64,
) {
	t.Helper()

	if gotFreshErr != nil || gotPopulatedErr != nil || gotFresh != gotPopulated {
		t.Fatalf(
			"Amount.UnmarshalJSON(%q) fresh/populated = (%v/%v, %v/%v), want equal admitted values and nil",
			data,
			gotFresh,
			gotFreshErr,
			gotPopulated,
			gotPopulatedErr,
		)
	}
	if gotValidateErr := gotFresh.Validate(); gotValidateErr != nil {
		t.Fatalf("accepted Amount.Validate() error = %v, want nil", gotValidateErr)
	}
	gotCode, gotCodeErr := gotFresh.Code()
	gotMinor, gotMinorErr := gotFresh.MinorUnits()
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
	canonical, gotMarshalErr := json.Marshal(gotFresh)
	if gotMarshalErr != nil || !jsontext.Value(canonical).IsValid() ||
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
	if gotRoundTripErr != nil || gotRoundTrip != gotFresh {
		t.Fatalf(
			"json.Unmarshal(canonical Amount) = (%v, %v), want (%v, nil)",
			gotRoundTrip,
			gotRoundTripErr,
			gotFresh,
		)
	}
	second, gotSecondErr := json.Marshal(gotRoundTrip)
	if gotSecondErr != nil || !bytes.Equal(second, canonical) {
		t.Fatalf(
			"second json.Marshal(Amount) = (%q, %v), want (%q, nil)",
			second,
			gotSecondErr,
			canonical,
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
