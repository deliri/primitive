package temporal

import (
	"math"
	"math/bits"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
)

const decimalDigits = "0123456789"

// AggregateDuration is an unsigned 128-bit nanosecond accumulator.
type AggregateDuration struct {
	high uint64
	low  uint64
}

// AggregateDurationFromNanoseconds constructs an unsigned nanosecond total.
func AggregateDurationFromNanoseconds(nanoseconds uint64) AggregateDuration {
	return AggregateDuration{low: nanoseconds}
}

// AggregateDurationFromDuration widens d without loss. Duration owns the
// nonnegative rule, so this reports that rule's failure rather than mapping a
// negative duration to zero. Silently returning the zero aggregate would turn a
// package-internal Duration{nanoseconds: negative} into lost time at the one
// boundary that is supposed to preserve it exactly.
func AggregateDurationFromDuration(d Duration) (AggregateDuration, error) {
	if err := d.Validate(); err != nil {
		return AggregateDuration{}, err
	}
	// #nosec G115 -- Validate proves the stored signed nanoseconds are nonnegative.
	return AggregateDuration{low: uint64(d.nanoseconds)}, nil
}

// ParseAggregateDuration accepts canonical unsigned base-10 nanoseconds.
func ParseAggregateDuration(decimal string) (AggregateDuration, error) {
	return parseAggregateDecimal(decimal)
}

// parseAggregateDecimal lets persistence parse the canonical decimal bytes in
// place, without first copying them into a string.
func parseAggregateDecimal[Text ~string | ~[]byte](decimal Text) (AggregateDuration, error) {
	if !canonicalUnsignedDecimal(decimal) {
		return AggregateDuration{}, contractError("aggregate duration decimal is not canonical")
	}
	// Fold up to nineteen digits into one word, then scale the accumulator
	// once per word. Every prefix is at most the final value, so a step
	// overflows exactly when the whole decimal exceeds 128 bits.
	var value AggregateDuration
	for start := 0; start < len(decimal); start += aggregateDecimalChunkDigits {
		end := min(start+aggregateDecimalChunkDigits, len(decimal))
		var word uint64
		scale := uint64(1)
		for index := start; index < end; index++ {
			word = word*10 + uint64(decimal[index]-'0')
			scale *= 10
		}
		next, err := value.multiplyAdd(scale, word)
		if err != nil {
			return AggregateDuration{}, err
		}
		value = next
	}
	return value, nil
}

func (a AggregateDuration) multiplyAdd(multiplier, addend uint64) (AggregateDuration, error) {
	scaled, err := a.Multiply(multiplier)
	if err != nil {
		return AggregateDuration{}, err
	}
	low, carry := bits.Add64(scaled.low, addend, 0)
	high, overflow := bits.Add64(scaled.high, 0, carry)
	if overflow != 0 {
		return AggregateDuration{}, overflowError("aggregate duration exceeded unsigned 128 bits")
	}
	return AggregateDuration{high: high, low: low}, nil
}

// Validate accepts every bit pattern in the unsigned 128-bit domain.
func (AggregateDuration) Validate() error {
	return nil
}

// IsZero reports whether no nanoseconds have accumulated.
func (a AggregateDuration) IsZero() bool {
	return a.high == 0 && a.low == 0
}

// Decimal returns canonical unsigned base-10 nanoseconds.
func (a AggregateDuration) Decimal() string {
	if a.IsZero() {
		return "0"
	}
	var buffer [AggregateDurationMaximumDecimalDigits]byte
	return string(a.appendDecimal(buffer[:0]))
}

// aggregateDecimalChunk is the largest power of ten below 2^64. Splitting the
// 128-bit value into base-10^19 limbs lets strconv format each word instead of
// performing one 128-bit division per digit.
const (
	aggregateDecimalChunk       = 10_000_000_000_000_000_000
	aggregateDecimalChunkDigits = 19
)

// appendDecimal appends the canonical unsigned base-10 spelling of a.
func (a AggregateDuration) appendDecimal(destination []byte) []byte {
	if a.high == 0 {
		return strconv.AppendUint(destination, a.low, 10)
	}
	upper, lowest := a.divide(aggregateDecimalChunk)
	if upper.high == 0 {
		destination = strconv.AppendUint(destination, upper.low, 10)
	} else {
		// 2^128 < 10^57, so the top limb is a single small word.
		top, middle := upper.divide(aggregateDecimalChunk)
		destination = strconv.AppendUint(destination, top.low, 10)
		destination = appendDecimalChunk(destination, middle)
	}
	return appendDecimalChunk(destination, lowest)
}

// appendDecimalChunk appends value as exactly nineteen zero-padded digits.
func appendDecimalChunk(destination []byte, value uint64) []byte {
	var digits [aggregateDecimalChunkDigits]byte
	for index := len(digits) - 1; index >= 0; index-- {
		digits[index] = decimalDigits[value%10]
		value /= 10
	}
	return append(destination, digits[:]...)
}

// Add returns an exact aggregate sum.
func (a AggregateDuration) Add(other AggregateDuration) (AggregateDuration, error) {
	low, carry := bits.Add64(a.low, other.low, 0)
	high, overflow := bits.Add64(a.high, other.high, carry)
	if overflow != 0 {
		return AggregateDuration{}, overflowError("aggregate duration addition exceeded unsigned 128 bits")
	}
	return AggregateDuration{high: high, low: low}, nil
}

// Subtract returns the exact nonnegative aggregate difference. Underflow
// refuses the operation rather than wrapping or erasing elapsed time.
func (a AggregateDuration) Subtract(other AggregateDuration) (AggregateDuration, error) {
	low, borrow := bits.Sub64(a.low, other.low, 0)
	high, underflow := bits.Sub64(a.high, other.high, borrow)
	if underflow != 0 {
		return AggregateDuration{}, overflowError("aggregate duration subtraction became negative")
	}
	return AggregateDuration{high: high, low: low}, nil
}

// AddDuration adds a bounded duration.
func (a AggregateDuration) AddDuration(duration Duration) (AggregateDuration, error) {
	widened, err := AggregateDurationFromDuration(duration)
	if err != nil {
		return AggregateDuration{}, err
	}
	return a.Add(widened)
}

// Multiply scales a by multiplier.
func (a AggregateDuration) Multiply(multiplier uint64) (AggregateDuration, error) {
	if multiplier == 0 || a.IsZero() {
		return AggregateDuration{}, nil
	}
	highCarry, low := bits.Mul64(a.low, multiplier)
	overflow, highBase := bits.Mul64(a.high, multiplier)
	high, carry := bits.Add64(highBase, highCarry, 0)
	if overflow != 0 || carry != 0 {
		return AggregateDuration{}, overflowError("aggregate duration multiplication exceeded unsigned 128 bits")
	}
	return AggregateDuration{high: high, low: low}, nil
}

// Compare orders two aggregate durations.
func (a AggregateDuration) Compare(other AggregateDuration) core.Comparison {
	switch {
	case a.high < other.high:
		return core.ComparisonLess
	case a.high > other.high:
		return core.ComparisonGreater
	case a.low < other.low:
		return core.ComparisonLess
	case a.low > other.low:
		return core.ComparisonGreater
	default:
		return core.ComparisonEqual
	}
}

// Duration narrows a to a bounded duration when possible.
func (a AggregateDuration) Duration() (Duration, error) {
	if a.high != 0 || a.low > math.MaxInt64 {
		return Duration{}, overflowError("aggregate duration does not fit bounded duration")
	}
	return Duration{nanoseconds: int64(a.low)}, nil
}

// MarshalJSON emits canonical decimal nanoseconds as a JSON string.
func (a AggregateDuration) MarshalJSON() ([]byte, error) {
	// Decimal digits need no JSON escaping, so the quoted spelling is the
	// canonical JSON string encoding/json would emit for Decimal().
	wire := make([]byte, 0, AggregateDurationCanonicalJSONMaximumBytes)
	wire = append(wire, '"')
	wire = a.appendDecimal(wire)
	return append(wire, '"'), nil
}

// UnmarshalJSON accepts canonical decimal nanoseconds without mutation on
// error.
func (a *AggregateDuration) UnmarshalJSON(data []byte) error {
	if a == nil {
		return jsonContractError("aggregate duration receiver is nil")
	}
	decimal, err := decodeNanosecondJSON(data, AggregateDurationJSONMaximumBytes)
	if err != nil {
		return err
	}
	parsed, err := parseAggregateDecimal(decimal)
	if err != nil {
		return jsonContractError(temporalJSONValueInvalidReason, err)
	}
	*a = parsed
	return nil
}

func (a AggregateDuration) divide(divisor uint64) (AggregateDuration, uint64) {
	high, remainder := bits.Div64(0, a.high, divisor)
	low, remainder := bits.Div64(remainder, a.low, divisor)
	return AggregateDuration{high: high, low: low}, remainder
}

var _ core.ValidatedJSONMarshaler = AggregateDuration{}
