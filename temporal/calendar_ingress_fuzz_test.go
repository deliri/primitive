package temporal_test

import (
	"errors"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzUTCDateTimeExactAdmission(f *testing.F) {
	seed := temporal.UTCDateTime{Year: 2024, Month: temporal.February, Day: 29, Nanosecond: 1}
	if err := seed.Validate(); err != nil {
		f.Fatalf("UTC seed validation = %v, want nil", err)
	}
	f.Add(seed.Year, uint8(seed.Month), seed.Day, seed.Hour, seed.Minute, seed.Second, seed.Nanosecond)
	f.Add(int32(0), uint8(0), uint8(0), uint8(24), uint8(60), uint8(60), int32(-1))
	f.Fuzz(func(t *testing.T, year int32, month, day, hour, minute, second uint8, nanosecond int32) {
		date := temporal.UTCDateTime{Year: year, Month: temporal.Month(month), Day: day, Hour: hour, Minute: minute, Second: second, Nanosecond: nanosecond}
		got, gotErr := date.Instant()
		native := time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), int(nanosecond), time.UTC)
		nativeYear, nativeMonth, nativeDay := native.Date()
		nativeHour, nativeMinute, nativeSecond := native.Clock()
		exact := nativeYear == int(year) && int(nativeMonth) == int(month) && nativeDay == int(day) && nativeHour == int(hour) && nativeMinute == int(minute) && nativeSecond == int(second) && native.Nanosecond() == int(nanosecond)
		want, wantErr := temporal.NewInstant(native)
		if !exact || wantErr != nil {
			if got != (temporal.Instant{}) || (!errors.Is(gotErr, core.ErrTemporalContract) && !errors.Is(gotErr, core.ErrTemporalOverflow)) {
				t.Fatalf("calendar refusal = (%v, %v), want zero and typed refusal", got, gotErr)
			}
			return
		}
		if gotErr != nil || got != want {
			t.Fatalf("calendar admission = (%v, %v), want (%v, nil)", got, gotErr, want)
		}
		observation, err := got.CalendarUTC()
		if err != nil || observation.DateTime != date {
			t.Fatalf("calendar observation = (%v, %v), want exact %v", observation, err, date)
		}
	})
}
