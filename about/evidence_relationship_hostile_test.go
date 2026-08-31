package about

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
		{name: "unknown outcome never becomes a terminal fact", outcome: OutcomeUnknown, terminal: TerminalCompleted, wantErr: core.ErrAboutContract},
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
		{name: "one fact terminal mutation cannot contradict passed outcome", outcome: OutcomePassed, terminal: TerminalFailed, wantErr: core.ErrAboutConflict},
	}

	for _, current := range cases {
		current := current
		t.Run(current.name, func(t *testing.T) {
			t.Parallel()
			got := fixtureExperimentObservationReference(t, current.outcome, current.terminal)
			gotErr := got.Validate()
			if current.wantErr == nil && gotErr != nil {
				t.Fatalf("AboutObservationReference.Validate() error = %v, want nil", gotErr)
			}
			if current.wantErr != nil && !errors.Is(gotErr, current.wantErr) {
				t.Fatalf("AboutObservationReference.Validate() error = %v, want %v", gotErr, current.wantErr)
			}
		})
	}
}

func TestObservationAdmissionBindingRejectsOneFactTargetMutation(t *testing.T) {
	t.Parallel()

	request, observation := fixtureAdmittedObservation(t)
	if gotErr := observation.ValidateFor(request); gotErr != nil {
		t.Fatalf("AboutObservationReference.ValidateFor(exact admission) error = %v, want nil", gotErr)
	}
	mutated := observation
	target := *mutated.Probe.Target.GoDeclaration
	target.Symbol = fixtureName(t, "TestForeign")
	mutated.Probe.Target.GoDeclaration = &target
	gotErr := mutated.ValidateFor(request)
	if !errors.Is(gotErr, core.ErrAboutConflict) {
		t.Fatalf("AboutObservationReference.ValidateFor(mutated target) error = %v, want %v", gotErr, core.ErrAboutConflict)
	}
}

func TestRequestAdmissionBindingRejectsOneFactEnvironmentMutation(t *testing.T) {
	t.Parallel()

	baseline, _ := fixtureAdmittedObservation(t)
	surface := AboutEvidenceSurface{
		ID: baseline.SurfaceID, Subject: baseline.Requested.Subject, Target: baseline.Requested.Target,
		EligibleKinds: baseline.Requested.Kinds, Profiles: []ProfileIdentity{baseline.Requested.Profile},
		Placement: ReportPlacementPackage,
	}
	if gotErr := baseline.ValidateFor(surface); gotErr != nil {
		t.Fatalf("AboutRequestReference.ValidateFor(exact environment) error = %v, want nil", gotErr)
	}
	mutated := baseline
	admission := *baseline.Disposition.Admitted
	admission.Probe.Environment.RequirementFingerprint = core.NewSHA256Digest([core.SHA256DigestBytes]byte{3})
	mutated.Disposition.Admitted = &admission
	gotErr := mutated.ValidateFor(surface)
	if !errors.Is(gotErr, core.ErrAboutConflict) {
		t.Fatalf("AboutRequestReference.ValidateFor(mutated admitted environment) error = %v, want %v", gotErr, core.ErrAboutConflict)
	}
}

func TestObservationIndependentAuthorityAndEnvironmentBindings(t *testing.T) {
	t.Parallel()

	_, baseline := fixtureAdmittedObservation(t)
	if gotErr := baseline.Validate(); gotErr != nil {
		t.Fatalf("AboutObservationReference.Validate(exact authorities and environment) error = %v, want nil", gotErr)
	}

	t.Run("subject cannot verify its own observation", func(t *testing.T) {
		t.Parallel()
		mutated := baseline
		mutated.Verifier = EvidenceAuthority{Offering: mutated.Probe.Subject.Project}
		gotErr := mutated.Validate()
		if !errors.Is(gotErr, core.ErrAboutConflict) {
			t.Fatalf("AboutObservationReference.Validate(self verifier) error = %v, want %v", gotErr, core.ErrAboutConflict)
		}
	})

	t.Run("experiment environment cannot differ from admitted probe", func(t *testing.T) {
		t.Parallel()
		mutated := baseline
		experiment := *baseline.Experiment
		experiment.EnvironmentFingerprint = core.NewSHA256Digest([core.SHA256DigestBytes]byte{2})
		mutated.Experiment = &experiment
		gotErr := mutated.Validate()
		if !errors.Is(gotErr, core.ErrAboutConflict) {
			t.Fatalf("AboutObservationReference.Validate(mutated environment) error = %v, want %v", gotErr, core.ErrAboutConflict)
		}
	})
}

func fixtureExperimentObservationReference(t testing.TB, outcome Outcome, terminal TerminalState) AboutObservationReference {
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

func fixtureAdmittedObservation(t testing.TB) (AboutRequestReference, AboutObservationReference) {
	t.Helper()
	uuid := fixtureAboutUUID(t)
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
			Module: fixtureIdentifier(t, "primitive"), Package: fixturePath(t, "about"),
			File: fixturePath(t, "about/evidence.go"), Symbol: fixtureName(t, "TestEvidence"),
		},
	}
	requirement := EnvironmentRequirement{MachineClass: fixtureIdentifier(t, "anvil-linux"), Fingerprint: digest}
	generationID, generationErr := NewMachineGenerationID(uuid)
	if generationErr != nil {
		t.Fatalf("NewMachineGenerationID() setup error = %v, want nil", generationErr)
	}
	environment := AdmittedEnvironment{
		MachineClass: requirement.MachineClass, RequirementFingerprint: requirement.Fingerprint,
		EnvironmentFingerprint: digest, MachineGeneration: generationID, MachineSheetDigest: digest,
	}
	probe := ProbeIdentity{
		Origin: OriginIdentity{Offering: core.Offering{Token: "blink-kernel"}}, Subject: subject,
		Source: source, Role: ProbeRoleExperiment, Kind: ProbeKindGoTest, Target: target,
		Profile: fixtureProfile(t, "acceptance"), Environment: environment,
	}
	status, statusErr := core.ParseHTTPEndpoint("https://control.example/runs/1")
	if statusErr != nil {
		t.Fatalf("core.ParseHTTPEndpoint() setup error = %v, want nil", statusErr)
	}
	requested := RequestedProbe{
		Origin: probe.Origin, Subject: subject, Source: source, Target: target,
		Kinds: []ProbeKind{ProbeKindGoTest}, Profile: probe.Profile, Constraints: requirement,
	}
	request := AboutRequestReference{
		SurfaceID: fixtureIdentifier(t, "package-proof"), Request: requestID, Source: source, Requested: requested,
		Disposition: RequestDisposition{Kind: DispositionAdmitted, Admitted: &Admission{Run: runID, Probe: probe, Status: status}},
	}
	observation := AboutObservationReference{
		Observation: observationID,
		Producer:    EvidenceAuthority{Offering: core.Offering{Token: "anvil"}},
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
