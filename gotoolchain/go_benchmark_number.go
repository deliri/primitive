package gotoolchain

import (
	"github.com/deliri/primitive/v2026/core"
	"math"
	"strconv"
)

// Go's binary64 decimal conversion retains 800 significant digits and a
// discarded-nonzero fact. Normalize into that same fixed native window before
// calling strconv: NumError must never clone an arbitrarily long input field.
const goFloatDecimalWindow = 800

func goBenchmarkInteger(source []byte) (int64, error) {
	negative := false
	if len(source) != 0 && (source[0] == '+' || source[0] == '-') {
		negative = source[0] == '-'
		source = source[1:]
	}
	if len(source) == 0 {
		return 0, core.ErrGoToolchainOutput
	}
	first := len(source)
	for index, digit := range source {
		if digit < '0' || digit > '9' {
			return 0, core.ErrGoToolchainOutput
		}
		if first == len(source) && digit != '0' {
			first = index
		}
	}
	if first == len(source) {
		return 0, nil
	}
	if negative || len(source)-first > 19 {
		return 0, core.ErrGoToolchainOutput
	}
	value, err := strconv.ParseInt(string(source[first:]), 10, 64)
	if err != nil {
		return 0, core.ErrGoToolchainOutput
	}
	return value, nil
}

func goBenchmarkFloat(source []byte) (float64, error) {
	negative := false
	if len(source) != 0 && (source[0] == '+' || source[0] == '-') {
		negative = source[0] == '-'
		source = source[1:]
	}
	var digits [goFloatDecimalWindow + 1]byte
	used, position, first := 0, 0, -1
	decimal := -1
	index := 0
	discardedNonzero := false
	for index < len(source) {
		char := source[index]
		if char == '.' && decimal < 0 {
			decimal = position
			index++
			continue
		}
		if char < '0' || char > '9' {
			break
		}
		if first < 0 && char != '0' {
			first = position
		}
		if first >= 0 {
			if used < goFloatDecimalWindow {
				digits[used] = char
				used++
			} else if char != '0' {
				discardedNonzero = true
			}
		}
		position++
		index++
	}
	if position == 0 {
		return 0, core.ErrGoToolchainOutput
	}
	if decimal < 0 {
		decimal = position
	}
	exponent, exponentRange, err := goBenchmarkExponent(source[index:])
	if err != nil {
		return 0, err
	}
	if first < 0 {
		return signedGoBenchmarkZero(negative), nil
	}
	if exponentRange == goBenchmarkExponentOverflow {
		return 0, core.ErrGoToolchainOutput
	}
	if exponentRange == goBenchmarkExponentUnderflow {
		return signedGoBenchmarkZero(negative), nil
	}
	offset := int64(decimal - first - 1)
	if (exponent > 0 && offset > math.MaxInt64-exponent) || (exponent < 0 && offset < math.MinInt64-exponent) {
		if exponent < 0 {
			return signedGoBenchmarkZero(negative), nil
		}
		return 0, core.ErrGoToolchainOutput
	}
	scientific := offset + exponent
	if scientific > 308 {
		return 0, core.ErrGoToolchainOutput
	}
	if scientific < -324 {
		return signedGoBenchmarkZero(negative), nil
	}
	if discardedNonzero {
		digits[used] = '1'
		used++
	}
	var normalized [goFloatDecimalWindow + 32]byte
	length := 0
	if negative {
		normalized[length] = '-'
		length++
	}
	normalized[length] = digits[0]
	length++
	normalized[length] = '.'
	length++
	length += copy(normalized[length:], digits[1:used])
	normalized[length] = 'e'
	length++
	encoded := strconv.AppendInt(normalized[:length], scientific, 10)
	value, err := strconv.ParseFloat(string(encoded), 64)
	if err != nil || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, core.ErrGoToolchainOutput
	}
	return value, nil
}

type goBenchmarkExponentRange uint8

const (
	goBenchmarkExponentRepresentable goBenchmarkExponentRange = iota
	goBenchmarkExponentOverflow
	goBenchmarkExponentUnderflow
)

func goBenchmarkExponent(source []byte) (int64, goBenchmarkExponentRange, error) {
	if len(source) == 0 {
		return 0, goBenchmarkExponentRepresentable, nil
	}
	if source[0] != 'e' && source[0] != 'E' {
		return 0, goBenchmarkExponentRepresentable, core.ErrGoToolchainOutput
	}
	source = source[1:]
	negative := false
	if len(source) != 0 && (source[0] == '+' || source[0] == '-') {
		negative = source[0] == '-'
		source = source[1:]
	}
	if len(source) == 0 {
		return 0, goBenchmarkExponentRepresentable, core.ErrGoToolchainOutput
	}
	var value int64
	overflow := false
	for _, char := range source {
		if char < '0' || char > '9' {
			return 0, goBenchmarkExponentRepresentable, core.ErrGoToolchainOutput
		}
		if overflow {
			continue
		}
		digit := int64(char - '0')
		if value > (math.MaxInt64-digit)/10 {
			overflow = true
			continue
		}
		value = value*10 + digit
	}
	if overflow {
		if negative {
			return 0, goBenchmarkExponentUnderflow, nil
		}
		return 0, goBenchmarkExponentOverflow, nil
	}
	if negative {
		value = -value
	}
	return value, goBenchmarkExponentRepresentable, nil
}

func signedGoBenchmarkZero(negative bool) float64 {
	if negative {
		return math.Copysign(0, -1)
	}
	return 0
}
