package currency

import (
	"encoding/json"
	"errors"
	"math"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// CodeTokenUSD is the canonical USD token.
	CodeTokenUSD = "USD"
	// CodeTokenEUR is the canonical EUR token.
	CodeTokenEUR = "EUR"
	// CodeTokenGBP is the canonical GBP token.
	CodeTokenGBP = "GBP"
	// CodeTokenCAD is the canonical CAD token.
	CodeTokenCAD = "CAD"
	// CodeTokenAUD is the canonical AUD token.
	CodeTokenAUD = "AUD"
	// CodeTokenJPY is the canonical JPY token.
	CodeTokenJPY = "JPY"
	// CodeTokenCHF is the canonical CHF token.
	CodeTokenCHF = "CHF"
	// CodeTokenNZD is the canonical NZD token.
	CodeTokenNZD = "NZD"
	// CodeTokenSGD is the canonical SGD token.
	CodeTokenSGD = "SGD"
	// CodeTokenHKD is the canonical HKD token.
	CodeTokenHKD = "HKD"
	// CodeTokenBHD is the canonical BHD token.
	CodeTokenBHD = "BHD"
	// CodeTokenCLF is the canonical CLF token.
	CodeTokenCLF = "CLF"
)

const currencyTokenNotAdmittedReason = "currency token is not admitted"

const (
	// MinorUnitDigitsZero identifies currencies without fractional minor units.
	MinorUnitDigitsZero uint8 = 0
	// MinorUnitDigitsTwo identifies currencies with two fractional digits.
	MinorUnitDigitsTwo uint8 = 2
	// MinorUnitDigitsThree identifies currencies with three fractional digits.
	MinorUnitDigitsThree uint8 = 3
	// MinorUnitDigitsFour identifies currencies with four fractional digits.
	MinorUnitDigitsFour uint8 = 4
)

const (
	// DecimalMaximumBytes bounds external decimal input.
	DecimalMaximumBytes = 21
	// AmountCanonicalJSONMaximumBytes is the exact maximum compact amount JSON extent.
	AmountCanonicalJSONMaximumBytes = 55
	// AmountJSONDocumentSlackBytes bounds accepted document bytes beyond the
	// largest canonical amount.
	AmountJSONDocumentSlackBytes = 256
	// AmountJSONMaximumBytes bounds a complete accepted amount JSON document.
	AmountJSONMaximumBytes = AmountCanonicalJSONMaximumBytes + AmountJSONDocumentSlackBytes
	// CodeJSONMaximumBytes bounds one code JSON value including whitespace.
	CodeJSONMaximumBytes = 37
	// JSONFieldCurrency is the canonical amount currency field.
	JSONFieldCurrency = "currency"
	// JSONFieldMinorUnits is the canonical amount minor-unit field.
	JSONFieldMinorUnits = "minor_units"
)

// Code is the closed supported currency domain.
type Code uint8

const (
	// CodeUnknown is the invalid zero currency.
	CodeUnknown Code = iota
	// CodeUSD identifies United States dollars.
	CodeUSD
	// CodeEUR identifies euros.
	CodeEUR
	// CodeGBP identifies pounds sterling.
	CodeGBP
	// CodeCAD identifies Canadian dollars.
	CodeCAD
	// CodeAUD identifies Australian dollars.
	CodeAUD
	// CodeJPY identifies Japanese yen.
	CodeJPY
	// CodeCHF identifies Swiss francs.
	CodeCHF
	// CodeNZD identifies New Zealand dollars.
	CodeNZD
	// CodeSGD identifies Singapore dollars.
	CodeSGD
	// CodeHKD identifies Hong Kong dollars.
	CodeHKD
	// CodeBHD identifies Bahraini dinars.
	CodeBHD
	// CodeCLF identifies Chilean unidades de fomento.
	CodeCLF
	codeLimit
)

type currencyDefinition struct {
	token          string
	fractionDigits uint8
}

func currencyDefinitions() [codeLimit]currencyDefinition {
	return [...]currencyDefinition{
		CodeUSD: {token: CodeTokenUSD, fractionDigits: MinorUnitDigitsTwo},
		CodeEUR: {token: CodeTokenEUR, fractionDigits: MinorUnitDigitsTwo},
		CodeGBP: {token: CodeTokenGBP, fractionDigits: MinorUnitDigitsTwo},
		CodeCAD: {token: CodeTokenCAD, fractionDigits: MinorUnitDigitsTwo},
		CodeAUD: {token: CodeTokenAUD, fractionDigits: MinorUnitDigitsTwo},
		CodeJPY: {token: CodeTokenJPY, fractionDigits: MinorUnitDigitsZero},
		CodeCHF: {token: CodeTokenCHF, fractionDigits: MinorUnitDigitsTwo},
		CodeNZD: {token: CodeTokenNZD, fractionDigits: MinorUnitDigitsTwo},
		CodeSGD: {token: CodeTokenSGD, fractionDigits: MinorUnitDigitsTwo},
		CodeHKD: {token: CodeTokenHKD, fractionDigits: MinorUnitDigitsTwo},
		CodeBHD: {token: CodeTokenBHD, fractionDigits: MinorUnitDigitsThree},
		CodeCLF: {token: CodeTokenCLF, fractionDigits: MinorUnitDigitsFour},
	}
}

// ParseCode accepts one exact canonical currency token.
func ParseCode(token string) (Code, error) {
	// The definition table is indexed by Code, so an admitted code whose
	// definition row is missing carries the empty token. Without this gate the
	// empty token would resolve to that code and every later boundary would
	// treat it as a real currency. Core applies the same gate to its Go
	// identifier domains for the same reason.
	if token == "" {
		return CodeUnknown, contractError(currencyTokenNotAdmittedReason)
	}
	definitions := currencyDefinitions()
	for code := CodeUSD; code < codeLimit; code++ {
		if definitions[code].token == token {
			return code, nil
		}
	}
	return CodeUnknown, contractError(currencyTokenNotAdmittedReason)
}

// IsValid reports whether c belongs to the closed supported domain.
func (c Code) IsValid() bool {
	return c > CodeUnknown && c < codeLimit
}

// Validate rejects currencies outside the closed supported domain.
func (c Code) Validate() error {
	if !c.IsValid() {
		return contractError("currency code is outside the admitted domain")
	}
	return nil
}

// String returns the canonical token, or an empty string for an invalid code.
func (c Code) String() string {
	if !c.IsValid() {
		return ""
	}
	return currencyDefinitions()[c].token
}

// FractionDigits returns the currency-owned minor-unit exponent.
func (c Code) FractionDigits() (uint8, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}
	return currencyDefinitions()[c].fractionDigits, nil
}

func (c Code) fractionDigits() uint8 {
	return currencyDefinitions()[c].fractionDigits
}

// MarshalJSON emits the canonical currency token.
func (c Code) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(c.String())
}

// UnmarshalJSON accepts an admitted token and preserves the receiver on failure.
func (c *Code) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(contractError("currency code receiver is nil"))
	}
	if len(data) > CodeJSONMaximumBytes {
		return jsonError(contractError("currency code JSON exceeds its byte limit"))
	}
	var token string
	if err := json.Unmarshal(data, &token); err != nil {
		return jsonError(errors.Join(contractError("currency code JSON is invalid"), err))
	}
	parsed, err := ParseCode(token)
	if err != nil {
		return jsonError(err)
	}
	*c = parsed
	return nil
}

var _ [math.MaxUint8 - int(codeLimit)]struct{}
var _ core.Validatable = CodeUnknown
