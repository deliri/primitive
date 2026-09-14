package permit

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func quietFixture(t testing.TB, start, end int64) QuietPeriods {
	t.Helper()
	q, err := NewQuietPeriods(QuietPeriod{Start: temporal.InstantFromNanoseconds(start), End: temporal.InstantFromNanoseconds(end)})
	if err != nil {
		t.Fatalf("quiet periods = %v, want nil", err)
	}
	return q
}
func TestNoBroadcastWindowsLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                  string
		now, start, end, want int64
	}{
		{"before exclusion remains eligible", 100, 101, 105, 100},
		{"exact exclusion start advances", 100, 100, 105, 105},
		{"inside exclusion advances", 104, 100, 105, 105},
		{"exclusive exclusion end allows", 105, 100, 105, 105},
		{"exclusion covers slot and skips period", 100, 100, 110, 200},
		{"exclusion covers multiple periods", 100, 100, 305, 305},
		{"exclusion ends in gap uses next slot", 100, 100, 350, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := reportSchedule(t)
			got, err := s.SendOutsideQuietPeriods(ReportTiming{ObservedAt: temporal.InstantFromNanoseconds(tc.now), NotBefore: temporal.InstantFromNanoseconds(0), ExpiresAt: temporal.InstantFromNanoseconds(1000)}, temporal.Duration{}, quietFixture(t, tc.start, tc.end))
			if err != nil || got != temporal.InstantFromNanoseconds(tc.want) {
				t.Fatalf("SendOutsideQuietPeriods = %v/%v, want %d/nil", got, err, tc.want)
			}
		})
	}
	if got, err := NewQuietPeriods(QuietPeriod{Start: temporal.InstantFromNanoseconds(100), End: temporal.InstantFromNanoseconds(100)}); !errors.Is(err, core.ErrReportSchedule) || got != (QuietPeriods{}) {
		t.Fatalf("zero-width window = %+v/%v, want zero/typed refusal", got, err)
	}
	none := QuietPeriods{}
	at := temporal.InstantFromNanoseconds(100)
	if got, err := none.NextAllowed(at); err != nil || got != at {
		t.Fatalf("no exclusions = %v/%v, want %v/nil", got, err, at)
	}
}
func FuzzReportPermissionResponse(f *testing.F) {
	r, key := permitFixture(f)
	scope := reportScopeFixture(f)
	s := reportSchedule(f)
	initial, err := SignProjectPermission(ProjectPermission{Scope: scope, IssuedAt: temporal.InstantFromNanoseconds(0), NotBefore: temporal.InstantFromNanoseconds(0), ExpiresAt: temporal.InstantFromNanoseconds(1000), Schedule: s}, key)
	if err != nil {
		f.Fatalf("initial = %v, want nil", err)
	}
	authorization, err := SignReportAuthorization(ReportAuthorization{Scope: scope, Enabled: true, ObservedAt: temporal.InstantFromNanoseconds(0), NotBefore: temporal.InstantFromNanoseconds(0), ExpiresAt: temporal.InstantFromNanoseconds(1000), Policy: s.Policy, Transmission: TransmissionPolicy{Reports: quietFixture(f, 105, 109)}}, key)
	if err != nil {
		f.Fatalf("authorization = %v, want nil", err)
	}
	document := ReportPermissionResponse{Authorization: authorization, InitialPermission: &initial}
	seed, err := document.MarshalJSON()
	if err != nil {
		f.Fatalf("seed = %v, want nil", err)
	}
	authSeed, err := authorization.MarshalJSON()
	if err != nil {
		f.Fatalf("auth seed = %v, want nil", err)
	}
	quietSeed, err := authorization.Payload.Transmission.Reports.MarshalJSON()
	if err != nil {
		f.Fatalf("quiet seed = %v, want nil", err)
	}
	f.Add(uint8(0), seed)
	f.Add(uint8(1), authSeed)
	f.Add(uint8(2), quietSeed)
	f.Add(uint8(2), []byte(`null`))
	f.Add(uint8(0), []byte{})
	f.Fuzz(func(t *testing.T, kind uint8, data []byte) {
		var encoded, baseline, second []byte
		var decodeErr, verifyErr, encodeErr, error2 error
		switch kind % 3 {
		case 0:
			got := document
			decodeErr = got.UnmarshalJSON(data)
			encoded, encodeErr = got.MarshalJSON()
			baseline = seed
			if decodeErr == nil {
				verifyErr = got.Verify(r.TrustedKeys, scope)
				var again ReportPermissionResponse
				error2 = again.UnmarshalJSON(encoded)
				if error2 == nil {
					second, error2 = again.MarshalJSON()
				}
			}
		case 1:
			got := authorization
			decodeErr = got.UnmarshalJSON(data)
			encoded, encodeErr = got.MarshalJSON()
			baseline = authSeed
			if decodeErr == nil {
				verifyErr = got.Verify(r.TrustedKeys, scope)
				var again SignedReportAuthorization
				error2 = again.UnmarshalJSON(encoded)
				if error2 == nil {
					second, error2 = again.MarshalJSON()
				}
			}
		case 2:
			got := authorization.Payload.Transmission.Reports
			decodeErr = got.UnmarshalJSON(data)
			encoded, encodeErr = got.MarshalJSON()
			baseline = quietSeed
			if decodeErr == nil {
				var again QuietPeriods
				error2 = again.UnmarshalJSON(encoded)
				if error2 == nil {
					second, error2 = again.MarshalJSON()
				}
				next, err := got.NextAllowed(temporal.InstantFromNanoseconds(106))
				if err != nil {
					t.Fatalf("NextAllowed = %v, want nil", err)
				}
				againAt, err := got.NextAllowed(next)
				if err != nil || againAt != next {
					t.Fatalf("quiet projection idempotence = %v/%v, want %v/nil", againAt, err, next)
				}
			}
		}
		if encodeErr != nil || len(encoded) > ReportDocumentMaximumBytes {
			t.Fatalf("marshal = %v/%d, want nil/bounded", encodeErr, len(encoded))
		}
		if decodeErr != nil {
			if !errors.Is(decodeErr, core.ErrReportContract) && !errors.Is(decodeErr, core.ErrReportSchedule) && !errors.Is(decodeErr, core.ErrReportBinding) {
				t.Fatalf("refusal = %v, want typed", decodeErr)
			}
			if !bytes.Equal(encoded, baseline) {
				t.Fatalf("rejected receiver = %q, want unchanged", encoded)
			}
			return
		}
		if error2 != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("roundtrip = %v/%t, want nil/true", error2, bytes.Equal(second, encoded))
		}
		if kind%3 != 2 && verifyErr == nil && !bytes.Equal(encoded, baseline) {
			t.Fatalf("authenticated mutation equals genuinely signed seed = false, want true")
		}
		if verifyErr != nil && !errors.Is(verifyErr, core.ErrReportAuthentication) && !errors.Is(verifyErr, core.ErrReportBinding) {
			t.Fatalf("verify = %v, want typed authentication/binding", verifyErr)
		}
	})
}
