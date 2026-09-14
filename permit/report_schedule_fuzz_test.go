package permit

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzReportScheduleSemanticClosure(f *testing.F) {
	s := reportSchedule(f)
	seed, err := core.MarshalCanonicalJSONDocument(s)
	if err != nil {
		f.Fatalf("marshal schedule = %v, want nil", err)
	}
	f.Add(seed, int64(109), int64(0), int64(1000), uint64(1))
	f.Add([]byte{}, int64(0), int64(0), int64(1), uint64(0))
	f.Fuzz(func(t *testing.T, data []byte, now, nb, expiry int64, jitter uint64) {
		s, err := core.DecodeStrictJSON[ReportSchedule](bytes.NewReader(data), reportLimits())
		if err != nil {
			if s != (ReportSchedule{}) || (!errors.Is(err, core.ErrJSONContract) && !errors.Is(err, core.ErrReportSchedule) && !errors.Is(err, core.ErrPrimitiveContract)) {
				t.Fatalf("schedule refusal = %+v/%v, want zero/typed", s, err)
			}
			return
		}
		if err := s.Validate(); err != nil {
			t.Fatalf("accepted schedule Validate() = %v, want nil", err)
		}
		encoded, err := core.MarshalCanonicalJSONDocument(s)
		if err != nil {
			t.Fatalf("marshal = %v, want nil", err)
		}
		again, err := core.DecodeStrictJSON[ReportSchedule](bytes.NewReader(encoded), reportLimits())
		if err != nil || again != s {
			t.Fatalf("schedule roundtrip = %+v/%v, want %+v/nil", again, err, s)
		}
		timing := ReportTiming{ObservedAt: temporal.InstantFromNanoseconds(now), NotBefore: temporal.InstantFromNanoseconds(nb), ExpiresAt: temporal.InstantFromNanoseconds(expiry)}
		occurrence, err := s.Occurrence(timing)
		if err != nil {
			if occurrence != (ReportOccurrence{}) || (!errors.Is(err, core.ErrReportSchedule) && !errors.Is(err, core.ErrReportOverflow) && !errors.Is(err, core.ErrPermitValidity)) {
				t.Fatalf("occurrence refusal = %+v/%v, want zero/typed", occurrence, err)
			}
			return
		}
		open, _ := occurrence.Open.Nanoseconds()
		closeAt, _ := occurrence.Close.Nanoseconds()
		origin, _ := s.NextReportAt.Nanoseconds()
		if open < nb || open < origin || open >= closeAt || closeAt > expiry || now >= closeAt {
			t.Fatalf("occurrence [%d,%d) at %d, want within authorization [%d,%d) and origin %d", open, closeAt, now, nb, expiry, origin)
		}
		admitted, admitErr := s.Admit(timing)
		if admitted != occurrence {
			t.Fatalf("admission occurrence = %+v, want %+v", admitted, occurrence)
		}
		if now >= open {
			if admitErr != nil {
				t.Fatalf("in-window error = %v, want nil", admitErr)
			}
		} else if !errors.Is(admitErr, core.ErrReportTooEarly) && !errors.Is(admitErr, core.ErrReportOutsideWindow) {
			t.Fatalf("future-window error = %v, want timing refusal", admitErr)
		}
		sampled := int64(jitter % (uint64(s.JitterMaximum.Nanoseconds()) + 1))
		duration, err := temporal.DurationFromNanoseconds(sampled)
		if err != nil {
			t.Fatalf("jitter = %v, want nil", err)
		}
		send, err := s.SendAt(timing, duration)
		if err != nil {
			t.Fatalf("SendAt = %v, want nil", err)
		}
		sendAt, _ := send.Nanoseconds()
		if sendAt < max(open, now) || sendAt >= closeAt || uint64(sendAt)-uint64(max(open, now)) > uint64(sampled) {
			t.Fatalf("send=%d, want [%d,%d) and jitter <=%d", sendAt, max(open, now), closeAt, sampled)
		}
	})
}
