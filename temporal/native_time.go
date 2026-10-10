package temporal

import (
	"time"

	"github.com/deliri/primitive/v2026/core"
)

// NativeTimePair borrows two Go timestamps, including their monotonic readings.
// These native operations accept Go's full calendar range and signed duration
// semantics; they do not project through Instant's Unix nanosecond encoding.
type NativeTimePair struct {
	First  time.Time
	Second time.Time
}

// NativeTimeOffsetRequest carries Go's signed offset without a policy clamp.
type NativeTimeOffsetRequest struct {
	Time   time.Time
	Offset time.Duration
}

// NativeTimeFacts projects Go's zone and zero-value observations directly.
type NativeTimeFacts struct {
	ZoneName          string
	ZoneOffsetSeconds int
	Zero              bool
}

func InspectNativeTime(value time.Time) NativeTimeFacts {
	name, offset := value.Zone()
	return NativeTimeFacts{ZoneName: name, ZoneOffsetSeconds: offset, Zero: IsNativeTimeZero(value)}
}

// IsNativeTimeZero asks only Go's zero predicate, without resolving a timezone.
func IsNativeTimeZero(value time.Time) bool { return value.IsZero() }

// CompareNativeTimes preserves Go's monotonic ordering when present.
func CompareNativeTimes(request NativeTimePair) core.Comparison {
	comparison := request.First.Compare(request.Second)
	if comparison < 0 {
		return core.ComparisonLess
	}
	if comparison > 0 {
		return core.ComparisonGreater
	}
	return core.ComparisonEqual
}

// DifferenceNativeTimes returns Go's signed and saturating duration exactly.
func DifferenceNativeTimes(request NativeTimePair) time.Duration {
	return request.First.Sub(request.Second)
}

// OffsetNativeTime preserves Go's full Add behavior, including monotonic data.
func OffsetNativeTime(request NativeTimeOffsetRequest) time.Time {
	return request.Time.Add(request.Offset)
}
