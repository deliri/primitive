package temporal

import "time"

// InstantFromUnixSeconds admits an exact whole-second Unix timestamp.
// NewInstant owns the signed-nanosecond representation and overflow refusal.
func InstantFromUnixSeconds(seconds int64) (Instant, error) {
	return NewInstant(time.Unix(seconds, 0))
}
