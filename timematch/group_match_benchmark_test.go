package timematch

import (
	"testing"

	"github.com/deliri/primitive/v2026/temporal"
)

// These fixtures measure pure matching, including admission, ranking and every
// removal impact. Calendar consent, HTTP, traffic estimation and booking commits
// are deliberately outside the stopwatch. Every dataset is built before timing.
type groupBenchmarkWorkload struct {
	name        string
	people      int
	months      bool
	duration    uint64
	dense       bool
	blockedCEO  bool
	optionalCEO bool
	exactFit    bool
}

var groupBenchmarkResult GroupMatchResult

func BenchmarkMatchGroup(b *testing.B) {
	b.ReportAllocs()
	workloads := []groupBenchmarkWorkload{
		{name: "20_people_14_days_15_minutes", people: 20, duration: 15, dense: true},
		{name: "120_people_14_days_15_minutes", people: 120, duration: 15, dense: true},
		{name: "20_people_18_months_15_minutes", people: 20, months: true, duration: 15, dense: true},
		{name: "120_people_18_months_15_minutes", people: 120, months: true, duration: 15, dense: true},
		{name: "required_CEO_blocks_120_people", people: 120, months: true, duration: 15, dense: true, blockedCEO: true},
		{name: "optional_CEO_keeps_other_119", people: 120, months: true, duration: 15, dense: true, blockedCEO: true, optionalCEO: true},
		{name: "clinic_18_months_3_hours_exact_fit", people: 2, months: true, duration: 180, exactFit: true},
		{name: "clinic_18_months_fully_booked", people: 2, months: true, duration: 180, blockedCEO: true},
	}
	for _, workload := range workloads {
		b.Run(workload.name, func(b *testing.B) {
			request, busyCount := groupBenchmarkRequest(b, workload)
			ctx := b.Context()
			got, err := MatchGroup(ctx, request)
			groupBenchmarkResultFact(b, workload, got, err)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				got, err = MatchGroup(ctx, request)
				if err != nil {
					b.Fatalf("MatchGroup() error = %v, want nil", err)
				}
				groupBenchmarkResult = got
			}
			groupBenchmarkResultFact(b, workload, got, err)
			b.ReportMetric(float64(workload.people), "participants/op")
			b.ReportMetric(float64(busyCount), "busy_intervals/op")
		})
	}
}

func groupBenchmarkRequest(b testing.TB, w groupBenchmarkWorkload) (GroupMatchRequest, int) {
	b.Helper()
	start := groupBenchmarkInstant(b, "2026-11-02T00:00:00Z")
	end := groupBenchmarkInstant(b, "2026-11-16T00:00:00Z")
	if w.months {
		end = groupBenchmarkInstant(b, "2028-05-02T00:00:00Z")
	}
	duration, err := temporal.DurationFromMinutes(w.duration)
	if err != nil {
		b.Fatalf("DurationFromMinutes(%d) error = %v, want nil", w.duration, err)
	}
	window := temporal.IntervalBounds{Start: start, End: end}
	request := GroupMatchRequest{Window: window, Duration: duration, MaximumWindows: 3}
	calendar := CalendarID{value: PrimaryCalendar}
	busyCount := 0
	for person := range w.people {
		var busy []temporal.IntervalBounds
		if w.dense {
			busy = groupBenchmarkBusy(b, window, person)
		}
		required := person < 2
		if person == w.people-1 && w.blockedCEO {
			busy = []temporal.IntervalBounds{window}
			required = !w.optionalCEO
		}
		if w.exactFit {
			lastStart, subtractErr := end.Subtract(duration)
			if subtractErr != nil {
				b.Fatalf("exact-fit end.Subtract() error = %v, want nil", subtractErr)
			}
			busy = []temporal.IntervalBounds{{Start: start, End: lastStart}}
		}
		busyCount += len(busy)
		request.Participants = append(request.Participants, GroupParticipant{Ordinal: uint64(person + 1), Required: required, Availability: Availability{Window: window, Calendars: []CalendarAvailability{{ID: calendar, State: AvailabilityKnown, Busy: busy}}}})
	}
	return request, busyCount
}

func groupBenchmarkBusy(b testing.TB, window temporal.IntervalBounds, person int) []temporal.IntervalBounds {
	b.Helper()
	start, startErr := window.Start.Nanoseconds()
	end, endErr := window.End.Nanoseconds()
	if startErr != nil || endErr != nil {
		b.Fatalf("fixture instant errors = %v/%v, want nil/nil", startErr, endErr)
	}
	minute := int64(temporal.NanosecondsPerMinute)
	hour := int64(temporal.NanosecondsPerHour)
	shift := int64(person%6) * 5 * minute
	var busy []temporal.IntervalBounds
	for begin := start + shift; begin < end; begin += 3 * hour {
		finish := min(begin+hour, end)
		busy = append(busy, temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(begin), End: temporal.InstantFromNanoseconds(finish)})
	}
	return busy
}

func groupBenchmarkInstant(b testing.TB, text string) temporal.Instant {
	b.Helper()
	value, err := temporal.ParseRFC3339(text)
	if err != nil {
		b.Fatalf("ParseRFC3339(%q) error = %v, want nil", text, err)
	}
	return value
}

func groupBenchmarkResultFact(b testing.TB, w groupBenchmarkWorkload, got GroupMatchResult, err error) {
	b.Helper()
	if err != nil || len(got.RemovalImpacts) != w.people {
		b.Fatalf("matching fixture error/impacts = %v/%d, want nil/%d", err, len(got.RemovalImpacts), w.people)
	}
	if w.blockedCEO && !w.optionalCEO {
		if len(got.Windows) != 0 || got.FeasibleStartNanoseconds != 0 || len(got.RemovalImpacts) == 0 || got.RemovalImpacts[0].Ordinal != uint64(w.people) || got.RemovalImpacts[0].AdditionalStartNanoseconds == 0 {
			b.Fatalf("required CEO windows/starts/impacts = %d/%d/%+v, want no fit and exact sole blocker %d", len(got.Windows), got.FeasibleStartNanoseconds, got.RemovalImpacts, w.people)
		}
		return
	}
	if len(got.Windows) == 0 || got.FeasibleStartNanoseconds == 0 {
		b.Fatalf("admitted fixture windows/starts = %d/%d, want both positive", len(got.Windows), got.FeasibleStartNanoseconds)
	}
	wantAttendees := uint64(w.people)
	if w.optionalCEO {
		wantAttendees--
	}
	if got.RequiredCount+got.Windows[0].OptionalAvailable != wantAttendees {
		b.Fatalf("best fixture attendance = %d, want %d people", got.RequiredCount+got.Windows[0].OptionalAvailable, wantAttendees)
	}
	if w.exactFit && (len(got.Windows) != 1 || got.Windows[0].Starts.Earliest != got.Windows[0].Starts.Latest || got.FeasibleStartNanoseconds != 1) {
		b.Fatalf("exact fit windows/starts = %+v/%d, want one singleton and one feasible nanosecond start", got.Windows, got.FeasibleStartNanoseconds)
	}
}
