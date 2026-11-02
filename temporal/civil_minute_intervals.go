package temporal

import (
	"slices"
	"time"
)

// CivilMinuteRange names one civil opening and elapsed wall-minute length.
// Duration may extend into the next civil date; no ambiguous local instant is
// chosen when this nominal civil range is admitted.
type CivilMinuteRange struct {
	Date            CivilDate
	OpenMinute      uint16
	DurationMinutes uint16
}

// Validate admits a real Gregorian date, an opening inside that date, and a
// positive wall-minute duration of at most one civil day.
func (r CivilMinuteRange) Validate() error {
	if err := r.Date.Validate(); err != nil {
		return err
	}
	if r.OpenMinute >= 1440 || r.DurationMinutes == 0 || r.DurationMinutes > 1440 {
		return contractError("civil minute range is invalid")
	}
	return nil
}

// CivilMinuteIntervals returns the exact preimage of one half-open civil
// minute range, clipped to window. The opening minute is 0..1439; duration is
// 1..1440 minutes and may continue into the following civil date. A gap has no
// preimage; a fold can have more than one. Adjacent results are merged.
//
// Each native ZoneBounds period has one fixed offset, so subtracting that
// offset from the UTC-shaped civil coordinates gives its exact preimage.
// Only clipped endpoints are projected to signed nanoseconds. Native time
// owns historical offsets and transitions, including non-hour/dateline jumps.
// Source: https://pkg.go.dev/time#Time.ZoneBounds
func (l Location) CivilMinuteIntervals(window IntervalBounds, civil CivilMinuteRange) ([]IntervalBounds, error) {
	if err := civil.Validate(); err != nil {
		return nil, err
	}
	wallStart := civil.Date.native().Add(time.Duration(civil.OpenMinute) * time.Minute)
	wallEnd := wallStart.Add(time.Duration(civil.DurationMinutes) * time.Minute)
	var result []IntervalBounds
	err := l.walkCivilZonePeriods(window, func(start, end time.Time, offset int) error {
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
		first, err := NewInstant(left)
		if err != nil {
			return err
		}
		last, err := NewInstant(right)
		if err != nil {
			return err
		}
		if len(result) > 0 && result[len(result)-1].End.nanoseconds >= first.nanoseconds {
			if last.nanoseconds > result[len(result)-1].End.nanoseconds {
				result[len(result)-1].End = last
			}
			return nil
		}
		result = append(result, IntervalBounds{Start: first, End: last})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CivilDates returns ascending unique civil dates actually touched by the
// half-open window. It walks every fixed-offset native period, so a backward
// dateline change cannot hide a date between the endpoint dates. An empty
// window touches no date. Allocation belongs to the returned whole value and
// grows with actual dates/zone periods, never a guessed input quota.
func (l Location) CivilDates(window IntervalBounds) ([]CivilDate, error) {
	var result []CivilDate
	err := l.walkCivilZonePeriods(window, func(start, end time.Time, _ int) error {
		first, err := civilDateFromNative(start.In(l.value))
		if err != nil {
			return err
		}
		last, err := civilDateFromNative(end.Add(-time.Nanosecond).In(l.value))
		if err != nil {
			return err
		}
		for current := first.native(); !current.After(last.native()); current = current.AddDate(0, 0, 1) {
			date, dateErr := civilDateFromNative(current)
			if dateErr != nil {
				return dateErr
			}
			result = append(result, date)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(result, func(a, b CivilDate) int { return a.native().Compare(b.native()) })
	return slices.Compact(result), nil
}

// walkCivilZonePeriods visits nonempty clipped native periods in absolute
// order. ZoneBounds' zero end means the offset continues indefinitely.
func (l Location) walkCivilZonePeriods(window IntervalBounds, visit func(time.Time, time.Time, int) error) error {
	if err := l.Validate(); err != nil {
		return err
	}
	if err := window.Validate(); err != nil {
		return err
	}
	start, err := window.Start.Time()
	if err != nil {
		return err
	}
	end, err := window.End.Time()
	if err != nil {
		return err
	}
	for cursor := start; cursor.Before(end); {
		local := cursor.In(l.value)
		_, offset := local.Zone()
		_, bound := local.ZoneBounds()
		finish := end
		if !bound.IsZero() && bound.Before(finish) {
			finish = bound
		}
		if !cursor.Before(finish) {
			return contractError("native zone period did not advance")
		}
		if err := visit(cursor, finish, offset); err != nil {
			return err
		}
		cursor = finish
	}
	return nil
}
