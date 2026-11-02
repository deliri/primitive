package timematch

import (
	"testing"

	"github.com/deliri/primitive/v2026/temporal"
)

// This is functional extent proof, not a latency benchmark. Every person has
// the same busy first hour of each three-hour period, so the independent answer
// is exactly the remaining two-hour gaps, less the requested appointment length.
func TestGroupMatchDenseOneHundredTwentyAcrossEighteenMonths(t *testing.T) {
	t.Parallel()
	start := groupBenchmarkInstant(t, "2026-11-02T00:00:00Z")
	end := groupBenchmarkInstant(t, "2028-05-02T00:00:00Z")
	window := temporal.IntervalBounds{Start: start, End: end}
	horizon, err := end.Since(start)
	if err != nil {
		t.Fatalf("end.Since(start) error = %v, want nil", err)
	}
	period, err := temporal.DurationFromHours(3)
	if err != nil {
		t.Fatal(err)
	}
	occupied, err := temporal.DurationFromHours(1)
	if err != nil {
		t.Fatal(err)
	}
	duration, err := temporal.DurationFromMinutes(15)
	if err != nil {
		t.Fatal(err)
	}
	periods := horizon.Nanoseconds() / period.Nanoseconds()
	if periods < 4000 || horizon.Nanoseconds()%period.Nanoseconds() != 0 {
		t.Fatalf("fixture periods/remainder = %d/%d, want more than4000 complete periods", periods, horizon.Nanoseconds()%period.Nanoseconds())
	}
	var busy []temporal.IntervalBounds
	for offset := int64(0); offset < horizon.Nanoseconds(); offset += period.Nanoseconds() {
		delta := groupDuration(t, offset)
		begin, err := start.Add(delta)
		if err != nil {
			t.Fatal(err)
		}
		finish, err := begin.Add(occupied)
		if err != nil {
			t.Fatal(err)
		}
		busy = append(busy, temporal.IntervalBounds{Start: begin, End: finish})
	}
	r := GroupMatchRequest{Window: window, Duration: duration, MaximumWindows: 3}
	for person := range 120 {
		r.Participants = append(r.Participants, GroupParticipant{
			Ordinal: uint64(person + 1), Required: person%2 == 0,
			Availability: Availability{Window: window, Calendars: []CalendarAvailability{{ID: CalendarID{value: PrimaryCalendar}, State: AvailabilityKnown, Busy: busy}}},
		})
	}
	got, err := MatchGroup(t.Context(), r)
	wantStarts := uint64(periods) * uint64(period.Nanoseconds()-occupied.Nanoseconds()-duration.Nanoseconds()+1)
	if err != nil || len(got.Windows) != 3 || got.RequiredCount != 60 || got.KnownOptionalCount != 60 || got.FeasibleStartNanoseconds != wantStarts || !got.Truncated || len(got.UnknownRequired) != 0 || len(got.UnknownOptional) != 0 || len(got.RemovalImpacts) != 120 {
		t.Fatalf("dense result = %+v, error = %v, want three best windows,60required/60optional,120impacts and%d starts", got, err, wantStarts)
	}
	for i, candidate := range got.Windows {
		first, firstErr := start.Add(groupDuration(t, int64(i)*period.Nanoseconds()+occupied.Nanoseconds()))
		last, lastErr := start.Add(groupDuration(t, int64(i+1)*period.Nanoseconds()-duration.Nanoseconds()))
		if firstErr != nil || lastErr != nil || candidate != (GroupWindow{Starts: GroupStartRange{Earliest: first, Latest: last}, OptionalAvailable: 60}) {
			t.Fatalf("dense window%d = %+v, errors = %v/%v, want exact [%v,%v] with60optional", i, candidate, firstErr, lastErr, first, last)
		}
	}
	for _, impact := range got.RemovalImpacts {
		if impact.AdditionalStartNanoseconds != 0 || impact.EarliestUnlocked != (temporal.Instant{}) || impact.Unknown {
			t.Fatalf("dense removal impact = %+v, want zero gain because60required people share every busy period", impact)
		}
	}
	for i, interval := range busy {
		first, firstErr := start.Add(groupDuration(t, int64(i)*period.Nanoseconds()))
		last, lastErr := first.Add(occupied)
		if firstErr != nil || lastErr != nil || interval != (temporal.IntervalBounds{Start: first, End: last}) {
			t.Fatalf("input interval%d = %+v, errors = %v/%v, want unchanged [%v,%v)", i, interval, firstErr, lastErr, first, last)
		}
	}
}
