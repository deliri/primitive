package temporal

import (
	"encoding/json"
	"math"
	"strconv"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

const (
	RFC3339MinimumTextBytes = 20
	RFC3339MaximumTextBytes = 35
)

const (
	rfc3339YearEnd       = 4
	rfc3339MonthAt       = 5
	rfc3339MonthEnd      = 7
	rfc3339DayAt         = 8
	rfc3339DayEnd        = 10
	rfc3339HourAt        = 11
	rfc3339HourEnd       = 13
	rfc3339MinuteAt      = 14
	rfc3339MinuteEnd     = 16
	rfc3339SecondAt      = 17
	rfc3339SecondEnd     = 19
	rfc3339FractionAt    = 19
	rfc3339OffsetHours   = 3
	rfc3339OffsetMinutes = 6
)

// Instant is a set signed Unix instant with nanosecond precision.
type Instant struct {
	nanoseconds int64
	set         bool
}

// ParseRFC3339 raises Go's documented RFC 3339 timestamp syntax into an exact
// signed-nanosecond Instant. The returned value is zero on every refusal.
func ParseRFC3339(value string) (Instant, error) {
	if !validRFC3339TextExtent(value) {
		return Instant{}, contractError("RFC 3339 instant text extent is invalid")
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return Instant{}, contractError("RFC 3339 instant text is invalid", err)
	}
	if !validRFC3339NanosecondText(value) {
		return Instant{}, contractError("RFC 3339 instant text is not exact nanosecond syntax")
	}
	instant, err := NewInstant(parsed)
	if err != nil {
		return Instant{}, contractError("RFC 3339 instant is outside signed nanoseconds", err)
	}
	return instant, nil
}

func validRFC3339NanosecondText(value string) bool {
	if !validRFC3339TextExtent(value) {
		return false
	}
	if !validRFC3339DateTimeShape(value) || !validRFC3339Clock(value) {
		return false
	}
	zoneAt, ok := rfc3339ZoneStart(value)
	if !ok {
		return false
	}
	return validRFC3339Zone(value[zoneAt:])
}

func validRFC3339TextExtent(value string) bool {
	return len(value) >= RFC3339MinimumTextBytes && len(value) <= RFC3339MaximumTextBytes
}

func validRFC3339DateTimeShape(value string) bool {
	if !validRFC3339Separators(value) {
		return false
	}
	return decimalText(value[:rfc3339YearEnd]) &&
		decimalText(value[rfc3339MonthAt:rfc3339MonthEnd]) &&
		decimalText(value[rfc3339DayAt:rfc3339DayEnd]) &&
		decimalText(value[rfc3339HourAt:rfc3339HourEnd]) &&
		decimalText(value[rfc3339MinuteAt:rfc3339MinuteEnd]) &&
		decimalText(value[rfc3339SecondAt:rfc3339SecondEnd])
}

func validRFC3339Separators(value string) bool {
	return value[rfc3339YearEnd] == '-' && value[rfc3339MonthEnd] == '-' && value[rfc3339DayEnd] == 'T' &&
		value[rfc3339HourEnd] == ':' && value[rfc3339MinuteEnd] == ':'
}

func validRFC3339Clock(value string) bool {
	hour := decimalPair(value[rfc3339HourAt:rfc3339HourEnd])
	minute := decimalPair(value[rfc3339MinuteAt:rfc3339MinuteEnd])
	second := decimalPair(value[rfc3339SecondAt:rfc3339SecondEnd])
	return hour <= 23 && minute <= 59 && second <= 59
}

func rfc3339ZoneStart(value string) (int, bool) {
	if value[rfc3339FractionAt] != '.' {
		return rfc3339FractionAt, true
	}
	zoneAt := rfc3339FractionAt + 1
	for zoneAt < len(value) && value[zoneAt] >= '0' && value[zoneAt] <= '9' {
		zoneAt++
	}
	fractionDigits := zoneAt - rfc3339FractionAt - 1
	return zoneAt, fractionDigits >= 1 && fractionDigits <= 9
}

func validRFC3339Zone(value string) bool {
	if value == "Z" {
		return true
	}
	if len(value) != rfc3339OffsetMinutes || value[0] != '+' && value[0] != '-' || value[rfc3339OffsetHours] != ':' {
		return false
	}
	if !decimalText(value[1:rfc3339OffsetHours]) || !decimalText(value[rfc3339OffsetHours+1:]) {
		return false
	}
	return decimalPair(value[1:rfc3339OffsetHours]) <= 23 && decimalPair(value[rfc3339OffsetHours+1:]) <= 59
}

func decimalText(value string) bool {
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func decimalPair(value string) int {
	return int(value[0]-'0')*10 + int(value[1]-'0')
}

// NewInstant projects a time.Time to exact Unix nanoseconds.
func NewInstant(value time.Time) (Instant, error) {
	nanoseconds := value.UnixNano()
	if !time.Unix(0, nanoseconds).Equal(value) {
		return Instant{}, overflowError("time is outside signed Unix nanoseconds")
	}
	return InstantFromNanoseconds(nanoseconds), nil
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

// RFC3339Nano returns the canonical UTC representation that preserves the
// Instant's exact nanosecond value.
func (i Instant) RFC3339Nano() (string, error) {
	value, err := i.Time()
	if err != nil {
		return "", err
	}
	return value.Format(time.RFC3339Nano), nil
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
