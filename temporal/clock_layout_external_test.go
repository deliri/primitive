package temporal_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestNativeClockAndCalendarChoiceIsExclusiveAndClosed(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		for _, date := range []temporal.TimeLayout{"", time.RFC3339Nano} {
			t.Run(fmt.Sprintf("clock_%d_calendar_%t", raw, date != ""), func(t *testing.T) {
				t.Parallel()
				clock := temporal.ClockLayout(raw)
				request := temporal.NativeTimeFormatRequest{Time: time.Date(2026, time.October, 14, 13, 4, 5, 123, time.UTC), Layout: date, Clock: clock}
				original := request
				got, err := temporal.FormatNativeTime(request)
				accepted := raw == 0 && date != "" || raw == 1 && date == ""
				if !accepted {
					if got != "" || !errors.Is(err, core.ErrTemporalContract) || !errors.Is(request.Validate(), core.ErrTemporalContract) {
						t.Fatalf("native choice=%q/%v, want zero/contract", got, err)
					}
				} else {
					layout := string(date)
					if raw == 1 {
						layout = time.TimeOnly
					}
					want := request.Time.Format(layout)
					if err != nil || request.Validate() != nil || got != want {
						t.Fatalf("native choice=%q/%v, want %q", got, err, want)
					}
				}
				if request != original {
					t.Fatalf("borrowed native request changed=%v, want %v", request, original)
				}
				if clock.IsValid() != (raw == 1) || (raw == 1 && clock.String() == core.UnknownEnumDiagnostic) || (raw != 1 && (!errors.Is(clock.Validate(), core.ErrTemporalContract) || clock.String() != core.UnknownEnumDiagnostic)) {
					t.Fatalf("clock enum %d=%q/%v, want exact closed member1", raw, clock.String(), clock.Validate())
				}
			})
		}
	}
}

func FuzzNativeClockAndCalendarLayoutAgreement(f *testing.F) {
	seed := temporal.NativeTimeFormatRequest{Time: time.Unix(0, 0).UTC(), Clock: temporal.ClockHourMinuteSecond}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	if _, err := temporal.FormatNativeTime(seed); err != nil {
		f.Fatal(err)
	}
	f.Add(int64(0), "", uint8(seed.Clock))
	f.Add(int64(1), time.RFC3339Nano, uint8(0))
	f.Add(int64(-1), "", uint8(0))
	f.Add(int64(0), time.RFC3339Nano, uint8(seed.Clock))
	f.Add(int64(0), "", uint8(255))
	f.Fuzz(func(t *testing.T, nanos int64, layout string, clock uint8) {
		request := temporal.NativeTimeFormatRequest{Time: time.Unix(0, nanos).UTC(), Layout: temporal.TimeLayout(layout), Clock: temporal.ClockLayout(clock)}
		original := request
		got, err := temporal.FormatNativeTime(request)
		accepted := clock == 0 && nativeLayoutVaries(layout) || clock == 1 && layout == ""
		if !accepted {
			if got != "" || !errors.Is(err, core.ErrTemporalContract) {
				t.Fatalf("native choice refusal=%q/%v, want empty/contract", got, err)
			}
			return
		}
		if clock == 1 {
			layout = time.TimeOnly
		}
		want := request.Time.Format(layout)
		if err != nil || got != want || request.Validate() != nil || request != original {
			t.Fatalf("native choice=%q/%v/%v, want exact borrowed/native %q", got, err, request, want)
		}
	})
}
