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

// DifferenceNativeTimes returns First minus Second, preserving Go's monotonic
// readings, signed duration and saturation at the native duration limits.
func DifferenceNativeTimes(request NativeTimePair) time.Duration {
	return request.First.Sub(request.Second)
}

// NativeTimeOffset borrows a Go timestamp and a signed native duration. Zero
// timestamps, negative offsets and the full Go calendar range remain valid.
type NativeTimeOffset struct {
	Time   time.Time
	Offset time.Duration
}

// ShiftNativeTime preserves Go's location, monotonic reading and native overflow
// behavior. It does not narrow the timestamp to Unix nanosecond encoding.
func ShiftNativeTime(request NativeTimeOffset) time.Time {
	return request.Time.Add(request.Offset)
}
