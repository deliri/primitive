package currency

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

// Decimal rejection reasons are the second tier of the decimal contract. Every
// decimal rejection carries core.ErrCurrencyDecimal, so these constants are the
// only thing that tells an operator which rule fired. Each rule has exactly one
// home here; production and its tests read the same constant.
type decimalRejection uint8

const (
	decimalRejectionUnknown decimalRejection = iota
	decimalRejectionLength
	decimalRejectionSign
	decimalRejectionWhole
	decimalRejectionFraction
	decimalRejectionNegativeZero
	decimalRejectionMinorUnitsUnset
	decimalRejectionReceiverNil
	decimalRejectionJSONString
	decimalRejectionCanonicalInt64
	decimalRejectionLimit
)

func decimalRejectionDiagnostics() [decimalRejectionLimit]string {
	return [...]string{
		"unknown currency decimal rejection",
		"currency decimal has an invalid byte length",
		"currency decimal has an invalid sign",
		"currency decimal whole units are invalid",
		"currency decimal fraction is invalid",
		"currency decimal does not admit negative zero",
		"currency minor units are unset",
		"currency minor-unit receiver is nil",
		"currency minor units must be a JSON string",
		"currency minor units are not a canonical int64",
	}
}

func (r decimalRejection) Error() string {
	if r <= decimalRejectionUnknown || r >= decimalRejectionLimit {
		return decimalRejectionDiagnostics()[decimalRejectionUnknown]
	}
	return decimalRejectionDiagnostics()[r]
}

// Parse constructs an exact amount from a bounded decimal representation.
func Parse(code Code, decimal string) (Amount, error) {
	if err := code.Validate(); err != nil {
		return Amount{}, err
	}
	minorUnits, err := parseDecimal(code, decimal)
	if err != nil {
		return Amount{}, err
	}
	return Amount{minorUnits: minorUnits, code: code}, nil
}

func parseDecimal(code Code, raw string) (int64, error) {
	digits, negative, err := decimalDigits(code, raw)
	if err != nil {
		return 0, err
	}
	// decimalDigits has already established the bounded decimal grammar.
	// Go owns unsigned conversion; this package owns the signed currency bound.
	magnitude, err := parseDecimalMagnitude(digits)
	if err != nil {
		return 0, err
	}
	if negative && magnitude == 0 {
		return 0, decimalError(decimalRejectionNegativeZero)
	}
	return signedValue(negative, magnitude)
}

// parseDecimalMagnitude owns the Go conversion and its error classification.
// The decimal grammar is checked by decimalDigits before this boundary.
func parseDecimalMagnitude(digits string) (uint64, error) {
	value, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return 0, errors.Join(overflowError(), err)
		}
		return 0, errors.Join(core.ErrCurrencyDecimal, err)
	}
	return value, nil
}

func decimalDigits(code Code, raw string) (string, bool, error) {
	if raw == "" || len(raw) > DecimalMaximumBytes {
		return "", false, decimalError(decimalRejectionLength)
	}
	negative := raw[0] == '-'
	unsigned := raw
	if negative {
		unsigned = raw[1:]
	}
	if unsigned == "" || raw[0] == '+' {
		return "", false, decimalError(decimalRejectionSign)
	}
	// strings.Cut splits at the first separator only, so any surplus separator
	// stays inside the fraction. validateFraction is the single owner of that
	// rejection: its digit rule already refuses a separator, and duplicating the
	// check here would let one rule report the other rule's failure.
	whole, fraction, hasFraction := strings.Cut(unsigned, ".")
	if whole == "" || !asciiDigits(whole) {
		return "", false, decimalError(decimalRejectionWhole)
	}
	exponent := code.fractionDigits()
	if err := validateFraction(fraction, hasFraction, exponent); err != nil {
		return "", false, err
	}
	return whole + fraction + strings.Repeat("0", int(exponent)-len(fraction)), negative, nil
}

func validateFraction(fraction string, present bool, exponent uint8) error {
	if !present {
		return nil
	}
	if exponent == MinorUnitDigitsZero || fraction == "" ||
		len(fraction) > int(exponent) || !asciiDigits(fraction) {
		return decimalError(decimalRejectionFraction)
	}
	return nil
}

func asciiDigits(value string) bool {
	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

// signedValue is the single owner of the int64 minor-unit domain. Both bounds
// are reachable because strconv.ParseUint stops at the unsigned ceiling.
func signedValue(negative bool, magnitude uint64) (int64, error) {
	if !negative {
		if magnitude > math.MaxInt64 {
			return 0, overflowError()
		}
		return int64(magnitude), nil
	}
	if magnitude == uint64(math.MaxInt64)+1 {
		return math.MinInt64, nil
	}
	if magnitude > math.MaxInt64 {
		return 0, overflowError()
	}
	return -int64(magnitude), nil
}

// Decimal returns the exact fixed-exponent decimal representation.
func (a Amount) Decimal() (string, error) {
	if err := a.Validate(); err != nil {
		return "", err
	}
	var integer [DecimalMaximumBytes]byte
	digits := strconv.AppendInt(integer[:0], a.minorUnits, 10)
	exponent := int(a.code.fractionDigits())
	if exponent == 0 {
		return string(digits), nil
	}
	var decimal [DecimalMaximumBytes]byte
	output := decimal[:0]
	if digits[0] == '-' {
		output = append(output, '-')
		digits = digits[1:]
	}
	if len(digits) <= exponent {
		output = append(output, '0', '.')
		for range exponent - len(digits) {
			output = append(output, '0')
		}
		output = append(output, digits...)
		return string(output), nil
	}
	split := len(digits) - exponent
	output = append(output, digits[:split]...)
	output = append(output, '.')
	output = append(output, digits[split:]...)
	return string(output), nil
}
