package controlplane_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

func TestCheckInResponseRefusesConflictThatItsProducerWouldAccept(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		previous bool
		wantErr  error
	}{
		{name: "different committed window remains a conflict"},
		{name: "unchanged predecessor cannot be called a conflict", previous: true, wantErr: core.ErrControlPlaneDecisionConsistency},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			issued, preparation := committedCheckInResponseFixture(t)
			server := issued.server(t)
			payload, err := server.PrepareCheckInResponse(preparation)
			if err != nil {
				t.Fatalf("PrepareCheckInResponse() error = %v, want nil", err)
			}
			payload.Disposition = controlplane.UsageDispositionConflict
			current := issued.request.Payload.PreviousWatermark
			if !tc.previous {
				current, err = controlplane.AdvanceUsageWatermark(current, testWindow(unitsOf(1, 3), outcomesOf(1, 3)))
				if err != nil {
					t.Fatalf("AdvanceUsageWatermark(foreign) error = %v, want nil", err)
				}
			}
			payload.Watermark = current
			payload.Lease = authorityLeaseDocument(t, authorityLeaseFixtureRequest{Signer: issued.authority, Subject: current.Subject, Generation: current.Generation, IssuedAt: payload.Header.ProviderTime})
			document, err := server.IssueCheckInResponse(payload, issued.authority)
			if err != nil {
				t.Fatalf("IssueCheckInResponse(structural conflict) error = %v, want nil", err)
			}
			got, err := issued.client(t).VerifyCheckInResponse(controlplane.CheckInResponseVerification{Document: document, Expected: expectationFor(payload.Header), PreviousWatermark: issued.request.Payload.PreviousWatermark, Window: issued.request.Payload.Window})
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("VerifyCheckInResponse() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && got != (controlplane.VerifiedCheckInResponse{}) {
				t.Fatalf("impossible conflict proof = %v, want zero", got)
			}
			if tc.wantErr == nil {
				retained, err := got.Payload()
				if err != nil || retained.Watermark != current || retained.Disposition != controlplane.UsageDispositionConflict {
					t.Fatalf("retained conflict = (%v, %v), want exact authoritative watermark", retained, err)
				}
			}
		})
	}
}
