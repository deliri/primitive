package temporal

import (
	json "encoding/json/v2"
	"math"
	"math/bits"

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
	if !canonicalUnsignedDecimal(decimal) {
		return AggregateDuration{}, contractError("aggregate duration decimal is not canonical")
	}
	var value AggregateDuration
	for index := range len(decimal) {
		next, err := value.multiplyAddDecimal(decimal[index] - '0')
		if err != nil {
			return AggregateDuration{}, err
		}
		value = next
	}
	return value, nil
}

func (a AggregateDuration) multiplyAddDecimal(digit byte) (AggregateDuration, error) {
	scaled, err := a.Multiply(10)
	if err != nil {
		return AggregateDuration{}, err
	}
	low, carry := bits.Add64(scaled.low, uint64(digit), 0)
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
	index := len(buffer)
	current := a
	for !current.IsZero() {
		var remainder uint64
		current, remainder = current.divide(10)
		index--
		buffer[index] = decimalDigits[remainder]
	}
	return string(buffer[index:])
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
	return json.Marshal(a.Decimal())
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
	parsed, err := ParseAggregateDuration(decimal)
	if err != nil {
		return err
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
