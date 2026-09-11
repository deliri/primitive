package runnercontrol_test

import (
	"bytes"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"testing"
)

func TestRunnerControlNominalJSONHasNoWhitespaceExtentQuota(t *testing.T) {
	t.Parallel()
	seeds := externalStructureSeeds(t)
	t.Run("AdmittedRun", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.admitted, (*runnercontrol.AdmittedRun).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("ClaimRequest", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.claimRequest, (*runnercontrol.ClaimRequest).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("ClaimResponse", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.claimResponse, (*runnercontrol.ClaimResponse).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("HeartbeatRequest", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.heartbeatRequest, (*runnercontrol.HeartbeatRequest).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("HeartbeatResponse", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.heartbeatResponse, (*runnercontrol.HeartbeatResponse).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("SourceAcquisitionRequest", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.sourceRequest, (*runnercontrol.SourceAcquisitionRequest).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("ExperimentCompletionPayload", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.experimentPayload, (*runnercontrol.ExperimentCompletionPayload).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("ExperimentCompletionReceipt", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.experimentReceipt, (*runnercontrol.ExperimentCompletionReceipt).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("RunnerCompletionPayload", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.runnerPayload, (*runnercontrol.RunnerCompletionPayload).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("RunnerCompletionReceipt", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.runnerReceipt, (*runnercontrol.RunnerCompletionReceipt).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("CleanupPayload", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.cleanupPayload, (*runnercontrol.CleanupPayload).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("CleanupReceipt", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.cleanupReceipt, (*runnercontrol.CleanupReceipt).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("ObservationEnvelopePayload", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.observationPayload, (*runnercontrol.ObservationEnvelopePayload).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("ExperimentDeliveryPage", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.deliveryPage, (*runnercontrol.ExperimentDeliveryPage).UnmarshalJSON, (8<<20)+1)
	})
	t.Run("ExpansionManifest", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.expansionManifest, (*runnercontrol.ExpansionManifest).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("ExpansionApproval", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.expansionApproval, (*runnercontrol.ExpansionApproval).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("ArtifactManifestReceipt", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.artifactManifestReceipt, (*runnercontrol.ArtifactManifestReceipt).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("ArtifactChunkReceipt", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.artifactChunkReceipt, (*runnercontrol.ArtifactChunkReceipt).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("ObservationDeliveryReceipt", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.deliveryReceipt, (*runnercontrol.ObservationDeliveryReceipt).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("MachineObservationReceipt", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.machineReceipt, (*runnercontrol.MachineObservationReceipt).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("SchedulingCapability", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.schedulingCapability, (*runnercontrol.SchedulingCapability).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("MemberCapability", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.memberCapability, (*runnercontrol.MemberCapability).UnmarshalJSON, (1<<20)+1)
	})
	t.Run("ExperimentCapability", func(t *testing.T) {
		t.Parallel()
		proveNominalJSONExtent(t, seeds.experimentCapability, (*runnercontrol.ExperimentCapability).UnmarshalJSON, (1<<20)+1)
	})
}

// These APIs explicitly accept and return caller-owned complete nominal values.
// They do not claim fixed-memory parsing of an arbitrarily large document.
func proveNominalJSONExtent[T structureJSONValue](t *testing.T, seed T, decode func(*T, []byte) error, paddingBytes int) {
	t.Helper()
	canonical, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON(seed) error = %v, want nil", err)
	}
	data := append(bytes.Repeat([]byte(" "), paddingBytes), canonical...)
	var got T
	if err := decode(&got, data); err != nil {
		t.Fatalf("UnmarshalJSON(padded nominal value) error = %v, want nil", err)
	}
	encoded, err := got.MarshalJSON()
	if err != nil || !bytes.Equal(encoded, canonical) {
		t.Fatalf("MarshalJSON(round trip) = %q/%v, want canonical %q/nil", encoded, err, canonical)
	}
}
