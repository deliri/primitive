package temporal_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestEmbeddedLocationsAdmitExactNamesAndReuseNativeCapabilities(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    temporal.LocationName
		refused bool
	}{
		{temporal.LocationUTC, false}, {"America/Toronto", false}, {"Asia/Tokyo", false}, {"Europe/London", false}, {"Australia/Lord_Howe", false}, {"Pacific/Apia", false}, {"Asia/Kathmandu", false}, {"Etc/UTC", false}, {"US/Eastern", false},
		{"", true}, {"Local", true}, {"utc", true}, {"america/toronto", true}, {"America/toronto", true}, {"America/Toronto/", true}, {"../America/Toronto", true}, {"Not/A_Zone", true}, {" UTC", true}, {"UTC\x00", true},
	} {
		t.Run(string(row.name), func(t *testing.T) {
			t.Parallel()
			got, err := temporal.NewLocation(row.name)
			if row.refused {
				if got != (temporal.Location{}) || !errors.Is(err, core.ErrTemporalContract) {
					t.Fatalf("location %q=%v/%v, want zero/contract", row.name, got, err)
				}
				return
			}
			if err != nil || got.Validate() != nil || got.Name() != row.name {
				t.Fatalf("location %q=%v/%v, want exact name/capability", row.name, got, err)
			}
			native, err := time.LoadLocation(string(row.name))
			if err != nil {
				t.Fatalf("native fixture %q=%v", row.name, err)
			}
			for _, nanos := range []int64{0, -1, 1, 1791244800123456789} {
				instant := temporal.InstantFromNanoseconds(nanos)
				text, textErr := got.Format(instant, time.RFC3339Nano)
				again, againErr := got.Format(instant, time.RFC3339Nano)
				want := time.Unix(0, nanos).In(native).Format(time.RFC3339Nano)
				if textErr != nil || againErr != nil || text != want || again != text {
					t.Fatalf("prepared format=%q/%q/%v/%v, want %q", text, again, textErr, againErr, want)
				}
				prefix := []byte{1, 2, 3}
				appended, appendErr := got.Append(prefix, instant, time.RFC3339Nano)
				if appendErr != nil || string(appended) != string(prefix)+want {
					t.Fatalf("prepared append=%v/%v, want prefix+%q", appended, appendErr, want)
				}
			}
		})
	}
}

func TestLocalCalendarRefusesTimezoneGapsAtNativeBoundaries(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, zone, date     string
		hour, minute, second uint8
		gap                  bool
	}{
		{"Toronto final pre-gap second", "America/Toronto", "2026-03-08", 1, 59, 59, false},
		{"Toronto first absent second", "America/Toronto", "2026-03-08", 2, 0, 0, true},
		{"Toronto middle absent hour", "America/Toronto", "2026-03-08", 2, 30, 0, true},
		{"Toronto last absent second", "America/Toronto", "2026-03-08", 2, 59, 59, true},
		{"Toronto first post-gap second", "America/Toronto", "2026-03-08", 3, 0, 0, false},
		{"Toronto native fold begins", "America/Toronto", "2026-11-01", 1, 0, 0, false},
		{"Toronto ambiguous fold delegates native choice", "America/Toronto", "2026-11-01", 1, 30, 0, false},
		{"Toronto after fold", "America/Toronto", "2026-11-01", 2, 0, 0, false},
		{"Lord Howe before thirty minute gap", "Australia/Lord_Howe", "2026-10-04", 1, 59, 59, false},
		{"Lord Howe gap start", "Australia/Lord_Howe", "2026-10-04", 2, 0, 0, true},
		{"Lord Howe middle thirty minute gap", "Australia/Lord_Howe", "2026-10-04", 2, 15, 0, true},
		{"Lord Howe gap final second", "Australia/Lord_Howe", "2026-10-04", 2, 29, 59, true},
		{"Lord Howe gap end", "Australia/Lord_Howe", "2026-10-04", 2, 30, 0, false},
		{"Apia before skipped date", "Pacific/Apia", "2011-12-29", 23, 59, 59, false},
		{"Apia skipped midnight", "Pacific/Apia", "2011-12-30", 0, 0, 0, true},
		{"Apia entire skipped date", "Pacific/Apia", "2011-12-30", 12, 0, 0, true},
		{"Apia after skipped date", "Pacific/Apia", "2011-12-31", 0, 0, 0, false},
		{"Kathmandu before fifteen minute gap", "Asia/Kathmandu", "1985-12-31", 23, 59, 59, false},
		{"Kathmandu midnight gap", "Asia/Kathmandu", "1986-01-01", 0, 0, 0, true},
		{"Kathmandu gap last second", "Asia/Kathmandu", "1986-01-01", 0, 14, 59, true},
		{"Kathmandu gap end", "Asia/Kathmandu", "1986-01-01", 0, 15, 0, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			location, err := temporal.NewLocation(temporal.LocationName(row.zone))
			if err != nil {
				t.Fatalf("location=%v", err)
			}
			date, err := temporal.ParseCivilDate(row.date)
			if err != nil {
				t.Fatalf("date=%v", err)
			}
			request := temporal.LocalTimeRequest{Location: location, Date: date, Hour: row.hour, Minute: row.minute, Second: row.second}
			got, err := request.Instant()
			text := fmt.Sprintf("%s %02d:%02d:%02d", row.date, row.hour, row.minute, row.second)
			parsed, parseErr := location.ParseLocalTime("2006-01-02 15:04:05", text)
			if row.gap {
				if got != (temporal.Instant{}) || parsed != (temporal.Instant{}) || !errors.Is(err, core.ErrTemporalContract) || !errors.Is(parseErr, core.ErrTemporalContract) {
					t.Fatalf("gap admission=%v/%v errors %v/%v, want both zero/contract", got, parsed, err, parseErr)
				}
				return
			}
			formatted, formatErr := location.Format(got, "2006-01-02 15:04:05")
			observed, dateErr := location.DateAt(got)
			if err != nil || parseErr != nil || formatErr != nil || dateErr != nil || formatted != text || observed != date {
				t.Fatalf("local admission=%v/%v/%q/%v errors %v/%v/%v/%v, want exact %q/%v", got, parsed, formatted, observed, err, parseErr, formatErr, dateErr, text, date)
			}
		})
	}
}

func TestLocalCalendarDaysFollowDSTRatherThanElapsedDay(t *testing.T) {
	t.Parallel()
	location, err := temporal.NewLocation("America/Toronto")
	if err != nil {
		t.Fatalf("location=%v", err)
	}
	for _, row := range []struct {
		start, want string
		hours       uint64
	}{{"2026-03-07 12:00:00", "2026-03-08 12:00:00", 23}, {"2026-10-31 12:00:00", "2026-11-01 12:00:00", 25}, {"2026-04-01 12:00:00", "2026-04-02 12:00:00", 24}} {
		start, err := location.ParseLocalTime("2006-01-02 15:04:05", row.start)
		if err != nil {
			t.Fatalf("start=%v", err)
		}
		got, err := location.AddCalendar(start, temporal.CalendarDelta{Days: 1})
		text, textErr := location.Format(got, "2006-01-02 15:04:05")
		elapsed, elapsedErr := got.Since(start)
		wantDuration, durationErr := temporal.DurationFromHours(row.hours)
		if err != nil || textErr != nil || elapsedErr != nil || durationErr != nil || text != row.want || elapsed != wantDuration {
			t.Fatalf("calendar day=%q/%v errors %v/%v/%v, want %q/%d hours", text, elapsed, err, textErr, elapsedErr, row.want, row.hours)
		}
		date, dateErr := location.DateAt(start)
		midnight, midnightErr := location.StartOfDay(date)
		clock, clockErr := location.Format(midnight, "2006-01-02 15:04:05")
		if dateErr != nil || midnightErr != nil || clockErr != nil || clock != row.start[:10]+" 00:00:00" {
			t.Fatalf("start of day=%q errors %v/%v/%v, want midnight", clock, dateErr, midnightErr, clockErr)
		}
	}
	start, err := location.ParseLocalTime("2006-01-02 15:04:05", "2026-03-07 02:30:00")
	if err != nil {
		t.Fatalf("gap seed=%v", err)
	}
	got, err := location.AddCalendar(start, temporal.CalendarDelta{Days: 1})
	if got != (temporal.Instant{}) || !errors.Is(err, core.ErrTemporalContract) {
		t.Fatalf("calendar gap=%v/%v, want zero/contract", got, err)
	}
}

func FuzzLocationExactNamesAndNativeProjection(f *testing.F) {
	seed, err := temporal.NewLocation("America/Toronto")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(string(seed.Name()), int64(0))
	f.Add("america/toronto", int64(1))
	f.Add("", int64(-1))
	f.Fuzz(func(t *testing.T, name string, nanos int64) {
		got, err := temporal.NewLocation(temporal.LocationName(name))
		if err != nil {
			if got != (temporal.Location{}) || !errors.Is(err, core.ErrTemporalContract) {
				t.Fatalf("zone refusal=%v/%v, want zero/contract", got, err)
			}
			return
		}
		if got.Validate() != nil || string(got.Name()) != name || name == "Local" || name == "" {
			t.Fatalf("zone custody=%v, want exact %q", got, name)
		}
		native, nativeErr := time.LoadLocation(name)
		text, textErr := got.Format(temporal.InstantFromNanoseconds(nanos), time.RFC3339Nano)
		if nativeErr != nil || textErr != nil || text != time.Unix(0, nanos).In(native).Format(time.RFC3339Nano) {
			t.Fatalf("zone projection=%q/%v/%v, want native exact", text, textErr, nativeErr)
		}
	})
}

func FuzzLocalWallClockExactAdmission(f *testing.F) {
	location, err := temporal.NewLocation("America/Toronto")
	if err != nil {
		f.Fatal(err)
	}
	date, err := temporal.NewCivilDate(temporal.CivilDateRequest{Year: 2026, Month: temporal.March, Day: 8})
	if err != nil {
		f.Fatal(err)
	}
	seed := temporal.LocalTimeRequest{Location: location, Date: date, Hour: 3}
	if seed.Validate() != nil {
		f.Fatal("seed invalid")
	}
	coordinates, err := date.Coordinates()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(coordinates.Year, uint8(coordinates.Month), coordinates.Day, seed.Hour, seed.Minute, seed.Second, seed.Nanosecond)
	f.Add(int32(2026), uint8(3), uint8(8), uint8(2), uint8(30), uint8(0), int32(0))
	nativeZone, err := time.LoadLocation(string(location.Name()))
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, year int32, month, day, hour, minute, second uint8, nanos int32) {
		date, dateErr := temporal.NewCivilDate(temporal.CivilDateRequest{Year: year, Month: temporal.Month(month), Day: day})
		got, err := (temporal.LocalTimeRequest{Location: location, Date: date, Hour: hour, Minute: minute, Second: second, Nanosecond: nanos}).Instant()
		native := time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), int(nanos), nativeZone)
		y, m, d := native.Date()
		h, min, s := native.Clock()
		want, extentErr := temporal.NewInstant(native)
		exact := dateErr == nil && y == int(year) && int(m) == int(month) && d == int(day) && h == int(hour) && min == int(minute) && s == int(second) && native.Nanosecond() == int(nanos) && extentErr == nil
		if !exact {
			if got != (temporal.Instant{}) || (!errors.Is(err, core.ErrTemporalContract) && !errors.Is(err, core.ErrTemporalOverflow)) {
				t.Fatalf("wall refusal=%v/%v, want zero/typed", got, err)
			}
			return
		}
		if err != nil || got != want || got.Validate() != nil {
			t.Fatalf("wall admission=%v/%v, want exact %v", got, err, want)
		}
	})
}

func FuzzLocalTextExactAdmission(f *testing.F) {
	location, err := temporal.NewLocation("America/Toronto")
	if err != nil {
		f.Fatal(err)
	}
	date, err := temporal.NewCivilDate(temporal.CivilDateRequest{Year: 2026, Month: temporal.March, Day: 8})
	if err != nil {
		f.Fatal(err)
	}
	instant, err := (temporal.LocalTimeRequest{Location: location, Date: date, Hour: 3}).Instant()
	if err != nil {
		f.Fatal(err)
	}
	layout := temporal.TimeLayout("2006-01-02 15:04:05")
	text, err := location.Format(instant, layout)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(string(layout), text)
	f.Add(string(layout), "2026-03-08 02:30:00")
	f.Add("YYYY-MM-DD", "")
	nativeZone, err := time.LoadLocation(string(location.Name()))
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, layout, text string) {
		got, err := location.ParseLocalTime(temporal.TimeLayout(layout), text)
		native, nativeErr := time.ParseInLocation(layout, text, nativeZone)
		want, extentErr := temporal.NewInstant(native)
		accepted := nativeLayoutVaries(layout) && nativeErr == nil && extentErr == nil && native.In(nativeZone).Format(layout) == text
		if !accepted {
			if got != (temporal.Instant{}) || (!errors.Is(err, core.ErrTemporalContract) && !errors.Is(err, core.ErrTemporalOverflow)) {
				t.Fatalf("local parse refusal=%v/%v, want zero/typed", got, err)
			}
			return
		}
		canonical, canonicalErr := location.Format(got, temporal.TimeLayout(layout))
		if err != nil || got != want || canonicalErr != nil || canonical != text {
			t.Fatalf("local parse=%v/%q errors %v/%v, want %v/%q", got, canonical, err, canonicalErr, want, text)
		}
	})
}
