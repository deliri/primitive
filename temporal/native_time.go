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
