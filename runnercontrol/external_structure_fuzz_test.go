package runnercontrol_test

import (
	"bytes"
	"crypto"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

type structureJSONValue interface {
	Validate() error
	MarshalJSON() ([]byte, error)
}

func FuzzRunnerControlExternalStructureJSONSemanticClosure(f *testing.F) {
	seeds := externalStructureSeeds(f)
	for index, seed := range seeds.encoded {
		f.Add(uint8(index), seed)
	}
	f.Add(uint8(0), []byte{})
	f.Add(uint8(len(seeds.encoded)-1), []byte(`{"schema_version":1}`))

	f.Fuzz(func(t *testing.T, selector uint8, data []byte) {
		switch int(selector) % len(seeds.encoded) {
		case 0:
			proveStructureJSONClosure(t, "AdmittedRun", seeds.admitted, data, (*runnercontrol.AdmittedRun).UnmarshalJSON)
		case 1:
			proveStructureJSONClosure(t, "ClaimRequest", seeds.claimRequest, data, (*runnercontrol.ClaimRequest).UnmarshalJSON)
		case 2:
			proveStructureJSONClosure(t, "ClaimResponse", seeds.claimResponse, data, (*runnercontrol.ClaimResponse).UnmarshalJSON)
		case 3:
			proveStructureJSONClosure(t, "HeartbeatRequest", seeds.heartbeatRequest, data, (*runnercontrol.HeartbeatRequest).UnmarshalJSON)
		case 4:
			proveStructureJSONClosure(t, "HeartbeatResponse", seeds.heartbeatResponse, data, (*runnercontrol.HeartbeatResponse).UnmarshalJSON)
		case 5:
			proveStructureJSONClosure(t, "SourceAcquisitionRequest", seeds.sourceRequest, data, (*runnercontrol.SourceAcquisitionRequest).UnmarshalJSON)
		case 6:
			proveStructureJSONClosure(t, "ExperimentCompletionPayload", seeds.experimentPayload, data, (*runnercontrol.ExperimentCompletionPayload).UnmarshalJSON)
		case 7:
			proveStructureJSONClosure(t, "ExperimentCompletionReceipt", seeds.experimentReceipt, data, (*runnercontrol.ExperimentCompletionReceipt).UnmarshalJSON)
		case 8:
			proveStructureJSONClosure(t, "RunnerCompletionPayload", seeds.runnerPayload, data, (*runnercontrol.RunnerCompletionPayload).UnmarshalJSON)
		case 9:
			proveStructureJSONClosure(t, "RunnerCompletionReceipt", seeds.runnerReceipt, data, (*runnercontrol.RunnerCompletionReceipt).UnmarshalJSON)
		case 10:
			proveStructureJSONClosure(t, "CleanupPayload", seeds.cleanupPayload, data, (*runnercontrol.CleanupPayload).UnmarshalJSON)
		case 11:
			proveStructureJSONClosure(t, "CleanupReceipt", seeds.cleanupReceipt, data, (*runnercontrol.CleanupReceipt).UnmarshalJSON)
		case 12:
			proveStructureJSONClosure(t, "ObservationEnvelopePayload", seeds.observationPayload, data, (*runnercontrol.ObservationEnvelopePayload).UnmarshalJSON)
		case 13:
			proveStructureJSONClosure(t, "ExperimentDeliveryPage", seeds.deliveryPage, data, (*runnercontrol.ExperimentDeliveryPage).UnmarshalJSON)
		case 14:
			proveStructureJSONClosure(t, "ExpansionManifest", seeds.expansionManifest, data, (*runnercontrol.ExpansionManifest).UnmarshalJSON)
		case 15:
			proveStructureJSONClosure(t, "ExpansionApproval", seeds.expansionApproval, data, (*runnercontrol.ExpansionApproval).UnmarshalJSON)
		case 16:
			proveStructureJSONClosure(t, "ArtifactManifestReceipt", seeds.artifactManifestReceipt, data, (*runnercontrol.ArtifactManifestReceipt).UnmarshalJSON)
		case 17:
			proveStructureJSONClosure(t, "ArtifactChunkReceipt", seeds.artifactChunkReceipt, data, (*runnercontrol.ArtifactChunkReceipt).UnmarshalJSON)
		case 18:
			proveStructureJSONClosure(t, "ObservationDeliveryReceipt", seeds.deliveryReceipt, data, (*runnercontrol.ObservationDeliveryReceipt).UnmarshalJSON)
		case 19:
			proveStructureJSONClosure(t, "MachineObservationReceipt", seeds.machineReceipt, data, (*runnercontrol.MachineObservationReceipt).UnmarshalJSON)
		case 20:
			proveStructureJSONClosure(t, "SchedulingCapability", seeds.schedulingCapability, data, (*runnercontrol.SchedulingCapability).UnmarshalJSON)
		case 21:
			proveStructureJSONClosure(t, "MemberCapability", seeds.memberCapability, data, (*runnercontrol.MemberCapability).UnmarshalJSON)
		case 22:
			proveStructureJSONClosure(t, "ExperimentCapability", seeds.experimentCapability, data, (*runnercontrol.ExperimentCapability).UnmarshalJSON)
		}
	})
}

type runnerControlStructureSeeds struct {
	admitted                runnercontrol.AdmittedRun
	claimRequest            runnercontrol.ClaimRequest
	claimResponse           runnercontrol.ClaimResponse
	heartbeatRequest        runnercontrol.HeartbeatRequest
	heartbeatResponse       runnercontrol.HeartbeatResponse
	sourceRequest           runnercontrol.SourceAcquisitionRequest
	experimentPayload       runnercontrol.ExperimentCompletionPayload
	experimentReceipt       runnercontrol.ExperimentCompletionReceipt
	runnerPayload           runnercontrol.RunnerCompletionPayload
	runnerReceipt           runnercontrol.RunnerCompletionReceipt
	cleanupPayload          runnercontrol.CleanupPayload
	cleanupReceipt          runnercontrol.CleanupReceipt
	observationPayload      runnercontrol.ObservationEnvelopePayload
	deliveryPage            runnercontrol.ExperimentDeliveryPage
	expansionManifest       runnercontrol.ExpansionManifest
	expansionApproval       runnercontrol.ExpansionApproval
	artifactManifestReceipt runnercontrol.ArtifactManifestReceipt
	artifactChunkReceipt    runnercontrol.ArtifactChunkReceipt
	deliveryReceipt         runnercontrol.ObservationDeliveryReceipt
	machineReceipt          runnercontrol.MachineObservationReceipt
	schedulingCapability    runnercontrol.SchedulingCapability
	memberCapability        runnercontrol.MemberCapability
	experimentCapability    runnercontrol.ExperimentCapability
	encoded                 [][]byte
}

func externalStructureSeeds(t testing.TB) runnerControlStructureSeeds {
	t.Helper()
	_, admitted := admissionFixture(t)
	experimentPayload := experimentCompletionPayloadFixture(t, true)
	runnerPayload := directRunnerCompletionPayloadFixture(t)
	cleanupPayload := cleanupPayloadFixture(t)
	envelope, _, pages, _, _ := completedObservationDeliveryFixture(t)
	expansionManifest := expansionManifestFixture(t, true)
	artifactManifest, artifactChunk := artifactFixture(t, []byte("receipt-evidence"))
	artifactRecord, artifactErr := runnercontrol.NewArtifactManifestRecord(artifactManifest)
	observationID, observationErr := projectstandards.NewMachineObservationID(completionUUIDFixture(t))
	if err := errors.Join(artifactErr, observationErr); err != nil {
		t.Fatalf("external structure seed identity error = %v, want nil", err)
	}
	experimentDocument, experimentErr := runnercontrol.IssueExperimentCompletion(experimentPayload, mustCompletionSigner(t))
	experimentRecord, experimentRecordErr := runnercontrol.NewExperimentCompletionRecord(experimentDocument)
	runnerDocument, runnerErr := runnercontrol.IssueRunnerCompletion(runnerPayload, mustCompletionSigner(t))
	runnerRecord, runnerRecordErr := runnercontrol.NewRunnerCompletionRecord(runnerDocument)
	cleanupDocument, cleanupErr := runnercontrol.IssueCleanup(cleanupPayload, mustCompletionSigner(t))
	cleanupRecord, cleanupRecordErr := runnercontrol.NewCleanupRecord(cleanupDocument)
	if err := errors.Join(experimentErr, experimentRecordErr, runnerErr, runnerRecordErr, cleanupErr, cleanupRecordErr); err != nil {
		t.Fatalf("external structure signed seed setup error = %v, want nil", err)
	}
	fence := experimentPayload.Fence.Machine
	claimRequest := runnercontrol.ClaimRequest{SchemaVersion: runnercontrol.SchemaVersion, Machine: fence.Machine, Generation: fence.Generation, Observation: observationID, RequestedAt: temporal.InstantFromNanoseconds(1)}
	claimResponse := runnercontrol.ClaimResponse{SchemaVersion: runnercontrol.SchemaVersion, Kind: runnercontrol.ClaimWait, Fence: fence}
	heartbeatRequest := runnercontrol.HeartbeatRequest{SchemaVersion: runnercontrol.SchemaVersion, Observation: observationID, Fence: fence, State: runnercontrol.HeartbeatReady, ActiveRuns: []projectstandards.RunID{}, ObservedAt: temporal.InstantFromNanoseconds(1)}
	heartbeatResponse := runnercontrol.HeartbeatResponse{SchemaVersion: runnercontrol.SchemaVersion, Fence: fence, Directive: runnercontrol.Directive{Kind: runnercontrol.DirectiveContinue}, NextAt: temporal.InstantFromNanoseconds(2)}
	sourceRequest := runnercontrol.SourceAcquisitionRequest{SchemaVersion: runnercontrol.SchemaVersion, Fence: experimentPayload.Fence, Members: experimentPayload.Members, Source: experimentPayload.Probe.Source, Grant: runnercontrol.SourceGrantIdentity{Digest: core.SHA256Of([]byte("source-grant"))}, RequestedAt: temporal.InstantFromNanoseconds(1)}
	experimentReceipt := runnercontrol.ExperimentCompletionReceipt{SchemaVersion: runnercontrol.SchemaVersion, Run: experimentPayload.Run, Experiment: experimentPayload.Observation.Experiment, Digest: experimentRecord.Digest, Bytes: experimentRecord.Bytes}
	runnerReceipt := runnercontrol.RunnerCompletionReceipt{SchemaVersion: runnercontrol.SchemaVersion, Run: runnerPayload.Run, Digest: runnerRecord.Digest, Bytes: runnerRecord.Bytes}
	cleanupReceipt := runnercontrol.CleanupReceipt{SchemaVersion: runnercontrol.SchemaVersion, Fence: cleanupPayload.Fence, Digest: cleanupRecord.Digest, Bytes: cleanupRecord.Bytes}
	expansionApproval := expansionApprovalSeed(t, expansionManifest, mustCompletionSigner(t))
	artifactManifestReceipt := runnercontrol.ArtifactManifestReceipt{SchemaVersion: runnercontrol.SchemaVersion, Run: artifactManifest.Run, Digest: artifactRecord.Digest, Bytes: artifactRecord.Bytes}
	artifactChunkReceipt := runnercontrol.ArtifactChunkReceipt{SchemaVersion: runnercontrol.SchemaVersion, Run: artifactChunk.Run, Manifest: artifactChunk.ManifestDigest, Artifact: artifactChunk.Entry.Digest, Committed: artifactChunk.Entry.Bytes, Complete: true}
	stage, _, _ := deliveryProtocolFixture(t)
	deliveryIdentity, identityErr := stage.Identity()
	if identityErr != nil {
		t.Fatalf("ObservationDeliveryStage.Identity(seed) error = %v, want nil", identityErr)
	}
	deliveryReceipt := runnercontrol.ObservationDeliveryReceipt{SchemaVersion: runnercontrol.SchemaVersion, Identity: deliveryIdentity, Run: stage.Envelope.Payload.Run, PagesStored: uint16(len(pages)), Published: true}
	machineReceipt := runnercontrol.MachineObservationReceipt{SchemaVersion: runnercontrol.SchemaVersion, ObservationID: observationID, CleanDigest: core.SHA256Of([]byte("clean-state"))}
	schedulingClaim, _ := schedulingClaimDocumentFixture(t)
	seeds := runnerControlStructureSeeds{
		admitted: admitted, claimRequest: claimRequest, claimResponse: claimResponse,
		heartbeatRequest: heartbeatRequest, heartbeatResponse: heartbeatResponse, sourceRequest: sourceRequest,
		experimentPayload: experimentPayload, experimentReceipt: experimentReceipt, runnerPayload: runnerPayload, runnerReceipt: runnerReceipt,
		cleanupPayload: cleanupPayload, cleanupReceipt: cleanupReceipt, observationPayload: envelope.Payload, deliveryPage: pages[0],
		expansionManifest: expansionManifest, expansionApproval: expansionApproval,
		artifactManifestReceipt: artifactManifestReceipt, artifactChunkReceipt: artifactChunkReceipt,
		deliveryReceipt: deliveryReceipt, machineReceipt: machineReceipt,
		schedulingCapability: schedulingClaim.Capability.Payload,
		memberCapability:     schedulingClaim.Members[0].Payload,
		experimentCapability: schedulingClaim.Direct[0].Payload,
	}
	seeds.encoded = encodeStructureSeeds(t, seeds)
	return seeds
}

func mustCompletionSigner(t testing.TB) crypto.Signer {
	key, _ := completionSignerFixture(t)
	return key
}

func expansionApprovalSeed(t testing.TB, manifest runnercontrol.ExpansionManifest, signer crypto.Signer) runnercontrol.ExpansionApproval {
	t.Helper()
	capability := experimentObservationRequestFixture(t).Capability
	child := manifest.Children[0]
	capability.Fence = manifest.Fence
	capability.Run = manifest.Run
	capability.Experiment = *child.Experiment
	capability.Probe = child.Probe
	capability.Source = manifest.Source
	capability.BuildContextDigest = child.BuildContextDigest
	manifestDigest, digestErr := manifest.Digest()
	if digestErr != nil {
		t.Fatalf("ExpansionManifest.Digest(approval seed) error = %v, want nil", digestErr)
	}
	capability.ExpansionManifestDigest = &manifestDigest
	capability.ExpiresAt = manifest.Fence.Machine.ExpiresAt
	document, issueErr := runnercontrol.IssueExperimentCapability(capability, signer)
	if issueErr != nil {
		t.Fatalf("IssueExperimentCapability(expansion approval seed) error = %v, want nil", issueErr)
	}
	approval := runnercontrol.ExpansionApproval{SchemaVersion: runnercontrol.SchemaVersion, Run: manifest.Run, ManifestDigest: manifestDigest, Approved: true, Experiments: []runnercontrol.ExperimentCapabilityDocument{document}}
	if err := approval.Validate(); err != nil {
		t.Fatalf("ExpansionApproval.Validate(seed) error = %v, want nil", err)
	}
	return approval
}

func encodeStructureSeeds(t testing.TB, seeds runnerControlStructureSeeds) [][]byte {
	t.Helper()
	values := []structureJSONValue{
		seeds.admitted, seeds.claimRequest, seeds.claimResponse, seeds.heartbeatRequest, seeds.heartbeatResponse,
		seeds.sourceRequest, seeds.experimentPayload, seeds.experimentReceipt, seeds.runnerPayload, seeds.runnerReceipt,
		seeds.cleanupPayload, seeds.cleanupReceipt, seeds.observationPayload, seeds.deliveryPage, seeds.expansionManifest,
		seeds.expansionApproval, seeds.artifactManifestReceipt, seeds.artifactChunkReceipt, seeds.deliveryReceipt, seeds.machineReceipt,
		seeds.schedulingCapability, seeds.memberCapability, seeds.experimentCapability,
	}
	encoded := make([][]byte, len(values))
	for index := range values {
		value, err := values[index].MarshalJSON()
		if err != nil {
			t.Fatalf("external structure selector %d MarshalJSON(seed) error = %v, want nil", index, err)
		}
		encoded[index] = value
	}
	return encoded
}

func proveStructureJSONClosure[T structureJSONValue](t *testing.T, name string, seed T, data []byte, decode func(*T, []byte) error) {
	t.Helper()
	seedBytes, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("%s.MarshalJSON(seed) error = %v, want nil", name, err)
	}
	got := seed
	gotErr := decode(&got, data)
	if gotErr != nil {
		gotBytes, marshalErr := got.MarshalJSON()
		if !errors.Is(gotErr, core.ErrJSONContract) || marshalErr != nil || !bytes.Equal(gotBytes, seedBytes) {
			t.Fatalf("%s.UnmarshalJSON(rejected) = (receiver %q, marshal error %v, error %v), want preserved %q and errors.Is(..., %v)", name, gotBytes, marshalErr, gotErr, seedBytes, core.ErrJSONContract)
		}
		return
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("%s.UnmarshalJSON(accepted).Validate() error = %v, want nil", name, err)
	}
	encoded, err := got.MarshalJSON()
	if err != nil {
		t.Fatalf("%s.MarshalJSON(accepted) error = %v, want nil", name, err)
	}
	var roundTrip T
	if err := decode(&roundTrip, encoded); err != nil {
		t.Fatalf("%s canonical round trip error = %v, want nil", name, err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, encoded) {
		t.Fatalf("%s second canonical projection = (%q, %v), want (%q, nil)", name, second, err, encoded)
	}
}
