package currency_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/currency"
)

func TestCodeDefinitionMatrixIsClosedAndCanonical(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		token          string
		code           currency.Code
		fractionDigits uint8
	}{
		{name: "USD uses two digits", code: currency.CodeUSD, token: currency.CodeTokenUSD, fractionDigits: currency.MinorUnitDigitsTwo},
		{name: "EUR uses two digits", code: currency.CodeEUR, token: currency.CodeTokenEUR, fractionDigits: currency.MinorUnitDigitsTwo},
		{name: "GBP uses two digits", code: currency.CodeGBP, token: currency.CodeTokenGBP, fractionDigits: currency.MinorUnitDigitsTwo},
		{name: "CAD uses two digits", code: currency.CodeCAD, token: currency.CodeTokenCAD, fractionDigits: currency.MinorUnitDigitsTwo},
		{name: "AUD uses two digits", code: currency.CodeAUD, token: currency.CodeTokenAUD, fractionDigits: currency.MinorUnitDigitsTwo},
		{name: "JPY uses zero digits", code: currency.CodeJPY, token: currency.CodeTokenJPY, fractionDigits: currency.MinorUnitDigitsZero},
		{name: "CHF uses two digits", code: currency.CodeCHF, token: currency.CodeTokenCHF, fractionDigits: currency.MinorUnitDigitsTwo},
		{name: "NZD uses two digits", code: currency.CodeNZD, token: currency.CodeTokenNZD, fractionDigits: currency.MinorUnitDigitsTwo},
		{name: "SGD uses two digits", code: currency.CodeSGD, token: currency.CodeTokenSGD, fractionDigits: currency.MinorUnitDigitsTwo},
		{name: "HKD uses two digits", code: currency.CodeHKD, token: currency.CodeTokenHKD, fractionDigits: currency.MinorUnitDigitsTwo},
		{name: "BHD uses three digits", code: currency.CodeBHD, token: currency.CodeTokenBHD, fractionDigits: currency.MinorUnitDigitsThree},
		{name: "CLF uses four digits", code: currency.CodeCLF, token: currency.CodeTokenCLF, fractionDigits: currency.MinorUnitDigitsFour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if gotErr := tc.code.Validate(); gotErr != nil {
				t.Fatalf("Code.Validate() error = %v, want nil", gotErr)
			}
			if got := tc.code.String(); got != tc.token {
				t.Fatalf("Code.String() = %q, want %q", got, tc.token)
			}
			gotDigits, gotDigitsErr := tc.code.FractionDigits()
			if gotDigitsErr != nil || gotDigits != tc.fractionDigits {
				t.Fatalf(
					"Code.FractionDigits() = (%d, %v), want (%d, nil)",
					gotDigits,
					gotDigitsErr,
					tc.fractionDigits,
				)
			}
			gotCode, gotParseErr := currency.ParseCode(tc.token)
			if gotParseErr != nil || gotCode != tc.code {
				t.Fatalf("ParseCode(%q) = (%v, %v), want (%v, nil)", tc.token, gotCode, gotParseErr, tc.code)
			}
			gotJSON, gotMarshalErr := json.Marshal(tc.code)
			wantJSON, gotWantJSONErr := json.Marshal(tc.token)
			if gotMarshalErr != nil || gotWantJSONErr != nil || !bytes.Equal(gotJSON, wantJSON) {
				t.Fatalf("json.Marshal(Code) = (%q, %v), want stdlib string projection (%q, %v)", gotJSON, gotMarshalErr, wantJSON, gotWantJSONErr)
			}
			var gotJSONCode currency.Code
			gotUnmarshalErr := json.Unmarshal(gotJSON, &gotJSONCode)
			if gotUnmarshalErr != nil || gotJSONCode != tc.code {
				t.Fatalf("json.Unmarshal(Code) = (%v, %v), want (%v, nil)", gotJSONCode, gotUnmarshalErr, tc.code)
			}
		})
	}
}

func TestCurrencySchemaLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive every admitted currency seals nonzero extrema", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name       string
			code       currency.Code
			minorUnits int64
		}{
			{name: "USD minimum", code: currency.CodeUSD, minorUnits: math.MinInt64},
			{name: "EUR one above minimum", code: currency.CodeEUR, minorUnits: math.MinInt64 + 1},
			{name: "GBP negative one", code: currency.CodeGBP, minorUnits: -1},
			{name: "CAD positive one", code: currency.CodeCAD, minorUnits: 1},
			{name: "AUD one below maximum", code: currency.CodeAUD, minorUnits: math.MaxInt64 - 1},
			{name: "JPY maximum", code: currency.CodeJPY, minorUnits: math.MaxInt64},
			{name: "CHF negative midpoint", code: currency.CodeCHF, minorUnits: math.MinInt64 / 2},
			{name: "NZD positive midpoint", code: currency.CodeNZD, minorUnits: math.MaxInt64 / 2},
			{name: "SGD negative ordinary value", code: currency.CodeSGD, minorUnits: -125},
			{name: "HKD positive ordinary value", code: currency.CodeHKD, minorUnits: 125},
			{name: "BHD negative exponent-sensitive value", code: currency.CodeBHD, minorUnits: -1001},
			{name: "CLF positive exponent-sensitive value", code: currency.CodeCLF, minorUnits: 10001},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got, gotErr := currency.New(tc.code, tc.minorUnits)
				if gotErr != nil {
					t.Fatalf("currency.New(%v, %d) error = %v, want nil", tc.code, tc.minorUnits, gotErr)
				}
				if gotValidateErr := got.Validate(); gotValidateErr != nil {
					t.Fatalf("Amount.Validate() error = %v, want nil", gotValidateErr)
				}
				gotCode, gotCodeErr := got.Code()
				gotMinor, gotMinorErr := got.MinorUnits()
				if gotCodeErr != nil || gotMinorErr != nil ||
					gotCode != tc.code || gotMinor != tc.minorUnits {
					t.Fatalf(
						"Amount projection = (code:%v/%v minor:%d/%v), want (%v, %d)",
						gotCode,
						gotCodeErr,
						gotMinor,
						gotMinorErr,
						tc.code,
						tc.minorUnits,
					)
				}
			})
		}
	})

	t.Run("negative the sole externally constructible invalid amount stays zero", func(t *testing.T) {
		t.Parallel()

		got := currency.Amount{}
		gotErr := got.Validate()
		if got != (currency.Amount{}) ||
			!errors.Is(gotErr, core.ErrCurrencyContract) ||
			!errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf(
				"zero Amount.Validate() = (%v, %v), want zero and %v/%v",
				got,
				gotErr,
				core.ErrCurrencyContract,
				core.ErrPrimitiveContract,
			)
		}
	})

	t.Run("neutral zero minor units remain real amounts for every currency", func(t *testing.T) {
		t.Parallel()

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
			t.Run(code.String(), func(t *testing.T) {
				t.Parallel()

				got, gotErr := currency.New(code, 0)
				if gotErr != nil {
					t.Fatalf("currency.New(%v, 0) error = %v, want nil", code, gotErr)
				}
				gotMinor, gotMinorErr := got.MinorUnits()
				gotCode, gotCodeErr := got.Code()
				if gotMinorErr != nil || gotCodeErr != nil ||
					gotMinor != 0 || gotCode != code {
					t.Fatalf(
						"zero Amount projection = (code:%v/%v minor:%d/%v), want (%v, 0)",
						gotCode,
						gotCodeErr,
						gotMinor,
						gotMinorErr,
						code,
					)
				}
			})
		}
	})
}

func TestCodeRejectsOutsideDomainAndNoncanonicalTokens(t *testing.T) {
	t.Parallel()

	codeCases := []struct {
		name string
		code currency.Code
	}{
		{name: "zero rejected", code: currency.CodeUnknown},
		{name: "one past admitted range rejected", code: currency.CodeCLF + 1},
		{name: "maximum backing value rejected", code: currency.Code(math.MaxUint8)},
	}
	for _, tc := range codeCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.code.Validate()
			if !errors.Is(gotErr, core.ErrCurrencyContract) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf(
					"Code(%d).Validate() error = %v, want %v and %v",
					tc.code,
					gotErr,
					core.ErrCurrencyContract,
					core.ErrPrimitiveContract,
				)
			}
			if got := tc.code.String(); got != "" {
				t.Fatalf("Code(%d).String() = %q, want empty", tc.code, got)
			}
			gotAmount, gotNewErr := currency.New(tc.code, 1)
			if gotAmount != (currency.Amount{}) ||
				!errors.Is(gotNewErr, core.ErrCurrencyContract) ||
				!errors.Is(gotNewErr, core.ErrPrimitiveContract) {
				t.Fatalf(
					"currency.New(Code(%d), 1) = (%v, %v), want zero and %v/%v",
					tc.code,
					gotAmount,
					gotNewErr,
					core.ErrCurrencyContract,
					core.ErrPrimitiveContract,
				)
			}
		})
	}

	tokenCases := []struct {
		name  string
		token string
	}{
		{name: "empty rejected"},
		{name: "lowercase rejected", token: "usd"},
		{name: "leading whitespace rejected", token: " USD"},
		{name: "trailing whitespace rejected", token: "USD "},
		{name: "unknown uppercase rejected", token: "ZZZ"},
		{name: "mixed case rejected", token: "Usd"},
		{name: "two-byte token rejected", token: "US"},
		{name: "four-byte token rejected", token: "USDX"},
		{name: "leading newline rejected", token: "\nUSD"},
		{name: "trailing tab rejected", token: "USD\t"},
		{name: "embedded NUL rejected", token: "U\x00D"},
		{name: "unicode homoglyph rejected", token: "UЅD"},
	}
	for _, tc := range tokenCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := currency.ParseCode(tc.token)
			if got != currency.CodeUnknown ||
				!errors.Is(gotErr, core.ErrCurrencyContract) {
				t.Fatalf(
					"ParseCode(%q) = (%v, %v), want (%v, %v)",
					tc.token,
					got,
					gotErr,
					currency.CodeUnknown,
					core.ErrCurrencyContract,
				)
			}
		})
	}
}

func TestCodeJSONHostileMatrixPreservesReceiverAndBoundsWork(t *testing.T) {
	t.Parallel()

	before := currency.CodeCAD
	canonical, gotCanonicalErr := json.Marshal(before)
	if gotCanonicalErr != nil {
		t.Fatalf("json.Marshal(Code) error = %v, want nil", gotCanonicalErr)
	}
	allowance := currency.CodeJSONMaximumBytes - len(canonical)
	validCases := []struct {
		name string
		data []byte
	}{
		{name: "canonical compact code", data: append([]byte(nil), canonical...)},
		{name: "one leading space", data: append([]byte{' '}, canonical...)},
		{name: "one trailing space", data: append(append([]byte(nil), canonical...), ' ')},
		{name: "exact document byte bound", data: append(append([]byte(nil), canonical...), bytes.Repeat([]byte{' '}, allowance)...)},
	}
	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got currency.Code
			gotErr := got.UnmarshalJSON(tc.data)
			if gotErr != nil || got != before {
				t.Fatalf("Code.UnmarshalJSON(%q) = (%v, %v), want (%v, nil)", tc.data, got, gotErr, before)
			}
		})
	}

	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty rejected"},
		{name: "null rejected", data: []byte("null")},
		{name: "boolean rejected", data: []byte("true")},
		{name: "number rejected", data: []byte("1")},
		{name: "array rejected", data: []byte("[]")},
		{name: "object rejected", data: []byte("{}")},
		{name: "lowercase token rejected", data: []byte(`"cad"`)},
		{name: "unknown token rejected", data: []byte(`"ZZZ"`)},
		{name: "embedded whitespace rejected", data: []byte(`"C AD"`)},
		{name: "invalid UTF-8 rejected", data: []byte{'"', 0xff, '"'}},
		{name: "trailing document rejected", data: append(append([]byte(nil), canonical...), canonical...)},
		{name: "one above document byte bound rejected", data: append(append([]byte(nil), canonical...), bytes.Repeat([]byte{' '}, allowance+1)...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := before
			gotErr := got.UnmarshalJSON(tc.data)
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrCurrencyContract) {
				t.Fatalf("Code.UnmarshalJSON(%q) error = %v, want %v and %v", tc.data, gotErr, core.ErrJSONContract, core.ErrCurrencyContract)
			}
			if got != before {
				t.Fatalf("rejected Code JSON receiver = %v, want preserved %v", got, before)
			}
		})
	}

	var nilCode *currency.Code
	if gotErr := nilCode.UnmarshalJSON(canonical); !errors.Is(gotErr, core.ErrJSONContract) ||
		!errors.Is(gotErr, core.ErrCurrencyContract) {
		t.Fatalf("nil Code.UnmarshalJSON() error = %v, want %v and %v", gotErr, core.ErrJSONContract, core.ErrCurrencyContract)
	}
}

func TestAmountArithmeticHostileMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr   error
		operation func(currency.Amount, currency.Amount) (currency.Amount, error)
		name      string
		left      currency.Amount
		right     currency.Amount
		want      currency.Amount
	}{
		{
			name:      "add ordinary same currency",
			operation: currency.Amount.Add,
			left:      mustAmount(t, currency.CodeCAD, 19),
			right:     mustAmount(t, currency.CodeCAD, 23),
			want:      mustAmount(t, currency.CodeCAD, 42),
		},
		{
			name:      "add reaches maximum",
			operation: currency.Amount.Add,
			left:      mustAmount(t, currency.CodeCAD, math.MaxInt64-1),
			right:     mustAmount(t, currency.CodeCAD, 1),
			want:      mustAmount(t, currency.CodeCAD, math.MaxInt64),
		},
		{
			name:      "add overflows above maximum",
			operation: currency.Amount.Add,
			left:      mustAmount(t, currency.CodeCAD, math.MaxInt64),
			right:     mustAmount(t, currency.CodeCAD, 1),
			wantErr:   core.ErrCurrencyOverflow,
		},
		{
			name:      "add reaches minimum",
			operation: currency.Amount.Add,
			left:      mustAmount(t, currency.CodeCAD, math.MinInt64+1),
			right:     mustAmount(t, currency.CodeCAD, -1),
			want:      mustAmount(t, currency.CodeCAD, math.MinInt64),
		},
		{
			name:      "add underflows below minimum",
			operation: currency.Amount.Add,
			left:      mustAmount(t, currency.CodeCAD, math.MinInt64),
			right:     mustAmount(t, currency.CodeCAD, -1),
			wantErr:   core.ErrCurrencyOverflow,
		},
		{
			name:      "subtract ordinary same currency",
			operation: currency.Amount.Subtract,
			left:      mustAmount(t, currency.CodeCAD, 19),
			right:     mustAmount(t, currency.CodeCAD, 23),
			want:      mustAmount(t, currency.CodeCAD, -4),
		},
		{
			name:      "subtract reaches maximum",
			operation: currency.Amount.Subtract,
			left:      mustAmount(t, currency.CodeCAD, math.MaxInt64-1),
			right:     mustAmount(t, currency.CodeCAD, -1),
			want:      mustAmount(t, currency.CodeCAD, math.MaxInt64),
		},
		{
			name:      "subtract overflows above maximum",
			operation: currency.Amount.Subtract,
			left:      mustAmount(t, currency.CodeCAD, math.MaxInt64),
			right:     mustAmount(t, currency.CodeCAD, -1),
			wantErr:   core.ErrCurrencyOverflow,
		},
		{
			name:      "subtract reaches minimum",
			operation: currency.Amount.Subtract,
			left:      mustAmount(t, currency.CodeCAD, math.MinInt64+1),
			right:     mustAmount(t, currency.CodeCAD, 1),
			want:      mustAmount(t, currency.CodeCAD, math.MinInt64),
		},
		{
			name:      "subtract underflows below minimum",
			operation: currency.Amount.Subtract,
			left:      mustAmount(t, currency.CodeCAD, math.MinInt64),
			right:     mustAmount(t, currency.CodeCAD, 1),
			wantErr:   core.ErrCurrencyOverflow,
		},
		{
			name:      "currency mismatch rejected",
			operation: currency.Amount.Add,
			left:      mustAmount(t, currency.CodeCAD, 1),
			right:     mustAmount(t, currency.CodeUSD, 1),
			wantErr:   core.ErrCurrencyMismatch,
		},
		{
			name:      "subtract currency mismatch rejected",
			operation: currency.Amount.Subtract,
			left:      mustAmount(t, currency.CodeCAD, 1),
			right:     mustAmount(t, currency.CodeUSD, 1),
			wantErr:   core.ErrCurrencyMismatch,
		},
		{
			name:      "zero left amount rejected",
			operation: currency.Amount.Add,
			left:      currency.Amount{},
			right:     mustAmount(t, currency.CodeCAD, 1),
			wantErr:   core.ErrCurrencyContract,
		},
		{
			name:      "subtract zero left amount rejected",
			operation: currency.Amount.Subtract,
			left:      currency.Amount{},
			right:     mustAmount(t, currency.CodeCAD, 1),
			wantErr:   core.ErrCurrencyContract,
		},
		{
			name:      "subtract zero right amount rejected",
			operation: currency.Amount.Subtract,
			left:      mustAmount(t, currency.CodeCAD, 1),
			right:     currency.Amount{},
			wantErr:   core.ErrCurrencyContract,
		},
		{
			name:      "opposite extrema add to negative one",
			operation: currency.Amount.Add,
			left:      mustAmount(t, currency.CodeCAD, math.MaxInt64),
			right:     mustAmount(t, currency.CodeCAD, math.MinInt64),
			want:      mustAmount(t, currency.CodeCAD, -1),
		},
		{
			name:      "maximum subtracts itself to zero",
			operation: currency.Amount.Subtract,
			left:      mustAmount(t, currency.CodeCAD, math.MaxInt64),
			right:     mustAmount(t, currency.CodeCAD, math.MaxInt64),
			want:      mustAmount(t, currency.CodeCAD, 0),
		},
		{
			name:      "minimum subtracts itself to zero",
			operation: currency.Amount.Subtract,
			left:      mustAmount(t, currency.CodeCAD, math.MinInt64),
			right:     mustAmount(t, currency.CodeCAD, math.MinInt64),
			want:      mustAmount(t, currency.CodeCAD, 0),
		},
		{
			name:      "zero right amount rejected",
			operation: currency.Amount.Add,
			left:      mustAmount(t, currency.CodeCAD, 1),
			right:     currency.Amount{},
			wantErr:   core.ErrCurrencyContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := tc.operation(tc.left, tc.right)
			if tc.wantErr != nil && !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("amount operation error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr == nil && (gotErr != nil || got != tc.want) {
				t.Fatalf("amount operation = (%v, %v), want (%v, %v)", got, gotErr, tc.want, tc.wantErr)
			}
			if errors.Is(gotErr, core.ErrCurrencyOverflow) &&
				!errors.Is(gotErr, core.ErrNumericOverflow) {
				t.Fatalf("amount overflow error = %v, want parent %v", gotErr, core.ErrNumericOverflow)
			}
		})
	}
}

func TestAmountArithmeticMatchesArbitraryPrecisionBoundaryMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		left  int64
		right int64
	}{
		{name: "zero and zero remain zero"},
		{name: "one and negative one cancel", left: 1, right: -1},
		{name: "negative one and one cancel", left: -1, right: 1},
		{name: "maximum and zero stay in range", left: math.MaxInt64},
		{name: "minimum and zero stay in range", left: math.MinInt64},
		{name: "maximum plus one crosses range", left: math.MaxInt64, right: 1},
		{name: "maximum plus negative one stays in range", left: math.MaxInt64, right: -1},
		{name: "minimum plus negative one crosses range", left: math.MinInt64, right: -1},
		{name: "minimum plus one stays in range", left: math.MinInt64, right: 1},
		{name: "opposite extrema sum to negative one", left: math.MaxInt64, right: math.MinInt64},
		{name: "minimum plus maximum sum to negative one", left: math.MinInt64, right: math.MaxInt64},
		{name: "maximum minus negative one crosses range", left: math.MaxInt64, right: -1},
		{name: "minimum minus one crosses range", left: math.MinInt64, right: 1},
		{name: "maximum minus maximum is zero", left: math.MaxInt64, right: math.MaxInt64},
		{name: "minimum minus minimum is zero", left: math.MinInt64, right: math.MinInt64},
		{name: "positive halves add without overflow", left: math.MaxInt64 / 2, right: math.MaxInt64 / 2},
		{name: "negative halves add without underflow", left: math.MinInt64 / 2, right: math.MinInt64 / 2},
		{name: "positive half subtracts negative half", left: math.MaxInt64 / 2, right: math.MinInt64 / 2},
		{name: "negative half subtracts positive half", left: math.MinInt64 / 2, right: math.MaxInt64 / 2},
		{name: "ordinary asymmetric operands preserve sign", left: 1_234_567_890, right: -987_654_321},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			left := mustAmount(t, currency.CodeCAD, tc.left)
			right := mustAmount(t, currency.CodeCAD, tc.right)
			gotAdd, gotAddErr := left.Add(right)
			gotSubtract, gotSubtractErr := left.Subtract(right)
			operations := []struct {
				gotErr     error
				wantOracle *big.Int
				name       string
				got        currency.Amount
			}{
				{
					name:       "addition",
					got:        gotAdd,
					gotErr:     gotAddErr,
					wantOracle: new(big.Int).Add(big.NewInt(tc.left), big.NewInt(tc.right)),
				},
				{
					name:       "subtraction",
					got:        gotSubtract,
					gotErr:     gotSubtractErr,
					wantOracle: new(big.Int).Sub(big.NewInt(tc.left), big.NewInt(tc.right)),
				},
			}
			for _, operation := range operations {
				if !operation.wantOracle.IsInt64() {
					if operation.got != (currency.Amount{}) ||
						!errors.Is(operation.gotErr, core.ErrCurrencyOverflow) ||
						!errors.Is(operation.gotErr, core.ErrNumericOverflow) ||
						!errors.Is(operation.gotErr, core.ErrCurrencyContract) {
						t.Fatalf(
							"%s = (%v, %v), want zero and %v/%v/%v for oracle %v",
							operation.name,
							operation.got,
							operation.gotErr,
							core.ErrCurrencyOverflow,
							core.ErrNumericOverflow,
							core.ErrCurrencyContract,
							operation.wantOracle,
						)
					}
					continue
				}
				want := mustAmount(t, currency.CodeCAD, operation.wantOracle.Int64())
				if operation.gotErr != nil || operation.got != want {
					t.Fatalf(
						"%s = (%v, %v), want (%v, nil)",
						operation.name,
						operation.got,
						operation.gotErr,
						want,
					)
				}
			}
		})
	}
}

func TestAmountZeroValueRejectsEveryPublicBoundary(t *testing.T) {
	t.Parallel()

	valid := mustAmount(t, currency.CodeCAD, 1)
	cases := []struct {
		operation func(*testing.T) error
		name      string
	}{
		{name: "Validate rejects zero amount", operation: func(*testing.T) error {
			return (currency.Amount{}).Validate()
		}},
		{name: "Code rejects zero amount", operation: func(t *testing.T) error {
			got, gotErr := (currency.Amount{}).Code()
			if got != currency.CodeUnknown {
				t.Fatalf("zero Amount.Code() value = %v, want %v", got, currency.CodeUnknown)
			}
			return gotErr
		}},
		{name: "MinorUnits rejects zero amount", operation: func(t *testing.T) error {
			got, gotErr := (currency.Amount{}).MinorUnits()
			if got != 0 {
				t.Fatalf("zero Amount.MinorUnits() value = %d, want 0", got)
			}
			return gotErr
		}},
		{name: "Add rejects zero left amount", operation: func(t *testing.T) error {
			got, gotErr := (currency.Amount{}).Add(valid)
			if got != (currency.Amount{}) {
				t.Fatalf("zero Amount.Add(valid) value = %v, want zero", got)
			}
			return gotErr
		}},
		{name: "Add rejects zero right amount", operation: func(t *testing.T) error {
			got, gotErr := valid.Add(currency.Amount{})
			if got != (currency.Amount{}) {
				t.Fatalf("valid Amount.Add(zero) value = %v, want zero", got)
			}
			return gotErr
		}},
		{name: "Subtract rejects zero left amount", operation: func(t *testing.T) error {
			got, gotErr := (currency.Amount{}).Subtract(valid)
			if got != (currency.Amount{}) {
				t.Fatalf("zero Amount.Subtract(valid) value = %v, want zero", got)
			}
			return gotErr
		}},
		{name: "Subtract rejects zero right amount", operation: func(t *testing.T) error {
			got, gotErr := valid.Subtract(currency.Amount{})
			if got != (currency.Amount{}) {
				t.Fatalf("valid Amount.Subtract(zero) value = %v, want zero", got)
			}
			return gotErr
		}},
		{name: "Compare rejects zero left amount", operation: func(t *testing.T) error {
			got, gotErr := (currency.Amount{}).Compare(valid)
			if got != core.ComparisonUnknown {
				t.Fatalf("zero Amount.Compare(valid) value = %v, want %v", got, core.ComparisonUnknown)
			}
			return gotErr
		}},
		{name: "Compare rejects zero right amount", operation: func(t *testing.T) error {
			got, gotErr := valid.Compare(currency.Amount{})
			if got != core.ComparisonUnknown {
				t.Fatalf("valid Amount.Compare(zero) value = %v, want %v", got, core.ComparisonUnknown)
			}
			return gotErr
		}},
		{name: "Decimal rejects zero amount", operation: func(t *testing.T) error {
			got, gotErr := (currency.Amount{}).Decimal()
			if got != "" {
				t.Fatalf("zero Amount.Decimal() value = %q, want empty", got)
			}
			return gotErr
		}},
		{name: "MarshalJSON rejects zero amount", operation: func(t *testing.T) error {
			got, gotErr := (currency.Amount{}).MarshalJSON()
			if got != nil {
				t.Fatalf("zero Amount.MarshalJSON() value = %q, want nil", got)
			}
			return gotErr
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.operation(t)
			if !errors.Is(gotErr, core.ErrCurrencyContract) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf(
					"zero Amount public boundary error = %v, want %v/%v",
					gotErr,
					core.ErrCurrencyContract,
					core.ErrPrimitiveContract,
				)
			}
		})
	}
}

func TestAmountComparisonReturnsCoreComparison(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		left    currency.Amount
		right   currency.Amount
		want    core.Comparison
	}{
		{name: "negative one is less than zero", left: mustAmount(t, currency.CodeCAD, -1), right: mustAmount(t, currency.CodeCAD, 0), want: core.ComparisonLess},
		{name: "zero equals zero", left: mustAmount(t, currency.CodeCAD, 0), right: mustAmount(t, currency.CodeCAD, 0), want: core.ComparisonEqual},
		{name: "positive one is greater than zero", left: mustAmount(t, currency.CodeCAD, 1), right: mustAmount(t, currency.CodeCAD, 0), want: core.ComparisonGreater},
		{name: "minimum is less than maximum", left: mustAmount(t, currency.CodeCAD, math.MinInt64), right: mustAmount(t, currency.CodeCAD, math.MaxInt64), want: core.ComparisonLess},
		{name: "maximum is greater than minimum", left: mustAmount(t, currency.CodeCAD, math.MaxInt64), right: mustAmount(t, currency.CodeCAD, math.MinInt64), want: core.ComparisonGreater},
		{name: "minimum equals minimum", left: mustAmount(t, currency.CodeCAD, math.MinInt64), right: mustAmount(t, currency.CodeCAD, math.MinInt64), want: core.ComparisonEqual},
		{name: "maximum equals maximum", left: mustAmount(t, currency.CodeCAD, math.MaxInt64), right: mustAmount(t, currency.CodeCAD, math.MaxInt64), want: core.ComparisonEqual},
		{name: "negative maximum magnitude is less", left: mustAmount(t, currency.CodeCAD, -math.MaxInt64), right: mustAmount(t, currency.CodeCAD, math.MaxInt64), want: core.ComparisonLess},
		{name: "positive maximum magnitude is greater", left: mustAmount(t, currency.CodeCAD, math.MaxInt64), right: mustAmount(t, currency.CodeCAD, -math.MaxInt64), want: core.ComparisonGreater},
		{name: "adjacent high values preserve order", left: mustAmount(t, currency.CodeCAD, math.MaxInt64-1), right: mustAmount(t, currency.CodeCAD, math.MaxInt64), want: core.ComparisonLess},
		{name: "mismatch rejected", left: mustAmount(t, currency.CodeCAD, 0), right: mustAmount(t, currency.CodeUSD, 0), want: core.ComparisonUnknown, wantErr: core.ErrCurrencyMismatch},
		{name: "zero left rejected", left: currency.Amount{}, right: mustAmount(t, currency.CodeCAD, 0), want: core.ComparisonUnknown, wantErr: core.ErrCurrencyContract},
		{name: "zero right rejected", left: mustAmount(t, currency.CodeCAD, 0), right: currency.Amount{}, want: core.ComparisonUnknown, wantErr: core.ErrCurrencyContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := tc.left.Compare(tc.right)
			if !errors.Is(gotErr, tc.wantErr) || got != tc.want {
				t.Fatalf("Amount.Compare() = (%v, %v), want (%v, %v)", got, gotErr, tc.want, tc.wantErr)
			}
			if gotErr == nil {
				if gotValidateErr := got.Validate(); gotValidateErr != nil {
					t.Fatalf("Comparison.Validate() error = %v, want nil", gotValidateErr)
				}
				if got.String() == core.ComparisonUnknown.String() {
					t.Fatalf("Comparison.String() = %q, want admitted diagnostic", got.String())
				}
			}
		})
	}
}

func mustAmount(t *testing.T, code currency.Code, minorUnits int64) currency.Amount {
	t.Helper()

	value, err := currency.New(code, minorUnits)
	if err != nil {
		t.Fatalf("currency.New(%v, %d) error = %v, want nil", code, minorUnits, err)
	}
	return value
}
