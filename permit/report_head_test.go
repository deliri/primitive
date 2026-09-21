package permit

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/temporal"
)

// This unit ratchet exhausts the head comparison decisions. The caller still
// verifies the signature and commits the summary and head in one transaction.
func TestReportHeadLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		wantErr       error
		name          string
		sequence      uint64
		headSequence  uint64
		sameDigest    bool
		wrongPrevious bool
		foreignScope  bool
		wantReplay    bool
	}{
		{name: "first report advances explicit initial head", sequence: 1},
		{name: "next report advances committed predecessor", sequence: 2, headSequence: 1},
		{name: "last representable sequence advances without overflow", sequence: math.MaxInt64, headSequence: math.MaxInt64 - 1},
		{name: "exact committed replay contributes no new report", sequence: 2, headSequence: 2, sameDigest: true, wantReplay: true},
		{name: "exhausted head still permits exact replay", sequence: math.MaxInt64, headSequence: math.MaxInt64, sameDigest: true, wantReplay: true},
		{name: "same sequence with different bytes conflicts", sequence: 2, headSequence: 2, wantErr: core.ErrReportConflict},
		{name: "older sequence cannot replay through latest head", sequence: 1, headSequence: 2, wantErr: core.ErrReportSequence},
		{name: "gap cannot skip a pending report", sequence: 3, headSequence: 1, wantErr: core.ErrReportSequence},
		{name: "next sequence requires exact predecessor digest", sequence: 2, headSequence: 1, wrongPrevious: true, wantErr: core.ErrReportSequence},
		{name: "another project cannot advance this head", sequence: 2, headSequence: 1, foreignScope: true, wantErr: core.ErrReportBinding},
		{name: "exhausted sequence never wraps", sequence: 1, headSequence: math.MaxInt64, wantErr: core.ErrReportSequence},
		{name: "out of domain head is rejected", sequence: 1, headSequence: math.MaxUint64, wantErr: core.ErrReportSequence},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, key := permitFixture(t)
			scope := reportScopeFixture(t)
			previous, err := InitialReportDigest(scope)
			if err != nil {
				t.Fatalf("InitialReportDigest() = %v, want nil", err)
			}
			payload := ReportPayload{Scope: scope, Sequence: tc.sequence, Previous: previous, Window: controlplane.UsageWindow{Bounds: temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(0), End: temporal.InstantFromNanoseconds(50)}, Freshness: temporal.InstantFromNanoseconds(50), Units: []controlplane.UsageCount{{Class: 1, Count: 2}}, Outcomes: []controlplane.OutcomeCount{{Class: 1, Count: 2}}}, Evidence: ReportEvidence{Digest: core.SHA256Of([]byte("manifest")), Bytes: 8}, Policy: reportSchedule(t).Policy}
			if tc.wrongPrevious {
				payload.Previous = core.SHA256Of([]byte("foreign predecessor"))
			}
			signed, err := SignReportPayload(payload, key)
			if err != nil {
				t.Fatalf("SignReportPayload() = %v, want nil", err)
			}
			head := ReportHead{Scope: scope, Sequence: tc.headSequence, Digest: previous}
			if tc.sameDigest {
				head.Digest, err = signed.Digest()
				if err != nil {
					t.Fatalf("Digest() = %v, want nil", err)
				}
			}
			if tc.foreignScope {
				head.Scope.Project, err = id.NewULIDFromBytes([16]byte{5})
				if err != nil {
					t.Fatalf("NewULIDFromBytes() = %v, want nil", err)
				}
			}
			before := head
			got, gotErr := head.Compare(signed)
			if got != tc.wantReplay || !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Compare() = (%v, %v), want (%v, %v)", got, gotErr, tc.wantReplay, tc.wantErr)
			}
			if head != before {
				t.Fatalf("head = %+v, want unchanged %+v", head, before)
			}
		})
	}
}
