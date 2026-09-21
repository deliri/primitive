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
		wantErr                             error
		name                                string
		now, notBefore, expiry, open, close int64
	}{
		{name: "before initial opening", now: 99, notBefore: 0, expiry: 1000, open: 100, close: 110, wantErr: core.ErrReportTooEarly},
		{name: "exact initial opening", now: 100, notBefore: 0, expiry: 1000, open: 100, close: 110, wantErr: nil},
		{name: "one above opening", now: 101, notBefore: 0, expiry: 1000, open: 100, close: 110, wantErr: nil},
		{name: "one before exclusive close", now: 109, notBefore: 0, expiry: 1000, open: 100, close: 110, wantErr: nil},
		{name: "exact exclusive close", now: 110, notBefore: 0, expiry: 1000, open: 200, close: 210, wantErr: core.ErrReportOutsideWindow},
		{name: "one after exclusive close", now: 111, notBefore: 0, expiry: 1000, open: 200, close: 210, wantErr: core.ErrReportOutsideWindow},
		{name: "one before second opening", now: 199, notBefore: 0, expiry: 1000, open: 200, close: 210, wantErr: core.ErrReportOutsideWindow},
		{name: "exact second opening", now: 200, notBefore: 0, expiry: 1000, open: 200, close: 210, wantErr: nil},
		{name: "one above second opening", now: 201, notBefore: 0, expiry: 1000, open: 200, close: 210, wantErr: nil},
		{name: "many missed periods preserve phase", now: 900, notBefore: 0, expiry: 1000, open: 900, close: 910, wantErr: nil},
		{name: "not-before clips initial opening", now: 100, notBefore: 105, expiry: 1000, open: 105, close: 110, wantErr: core.ErrReportOutsideWindow},
		{name: "not-before exact clipped opening", now: 105, notBefore: 105, expiry: 1000, open: 105, close: 110, wantErr: nil},
		{name: "not-before skips expired occurrence", now: 105, notBefore: 110, expiry: 1000, open: 200, close: 210, wantErr: core.ErrReportOutsideWindow},
		{name: "expiry clips close", now: 104, notBefore: 0, expiry: 105, open: 100, close: 105, wantErr: nil},
		{name: "expiry is exclusive", now: 105, notBefore: 0, expiry: 105, open: 0, close: 0, wantErr: core.ErrPermitValidity},
		{name: "expiry before first opening", now: 99, notBefore: 0, expiry: 100, open: 0, close: 0, wantErr: core.ErrPermitValidity},
		{name: "not-before ahead is not proof of current eligibility", now: 0, notBefore: 901, expiry: 1000, open: 901, close: 910, wantErr: core.ErrReportTooEarly},
		{name: "last representable occurrence overflow refuses", now: math.MaxInt64, notBefore: 0, expiry: math.MaxInt64, open: 0, close: 0, wantErr: core.ErrReportOverflow},
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
		mutate func(*ReportSchedule)
		name   string
	}{
		{name: "unset opening", mutate: func(s *ReportSchedule) { s.NextReportAt = temporal.Instant{} }},
		{name: "zero window", mutate: func(s *ReportSchedule) { s.WindowDuration = temporal.Duration{} }},
		{name: "zero period", mutate: func(s *ReportSchedule) { s.RepeatInterval = temporal.Duration{} }},
		{name: "window equal period", mutate: func(s *ReportSchedule) { s.WindowDuration = s.RepeatInterval }},
		{name: "window above period", mutate: func(s *ReportSchedule) { s.WindowDuration = reportDuration(t, 101) }},
		{name: "jitter equal window", mutate: func(s *ReportSchedule) { s.JitterMaximum = s.WindowDuration }},
		{name: "jitter above window", mutate: func(s *ReportSchedule) { s.JitterMaximum = reportDuration(t, 11) }},
		{name: "unset policy", mutate: func(s *ReportSchedule) { s.Policy = controlwire.PolicyCursor{} }},
		{name: "overflow first close", mutate: func(s *ReportSchedule) { s.NextReportAt = temporal.InstantFromNanoseconds(math.MaxInt64) }},
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
		{name: "before opening", now: 99, jitter: 1, want: 101}, {name: "exact opening", now: 100, jitter: 1, want: 101}, {name: "late wake clips at last nanosecond", now: 109, jitter: 1, want: 109}, {name: "closed occurrence skips forward", now: 110, jitter: 1, want: 201}, {name: "zero jitter stays at opening", now: 100, jitter: 0, want: 100},
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
