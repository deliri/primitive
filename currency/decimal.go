package currency

import (
	"math"
	"strconv"
	"strings"
)

// Decimal rejection reasons are the second tier of the decimal contract. Every
// decimal rejection carries core.ErrCurrencyDecimal, so these constants are the
// only thing that tells an operator which rule fired. Each rule has exactly one
// home here; production and its tests read the same constant.
const (
	decimalLengthReason       = "currency decimal has an invalid byte length"
	decimalSignReason         = "currency decimal has an invalid sign"
	decimalWholeReason        = "currency decimal whole units are invalid"
	decimalFractionReason     = "currency decimal fraction is invalid"
	decimalNegativeZeroReason = "currency decimal does not admit negative zero"
)

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
	magnitude, err := accumulateDecimal(digits)
	if err != nil {
		return 0, err
	}
	if negative && magnitude == 0 {
		return 0, decimalError(decimalNegativeZeroReason)
	}
	return signedValue(negative, magnitude)
}

func decimalDigits(code Code, raw string) (string, bool, error) {
	if raw == "" || len(raw) > DecimalMaximumBytes {
		return "", false, decimalError(decimalLengthReason)
	}
	negative := raw[0] == '-'
	unsigned := raw
	if negative {
		unsigned = raw[1:]
	}
	if unsigned == "" || raw[0] == '+' {
		return "", false, decimalError(decimalSignReason)
	}
	// strings.Cut splits at the first separator only, so any surplus separator
	// stays inside the fraction. validateFraction is the single owner of that
	// rejection: its digit rule already refuses a separator, and duplicating the
	// check here would let one rule report the other rule's failure.
	whole, fraction, hasFraction := strings.Cut(unsigned, ".")
	if whole == "" || !asciiDigits(whole) {
		return "", false, decimalError(decimalWholeReason)
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
		return decimalError(decimalFractionReason)
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

// accumulateDecimal owns only unsigned accumulation. The signed minor-unit
// domain belongs to signedValue, so neither bound is expressed twice.
func accumulateDecimal(digits string) (uint64, error) {
	var value uint64
	for index := range len(digits) {
		digit := uint64(digits[index] - '0')
		if value > (math.MaxUint64-digit)/10 {
			return 0, overflowError()
		}
		value = value*10 + digit
	}
	return value, nil
}

func signedMagnitude(value int64) uint64 {
	if value >= 0 {
		return uint64(value)
	}
	if value == math.MinInt64 {
		return uint64(math.MaxInt64) + 1
	}
	return uint64(-value)
}

// signedValue is the single owner of the int64 minor-unit domain. Both bounds
// are reachable because accumulateDecimal stops only at the unsigned ceiling.
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
	exponent := int(a.code.fractionDigits())
	digits := strconv.FormatUint(signedMagnitude(a.minorUnits), 10)
	if exponent > 0 {
		digits = fixedExponentDecimal(digits, exponent)
	}
	if a.minorUnits < 0 {
		return "-" + digits, nil
	}
	return digits, nil
}

func fixedExponentDecimal(digits string, exponent int) string {
	if len(digits) <= exponent {
		digits = strings.Repeat("0", exponent-len(digits)+1) + digits
	}
	split := len(digits) - exponent
	return digits[:split] + "." + digits[split:]
}
