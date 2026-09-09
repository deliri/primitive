package controlplane_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

type authorityCurrentFact uint8

const (
	currentPredecessor authorityCurrentFact = iota
	currentSuccessor
	currentOtherWindow
	currentOtherChain
	currentOtherGeneration
)

type authorityTransitionClass uint8

const (
	transitionBoundary authorityTransitionClass = iota + 1
	transitionNeutral
	transitionContradiction
	transitionRefusal
)

// The comparison's domain is exhaustively current==predecessor,
// current==successor, or neither; prior signatures and policy are admission
// gates. Opaque offering permutations do not add classification states.
func TestCheckInAuthorityProducerComparisonLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		primary          authorityTransitionClass
		position         authorityCurrentFact
		corruptSignature bool
		zeroProof        bool
		wantProducerErr  error
		wantErr          error
		wantDisposition  controlplane.UsageDisposition
	}{
		{name: "authenticated predecessor advances exactly once", primary: transitionBoundary, wantDisposition: controlplane.UsageDispositionAccepted},
		{name: "exact authenticated successor is a replay", primary: transitionNeutral, position: currentSuccessor, wantDisposition: controlplane.UsageDispositionReplay},
		{name: "different authenticated window at same generation conflicts", primary: transitionContradiction, position: currentOtherWindow, wantDisposition: controlplane.UsageDispositionConflict},
		{name: "same window digest with a different chain conflicts", primary: transitionContradiction, position: currentOtherChain, wantDisposition: controlplane.UsageDispositionConflict},
		{name: "same digests with a different generation conflict", primary: transitionContradiction, position: currentOtherGeneration, wantDisposition: controlplane.UsageDispositionConflict},
		{name: "corrupt producer signature cannot become a commit", primary: transitionRefusal, corruptSignature: true, wantProducerErr: core.ErrAttestVerification, wantErr: core.ErrControlPlaneCheckIn},
		{name: "absent producer proof cannot become usage", primary: transitionRefusal, zeroProof: true, wantErr: core.ErrControlPlaneCheckIn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			issued := issueTestCheckIn(t, controlplaneOffering(t, 3), testCheckInWindow())
			server := issued.server(t)
			canonical, err := issued.request.MarshalJSON()
			if err != nil {
				t.Fatalf("producer MarshalJSON() error = %v, want nil", err)
			}
			var request controlplane.CheckInRequest
			if err := request.UnmarshalJSON(canonical); err != nil {
				t.Fatalf("producer UnmarshalJSON() error = %v, want nil", err)
			}
			if tc.corruptSignature {
				request.Attestation.BodySHA256 = corruptDigest()
			}
			proof, producerErr := server.VerifyCheckIn(controlplane.CheckInVerification{Request: request})
			if !errors.Is(producerErr, tc.wantProducerErr) {
				t.Fatalf("producer error = %v, want %v", producerErr, tc.wantProducerErr)
			}
			if producerErr == nil {
				got, err := proof.Request()
				if err != nil {
					t.Fatalf("producer Request() error = %v, want nil", err)
				}
				projected, err := got.MarshalJSON()
				if err != nil || !bytes.Equal(projected, canonical) {
					t.Fatalf("producer facts = (%d bytes, %v), want exact %d bytes", len(projected), err, len(canonical))
				}
			}
			previous := issued.request.Payload.PreviousWatermark
			successor, err := controlplane.AdvanceUsageWatermark(previous, issued.request.Payload.Window)
			if err != nil || successor == previous {
				t.Fatalf("AdvanceUsageWatermark() = (%v, %v), want distinct successor", successor, err)
			}
			current := previous
			switch tc.position {
			case currentPredecessor:
			case currentSuccessor:
				current = successor
			case currentOtherWindow:
				current, err = controlplane.AdvanceUsageWatermark(previous, testWindow(unitsOf(1, 3), outcomesOf(1, 3)))
				if err != nil {
					t.Fatalf("AdvanceUsageWatermark(other window) error = %v, want nil", err)
				}
			case currentOtherChain:
				current = successor
				current.ChainDigest = corruptDigest()
			case currentOtherGeneration:
				current = successor
				current.Generation = previous.Generation
			default:
				t.Fatalf("fixture position = %d, want governed state", tc.position)
			}
			if tc.zeroProof {
				proof = controlplane.VerifiedCheckIn{}
			}
			if tc.primary == transitionContradiction && (current == previous || current == successor) {
				t.Fatalf("contradiction fixture = %v, want changed load-bearing current fact", current)
			}
			commit, gotErr := server.CommitCheckIn(controlplane.CheckInCommitRequest{CheckIn: proof, Current: current, RequiredPolicy: issued.request.Payload.AppliedPolicy})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("CommitCheckIn() error = %v, want %v", gotErr, tc.wantErr)
			}
			disposition, dispositionErr := commit.Disposition()
			watermark, watermarkErr := commit.Watermark()
			if tc.wantErr != nil {
				if disposition != controlplane.UsageDispositionUnknown || watermark != (controlplane.UsageWatermark{}) || !errors.Is(dispositionErr, tc.wantErr) || !errors.Is(watermarkErr, tc.wantErr) {
					t.Fatalf("refused output = (%v,%v,%v,%v), want zero and typed errors", disposition, watermark, dispositionErr, watermarkErr)
				}
				return
			}
			want := current
			if tc.wantDisposition == controlplane.UsageDispositionAccepted {
				want = successor
			}
			if dispositionErr != nil || watermarkErr != nil || disposition != tc.wantDisposition || watermark != want {
				t.Fatalf("commit output = (%v,%v,%v,%v), want (%v,%v,nil,nil)", disposition, watermark, dispositionErr, watermarkErr, tc.wantDisposition, want)
			}
			if disposition.AdvancesWatermark() != (tc.primary == transitionBoundary) {
				t.Fatalf("AdvancesWatermark() = %t, want advancement only for accepted predecessor", disposition.AdvancesWatermark())
			}
		})
	}
}
