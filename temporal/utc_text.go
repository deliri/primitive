package temporal

import (
	"strings"
	"time"
)

const compactUTCLayout = "20060102T150405Z"

// CompactUTCTextBytes is the exact extent of a basic ISO 8601 UTC timestamp
// with second precision: YYYYMMDDTHHMMSSZ.
const CompactUTCTextBytes = len(compactUTCLayout)

// ParseCompactUTC admits one exact second-precision basic ISO 8601 UTC
// timestamp. Go owns calendar parsing; Instant owns representable extent.
func ParseCompactUTC(value string) (Instant, error) {
	if len(value) != CompactUTCTextBytes {
		return Instant{}, contractError("compact UTC timestamp extent is invalid")
	}
	parsed, err := time.Parse(compactUTCLayout, value)
	if err != nil {
		return Instant{}, contractError("compact UTC timestamp is invalid", err)
	}
	var canonical [CompactUTCTextBytes]byte
	if string(parsed.AppendFormat(canonical[:0], compactUTCLayout)) != value {
		return Instant{}, contractError("compact UTC timestamp is not canonical")
	}
	instant, err := NewInstant(parsed)
	if err != nil {
		return Instant{}, contractError("compact UTC timestamp is outside signed nanoseconds", err)
	}
	return instant, nil
}

// CompactUTC emits exact second-precision basic ISO 8601 UTC text. Fractional
// instants are refused rather than silently losing nanoseconds.
func (i Instant) CompactUTC() (string, error) {
	value, err := i.Time()
	if err != nil {
		return "", err
	}
	if value.Nanosecond() != 0 {
		return "", contractError("compact UTC timestamp cannot represent fractional seconds")
	}
	return value.Format(compactUTCLayout), nil
}

// ParseRFC3339UTC admits exact RFC 3339 text only when its declared offset is
// zero. ParseRFC3339 owns syntax and extent before offset spelling is inspected.
func ParseRFC3339UTC(value string) (Instant, error) {
	instant, err := ParseRFC3339(value)
	if err != nil {
		return Instant{}, err
	}
	if !strings.HasSuffix(value, "Z") && !strings.HasSuffix(value, "+00:00") && !strings.HasSuffix(value, "-00:00") {
		return Instant{}, contractError("RFC 3339 timestamp offset is not UTC")
	}
	return instant, nil
}
