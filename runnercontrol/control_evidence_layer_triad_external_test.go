package runnercontrol_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/standard"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestObservationDeliveryLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive control envelope independently closes runner signature experiment page and cleanup", func(t *testing.T) {
		t.Parallel()
		envelope, manifest, pages, controlKeys, runnerKeys := completedObservationDeliveryFixture(t)
		gotErr := runnercontrol.VerifyObservationDelivery(runnercontrol.ObservationDeliveryVerification{
			Stage: runnercontrol.ObservationDeliveryStage{SchemaVersion: runnercontrol.SchemaVersion, Envelope: envelope, Manifest: manifest},
			Pages: pages, ControlKeys: controlKeys, RunnerKeys: runnerKeys,
		})
		if gotErr != nil {
			t.Fatalf("VerifyObservationDelivery(completed) error = %v, want nil", gotErr)
		}
	})

	t.Run("negative one-byte delivery digest mutation is refused despite valid signatures", func(t *testing.T) {
		t.Parallel()
		envelope, manifest, pages, controlKeys, runnerKeys := completedObservationDeliveryFixture(t)
		manifest.Entries[0].Digest = core.SHA256Of([]byte("forged-completion"))
		gotErr := runnercontrol.VerifyObservationDelivery(runnercontrol.ObservationDeliveryVerification{
			Stage: runnercontrol.ObservationDeliveryStage{SchemaVersion: runnercontrol.SchemaVersion, Envelope: envelope, Manifest: manifest},
			Pages: pages, ControlKeys: controlKeys, RunnerKeys: runnerKeys,
		})
		if !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("VerifyObservationDelivery(forged digest) error = %v, want errors.Is(..., %v)", gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral pre-runner infrastructure failure carries no experiment pages or fabricated cleanup", func(t *testing.T) {
		t.Parallel()
		envelope, manifest, controlKeys, runnerKeys := preRunnerObservationDeliveryFixture(t)
		gotErr := runnercontrol.VerifyObservationDelivery(runnercontrol.ObservationDeliveryVerification{
			Stage: runnercontrol.ObservationDeliveryStage{SchemaVersion: runnercontrol.SchemaVersion, Envelope: envelope, Manifest: manifest},
			Pages: []runnercontrol.ExperimentDeliveryPage{}, ControlKeys: controlKeys, RunnerKeys: runnerKeys,
		})
		if gotErr != nil || manifest.PageCount != 0 || len(manifest.Entries) != 0 || envelope.Payload.Cleanup.Kind != runnercontrol.CleanupNotApplicable {
			t.Fatalf("VerifyObservationDelivery(pre-runner) = (%v, pages %d, entries %d, cleanup %v), want nil, 0, 0, not-applicable", gotErr, manifest.PageCount, len(manifest.Entries), envelope.Payload.Cleanup.Kind)
		}
	})
}

func completedObservationDeliveryFixture(t testing.TB) (runnercontrol.ObservationEnvelope, runnercontrol.ExperimentDeliveryManifest, []runnercontrol.ExperimentDeliveryPage, attest.TrustedKeys, attest.TrustedKeys) {
	t.Helper()
	runnerPayload := directRunnerCompletionPayloadFixture(t)
	runnerKey, runnerKeys := completionSignerFixture(t)
	runnerDocument, runnerIssueErr := runnercontrol.IssueRunnerCompletion(runnerPayload, runnerKey)
	experimentPayload := experimentCompletionPayloadFixture(t, true)
	experimentDocument, experimentIssueErr := runnercontrol.IssueExperimentCompletion(experimentPayload, runnerKey)
	experimentRecord, recordErr := runnercontrol.NewExperimentCompletionRecord(experimentDocument)
	entry := runnercontrol.ExperimentDeliveryEntry{Experiment: experimentPayload.Observation.Experiment, Probe: experimentPayload.Probe, Digest: experimentRecord.Digest, Bytes: experimentRecord.Bytes, Page: 1, Position: 0}
	manifest := runnercontrol.ExperimentDeliveryManifest{SchemaVersion: runnercontrol.SchemaVersion, Run: runnerPayload.Run, Entries: []runnercontrol.ExperimentDeliveryEntry{entry}, PageCount: 1}
	manifestDigest, manifestErr := manifest.Digest()
	cleanup := runnercontrol.CleanupOutcome{Kind: runnercontrol.CleanupSucceeded, ReceiptDigest: new(core.SHA256Of([]byte("cleanup-receipt")))}
	cleanupDigest, cleanupErr := cleanup.Digest()
	request, requestErr := standard.NewRequestIdentity(completionUUIDFixture(t))
	destination, destinationErr := standard.NewIdentifier("origin-observation-api")
	audience, audienceErr := standard.NewIdentifier("origin-runner")
	if err := errors.Join(runnerIssueErr, experimentIssueErr, recordErr, manifestErr, cleanupErr, requestErr, destinationErr, audienceErr); err != nil {
		t.Fatalf("completed observation delivery fixture error = %v, want nil", err)
	}
	payload := runnercontrol.ObservationEnvelopePayload{
		SchemaVersion: runnercontrol.SchemaVersion, Request: request, Run: runnerPayload.Run,
		AdmissionDigest: core.SHA256Of([]byte("admission")), Fence: runnerPayload.Fence, Members: runnerPayload.Members,
		Cleanup: cleanup, CleanupDigest: cleanupDigest, Origin: experimentPayload.Probe.Origin,
		DeliveryGrant: core.SHA256Of([]byte("delivery-grant")), Destination: destination, Audience: audience,
		Terminal: standard.TerminalCompleted, CapturedAt: temporal.InstantFromNanoseconds(5), ExperimentDeliveryManifest: manifestDigest,
		Evidence: runnercontrol.ObservationEvidenceBody{Kind: runnercontrol.EvidenceCompletedRunner, Completed: &runnerDocument},
	}
	controlKey, controlKeys := completionSignerFixture(t)
	envelope, envelopeErr := runnercontrol.IssueObservationEnvelope(payload, controlKey)
	if envelopeErr != nil {
		t.Fatalf("IssueObservationEnvelope(completed) fixture error = %v, want nil", envelopeErr)
	}
	page := runnercontrol.ExperimentDeliveryPage{SchemaVersion: runnercontrol.SchemaVersion, Run: runnerPayload.Run, Page: 1, Documents: []runnercontrol.ExperimentCompletionDocument{experimentDocument}}
	return envelope, manifest, []runnercontrol.ExperimentDeliveryPage{page}, controlKeys, runnerKeys
}

func preRunnerObservationDeliveryFixture(t testing.TB) (runnercontrol.ObservationEnvelope, runnercontrol.ExperimentDeliveryManifest, attest.TrustedKeys, attest.TrustedKeys) {
	t.Helper()
	completion := experimentCompletionPayloadFixture(t, false)
	manifest := runnercontrol.ExperimentDeliveryManifest{SchemaVersion: runnercontrol.SchemaVersion, Run: completion.Run, Entries: []runnercontrol.ExperimentDeliveryEntry{}, PageCount: 0}
	manifestDigest, manifestErr := manifest.Digest()
	cleanup := runnercontrol.CleanupOutcome{Kind: runnercontrol.CleanupNotApplicable}
	cleanupDigest, cleanupErr := cleanup.Digest()
	request, requestErr := standard.NewRequestIdentity(completionUUIDFixture(t))
	destination, destinationErr := standard.NewIdentifier("origin-observation-api")
	audience, audienceErr := standard.NewIdentifier("origin-runner")
	stage, stageErr := standard.NewIdentifier("source-acquisition")
	if err := errors.Join(manifestErr, cleanupErr, requestErr, destinationErr, audienceErr, stageErr); err != nil {
		t.Fatalf("pre-runner observation delivery fixture error = %v, want nil", err)
	}
	payload := runnercontrol.ObservationEnvelopePayload{
		SchemaVersion: runnercontrol.SchemaVersion, Request: request, Run: completion.Run,
		AdmissionDigest: core.SHA256Of([]byte("admission")), Fence: completion.Fence, Members: completion.Members,
		Cleanup: cleanup, CleanupDigest: cleanupDigest, Origin: completion.Probe.Origin,
		DeliveryGrant: core.SHA256Of([]byte("delivery-grant")), Destination: destination, Audience: audience,
		Terminal: standard.TerminalUnavailable, CapturedAt: temporal.InstantFromNanoseconds(5), ExperimentDeliveryManifest: manifestDigest,
		Evidence: runnercontrol.ObservationEvidenceBody{Kind: runnercontrol.EvidencePreRunnerInfrastructure, PreRunner: &runnercontrol.PreRunnerEvidence{Stage: stage, Failure: core.ErrFilestoreSource}},
	}
	controlKey, controlKeys := completionSignerFixture(t)
	envelope, envelopeErr := runnercontrol.IssueObservationEnvelope(payload, controlKey)
	if envelopeErr != nil {
		t.Fatalf("IssueObservationEnvelope(pre-runner) fixture error = %v, want nil", envelopeErr)
	}
	_, runnerKeys := completionSignerFixture(t)
	return envelope, manifest, controlKeys, runnerKeys
}
