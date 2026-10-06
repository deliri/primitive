package temporal_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestUTCCalendarExactCoordinatesLayerTriad(t *testing.T) {
	t.Parallel()
	base := temporal.UTCDateTime{Year: 2024, Month: temporal.February, Day: 29}
	cases := []struct {
		wantErr error
		name    string
		date    temporal.UTCDateTime
	}{
		{name: "leap day admitted", date: base},
		{name: "epoch admitted", date: temporal.UTCDateTime{Year: 1970, Month: temporal.January, Day: 1}},
		{name: "before epoch admitted", date: temporal.UTCDateTime{Year: 1969, Month: temporal.December, Day: 31}},
		{name: "nanosecond preserved", date: temporal.UTCDateTime{Year: 2026, Month: temporal.October, Day: 6, Nanosecond: 1}},
		{name: "last clock coordinates admitted", date: temporal.UTCDateTime{Year: 2026, Month: temporal.December, Day: 31, Hour: 23, Minute: 59, Second: 59, Nanosecond: 999999999}},
		{name: "century leap day admitted", date: temporal.UTCDateTime{Year: 2000, Month: temporal.February, Day: 29}},
		{name: "first representable calendar day interior", date: temporal.UTCDateTime{Year: 1677, Month: temporal.September, Day: 22}},
		{name: "last representable calendar day interior", date: temporal.UTCDateTime{Year: 2262, Month: temporal.April, Day: 11}},
		{name: "thirty day month end", date: temporal.UTCDateTime{Year: 2026, Month: temporal.April, Day: 30}},
		{name: "ordinary February end", date: temporal.UTCDateTime{Year: 2025, Month: temporal.February, Day: 28}},
		{name: "zero date refused", wantErr: core.ErrTemporalContract},
		{name: "ordinary year leap day refused", date: temporal.UTCDateTime{Year: 2025, Month: temporal.February, Day: 29}, wantErr: core.ErrTemporalContract},
		{name: "century nonleap date refused", date: temporal.UTCDateTime{Year: 2100, Month: temporal.February, Day: 29}, wantErr: core.ErrTemporalContract},
		{name: "normalized month refused", date: temporal.UTCDateTime{Year: 2026, Month: temporal.Month(13), Day: 1}, wantErr: core.ErrTemporalContract},
		{name: "unknown month refused", date: temporal.UTCDateTime{Year: 2026, Day: 1}, wantErr: core.ErrTemporalContract},
		{name: "normalized day refused", date: temporal.UTCDateTime{Year: 2026, Month: temporal.April, Day: 31}, wantErr: core.ErrTemporalContract},
		{name: "previous month day refused", date: temporal.UTCDateTime{Year: 2026, Month: temporal.January}, wantErr: core.ErrTemporalContract},
		{name: "normalized hour refused", date: temporal.UTCDateTime{Year: 2026, Month: temporal.January, Day: 1, Hour: 24}, wantErr: core.ErrTemporalContract},
		{name: "normalized minute refused", date: temporal.UTCDateTime{Year: 2026, Month: temporal.January, Day: 1, Minute: 60}, wantErr: core.ErrTemporalContract},
		{name: "leap second refused", date: temporal.UTCDateTime{Year: 2026, Month: temporal.January, Day: 1, Second: 60}, wantErr: core.ErrTemporalContract},
		{name: "negative nanosecond refused", date: temporal.UTCDateTime{Year: 2026, Month: temporal.January, Day: 1, Nanosecond: -1}, wantErr: core.ErrTemporalContract},
		{name: "normalized nanosecond refused", date: temporal.UTCDateTime{Year: 2026, Month: temporal.January, Day: 1, Nanosecond: 1000000000}, wantErr: core.ErrTemporalContract},
		{name: "calendar beyond native instant refused", date: temporal.UTCDateTime{Year: math.MaxInt32, Month: temporal.January, Day: 1}, wantErr: core.ErrTemporalOverflow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.date.Instant()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("UTCDateTime.Instant() error = %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				if got != (temporal.Instant{}) {
					t.Fatalf("refused instant = %v, want zero", got)
				}
				return
			}
			calendar, err := got.CalendarUTC()
			if err != nil || calendar.DateTime != tc.date {
				t.Fatalf("CalendarUTC() = (%v, %v), want exact %v", calendar, err, tc.date)
			}
			unchanged, err := got.AddCalendar(temporal.CalendarDelta{})
			if err != nil || unchanged != got {
				t.Fatalf("zero calendar delta = (%v, %v), want (%v, nil)", unchanged, err, got)
			}
		})
	}
}

func FuzzUTCCalendarNativeParity(f *testing.F) {
	f.Add(int64(0), int32(0), int32(0), int32(0))
	f.Add(int64(math.MaxInt64), int32(0), int32(0), int32(1))
	f.Add(int64(math.MinInt64), int32(-1), int32(0), int32(0))
	f.Fuzz(func(t *testing.T, ns int64, years, months, days int32) {
		instant := temporal.InstantFromNanoseconds(ns)
		calendar, err := instant.CalendarUTC()
		if err != nil {
			t.Fatalf("CalendarUTC() error = %v, want nil", err)
		}
		roundTrip, err := calendar.DateTime.Instant()
		if err != nil || roundTrip != instant {
			t.Fatalf("calendar round trip = (%v, %v), want (%v, nil)", roundTrip, err, instant)
		}
		native := time.Unix(0, ns).UTC()
		year, week := native.ISOWeek()
		if int(calendar.ISOYear) != year || int(calendar.ISOWeek) != week || int(calendar.Weekday) != (int(native.Weekday())+6)%7+1 {
			t.Fatalf("calendar observation = %v, want native ISO coordinates", calendar)
		}
		want, wantErr := temporal.NewInstant(native.AddDate(int(years), int(months), int(days)))
		got, gotErr := instant.AddCalendar(temporal.CalendarDelta{Years: years, Months: months, Days: days})
		if (gotErr == nil) != (wantErr == nil) || got != want {
			t.Fatalf("AddCalendar() = (%v, %v), want (%v, %v)", got, gotErr, want, wantErr)
		}
		if gotErr != nil && !errors.Is(gotErr, core.ErrTemporalOverflow) {
			t.Fatalf("calendar refusal = %v, want native overflow", gotErr)
		}
	})
}
