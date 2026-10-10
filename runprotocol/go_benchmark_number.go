package runprotocol

import (
	"bufio"
	"errors"
	"io"
	"math"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
)

// Go's binary64 decimal conversion retains 800 significant digits and a
// discarded-nonzero fact. Normalize into that same fixed native window before
// calling strconv: NumError must never clone an arbitrarily long input field.
const goFloatDecimalWindow = 800

func goBenchmarkInteger(source io.Reader) (int64, error) {
	input := bufio.NewReader(source)
	negative, err := goBenchmarkSign(input)
	if err != nil {
		return 0, err
	}
	var digits [19]byte
	used := 0
	seen := false
	for {
		digit, err := input.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, errors.Join(core.ErrGoToolchainOutput, err)
		}
		if digit < '0' || digit > '9' {
			return 0, core.ErrGoToolchainOutput
		}
		seen = true
		if used == 0 && digit == '0' {
			continue
		}
		if negative || used == len(digits) {
			return 0, core.ErrGoToolchainOutput
		}
		digits[used] = digit
		used++
	}
	if !seen {
		return 0, core.ErrGoToolchainOutput
	}
	if used == 0 {
		return 0, nil
	}
	value, err := strconv.ParseInt(string(digits[:used]), 10, 64)
	if err != nil {
		return 0, core.ErrGoToolchainOutput
	}
	return value, nil
}

func goBenchmarkSign(source *bufio.Reader) (bool, error) {
	first, err := source.Peek(1)
	if err != nil {
		return false, errors.Join(core.ErrGoToolchainOutput, err)
	}
	if first[0] != '+' && first[0] != '-' {
		return false, nil
	}
	_, err = source.ReadByte()
	return first[0] == '-', err
}

func goBenchmarkFloat(source io.Reader) (float64, error) {
	input := bufio.NewReader(source)
	negative, err := goBenchmarkSign(input)
	if err != nil {
		return 0, err
	}
	var digits [goFloatDecimalWindow + 1]byte
	used, position, first := 0, int64(0), int64(-1)
	decimal := int64(-1)
	discardedNonzero := false
	for {
		next, err := input.Peek(1)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, errors.Join(core.ErrGoToolchainOutput, err)
		}
		char := next[0]
		if char == '.' && decimal < 0 {
			decimal = position
			if _, err := input.ReadByte(); err != nil {
				return 0, errors.Join(core.ErrGoToolchainOutput, err)
			}
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
		if position == math.MaxInt64 {
			return 0, core.ErrGoToolchainOutput
		}
		position++
		if _, err := input.ReadByte(); err != nil {
			return 0, errors.Join(core.ErrGoToolchainOutput, err)
		}
	}
	if position == 0 {
		return 0, core.ErrGoToolchainOutput
	}
	if decimal < 0 {
		decimal = position
	}
	exponent, exponentRange, err := goBenchmarkExponent(input)
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
	offset := decimal - first - 1
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

func goBenchmarkExponent(source *bufio.Reader) (int64, goBenchmarkExponentRange, error) {
	char, err := source.ReadByte()
	if err == io.EOF {
		return 0, goBenchmarkExponentRepresentable, nil
	}
	if err != nil {
		return 0, goBenchmarkExponentRepresentable, errors.Join(core.ErrGoToolchainOutput, err)
	}
	if char != 'e' && char != 'E' {
		return 0, goBenchmarkExponentRepresentable, core.ErrGoToolchainOutput
	}
	negative, err := goBenchmarkSign(source)
	if err != nil {
		return 0, goBenchmarkExponentRepresentable, err
	}
	var value int64
	overflow := false
	seen := false
	for {
		char, err := source.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, goBenchmarkExponentRepresentable, errors.Join(core.ErrGoToolchainOutput, err)
		}
		if char < '0' || char > '9' {
			return 0, goBenchmarkExponentRepresentable, core.ErrGoToolchainOutput
		}
		seen = true
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
	if !seen {
		return 0, goBenchmarkExponentRepresentable, core.ErrGoToolchainOutput
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
