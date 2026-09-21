package permit

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestReportPermissionProjectsExactCarrierAndAuthorizationIntersection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr                                   error
		name                                      string
		now, authorizationStart, authorizationEnd int64
		wantStart, wantEnd                        int64
		initial                                   bool
		enabled                                   bool
	}{
		{name: "initial clips wider authorization", initial: true, enabled: true, now: 100, authorizationStart: 0, authorizationEnd: 1000, wantStart: 20, wantEnd: 900},
		{name: "authorization clips initial permission", initial: true, enabled: true, now: 100, authorizationStart: 50, authorizationEnd: 800, wantStart: 50, wantEnd: 800},
		{name: "acknowledgment retains refreshed authorization", enabled: true, now: 100, authorizationStart: 30, authorizationEnd: 950, wantStart: 30, wantEnd: 950},
		{name: "disabled grants no timing", initial: true, now: 100, authorizationStart: 0, authorizationEnd: 1000, wantErr: core.ErrPermitAction},
		{name: "exclusive initial expiry refuses", initial: true, enabled: true, now: 900, authorizationStart: 0, authorizationEnd: 1000, wantErr: core.ErrPermitValidity},
		{name: "exclusive refreshed expiry refuses", enabled: true, now: 800, authorizationStart: 0, authorizationEnd: 800, wantErr: core.ErrPermitValidity},
		{name: "disjoint genuine authorizations emit no timing", initial: true, enabled: true, now: 100, authorizationStart: 950, authorizationEnd: 1000, wantErr: core.ErrReportSchedule},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			trust, key := permitFixture(t)
			scope := reportScopeFixture(t)
			schedule := reportSchedule(t)
			authorization, err := SignReportAuthorization(ReportAuthorization{
				Scope: scope, Enabled: tc.enabled, ObservedAt: temporal.InstantFromNanoseconds(0),
				NotBefore: temporal.InstantFromNanoseconds(tc.authorizationStart), ExpiresAt: temporal.InstantFromNanoseconds(tc.authorizationEnd), Policy: schedule.Policy,
			}, key)
			if err != nil {
				t.Fatalf("sign authorization = %v, want nil", err)
			}
			response := ReportPermissionResponse{Authorization: authorization}
			if tc.initial {
				initial, err := SignProjectPermission(ProjectPermission{Scope: scope, IssuedAt: temporal.InstantFromNanoseconds(0), NotBefore: temporal.InstantFromNanoseconds(20), ExpiresAt: temporal.InstantFromNanoseconds(900), Schedule: schedule}, key)
				if err != nil {
					t.Fatalf("sign initial permission = %v, want nil", err)
				}
				response.InitialPermission = &initial
			} else {
				digest := core.NewSHA256Digest([core.SHA256DigestBytes]byte{0x7b})
				ack, err := SignReportAcknowledgment(ReportAcknowledgment{Scope: scope, Sequence: 1, ReportDigest: digest, AcceptedAt: temporal.InstantFromNanoseconds(0), Schedule: schedule}, key)
				if err != nil {
					t.Fatalf("sign acknowledgment = %v, want nil", err)
				}
				response.Acknowledgment = &ack
			}
			if err := response.Verify(trust.TrustedKeys, scope); err != nil {
				t.Fatalf("verify genuine response = %v, want nil", err)
			}
			gotSchedule, err := response.Schedule()
			if err != nil || gotSchedule != schedule {
				t.Fatalf("schedule = %+v/%v, want exact %+v/nil", gotSchedule, err, schedule)
			}
			got, err := response.Timing(temporal.InstantFromNanoseconds(tc.now))
			want := ReportTiming{}
			if tc.wantErr == nil {
				want = ReportTiming{ObservedAt: temporal.InstantFromNanoseconds(tc.now), NotBefore: temporal.InstantFromNanoseconds(tc.wantStart), ExpiresAt: temporal.InstantFromNanoseconds(tc.wantEnd)}
			}
			if !errors.Is(err, tc.wantErr) || got != want {
				t.Fatalf("timing = %+v/%v, want %+v/%v", got, err, want, tc.wantErr)
			}
		})
	}
	zero := ReportPermissionResponse{}
	if got, err := zero.Schedule(); err == nil || got != (ReportSchedule{}) {
		t.Fatalf("zero response schedule = %+v/%v, want zero/refusal", got, err)
	}
	if got, err := zero.Timing(temporal.InstantFromNanoseconds(100)); err == nil || got != (ReportTiming{}) {
		t.Fatalf("zero response timing = %+v/%v, want zero/refusal", got, err)
	}
}
