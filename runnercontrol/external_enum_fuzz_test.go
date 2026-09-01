package runnercontrol_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

type enumJSONValue interface {
	comparable
	Validate() error
	MarshalJSON() ([]byte, error)
}

func FuzzRunnerControlExternalEnumJSONSemanticClosure(f *testing.F) {
	buildTag, err := runnercontrol.NewGoBuildTag("go1.27")
	if err != nil {
		f.Fatalf("NewGoBuildTag(seed) error = %v, want nil", err)
	}
	addEnumSeed(f, 0, runnercontrol.HeartbeatReady)
	addEnumSeed(f, 1, runnercontrol.DirectiveContinue)
	addEnumSeed(f, 2, runnercontrol.GoProfileFocused)
	addEnumSeed(f, 3, runnercontrol.CoverageSet)
	addEnumSeed(f, 4, runnercontrol.SourceSigningDomainArchiveV1)
	addEnumSeed(f, 5, runnercontrol.ClaimWait)
	addEnumSeed(f, 6, runnercontrol.PeerRoleOrigin)
	addEnumSeed(f, 7, runnercontrol.SchedulingUnitRunPlan)
	addEnumSeed(f, 8, buildTag)
	addEnumSeed(f, 9, runnercontrol.GoInstrumentationOrdinary)
	addEnumSeed(f, 10, runnercontrol.GoModuleModeModule)
	addEnumSeed(f, 11, runnercontrol.ExpansionAdmitted)
	addEnumSeed(f, 12, runnercontrol.CompletionSigningDomainExperimentV1)
	addEnumSeed(f, 13, runnercontrol.ArtifactStdout)
	addEnumSeed(f, 14, runnercontrol.NetworkTCP)
	addEnumSeed(f, 15, runnercontrol.EgressDenied)
	addEnumSeed(f, 16, runnercontrol.EvidenceSigningDomainObservationV1)
	addEnumSeed(f, 17, runnercontrol.CapabilitySigningDomainSchedulingV1)
	addEnumSeed(f, 17, runnercontrol.CapabilitySigningDomainMemberV1)
	addEnumSeed(f, 17, runnercontrol.CapabilitySigningDomainExperimentV1)
	addEnumSeed(f, 18, runnercontrol.CleanupSucceeded)
	addEnumSeed(f, 19, runnercontrol.EvidenceCompletedRunner)
	addEnumSeed(f, 20, runnercontrol.ObservationGoTestJSON)
	addEnumSeed(f, 20, runnercontrol.ObservationJUnitXML)
	addEnumSeed(f, 21, runnercontrol.SubjectIsolationSystemd)
	addEnumSeed(f, 22, runnercontrol.RunControlQueued)
	f.Add(uint8(0), []byte{})
	f.Add(uint8(22), []byte(`"future"`))

	f.Fuzz(func(t *testing.T, selector uint8, data []byte) {
		switch selector % 23 {
		case 0:
			proveEnumJSONClosure(t, "HeartbeatState", runnercontrol.HeartbeatReady, data, (*runnercontrol.HeartbeatState).UnmarshalJSON)
		case 1:
			proveEnumJSONClosure(t, "DirectiveKind", runnercontrol.DirectiveContinue, data, (*runnercontrol.DirectiveKind).UnmarshalJSON)
		case 2:
			proveEnumJSONClosure(t, "GoProfileKind", runnercontrol.GoProfileFocused, data, (*runnercontrol.GoProfileKind).UnmarshalJSON)
		case 3:
			proveEnumJSONClosure(t, "CoverageMode", runnercontrol.CoverageSet, data, (*runnercontrol.CoverageMode).UnmarshalJSON)
		case 4:
			proveEnumJSONClosure(t, "SourceSigningDomain", runnercontrol.SourceSigningDomainArchiveV1, data, (*runnercontrol.SourceSigningDomain).UnmarshalJSON)
		case 5:
			proveEnumJSONClosure(t, "ClaimKind", runnercontrol.ClaimWait, data, (*runnercontrol.ClaimKind).UnmarshalJSON)
		case 6:
			proveEnumJSONClosure(t, "PeerRole", runnercontrol.PeerRoleOrigin, data, (*runnercontrol.PeerRole).UnmarshalJSON)
		case 7:
			proveEnumJSONClosure(t, "SchedulingUnitKind", runnercontrol.SchedulingUnitRunPlan, data, (*runnercontrol.SchedulingUnitKind).UnmarshalJSON)
		case 8:
			proveEnumJSONClosure(t, "GoBuildTag", buildTag, data, (*runnercontrol.GoBuildTag).UnmarshalJSON)
		case 9:
			proveEnumJSONClosure(t, "GoInstrumentation", runnercontrol.GoInstrumentationOrdinary, data, (*runnercontrol.GoInstrumentation).UnmarshalJSON)
		case 10:
			proveEnumJSONClosure(t, "GoModuleMode", runnercontrol.GoModuleModeModule, data, (*runnercontrol.GoModuleMode).UnmarshalJSON)
		case 11:
			proveEnumJSONClosure(t, "ExpansionDisposition", runnercontrol.ExpansionAdmitted, data, (*runnercontrol.ExpansionDisposition).UnmarshalJSON)
		case 12:
			proveEnumJSONClosure(t, "CompletionSigningDomain", runnercontrol.CompletionSigningDomainExperimentV1, data, (*runnercontrol.CompletionSigningDomain).UnmarshalJSON)
		case 13:
			proveEnumJSONClosure(t, "ArtifactKind", runnercontrol.ArtifactStdout, data, (*runnercontrol.ArtifactKind).UnmarshalJSON)
		case 14:
			proveEnumJSONClosure(t, "NetworkProtocol", runnercontrol.NetworkTCP, data, (*runnercontrol.NetworkProtocol).UnmarshalJSON)
		case 15:
			proveEnumJSONClosure(t, "EgressMode", runnercontrol.EgressDenied, data, (*runnercontrol.EgressMode).UnmarshalJSON)
		case 16:
			proveEnumJSONClosure(t, "EvidenceSigningDomain", runnercontrol.EvidenceSigningDomainObservationV1, data, (*runnercontrol.EvidenceSigningDomain).UnmarshalJSON)
		case 17:
			proveEnumJSONClosure(t, "CapabilitySigningDomain", runnercontrol.CapabilitySigningDomainSchedulingV1, data, (*runnercontrol.CapabilitySigningDomain).UnmarshalJSON)
		case 18:
			proveEnumJSONClosure(t, "CleanupOutcomeKind", runnercontrol.CleanupSucceeded, data, (*runnercontrol.CleanupOutcomeKind).UnmarshalJSON)
		case 19:
			proveEnumJSONClosure(t, "EvidenceBodyKind", runnercontrol.EvidenceCompletedRunner, data, (*runnercontrol.EvidenceBodyKind).UnmarshalJSON)
		case 20:
			proveEnumJSONClosure(t, "ObservationFormat", runnercontrol.ObservationGoTestJSON, data, (*runnercontrol.ObservationFormat).UnmarshalJSON)
		case 21:
			proveEnumJSONClosure(t, "SubjectIsolationEngine", runnercontrol.SubjectIsolationSystemd, data, (*runnercontrol.SubjectIsolationEngine).UnmarshalJSON)
		case 22:
			proveEnumJSONClosure(t, "RunControlState", runnercontrol.RunControlQueued, data, (*runnercontrol.RunControlState).UnmarshalJSON)
		}
	})
}

func addEnumSeed[T enumJSONValue](f *testing.F, selector uint8, value T) {
	f.Helper()
	encoded, err := value.MarshalJSON()
	if err != nil {
		f.Fatalf("enum selector %d MarshalJSON(seed) error = %v, want nil", selector, err)
	}
	f.Add(selector, encoded)
}

func proveEnumJSONClosure[T enumJSONValue](t *testing.T, name string, seed T, data []byte, decode func(*T, []byte) error) {
	t.Helper()
	got := seed
	gotErr := decode(&got, data)
	if gotErr != nil {
		if !errors.Is(gotErr, core.ErrJSONContract) || got != seed {
			t.Fatalf("%s.UnmarshalJSON(rejected) = (%v, %v), want preserved %v and errors.Is(..., %v)", name, got, gotErr, seed, core.ErrJSONContract)
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
	if err := decode(&roundTrip, encoded); err != nil || roundTrip != got {
		t.Fatalf("%s canonical round trip = (%v, %v), want (%v, nil)", name, roundTrip, err, got)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, encoded) {
		t.Fatalf("%s second canonical projection = (%q, %v), want (%q, nil)", name, second, err, encoded)
	}
}
