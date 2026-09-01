package projectstandards

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestExperimentOutcomeTerminalRelationshipExhaustive(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		outcome  Outcome
		terminal TerminalState
		wantErr  error
	}{
		{name: "unknown outcome never becomes a terminal fact", outcome: OutcomeUnknown, terminal: TerminalCompleted, wantErr: core.ErrProjectStandardsContract},
		{name: "passed experiment closes as completed", outcome: OutcomePassed, terminal: TerminalCompleted},
		{name: "failed assertion closes as completed execution", outcome: OutcomeFailed, terminal: TerminalCompleted},
		{name: "skipped experiment closes as completed accounting", outcome: OutcomeSkipped, terminal: TerminalCompleted},
		{name: "unavailable experiment retains unavailable terminal", outcome: OutcomeUnavailable, terminal: TerminalUnavailable},
		{name: "timed out experiment retains timed out terminal", outcome: OutcomeTimedOut, terminal: TerminalTimedOut},
		{name: "cancelled experiment retains cancelled terminal", outcome: OutcomeCancelled, terminal: TerminalCancelled},
		{name: "setup failure closes as completed execution fact", outcome: OutcomeSetupFailed, terminal: TerminalCompleted},
		{name: "infrastructure failure retains failed terminal", outcome: OutcomeInfrastructureFailed, terminal: TerminalFailed},
		{name: "not run experiment retains not run terminal", outcome: OutcomeNotRun, terminal: TerminalNotRun},
		{name: "nonaccepting result closes as completed execution fact", outcome: OutcomeNonAccepting, terminal: TerminalCompleted},
		{name: "one fact terminal mutation cannot contradict passed outcome", outcome: OutcomePassed, terminal: TerminalFailed, wantErr: core.ErrProjectStandardsConflict},
	}

	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			t.Parallel()
			mapped, mappingErr := TerminalForOutcome(current.outcome)
			if current.outcome == OutcomeUnknown {
				if mapped != TerminalUnknown || !errors.Is(mappingErr, core.ErrProjectStandardsContract) {
					t.Fatalf("TerminalForOutcome(%v) = (%v, %v), want (%v, errors.Is(..., %v))",
						current.outcome, mapped, mappingErr, TerminalUnknown, core.ErrProjectStandardsContract)
				}
			} else if mappingErr != nil || !experimentTerminalMatches(current.outcome, mapped) {
				t.Fatalf("TerminalForOutcome(%v) = (%v, %v), want its admitted terminal and nil",
					current.outcome, mapped, mappingErr)
			}
			got := fixtureExperimentObservationReference(t, current.outcome, current.terminal)
			gotErr := got.Validate()
			if current.wantErr == nil && gotErr != nil {
				t.Fatalf("ProjectStandardsObservationReference.Validate() error = %v, want nil", gotErr)
			}
			if current.wantErr != nil && !errors.Is(gotErr, current.wantErr) {
				t.Fatalf("ProjectStandardsObservationReference.Validate() error = %v, want %v", gotErr, current.wantErr)
			}
		})
	}
}

func TestObservationAdmissionBindingRejectsOneFactTargetMutation(t *testing.T) {
	t.Parallel()

	request, observation := fixtureAdmittedObservation(t)
	if gotErr := observation.ValidateFor(request); gotErr != nil {
		t.Fatalf("ProjectStandardsObservationReference.ValidateFor(exact admission) error = %v, want nil", gotErr)
	}
	mutated := observation
	target := *mutated.Probe.Target.GoDeclaration
	target.Symbol = fixtureName(t, "TestForeign")
	mutated.Probe.Target.GoDeclaration = &target
	gotErr := mutated.ValidateFor(request)
	if !errors.Is(gotErr, core.ErrProjectStandardsConflict) {
		t.Fatalf("ProjectStandardsObservationReference.ValidateFor(mutated target) error = %v, want %v", gotErr, core.ErrProjectStandardsConflict)
	}
}

func TestRequestAdmissionBindingRejectsOneFactEnvironmentMutation(t *testing.T) {
	t.Parallel()

	baseline, _ := fixtureAdmittedObservation(t)
	surface := ProjectStandardsEvidenceSurface{
		ID: baseline.SurfaceID, Subject: baseline.Requested.Subject, Target: baseline.Requested.Target,
		EligibleKinds: baseline.Requested.Kinds, Profiles: []ProfileIdentity{baseline.Requested.Profile},
		Placement: ReportPlacementPackage,
	}
	if gotErr := baseline.ValidateFor(surface); gotErr != nil {
		t.Fatalf("ProjectStandardsRequestReference.ValidateFor(exact environment) error = %v, want nil", gotErr)
	}
	mutated := baseline
	admission := *baseline.Disposition.Admitted
	admission.Probe.Environment.RequirementFingerprint = core.NewSHA256Digest([core.SHA256DigestBytes]byte{3})
	mutated.Disposition.Admitted = &admission
	gotErr := mutated.ValidateFor(surface)
	if !errors.Is(gotErr, core.ErrProjectStandardsConflict) {
		t.Fatalf("ProjectStandardsRequestReference.ValidateFor(mutated admitted environment) error = %v, want %v", gotErr, core.ErrProjectStandardsConflict)
	}
}

func TestObservationIndependentAuthorityAndEnvironmentBindings(t *testing.T) {
	t.Parallel()

	_, baseline := fixtureAdmittedObservation(t)
	if gotErr := baseline.Validate(); gotErr != nil {
		t.Fatalf("ProjectStandardsObservationReference.Validate(exact authorities and environment) error = %v, want nil", gotErr)
	}

	t.Run("subject cannot verify its own observation", func(t *testing.T) {
		t.Parallel()
		mutated := baseline
		mutated.Verifier = EvidenceAuthority{Offering: mutated.Probe.Subject.Project}
		gotErr := mutated.Validate()
		if !errors.Is(gotErr, core.ErrProjectStandardsConflict) {
			t.Fatalf("ProjectStandardsObservationReference.Validate(self verifier) error = %v, want %v", gotErr, core.ErrProjectStandardsConflict)
		}
	})

	t.Run("experiment environment cannot differ from admitted probe", func(t *testing.T) {
		t.Parallel()
		mutated := baseline
		experiment := *baseline.Experiment
		experiment.EnvironmentFingerprint = core.NewSHA256Digest([core.SHA256DigestBytes]byte{2})
		mutated.Experiment = &experiment
		gotErr := mutated.Validate()
		if !errors.Is(gotErr, core.ErrProjectStandardsConflict) {
			t.Fatalf("ProjectStandardsObservationReference.Validate(mutated environment) error = %v, want %v", gotErr, core.ErrProjectStandardsConflict)
		}
	})
}

func fixtureExperimentObservationReference(t testing.TB, outcome Outcome, terminal TerminalState) ProjectStandardsObservationReference {
	t.Helper()
	_, observation := fixtureAdmittedObservation(t)
	observation.Experiment.Outcome = outcome
	observation.Terminal = terminal
	if outcome == OutcomeNotRun {
		observation.Experiment.Started = false
		observation.Experiment.Measurements = ExperimentMeasurements{}
		observation.Experiment.Artifacts = nil
	}
	return observation
}

func fixtureAdmittedObservation(t testing.TB) (ProjectStandardsRequestReference, ProjectStandardsObservationReference) {
	t.Helper()
	uuid := fixtureProjectStandardsUUID(t)
	requestID, requestErr := NewRequestIdentity(uuid)
	if requestErr != nil {
		t.Fatalf("NewRequestIdentity() setup error = %v, want nil", requestErr)
	}
	runID, runErr := NewRunID(uuid)
	if runErr != nil {
		t.Fatalf("NewRunID() setup error = %v, want nil", runErr)
	}
	experimentID, experimentErr := NewExperimentID(uuid)
	if experimentErr != nil {
		t.Fatalf("NewExperimentID() setup error = %v, want nil", experimentErr)
	}
	observationID, observationErr := NewObservationID(uuid)
	if observationErr != nil {
		t.Fatalf("NewObservationID() setup error = %v, want nil", observationErr)
	}
	digest := core.NewSHA256Digest([core.SHA256DigestBytes]byte{1})
	commit := fixtureCommit(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	repository := fixtureRepository(t, "github.com/deliri/primitive")
	subject := SubjectIdentity{Project: core.Offering{Token: "primitive"}, Repository: repository}
	source := SourceCoordinate{Repository: repository, Commit: commit, Tree: digest}
	target := ProbeTarget{
		Kind: ProbeTargetGoDeclaration,
		GoDeclaration: &GoDeclarationTarget{
			Module: fixtureIdentifier(t, "primitive"), Package: fixturePath(t, "projectstandards"),
			File: fixturePath(t, "projectstandards/evidence.go"), Symbol: fixtureName(t, "TestEvidence"),
		},
	}
	requirement := EnvironmentRequirement{MachineClass: fixtureIdentifier(t, "runner-linux"), Fingerprint: digest}
	generationID, generationErr := NewMachineGenerationID(uuid)
	if generationErr != nil {
		t.Fatalf("NewMachineGenerationID() setup error = %v, want nil", generationErr)
	}
	environment := AdmittedEnvironment{
		MachineClass: requirement.MachineClass, RequirementFingerprint: requirement.Fingerprint,
		EnvironmentFingerprint: digest, MachineGeneration: generationID, MachineSheetDigest: digest,
	}
	origin := OriginIdentity{Offering: core.Offering{Token: "origin"}}
	profile := fixtureProfile(t, "acceptance")
	requested := RequestedProbe{
		Origin: origin, Subject: subject, Source: source, Target: target,
		Kinds: []ProbeKind{ProbeKindGoTest}, Profile: profile, Constraints: requirement,
	}
	probe, probeErr := AdmitRequestedProbe(requested, environment)
	if probeErr != nil {
		t.Fatalf("AdmitRequestedProbe(exact environment) error = %v, want nil", probeErr)
	}
	status, statusErr := core.ParseHTTPEndpoint("https://control.example/runs/1")
	if statusErr != nil {
		t.Fatalf("core.ParseHTTPEndpoint() setup error = %v, want nil", statusErr)
	}
	request := ProjectStandardsRequestReference{
		SurfaceID: fixtureIdentifier(t, "package-proof"), Request: requestID, Source: source, Requested: requested,
		Disposition: RequestDisposition{Kind: DispositionAdmitted, Admitted: &Admission{Run: runID, Probe: probe, Status: status}},
	}
	observation := ProjectStandardsObservationReference{
		Observation: observationID,
		Producer:    EvidenceAuthority{Offering: core.Offering{Token: "runner"}},
		Verifier:    EvidenceAuthority{Offering: core.Offering{Token: "primitive-control"}},
		Kind:        ObservationExperiment, Request: requestID, Run: runID,
		Source: source, EnvelopeDigest: digest, Probe: probe, Terminal: TerminalCompleted,
		CapturedAt: temporal.InstantFromNanoseconds(2_000_000),
		Experiment: &ExperimentObservation{
			Experiment: experimentID, Started: true, Outcome: OutcomePassed,
			EnvironmentFingerprint: digest, ExecutionFingerprint: digest, MachineSheetDigest: digest,
			Measurements: ExperimentMeasurements{DurationNs: 1},
		},
	}
	return request, observation
}
