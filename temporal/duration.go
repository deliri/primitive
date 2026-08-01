package temporal

import (
	"encoding/json"
	"math"
	"strconv"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

const durationNegativeReason = "duration is negative"

// Duration is nonnegative elapsed time bounded by signed 64-bit nanoseconds.
type Duration struct {
	nanoseconds int64
}

// NewDuration constructs a bounded duration from a standard-library duration.
func NewDuration(value time.Duration) (Duration, error) {
	return DurationFromNanoseconds(int64(value))
}

// ParseDuration raises Go's documented duration syntax into a validated,
// nonnegative compiler-owned duration.
func ParseDuration(value string) (Duration, error) {
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return Duration{}, contractError("duration text is invalid", err)
	}
	return NewDuration(parsed)
}

// DurationFromNanoseconds constructs exact nonnegative nanoseconds.
func DurationFromNanoseconds(nanoseconds int64) (Duration, error) {
	if nanoseconds < 0 {
		return Duration{}, contractError(durationNegativeReason)
	}
	return Duration{nanoseconds: nanoseconds}, nil
}

// DurationFromMicroseconds constructs exact microseconds.
func DurationFromMicroseconds(value uint64) (Duration, error) {
	return durationFromMagnitude(value, NanosecondsPerMicrosecond)
}

// DurationFromMilliseconds constructs exact milliseconds.
func DurationFromMilliseconds(value uint64) (Duration, error) {
	return durationFromMagnitude(value, NanosecondsPerMillisecond)
}

// DurationFromSeconds constructs exact seconds.
func DurationFromSeconds(value uint64) (Duration, error) {
	return durationFromMagnitude(value, NanosecondsPerSecond)
}

// DurationFromMinutes constructs exact minutes.
func DurationFromMinutes(value uint64) (Duration, error) {
	return durationFromMagnitude(value, NanosecondsPerMinute)
}

// DurationFromHours constructs exact hours.
func DurationFromHours(value uint64) (Duration, error) {
	return durationFromMagnitude(value, NanosecondsPerHour)
}

// DurationFromDays constructs exact 24-hour days.
func DurationFromDays(value uint64) (Duration, error) {
	return durationFromMagnitude(value, NanosecondsPerDay)
}

// Validate rejects negative durations.
func (d Duration) Validate() error {
	if d.nanoseconds < 0 {
		return contractError(durationNegativeReason)
	}
	return nil
}

// Nanoseconds returns exact elapsed nanoseconds.
func (d Duration) Nanoseconds() int64 {
	return d.nanoseconds
}

// Stdlib projects d to time.Duration.
func (d Duration) Stdlib() (time.Duration, error) {
	if err := d.Validate(); err != nil {
		return 0, err
	}
	return time.Duration(d.nanoseconds), nil
}

// IsZero reports whether no time elapsed.
func (d Duration) IsZero() bool {
	return d.nanoseconds == 0
}

// Add returns the exact sum.
func (d Duration) Add(other Duration) (Duration, error) {
	if err := d.Validate(); err != nil {
		return Duration{}, err
	}
	if err := other.Validate(); err != nil {
		return Duration{}, err
	}
	if other.nanoseconds > math.MaxInt64-d.nanoseconds {
		return Duration{}, overflowError("duration addition exceeded int64 nanoseconds")
	}
	return Duration{nanoseconds: d.nanoseconds + other.nanoseconds}, nil
}

// Subtract returns the exact nonnegative difference.
func (d Duration) Subtract(other Duration) (Duration, error) {
	if err := d.Validate(); err != nil {
		return Duration{}, err
	}
	if err := other.Validate(); err != nil {
		return Duration{}, err
	}
	if other.nanoseconds > d.nanoseconds {
		return Duration{}, overflowError("duration subtraction became negative")
	}
	return Duration{nanoseconds: d.nanoseconds - other.nanoseconds}, nil
}

// Multiply scales d by multiplier.
func (d Duration) Multiply(multiplier uint64) (Duration, error) {
	if err := d.Validate(); err != nil {
		return Duration{}, err
	}
	if d.nanoseconds == 0 || multiplier == 0 {
		return Duration{}, nil
	}
	scalar, err := core.CheckedInt64FromUint64(multiplier)
	if err != nil || d.nanoseconds > math.MaxInt64/scalar {
		return Duration{}, overflowError("duration multiplication exceeded int64 nanoseconds")
	}
	return Duration{nanoseconds: d.nanoseconds * scalar}, nil
}

// Compare orders two durations.
func (d Duration) Compare(other Duration) (core.Comparison, error) {
	if err := d.Validate(); err != nil {
		return core.ComparisonUnknown, err
	}
	if err := other.Validate(); err != nil {
		return core.ComparisonUnknown, err
	}
	return compareInt64(d.nanoseconds, other.nanoseconds), nil
}

// Aggregate widens d without loss.
func (d Duration) Aggregate() (AggregateDuration, error) {
	return AggregateDurationFromDuration(d)
}

// MarshalJSON emits exact nanoseconds as a canonical JSON string.
func (d Duration) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(strconv.FormatInt(d.nanoseconds, 10))
}

// UnmarshalJSON accepts canonical nonnegative nanoseconds without mutation on
// error.
func (d *Duration) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonContractError("duration receiver is nil")
	}
	decimal, err := decodeNanosecondJSON(data, DurationJSONMaximumBytes)
	if err != nil {
		return err
	}
	nanoseconds, err := parseSignedNanoseconds(decimal)
	if err != nil {
		return err
	}
	parsed, err := DurationFromNanoseconds(nanoseconds)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

var _ core.ValidatedJSONMarshaler = Duration{}
