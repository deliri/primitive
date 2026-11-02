package temporal

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

func civilPreimageInstant(t testing.TB, text string) Instant {
	t.Helper()
	v, err := ParseRFC3339(text)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func civilPreimageDate(t testing.TB, text string) CivilDate {
	t.Helper()
	v, err := ParseCivilDate(text)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func civilPreimageLocation(t testing.TB, name LocationName) Location {
	t.Helper()
	v, err := NewLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func civilPreimageBounds(t testing.TB, start, end string) IntervalBounds {
	t.Helper()
	return IntervalBounds{Start: civilPreimageInstant(t, start), End: civilPreimageInstant(t, end)}
}

func TestCivilMinuteIntervalsExactKnownTransitions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		zone             LocationName
		date, start, end string
		open, duration   uint16
		want             []string
	}{
		{name: "UTC ordinary opening", zone: LocationUTC, date: "2026-11-02", start: "2026-11-02T00:00:00Z", end: "2026-11-03T00:00:00Z", open: 540, duration: 60, want: []string{"2026-11-02T09:00:00Z/2026-11-02T10:00:00Z"}},
		{name: "Toronto ordinary offset", zone: "America/Toronto", date: "2026-11-02", start: "2026-11-02T00:00:00Z", end: "2026-11-03T00:00:00Z", open: 540, duration: 60, want: []string{"2026-11-02T14:00:00Z/2026-11-02T15:00:00Z"}},
		{name: "spring nonexistent hour has no preimage", zone: "America/Toronto", date: "2026-03-08", start: "2026-03-08T00:00:00Z", end: "2026-03-09T00:00:00Z", open: 120, duration: 60},
		{name: "spring partial gap keeps real halves adjacent", zone: "America/Toronto", date: "2026-03-08", start: "2026-03-08T00:00:00Z", end: "2026-03-09T00:00:00Z", open: 90, duration: 120, want: []string{"2026-03-08T06:30:00Z/2026-03-08T07:30:00Z"}},
		{name: "fall both narrow folds returned", zone: "America/Toronto", date: "2026-11-01", start: "2026-11-01T00:00:00Z", end: "2026-11-02T00:00:00Z", open: 75, duration: 30, want: []string{"2026-11-01T05:15:00Z/2026-11-01T05:45:00Z", "2026-11-01T06:15:00Z/2026-11-01T06:45:00Z"}},
		{name: "fall broad range merges adjoining periods", zone: "America/Toronto", date: "2026-11-01", start: "2026-11-01T00:00:00Z", end: "2026-11-02T00:00:00Z", open: 30, duration: 120, want: []string{"2026-11-01T04:30:00Z/2026-11-01T07:30:00Z"}},
		{name: "Lord Howe half hour fold", zone: "Australia/Lord_Howe", date: "2026-04-05", start: "2026-04-04T00:00:00Z", end: "2026-04-06T00:00:00Z", open: 105, duration: 15, want: []string{"2026-04-04T14:45:00Z/2026-04-04T15:00:00Z", "2026-04-04T15:15:00Z/2026-04-04T15:30:00Z"}},
		{name: "Kathmandu fractional offset", zone: "Asia/Kathmandu", date: "2026-11-02", start: "2026-11-01T00:00:00Z", end: "2026-11-03T00:00:00Z", open: 540, duration: 60, want: []string{"2026-11-02T03:15:00Z/2026-11-02T04:15:00Z"}},
		{name: "Kiritimati civil day begins previous UTC day", zone: "Pacific/Kiritimati", date: "2026-11-02", start: "2026-11-01T00:00:00Z", end: "2026-11-03T00:00:00Z", open: 0, duration: 60, want: []string{"2026-11-01T10:00:00Z/2026-11-01T11:00:00Z"}},
		{name: "Apia skipped entire date stays absent", zone: "Pacific/Apia", date: "2011-12-30", start: "2011-12-29T00:00:00Z", end: "2012-01-01T00:00:00Z", open: 0, duration: 1440},
		{name: "Berlin both folds independent of Toronto", zone: "Europe/Berlin", date: "2026-10-25", start: "2026-10-25T00:00:00Z", end: "2026-10-26T00:00:00Z", open: 135, duration: 30, want: []string{"2026-10-25T00:15:00Z/2026-10-25T00:45:00Z", "2026-10-25T01:15:00Z/2026-10-25T01:45:00Z"}},
		{name: "overnight interval crosses civil date", zone: LocationUTC, date: "2026-11-02", start: "2026-11-02T00:00:00Z", end: "2026-11-04T00:00:00Z", open: 1380, duration: 120, want: []string{"2026-11-02T23:00:00Z/2026-11-03T01:00:00Z"}},
		{name: "clip left nanosecond exactly", zone: LocationUTC, date: "2026-11-02", start: "2026-11-02T09:00:00.000000001Z", end: "2026-11-02T10:00:00Z", open: 540, duration: 60, want: []string{"2026-11-02T09:00:00.000000001Z/2026-11-02T10:00:00Z"}},
		{name: "clip right nanosecond exactly", zone: LocationUTC, date: "2026-11-02", start: "2026-11-02T09:00:00Z", end: "2026-11-02T09:59:59.999999999Z", open: 540, duration: 60, want: []string{"2026-11-02T09:00:00Z/2026-11-02T09:59:59.999999999Z"}},
		{name: "window ends at opening touches nothing", zone: LocationUTC, date: "2026-11-02", start: "2026-11-02T08:00:00Z", end: "2026-11-02T09:00:00Z", open: 540, duration: 60},
		{name: "window starts at closing touches nothing", zone: LocationUTC, date: "2026-11-02", start: "2026-11-02T10:00:00Z", end: "2026-11-02T11:00:00Z", open: 540, duration: 60},
		{name: "window one nanosecond into opening", zone: LocationUTC, date: "2026-11-02", start: "2026-11-02T08:00:00Z", end: "2026-11-02T09:00:00.000000001Z", open: 540, duration: 60, want: []string{"2026-11-02T09:00:00Z/2026-11-02T09:00:00.000000001Z"}},
		{name: "window one nanosecond before closing", zone: LocationUTC, date: "2026-11-02", start: "2026-11-02T09:59:59.999999999Z", end: "2026-11-02T11:00:00Z", open: 540, duration: 60, want: []string{"2026-11-02T09:59:59.999999999Z/2026-11-02T10:00:00Z"}},
		{name: "zero window remains empty", zone: LocationUTC, date: "2026-11-02", start: "2026-11-02T09:00:00Z", end: "2026-11-02T09:00:00Z", open: 540, duration: 60},
		{name: "opening last civil minute", zone: LocationUTC, date: "2026-11-02", start: "2026-11-02T00:00:00Z", end: "2026-11-03T00:00:00Z", open: 1439, duration: 1, want: []string{"2026-11-02T23:59:00Z/2026-11-03T00:00:00Z"}},
		{name: "duration maximum full day", zone: LocationUTC, date: "2026-11-02", start: "2026-11-01T00:00:00Z", end: "2026-11-04T00:00:00Z", open: 0, duration: 1440, want: []string{"2026-11-02T00:00:00Z/2026-11-03T00:00:00Z"}},
		{name: "full day from last minute ends next last minute", zone: LocationUTC, date: "2026-11-02", start: "2026-11-02T00:00:00Z", end: "2026-11-04T00:00:00Z", open: 1439, duration: 1440, want: []string{"2026-11-02T23:59:00Z/2026-11-03T23:59:00Z"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := civilPreimageLocation(t, tc.zone)
			window := civilPreimageBounds(t, tc.start, tc.end)
			date := civilPreimageDate(t, tc.date)
			got, err := l.CivilMinuteIntervals(window, CivilMinuteRange{Date: date, OpenMinute: tc.open, DurationMinutes: tc.duration})
			if err != nil {
				t.Fatalf("CivilMinuteIntervals() error=%v, want nil", err)
			}
			text := make([]string, 0, len(got))
			for _, span := range got {
				a, ae := span.Start.RFC3339Nano()
				b, be := span.End.RFC3339Nano()
				if ae != nil || be != nil {
					t.Fatal(errors.Join(ae, be))
				}
				text = append(text, a+"/"+b)
			}
			if !slices.Equal(text, tc.want) {
				t.Fatalf("preimage=%v, want %v", text, tc.want)
			}
			civilPreimagePointOracle(t, l, window, date, tc.open, tc.duration, got)
		})
	}
}

// This oracle never uses ZoneBounds or offset subtraction: it asks native Go
// for each sampled instant's actual wall date/clock and compares civil values.
// Endpoint neighbours pressure half-open membership at nanosecond precision.
func civilPreimagePointOracle(t *testing.T, l Location, window IntervalBounds, date CivilDate, open, duration uint16, got []IntervalBounds) {
	t.Helper()
	start, _ := window.Start.Time()
	end, _ := window.End.Time()
	wallStart := date.native().Add(time.Duration(open) * time.Minute)
	wallEnd := wallStart.Add(time.Duration(duration) * time.Minute)
	probe := func(point time.Time) {
		if point.Before(start) || !point.Before(end) {
			return
		}
		local := point.In(l.value)
		wall := time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), time.UTC)
		want := !wall.Before(wallStart) && wall.Before(wallEnd)
		found := false
		for _, span := range got {
			a, _ := span.Start.Time()
			b, _ := span.End.Time()
			found = found || !point.Before(a) && point.Before(b)
		}
		if found != want {
			t.Fatalf("membership at %v got=%t, want native civil=%t", point, found, want)
		}
	}
	for point := start; point.Before(end); point = point.Add(5 * time.Minute) {
		probe(point)
	}
	for i, span := range got {
		if span.Validate() != nil || span.Start.nanoseconds >= span.End.nanoseconds || span.Start.nanoseconds < window.Start.nanoseconds || span.End.nanoseconds > window.End.nanoseconds {
			t.Fatalf("span=%+v, want positive bounded interval", span)
		}
		if i > 0 && got[i-1].End.nanoseconds >= span.Start.nanoseconds {
			t.Fatalf("preimage neighbours=(%+v,%+v), want separated ordered spans", got[i-1], span)
		}
		a, _ := span.Start.Time()
		b, _ := span.End.Time()
		probe(a)
		probe(a.Add(-time.Nanosecond))
		probe(b)
		probe(b.Add(-time.Nanosecond))
	}
}

func TestCivilMinuteIntervalsTypedRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		change  func(*Location, *IntervalBounds, *CivilDate, *uint16, *uint16)
		wantErr error
	}{
		{name: "unset location", change: func(l *Location, _ *IntervalBounds, _ *CivilDate, _ *uint16, _ *uint16) { *l = Location{} }, wantErr: core.ErrTemporalContract},
		{name: "unset date", change: func(_ *Location, _ *IntervalBounds, d *CivilDate, _ *uint16, _ *uint16) { *d = CivilDate{} }, wantErr: core.ErrTemporalContract},
		{name: "unset start", change: func(_ *Location, w *IntervalBounds, _ *CivilDate, _ *uint16, _ *uint16) { w.Start = Instant{} }, wantErr: core.ErrTemporalContract},
		{name: "unset end", change: func(_ *Location, w *IntervalBounds, _ *CivilDate, _ *uint16, _ *uint16) { w.End = Instant{} }, wantErr: core.ErrTemporalContract},
		{name: "reversed window", change: func(_ *Location, w *IntervalBounds, _ *CivilDate, _ *uint16, _ *uint16) {
			w.Start, w.End = w.End, w.Start
		}, wantErr: core.ErrTemporalContract},
		{name: "whole signed span overflows elapsed duration", change: func(_ *Location, w *IntervalBounds, _ *CivilDate, _ *uint16, _ *uint16) {
			w.Start = InstantFromNanoseconds(math.MinInt64)
			w.End = InstantFromNanoseconds(math.MaxInt64)
		}, wantErr: core.ErrTemporalOverflow},
		{name: "opening exactly outside day", change: func(_ *Location, _ *IntervalBounds, _ *CivilDate, o *uint16, _ *uint16) { *o = 1440 }, wantErr: core.ErrTemporalContract},
		{name: "opening pathological uint16", change: func(_ *Location, _ *IntervalBounds, _ *CivilDate, o *uint16, _ *uint16) { *o = math.MaxUint16 }, wantErr: core.ErrTemporalContract},
		{name: "zero duration", change: func(_ *Location, _ *IntervalBounds, _ *CivilDate, _ *uint16, d *uint16) { *d = 0 }, wantErr: core.ErrTemporalContract},
		{name: "duration one past civil day", change: func(_ *Location, _ *IntervalBounds, _ *CivilDate, _ *uint16, d *uint16) { *d = 1441 }, wantErr: core.ErrTemporalContract},
		{name: "duration pathological uint16", change: func(_ *Location, _ *IntervalBounds, _ *CivilDate, _ *uint16, d *uint16) { *d = math.MaxUint16 }, wantErr: core.ErrTemporalContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := civilPreimageLocation(t, LocationUTC)
			window := civilPreimageBounds(t, "2026-11-02T00:00:00Z", "2026-11-03T00:00:00Z")
			date := civilPreimageDate(t, "2026-11-02")
			open, duration := uint16(540), uint16(60)
			tc.change(&l, &window, &date, &open, &duration)
			got, err := l.CivilMinuteIntervals(window, CivilMinuteRange{Date: date, OpenMinute: open, DurationMinutes: duration})
			if !errors.Is(err, tc.wantErr) || len(got) != 0 {
				t.Fatalf("CivilMinuteIntervals()=(%v,%v), want empty/%v", got, err, tc.wantErr)
			}
		})
	}
}

func TestCivilMinuteRangeAdmission(t *testing.T) {
	t.Parallel()
	date := civilPreimageDate(t, "2026-11-02")
	cases := []struct {
		name       string
		rangeValue CivilMinuteRange
		wantErr    error
	}{
		{name: "ordinary opening", rangeValue: CivilMinuteRange{Date: date, OpenMinute: 540, DurationMinutes: 60}},
		{name: "first minute minimum duration", rangeValue: CivilMinuteRange{Date: date, OpenMinute: 0, DurationMinutes: 1}},
		{name: "last opening crosses next date", rangeValue: CivilMinuteRange{Date: date, OpenMinute: 1439, DurationMinutes: 2}},
		{name: "full day starts at first minute", rangeValue: CivilMinuteRange{Date: date, OpenMinute: 0, DurationMinutes: 1440}},
		{name: "full day starts at last minute", rangeValue: CivilMinuteRange{Date: date, OpenMinute: 1439, DurationMinutes: 1440}},
		{name: "missing date", rangeValue: CivilMinuteRange{OpenMinute: 540, DurationMinutes: 60}, wantErr: core.ErrTemporalContract},
		{name: "opening one beyond last minute", rangeValue: CivilMinuteRange{Date: date, OpenMinute: 1440, DurationMinutes: 1}, wantErr: core.ErrTemporalContract},
		{name: "opening pathological unsigned", rangeValue: CivilMinuteRange{Date: date, OpenMinute: math.MaxUint16, DurationMinutes: 1}, wantErr: core.ErrTemporalContract},
		{name: "zero duration", rangeValue: CivilMinuteRange{Date: date, OpenMinute: 540}, wantErr: core.ErrTemporalContract},
		{name: "duration one beyond full day", rangeValue: CivilMinuteRange{Date: date, OpenMinute: 540, DurationMinutes: 1441}, wantErr: core.ErrTemporalContract},
		{name: "duration pathological unsigned", rangeValue: CivilMinuteRange{Date: date, OpenMinute: 540, DurationMinutes: math.MaxUint16}, wantErr: core.ErrTemporalContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.rangeValue.Validate()
			if (tc.wantErr == nil && err != nil) || (tc.wantErr != nil && !errors.Is(err, tc.wantErr)) {
				t.Fatalf("CivilMinuteRange.Validate()=%v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestCivilDatesActualDatesAndExtrema(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		zone       LocationName
		start, end string
		want       []string
	}{
		{name: "midnight end excludes next date", zone: LocationUTC, start: "2026-11-02T00:00:00Z", end: "2026-11-03T00:00:00Z", want: []string{"2026-11-02"}},
		{name: "midnight one nano includes next date", zone: LocationUTC, start: "2026-11-02T23:59:59.999999999Z", end: "2026-11-03T00:00:00.000000001Z", want: []string{"2026-11-02", "2026-11-03"}},
		{name: "zero window touches no date", zone: LocationUTC, start: "2026-11-02T12:00:00Z", end: "2026-11-02T12:00:00Z"},
		{name: "Apia skips missing date", zone: "Pacific/Apia", start: "2011-12-29T10:00:00Z", end: "2011-12-31T10:00:00Z", want: []string{"2011-12-29", "2011-12-31"}},
		{name: "Kwajalein rollback deduplicates repeated date", zone: "Pacific/Kwajalein", start: "1969-09-29T13:00:00Z", end: "1969-10-01T12:00:00Z", want: []string{"1969-09-30"}},
		{name: "fall fold remains one date", zone: "America/Toronto", start: "2026-11-01T04:00:00Z", end: "2026-11-02T05:00:00Z", want: []string{"2026-11-01"}},
		{name: "spring gap remains one date", zone: "America/Toronto", start: "2026-03-08T05:00:00Z", end: "2026-03-09T04:00:00Z", want: []string{"2026-03-08"}},
		{name: "leap day exact dates", zone: LocationUTC, start: "2024-02-28T12:00:00Z", end: "2024-03-01T12:00:00Z", want: []string{"2024-02-28", "2024-02-29", "2024-03-01"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := civilPreimageLocation(t, tc.zone)
			got, err := l.CivilDates(civilPreimageBounds(t, tc.start, tc.end))
			texts := make([]string, 0, len(got))
			for _, date := range got {
				text, e := date.Text()
				if e != nil {
					t.Fatal(e)
				}
				texts = append(texts, text)
			}
			if err != nil || !slices.Equal(texts, tc.want) {
				t.Fatalf("CivilDates()=(%v,%v), want %v", texts, err, tc.want)
			}
		})
	}
	for _, zone := range []LocationName{LocationUTC, "America/Toronto", "Asia/Kathmandu", "Pacific/Kiritimati"} {
		for _, edge := range []int64{math.MinInt64, math.MaxInt64 - 1} {
			t.Run(string(zone)+"_"+strconv.FormatInt(edge, 10), func(t *testing.T) {
				t.Parallel()
				l := civilPreimageLocation(t, zone)
				window := IntervalBounds{Start: InstantFromNanoseconds(edge), End: InstantFromNanoseconds(edge + 1)}
				date, err := l.DateAt(window.Start)
				if err != nil {
					t.Fatal(err)
				}
				dates, err := l.CivilDates(window)
				if err != nil || len(dates) != 1 || dates[0] != date {
					t.Fatalf("CivilDates(extreme)=(%v,%v), want native date %v", dates, err, date)
				}
				got, err := l.CivilMinuteIntervals(window, CivilMinuteRange{Date: date, OpenMinute: 0, DurationMinutes: 1440})
				if err != nil || len(got) != 1 || got[0] != window {
					t.Fatalf("CivilMinuteIntervals(extreme)=(%v,%v), want exact one-nanosecond window", got, err)
				}
			})
		}
	}
}
