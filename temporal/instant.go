package temporal

import (
	"encoding/json"
	"math"
	"strconv"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

const (
	minimumInstantSeconds     = int64(math.MinInt64 / int64(NanosecondsPerSecond))
	minimumInstantNanoseconds = int64(math.MinInt64 % int64(NanosecondsPerSecond))
	maximumInstantSeconds     = int64(math.MaxInt64 / int64(NanosecondsPerSecond))
	maximumInstantNanoseconds = int64(math.MaxInt64 % int64(NanosecondsPerSecond))
)

// Instant is a set signed Unix instant with nanosecond precision.
type Instant struct {
	nanoseconds int64
	set         bool
}

// NewInstant projects a time.Time to exact Unix nanoseconds.
func NewInstant(value time.Time) (Instant, error) {
	seconds := value.Unix()
	nanoseconds := int64(value.Nanosecond())
	if !instantPartsRepresentable(seconds, nanoseconds) {
		return Instant{}, overflowError("time is outside signed Unix nanoseconds")
	}
	return InstantFromNanoseconds(seconds*int64(NanosecondsPerSecond) + nanoseconds), nil
}

func instantPartsRepresentable(seconds, nanoseconds int64) bool {
	switch {
	case seconds < minimumInstantSeconds-1, seconds > maximumInstantSeconds:
		return false
	case seconds == minimumInstantSeconds-1:
		return nanoseconds >= int64(NanosecondsPerSecond)+minimumInstantNanoseconds
	case seconds == maximumInstantSeconds:
		return nanoseconds <= maximumInstantNanoseconds
	default:
		return true
	}
}

// InstantFromNanoseconds constructs an exact signed Unix instant.
func InstantFromNanoseconds(nanoseconds int64) Instant {
	return Instant{nanoseconds: nanoseconds, set: true}
}

// Validate rejects the unavoidable unset Go zero value.
func (i Instant) Validate() error {
	if !i.set {
		return contractError("instant is unset")
	}
	return nil
}

// IsSet reports whether i crossed a constructor or decode boundary.
func (i Instant) IsSet() bool {
	return i.set
}

// Nanoseconds returns exact signed Unix nanoseconds.
func (i Instant) Nanoseconds() (int64, error) {
	if err := i.Validate(); err != nil {
		return 0, err
	}
	return i.nanoseconds, nil
}

// Time projects i to a UTC time.Time without a monotonic reading.
func (i Instant) Time() (time.Time, error) {
	if err := i.Validate(); err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, i.nanoseconds).UTC(), nil
}

// RFC3339 returns the canonical UTC second-precision representation used by
// human-facing protocols that require RFC 3339 text.
func (i Instant) RFC3339() (string, error) {
	value, err := i.Time()
	if err != nil {
		return "", err
	}
	return value.Format(time.RFC3339), nil
}

// Add returns i advanced by duration.
func (i Instant) Add(duration Duration) (Instant, error) {
	if err := i.Validate(); err != nil {
		return Instant{}, err
	}
	if err := duration.Validate(); err != nil {
		return Instant{}, err
	}
	if i.nanoseconds > math.MaxInt64-duration.nanoseconds {
		return Instant{}, overflowError("instant addition exceeded signed Unix nanoseconds")
	}
	return InstantFromNanoseconds(i.nanoseconds + duration.nanoseconds), nil
}

// Subtract returns i moved backward by duration.
func (i Instant) Subtract(duration Duration) (Instant, error) {
	if err := i.Validate(); err != nil {
		return Instant{}, err
	}
	if err := duration.Validate(); err != nil {
		return Instant{}, err
	}
	if i.nanoseconds < math.MinInt64+duration.nanoseconds {
		return Instant{}, overflowError("instant subtraction exceeded signed Unix nanoseconds")
	}
	return InstantFromNanoseconds(i.nanoseconds - duration.nanoseconds), nil
}

// Since returns the nonnegative elapsed duration since earlier.
func (i Instant) Since(earlier Instant) (Duration, error) {
	if err := i.Validate(); err != nil {
		return Duration{}, err
	}
	if err := earlier.Validate(); err != nil {
		return Duration{}, err
	}
	if i.nanoseconds < earlier.nanoseconds {
		return Duration{}, contractError("instant precedes the requested earlier instant")
	}
	// The difference can only exceed int64 when earlier is negative; with a
	// nonnegative earlier and i >= earlier it is bounded by i. The sign test
	// comes first so that MaxInt64+earlier is evaluated only where it cannot
	// wrap, which keeps the guard readable without relying on wraparound.
	if earlier.nanoseconds < 0 && i.nanoseconds > math.MaxInt64+earlier.nanoseconds {
		return Duration{}, overflowError("instant difference exceeded bounded duration")
	}
	return Duration{nanoseconds: i.nanoseconds - earlier.nanoseconds}, nil
}

// Compare orders two set instants.
func (i Instant) Compare(other Instant) (core.Comparison, error) {
	if err := i.Validate(); err != nil {
		return core.ComparisonUnknown, err
	}
	if err := other.Validate(); err != nil {
		return core.ComparisonUnknown, err
	}
	return compareInt64(i.nanoseconds, other.nanoseconds), nil
}

// Truncate returns i at the preceding boundary of precision.
func (i Instant) Truncate(precision Precision) (Instant, error) {
	if err := i.Validate(); err != nil {
		return Instant{}, err
	}
	unit, err := precision.nanoseconds()
	if err != nil {
		return Instant{}, err
	}
	remainder := i.nanoseconds % unit
	if remainder >= 0 {
		return InstantFromNanoseconds(i.nanoseconds - remainder), nil
	}
	adjustment := unit + remainder
	if i.nanoseconds < math.MinInt64+adjustment {
		return Instant{}, overflowError("instant truncation exceeded signed Unix nanoseconds")
	}
	return InstantFromNanoseconds(i.nanoseconds - adjustment), nil
}

// MarshalJSON emits exact nanoseconds as a canonical JSON string.
func (i Instant) MarshalJSON() ([]byte, error) {
	nanoseconds, err := i.Nanoseconds()
	if err != nil {
		return nil, err
	}
	return json.Marshal(strconv.FormatInt(nanoseconds, 10))
}

// UnmarshalJSON accepts canonical signed nanoseconds without mutation on error.
func (i *Instant) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonContractError("instant receiver is nil")
	}
	decimal, err := decodeNanosecondJSON(data, InstantJSONMaximumBytes)
	if err != nil {
		return err
	}
	nanoseconds, err := parseSignedNanoseconds(decimal)
	if err != nil {
		return err
	}
	*i = InstantFromNanoseconds(nanoseconds)
	return nil
}

func compareInt64(left, right int64) core.Comparison {
	switch {
	case left < right:
		return core.ComparisonLess
	case left > right:
		return core.ComparisonGreater
	default:
		return core.ComparisonEqual
	}
}

var _ core.ValidatedJSONMarshaler = Instant{}
