package permit

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func reportDuration(t testing.TB, n int64) temporal.Duration {
	t.Helper()
	d, err := temporal.DurationFromNanoseconds(n)
	if err != nil {
		t.Fatalf("duration(%d) = %v, want nil", n, err)
	}
	return d
}
func reportSchedule(t testing.TB) ReportSchedule {
	t.Helper()
	return ReportSchedule{NextReportAt: temporal.InstantFromNanoseconds(100), WindowDuration: reportDuration(t, 10), RepeatInterval: reportDuration(t, 100), JitterMaximum: reportDuration(t, 1), Policy: controlwire.PolicyCursor{Revision: controlwire.PolicyRevisionID{1}, Activation: 1}}
}
func TestReportScheduleLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                string
		now, notBefore, expiry, open, close int64
		wantErr                             error
	}{
		{"before initial opening", 99, 0, 1000, 100, 110, core.ErrReportTooEarly},
		{"exact initial opening", 100, 0, 1000, 100, 110, nil},
		{"one above opening", 101, 0, 1000, 100, 110, nil},
		{"one before exclusive close", 109, 0, 1000, 100, 110, nil},
		{"exact exclusive close", 110, 0, 1000, 200, 210, core.ErrReportOutsideWindow},
		{"one after exclusive close", 111, 0, 1000, 200, 210, core.ErrReportOutsideWindow},
		{"one before second opening", 199, 0, 1000, 200, 210, core.ErrReportOutsideWindow},
		{"exact second opening", 200, 0, 1000, 200, 210, nil},
		{"one above second opening", 201, 0, 1000, 200, 210, nil},
		{"many missed periods preserve phase", 900, 0, 1000, 900, 910, nil},
		{"not-before clips initial opening", 100, 105, 1000, 105, 110, core.ErrReportOutsideWindow},
		{"not-before exact clipped opening", 105, 105, 1000, 105, 110, nil},
		{"not-before skips expired occurrence", 105, 110, 1000, 200, 210, core.ErrReportOutsideWindow},
		{"expiry clips close", 104, 0, 105, 100, 105, nil},
		{"expiry is exclusive", 105, 0, 105, 0, 0, core.ErrPermitValidity},
		{"expiry before first opening", 99, 0, 100, 0, 0, core.ErrPermitValidity},
		{"not-before ahead is not proof of current eligibility", 0, 901, 1000, 901, 910, core.ErrReportTooEarly},
		{"last representable occurrence overflow refuses", math.MaxInt64, 0, math.MaxInt64, 0, 0, core.ErrReportOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := reportSchedule(t)
			got, err := s.Admit(ReportTiming{ObservedAt: temporal.InstantFromNanoseconds(tc.now), NotBefore: temporal.InstantFromNanoseconds(tc.notBefore), ExpiresAt: temporal.InstantFromNanoseconds(tc.expiry)})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Admit() error = %v, want %v", err, tc.wantErr)
			}
			if tc.open == 0 {
				if got != (ReportOccurrence{}) {
					t.Fatalf("refused occurrence = %+v, want zero", got)
				}
				return
			}
			want := ReportOccurrence{Open: temporal.InstantFromNanoseconds(tc.open), Close: temporal.InstantFromNanoseconds(tc.close)}
			if got != want {
				t.Fatalf("occurrence = %+v, want %+v", got, want)
			}
		})
	}
}
func TestReportScheduleRejectsInvalidShape(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*ReportSchedule)
	}{
		{"unset opening", func(s *ReportSchedule) { s.NextReportAt = temporal.Instant{} }},
		{"zero window", func(s *ReportSchedule) { s.WindowDuration = temporal.Duration{} }},
		{"zero period", func(s *ReportSchedule) { s.RepeatInterval = temporal.Duration{} }},
		{"window equal period", func(s *ReportSchedule) { s.WindowDuration = s.RepeatInterval }},
		{"window above period", func(s *ReportSchedule) { s.WindowDuration = reportDuration(t, 101) }},
		{"jitter equal window", func(s *ReportSchedule) { s.JitterMaximum = s.WindowDuration }},
		{"jitter above window", func(s *ReportSchedule) { s.JitterMaximum = reportDuration(t, 11) }},
		{"unset policy", func(s *ReportSchedule) { s.Policy = controlwire.PolicyCursor{} }},
		{"overflow first close", func(s *ReportSchedule) { s.NextReportAt = temporal.InstantFromNanoseconds(math.MaxInt64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := reportSchedule(t)
			before := s
			tc.mutate(&s)
			if s == before {
				t.Fatalf("mutation = %+v, want different from %+v", s, before)
			}
			if err := s.Validate(); !errors.Is(err, core.ErrReportSchedule) {
				t.Fatalf("Validate() = %v, want %v", err, core.ErrReportSchedule)
			}
		})
	}
}
func TestReportJitterNeverCrossesClose(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name              string
		now, jitter, want int64
	}{
		{"before opening", 99, 1, 101}, {"exact opening", 100, 1, 101}, {"late wake clips at last nanosecond", 109, 1, 109}, {"closed occurrence skips forward", 110, 1, 201}, {"zero jitter stays at opening", 100, 0, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := reportSchedule(t)
			got, err := s.SendAt(ReportTiming{ObservedAt: temporal.InstantFromNanoseconds(tc.now), NotBefore: temporal.InstantFromNanoseconds(0), ExpiresAt: temporal.InstantFromNanoseconds(1000)}, reportDuration(t, tc.jitter))
			if err != nil || got != temporal.InstantFromNanoseconds(tc.want) {
				t.Fatalf("SendAt() = %v/%v, want %d/nil", got, err, tc.want)
			}
		})
	}
}
