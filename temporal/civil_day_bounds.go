package temporal

import (
	"errors"
	"math"
	"time"
)

// CivilDayBounds returns the complete half-open preimage of date. It refuses
// a skipped date, a date with separated preimages, or endpoints outside the
// signed-nanosecond Instant profile. It never silently chooses a midnight fold.
// Native TZif offsets are signed32 seconds; the TZ-string extension's offsets
// are narrower. That representation gives a complete search extent without a
// guessed hours-per-day limit, even for historical dateline changes.
// Sources: https://www.rfc-editor.org/rfc/rfc9636.html#section-3.2
// https://go.dev/src/time/zoneinfo_read.go
func (l Location) CivilDayBounds(date CivilDate) (IntervalBounds, error) {
	if err := errors.Join(l.Validate(), date.Validate()); err != nil {
		return IntervalBounds{}, err
	}
	wallStart := date.native()
	wallEnd := wallStart.AddDate(0, 0, 1)
	earliest := wallStart.Add(-time.Duration(math.MaxInt32) * time.Second)
	latest := wallEnd.Add(-time.Duration(math.MinInt32) * time.Second)
	var first, last time.Time
	separated := false
	found := false
	err := l.walkNativeCivilZonePeriods(earliest, latest, func(start, end time.Time, offset int) error {
		shift := -time.Duration(offset) * time.Second
		left, right := wallStart.Add(shift), wallEnd.Add(shift)
		if left.Before(start) {
			left = start
		}
		if right.After(end) {
			right = end
		}
		if !left.Before(right) {
			return nil
		}
		if !found {
			found = true
			first, last = left, right
			return nil
		}
		if left.After(last) {
			separated = true
			return nil
		}
		if right.After(last) {
			last = right
		}
		return nil
	})
	if err != nil {
		return IntervalBounds{}, err
	}
	if !found || separated {
		return IntervalBounds{}, contractError("civil date has no single complete interval")
	}
	start, startErr := NewInstant(first)
	end, endErr := NewInstant(last)
	if err := errors.Join(startErr, endErr); err != nil {
		return IntervalBounds{}, err
	}
	return IntervalBounds{Start: start, End: end}, nil
}
