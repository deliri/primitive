package about

import (
	"errors"
	"slices"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	EvidenceSurfaceMaximum      = 32
	RequestReferenceMaximum     = 32
	ObservationReferenceMaximum = 32
	ArtifactReferenceMaximum    = 32
	BenchmarkMeasurementMaximum = 32
)

type DispositionKind uint8

const (
	DispositionUnknown DispositionKind = iota
	DispositionAdmitted
	DispositionRefused
	dispositionLimit
)

func dispositionLabels() []string { return []string{"", "admitted", "refused"} }
func (k DispositionKind) Validate() error {
	return validateEnum(uint8(k), dispositionLabels(), "about disposition kind is invalid")
}
func (k DispositionKind) IsValid() bool  { return k.Validate() == nil }
func (k DispositionKind) String() string { return enumString(uint8(k), dispositionLabels()) }
func (k DispositionKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(k), dispositionLabels(), "about disposition kind is invalid")
}
func (k *DispositionKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return jsonError(errors.New("nil about disposition receiver"))
	}
	value, err := unmarshalEnum(data, dispositionLabels(), "about disposition kind is invalid")
	if err == nil {
		*k = DispositionKind(value)
	}
	return err
}

type RefusalReason uint8

const (
	RefusalUnknown RefusalReason = iota
	RefusalUnauthorized
	RefusalRepository
	RefusalProbe
	RefusalProfile
	RefusalBudget
	RefusalSource
	RefusalConflict
	refusalLimit
)

func refusalLabels() []string {
	return []string{"", "unauthorized", "repository", "probe", "profile", "budget", "source", "conflict"}
}
func (r RefusalReason) Validate() error {
	return validateEnum(uint8(r), refusalLabels(), "about refusal reason is invalid")
}
func (r RefusalReason) IsValid() bool  { return r.Validate() == nil }
func (r RefusalReason) String() string { return enumString(uint8(r), refusalLabels()) }
func (r RefusalReason) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(r), refusalLabels(), "about refusal reason is invalid")
}
func (r *RefusalReason) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("nil about refusal reason receiver"))
	}
	value, err := unmarshalEnum(data, refusalLabels(), "about refusal reason is invalid")
	if err == nil {
		*r = RefusalReason(value)
	}
	return err
}

type TerminalState uint8

const (
	TerminalUnknown TerminalState = iota
	TerminalCompleted
	TerminalFailed
	TerminalCancelled
	TerminalTimedOut
	TerminalUnavailable
	TerminalNotRun
	terminalLimit
)

func terminalLabels() []string {
	return []string{"", "completed", "failed", "cancelled", "timed_out", "unavailable", "not_run"}
}
func (s TerminalState) Validate() error {
	return validateEnum(uint8(s), terminalLabels(), "about terminal state is invalid")
}
func (s TerminalState) IsValid() bool  { return s.Validate() == nil }
func (s TerminalState) String() string { return enumString(uint8(s), terminalLabels()) }
func (s TerminalState) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(s), terminalLabels(), "about terminal state is invalid")
}
func (s *TerminalState) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(errors.New("nil about terminal state receiver"))
	}
	value, err := unmarshalEnum(data, terminalLabels(), "about terminal state is invalid")
	if err == nil {
		*s = TerminalState(value)
	}
	return err
}

type ObservationKind uint8

const (
	ObservationUnknown ObservationKind = iota
	ObservationSelection
	ObservationExperiment
	ObservationInfrastructure
	observationLimit
)

func observationLabels() []string { return []string{"", "selection", "experiment", "infrastructure"} }
func (k ObservationKind) Validate() error {
	return validateEnum(uint8(k), observationLabels(), "about observation kind is invalid")
}
func (k ObservationKind) IsValid() bool  { return k.Validate() == nil }
func (k ObservationKind) String() string { return enumString(uint8(k), observationLabels()) }
func (k ObservationKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(k), observationLabels(), "about observation kind is invalid")
}
func (k *ObservationKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return jsonError(errors.New("nil about observation kind receiver"))
	}
	value, err := unmarshalEnum(data, observationLabels(), "about observation kind is invalid")
	if err == nil {
		*k = ObservationKind(value)
	}
	return err
}

type InfrastructureStage uint8

const (
	InfrastructureStageUnknown InfrastructureStage = iota
	InfrastructureStageAdmission
	InfrastructureStageBoot
	InfrastructureStageReadiness
	InfrastructureStageSource
	InfrastructureStageExpansion
	InfrastructureStageExecution
	InfrastructureStageDelivery
	InfrastructureStageCleanup
	infrastructureStageLimit
)

func infrastructureStageLabels() []string {
	return []string{"", "admission", "boot", "readiness", "source", "expansion", "execution", "delivery", "cleanup"}
}
func (s InfrastructureStage) Validate() error {
	return validateEnum(uint8(s), infrastructureStageLabels(), "about infrastructure stage is invalid")
}
func (s InfrastructureStage) IsValid() bool { return s.Validate() == nil }
func (s InfrastructureStage) String() string {
	return enumString(uint8(s), infrastructureStageLabels())
}
func (s InfrastructureStage) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(s), infrastructureStageLabels(), "about infrastructure stage is invalid")
}
func (s *InfrastructureStage) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(errors.New("nil about infrastructure stage receiver"))
	}
	value, err := unmarshalEnum(data, infrastructureStageLabels(), "about infrastructure stage is invalid")
	if err == nil {
		*s = InfrastructureStage(value)
	}
	return err
}

type AboutEvidenceSurface struct {
	ID                 Identifier        `json:"id"`
	Subject            SubjectIdentity   `json:"subject"`
	Target             ProbeTarget       `json:"target"`
	EligibleKinds      []ProbeKind       `json:"eligible_kinds"`
	Profiles           []ProfileIdentity `json:"profiles"`
	Placement          ReportPlacement   `json:"report_placement"`
	ComplexityClaimIDs []Identifier      `json:"complexity_claim_ids"`
}

func (s AboutEvidenceSurface) Validate() error {
	if err := contractJoin(s.ID.Validate(), s.Subject.Validate(), s.Target.Validate(), s.Placement.Validate()); err != nil {
		return err
	}
	if len(s.EligibleKinds) == 0 || len(s.EligibleKinds) > ProbeKindMaximum || len(s.Profiles) == 0 || len(s.Profiles) > ProbeKindMaximum || len(s.ComplexityClaimIDs) > ComplexityClaimMaximum {
		return contractError(errors.New("about evidence surface bounds are invalid"))
	}
	if err := validateSurfaceKinds(s.Target, s.EligibleKinds); err != nil {
		return err
	}
	if err := validateProfiles(s.Profiles); err != nil {
		return err
	}
	return validateIdentifiers(s.ComplexityClaimIDs)
}

func validateSurfaceKinds(target ProbeTarget, kinds []ProbeKind) error {
	for index := range kinds {
		if err := kinds[index].Validate(); err != nil {
			return err
		}
		if !requestedTargetAdmitsKind(target.Kind, kinds[index]) || duplicateProbeKind(kinds, index) {
			return conflictError(errors.New("about evidence surface kind is contradictory"))
		}
		if index > 0 && kinds[index-1] >= kinds[index] {
			return conflictError(errors.New("about evidence surface kinds are not canonical"))
		}
	}
	if !requestedKindsMatchSelectionTarget(target, kinds) {
		return conflictError(errors.New("about evidence surface kinds differ from selection child kinds"))
	}
	return nil
}

func validateProfiles(values []ProfileIdentity) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		for previous := 0; previous < index; previous++ {
			if values[previous] == values[index] {
				return conflictError(errors.New("about profile identity is duplicated"))
			}
		}
	}
	return nil
}

func validateIdentifiers(values []Identifier) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		for previous := 0; previous < index; previous++ {
			if values[previous].value == values[index].value {
				return conflictError(errors.New("about identifier is duplicated"))
			}
		}
	}
	return nil
}

type Admission struct {
	Run    RunID             `json:"run_id"`
	Probe  ProbeIdentity     `json:"probe"`
	Status core.HTTPEndpoint `json:"status"`
}

func (a Admission) Validate() error {
	return contractJoin(a.Run.Validate(), a.Probe.Validate(), a.Status.Validate())
}

type Refusal struct {
	Reason RefusalReason `json:"reason"`
}

func (r Refusal) Validate() error { return r.Reason.Validate() }

type RequestDisposition struct {
	Kind     DispositionKind `json:"kind"`
	Admitted *Admission      `json:"admitted,omitempty"`
	Refused  *Refusal        `json:"refused,omitempty"`
}

func (d RequestDisposition) Validate() error {
	if err := d.Kind.Validate(); err != nil {
		return err
	}
	if (d.Admitted != nil) == (d.Refused != nil) {
		return contractError(errors.New("about disposition must carry exactly one variant"))
	}
	if d.Kind == DispositionAdmitted && d.Admitted != nil {
		return d.Admitted.Validate()
	}
	if d.Kind == DispositionRefused && d.Refused != nil {
		return d.Refused.Validate()
	}
	return conflictError(errors.New("about disposition payload differs from kind"))
}

type AboutRequestReference struct {
	SurfaceID   Identifier         `json:"surface_id"`
	Request     RequestIdentity    `json:"request_id"`
	Source      SourceCoordinate   `json:"source"`
	Requested   RequestedProbe     `json:"requested"`
	Disposition RequestDisposition `json:"disposition"`
}

func (r AboutRequestReference) Validate() error {
	return contractJoin(r.SurfaceID.Validate(), r.Request.Validate(), r.Source.Validate(), r.Requested.Validate(), r.Disposition.Validate())
}

func (r AboutRequestReference) ValidateFor(surface AboutEvidenceSurface) error {
	if err := contractJoin(r.Validate(), surface.Validate()); err != nil {
		return err
	}
	if r.SurfaceID.value != surface.ID.value || !sameSubject(r.Requested.Subject, surface.Subject) || !sameTarget(r.Requested.Target, surface.Target) || r.Source != r.Requested.Source {
		return conflictError(errors.New("about request does not descend from its evidence surface"))
	}
	if !requestedKindsAllowed(r.Requested.Kinds, surface.EligibleKinds) || !profileAllowed(r.Requested.Profile, surface.Profiles) {
		return conflictError(errors.New("about request exceeds its evidence surface"))
	}
	if r.Disposition.Admitted != nil {
		return validateAdmissionForRequest(*r.Disposition.Admitted, r.Requested)
	}
	return nil
}

func validateAdmissionForRequest(admission Admission, requested RequestedProbe) error {
	if !sameSubject(admission.Probe.Subject, requested.Subject) || admission.Probe.Origin != requested.Origin || admission.Probe.Source != requested.Source || !sameTarget(admission.Probe.Target, requested.Target) || admission.Probe.Profile != requested.Profile || !admission.Probe.Environment.Satisfies(requested.Constraints) || admission.Probe.Parent != nil {
		return conflictError(errors.New("about admission differs from requested probe"))
	}
	want, err := admittedKindForRequest(requested)
	if err != nil {
		return err
	}
	if admission.Probe.Kind != want {
		return conflictError(errors.New("about admitted probe kind differs from requested target"))
	}
	return nil
}

func admittedKindForRequest(requested RequestedProbe) (ProbeKind, error) {
	if len(requested.Kinds) == 0 {
		return ProbeKindUnknown, contractError(errors.New("about requested probe has no kind"))
	}
	switch requested.Target.Kind {
	case ProbeTargetGoFile:
		return ProbeKindGoFileSelection, nil
	case ProbeTargetGoPackage:
		return ProbeKindGoPackageSelection, nil
	case ProbeTargetCIPlan:
		return ProbeKindCISelection, nil
	case ProbeTargetGoDeclaration:
		return soleRequestedKind(requested, "about direct declaration admission requires one requested kind")
	case ProbeTargetJavaScriptFile:
		return ProbeKindJavaScriptTest, nil
	case ProbeTargetSmokeSuite:
		return ProbeKindSmoke, nil
	case ProbeTargetTool:
		return ProbeKindTool, nil
	case ProbeTargetUnknown:
		return ProbeKindUnknown, contractError(errors.New("about admitted target is invalid"))
	default:
		return ProbeKindUnknown, contractError(errors.New("about admitted target is invalid"))
	}
}

func soleRequestedKind(requested RequestedProbe, diagnostic string) (ProbeKind, error) {
	if len(requested.Kinds) != 1 {
		return ProbeKindUnknown, conflictError(errors.New(diagnostic))
	}
	return requested.Kinds[0], nil
}

func requestedKindsAllowed(requested, allowed []ProbeKind) bool {
	for _, kind := range requested {
		if !slices.Contains(allowed, kind) {
			return false
		}
	}
	return true
}

func profileAllowed(profile ProfileIdentity, allowed []ProfileIdentity) bool {
	for index := range allowed {
		if profile == allowed[index] {
			return true
		}
	}
	return false
}

type ArtifactReference struct {
	Path   SourcePath        `json:"path"`
	Digest core.SHA256Digest `json:"digest"`
	Bytes  core.ByteCount    `json:"bytes"`
}

func (r ArtifactReference) Validate() error {
	return contractJoin(r.Path.Validate(), r.Digest.Validate(), r.Bytes.Validate())
}

type BenchmarkMeasurement struct {
	Name             Name   `json:"name"`
	Iterations       uint64 `json:"iterations"`
	NanosecondsPerOp uint64 `json:"nanoseconds_per_op"`
	BytesPerOp       uint64 `json:"bytes_per_op"`
	AllocationsPerOp uint64 `json:"allocations_per_op"`
}

func (m BenchmarkMeasurement) Validate() error {
	if err := m.Name.Validate(); err != nil {
		return err
	}
	if m.Iterations == 0 || m.NanosecondsPerOp == 0 {
		return contractError(errors.New("about benchmark measurement is invalid"))
	}
	return nil
}

type ExperimentMeasurements struct {
	DurationNs          uint64                 `json:"duration_ns"`
	PeakMemoryBytes     uint64                 `json:"peak_memory_bytes"`
	CoverageBasisPoints *uint16                `json:"coverage_basis_points,omitempty"`
	Benchmarks          []BenchmarkMeasurement `json:"benchmarks"`
	Complexity          []ComplexityCapture    `json:"complexity"`
}

func (m ExperimentMeasurements) Validate() error {
	if m.CoverageBasisPoints != nil && *m.CoverageBasisPoints > 10_000 {
		return contractError(errors.New("about coverage exceeds one hundred percent"))
	}
	if len(m.Benchmarks) > BenchmarkMeasurementMaximum || len(m.Complexity) > ComplexityCaptureMaximum {
		return contractError(errors.New("about measurement collection exceeds its bound"))
	}
	for index := range m.Benchmarks {
		if err := m.Benchmarks[index].Validate(); err != nil {
			return err
		}
	}
	for index := range m.Complexity {
		if err := m.Complexity[index].Validate(); err != nil {
			return err
		}
	}
	return nil
}

type SelectionObservation struct {
	ExpansionDigest core.SHA256Digest `json:"expansion_digest"`
	Planned         uint16            `json:"planned"`
	Admitted        uint16            `json:"admitted"`
	Refused         uint16            `json:"refused"`
	Executed        uint16            `json:"executed"`
	NotRun          uint16            `json:"not_run"`
}

func (o SelectionObservation) Validate() error {
	if err := o.ExpansionDigest.Validate(); err != nil {
		return contractError(err)
	}
	if o.Admitted+o.Refused != o.Planned || o.Executed+o.NotRun != o.Admitted {
		return conflictError(errors.New("about selection accounting does not close"))
	}
	return nil
}

type ExperimentObservation struct {
	Experiment             ExperimentID           `json:"experiment_id"`
	Started                bool                   `json:"started"`
	Outcome                Outcome                `json:"outcome"`
	EnvironmentFingerprint core.SHA256Digest      `json:"environment_fingerprint"`
	ExecutionFingerprint   core.SHA256Digest      `json:"execution_fingerprint"`
	MachineSheetDigest     core.SHA256Digest      `json:"machine_sheet_digest"`
	Measurements           ExperimentMeasurements `json:"measurements"`
	Artifacts              []ArtifactReference    `json:"artifacts"`
}

func (o ExperimentObservation) Validate() error {
	if len(o.Artifacts) > ArtifactReferenceMaximum {
		return contractError(errors.New("about experiment artifact count exceeds its bound"))
	}
	if err := contractJoin(o.Experiment.Validate(), o.Outcome.Validate(), o.EnvironmentFingerprint.Validate(), o.ExecutionFingerprint.Validate(), o.MachineSheetDigest.Validate(), o.Measurements.Validate()); err != nil {
		return err
	}
	if err := o.validateExecutionState(); err != nil {
		return err
	}
	return validateArtifacts(o.Artifacts)
}

func (o ExperimentObservation) validateExecutionState() error {
	if o.Started {
		if o.Measurements.DurationNs == 0 || o.Outcome == OutcomeNotRun {
			return conflictError(errors.New("about started experiment has contradictory execution facts"))
		}
		return nil
	}
	if !o.Measurements.isZero() || len(o.Artifacts) != 0 || !outcomePermitsUnstarted(o.Outcome) {
		return conflictError(errors.New("about unstarted experiment has contradictory execution facts"))
	}
	return nil
}

func (m ExperimentMeasurements) isZero() bool {
	return m.DurationNs == 0 && m.PeakMemoryBytes == 0 && m.CoverageBasisPoints == nil && len(m.Benchmarks) == 0 && len(m.Complexity) == 0
}

func outcomePermitsUnstarted(outcome Outcome) bool {
	return outcome == OutcomeCancelled || outcome == OutcomeUnavailable || outcome == OutcomeSetupFailed ||
		outcome == OutcomeInfrastructureFailed || outcome == OutcomeNotRun
}

type InfrastructureObservation struct {
	Stage           InfrastructureStage `json:"stage"`
	Failure         core.ErrorIdentity  `json:"failure"`
	PartialEvidence []ArtifactReference `json:"partial_evidence"`
}

func (o InfrastructureObservation) Validate() error {
	if len(o.PartialEvidence) > ArtifactReferenceMaximum {
		return contractError(errors.New("about partial evidence exceeds its bound"))
	}
	if err := contractJoin(o.Stage.Validate(), o.Failure.Validate()); err != nil {
		return err
	}
	return validateArtifacts(o.PartialEvidence)
}

func validateArtifacts(values []ArtifactReference) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		for previous := 0; previous < index; previous++ {
			if values[previous].Path.value == values[index].Path.value {
				return conflictError(errors.New("about artifact path is duplicated"))
			}
		}
	}
	return nil
}

type AboutObservationReference struct {
	Observation    ObservationID              `json:"observation_id"`
	Producer       EvidenceAuthority          `json:"producer_authority"`
	Verifier       EvidenceAuthority          `json:"verifier_authority"`
	Kind           ObservationKind            `json:"kind"`
	Request        RequestIdentity            `json:"request_id"`
	Run            RunID                      `json:"run_id"`
	Source         SourceCoordinate           `json:"source"`
	EnvelopeDigest core.SHA256Digest          `json:"envelope_digest"`
	Probe          ProbeIdentity              `json:"probe"`
	Terminal       TerminalState              `json:"terminal"`
	CapturedAt     temporal.Instant           `json:"captured_at"`
	Selection      *SelectionObservation      `json:"selection,omitempty"`
	Experiment     *ExperimentObservation     `json:"experiment,omitempty"`
	Infrastructure *InfrastructureObservation `json:"infrastructure,omitempty"`
}

func (r AboutObservationReference) Validate() error {
	if err := contractJoin(r.Observation.Validate(), r.Producer.Validate(), r.Verifier.Validate(), r.Kind.Validate(), r.Request.Validate(), r.Run.Validate(), r.Source.Validate(), r.EnvelopeDigest.Validate(), r.Probe.Validate(), r.Terminal.Validate(), r.CapturedAt.Validate()); err != nil {
		return err
	}
	if !r.independentAuthorities() {
		return conflictError(errors.New("about evidence authorities are not independent of requester and subject"))
	}
	if r.payloadCount() != 1 {
		return contractError(errors.New("about observation must carry exactly one variant"))
	}
	return r.validatePayload()
}

func (r AboutObservationReference) independentAuthorities() bool {
	producer := r.Producer.Offering
	verifier := r.Verifier.Offering
	origin := r.Probe.Origin.Offering
	subject := r.Probe.Subject.Project
	return producer != verifier && producer != origin && producer != subject && verifier != origin && verifier != subject
}

func (r AboutObservationReference) validatePayload() error {
	switch r.Kind {
	case ObservationSelection:
		return r.validateSelectionPayload()
	case ObservationExperiment:
		return r.validateExperimentPayload()
	case ObservationInfrastructure:
		return r.validateInfrastructurePayload()
	case ObservationUnknown:
		return conflictError(errors.New("about observation payload, kind, and probe role disagree"))
	default:
		return conflictError(errors.New("about observation payload, kind, and probe role disagree"))
	}
}

func (r AboutObservationReference) validateSelectionPayload() error {
	return validateObservationPayload(r.Selection, r.Probe.Role == ProbeRoleSelection && r.Terminal == TerminalCompleted)
}

func (r AboutObservationReference) validateExperimentPayload() error {
	if r.Experiment == nil || r.Probe.Role != ProbeRoleExperiment {
		return conflictError(errors.New("about observation payload, kind, and probe role disagree"))
	}
	if err := r.Experiment.Validate(); err != nil {
		return err
	}
	if err := r.validateExperimentBinding(); err != nil {
		return err
	}
	if !experimentTerminalMatches(r.Experiment.Outcome, r.Terminal) {
		return conflictError(errors.New("about experiment outcome and terminal state disagree"))
	}
	return nil
}

func (r AboutObservationReference) validateInfrastructurePayload() error {
	return validateObservationPayload(r.Infrastructure, r.Terminal != TerminalCompleted)
}

func (r AboutObservationReference) validateExperimentBinding() error {
	if r.Experiment.EnvironmentFingerprint != r.Probe.Environment.EnvironmentFingerprint || r.Experiment.MachineSheetDigest != r.Probe.Environment.MachineSheetDigest {
		return conflictError(errors.New("about experiment environment differs from admitted probe"))
	}
	for _, capture := range r.Experiment.Measurements.Complexity {
		if capture.EnvironmentFingerprint != r.Experiment.EnvironmentFingerprint || capture.Profile != r.Probe.Profile {
			return conflictError(errors.New("about complexity capture differs from experiment identity"))
		}
	}
	return nil
}

func experimentTerminalMatches(outcome Outcome, terminal TerminalState) bool {
	if completedExperimentOutcome(outcome) {
		return terminal == TerminalCompleted
	}
	if outcome == OutcomeInfrastructureFailed {
		return terminal == TerminalFailed
	}
	if outcome == OutcomeCancelled {
		return terminal == TerminalCancelled
	}
	if outcome == OutcomeTimedOut {
		return terminal == TerminalTimedOut
	}
	if outcome == OutcomeUnavailable {
		return terminal == TerminalUnavailable
	}
	if outcome == OutcomeNotRun {
		return terminal == TerminalNotRun
	}
	return false
}

func completedExperimentOutcome(outcome Outcome) bool {
	for _, candidate := range [...]Outcome{OutcomePassed, OutcomeFailed, OutcomeSkipped, OutcomeSetupFailed, OutcomeNonAccepting} {
		if outcome == candidate {
			return true
		}
	}
	return false
}

func validateObservationPayload[T interface{ Validate() error }](value *T, roleMatches bool) error {
	if value == nil || !roleMatches {
		return conflictError(errors.New("about observation payload, kind, and probe role disagree"))
	}
	return (*value).Validate()
}

func (r AboutObservationReference) payloadCount() int {
	count := 0
	for _, present := range [...]bool{r.Selection != nil, r.Experiment != nil, r.Infrastructure != nil} {
		if present {
			count++
		}
	}
	return count
}

func (r AboutObservationReference) ValidateFor(request AboutRequestReference) error {
	if err := contractJoin(r.Validate(), request.Validate()); err != nil {
		return err
	}
	if request.Disposition.Admitted == nil {
		return conflictError(errors.New("about observation descends from a refused request"))
	}
	admission := request.Disposition.Admitted
	if !r.matchesAdmission(request, *admission) {
		return conflictError(errors.New("about observation does not descend from its admission"))
	}
	if r.Probe.Source != r.Source {
		return conflictError(errors.New("about observation probe source differs from observation source"))
	}
	if r.Probe.Parent != nil && !r.matchesParent(*admission) {
		return conflictError(errors.New("about observation child differs from admitted selection"))
	}
	return nil
}

func (r AboutObservationReference) matchesAdmission(request AboutRequestReference, admission Admission) bool {
	if r.Request != request.Request || r.Run != admission.Run || r.Source != request.Source {
		return false
	}
	if r.Probe.Parent == nil {
		return sameProbeIdentity(r.Probe, admission.Probe)
	}
	return sameSubject(r.Probe.Subject, admission.Probe.Subject) && r.Probe.Origin == admission.Probe.Origin &&
		r.Probe.Source == admission.Probe.Source && r.Probe.Profile == admission.Probe.Profile &&
		r.Probe.Environment == admission.Probe.Environment
}

func sameProbeIdentity(left, right ProbeIdentity) bool {
	return left.Origin == right.Origin && sameSubject(left.Subject, right.Subject) && left.Source == right.Source &&
		left.Role == right.Role && left.Kind == right.Kind && sameTarget(left.Target, right.Target) &&
		left.Profile == right.Profile && left.Environment == right.Environment && left.Parent == nil && right.Parent == nil
}

func (r AboutObservationReference) matchesParent(admission Admission) bool {
	return r.Probe.Parent.Request == r.Request && sameTarget(r.Probe.Parent.Target, admission.Probe.Target) && r.Probe.Parent.Kind == admission.Probe.Kind
}

func sameTarget(left, right ProbeTarget) bool {
	if !sameTargetShape(left, right) {
		return false
	}
	if left.Kind == ProbeTargetGoDeclaration {
		return sameComparableTarget(left.GoDeclaration, right.GoDeclaration)
	}
	if left.Kind == ProbeTargetGoPackage {
		return sameGoPackageTarget(left.GoPackage, right.GoPackage)
	}
	if left.Kind == ProbeTargetJavaScriptFile {
		return sameComparableTarget(left.JavaScript, right.JavaScript)
	}
	if left.Kind == ProbeTargetSmokeSuite {
		return sameComparableTarget(left.Smoke, right.Smoke)
	}
	if left.Kind == ProbeTargetTool {
		return sameToolTarget(left.Tool, right.Tool)
	}
	if left.Kind == ProbeTargetCIPlan {
		return sameComparableTarget(left.CI, right.CI)
	}
	if left.Kind == ProbeTargetGoFile {
		return sameGoFileTarget(left.GoFile, right.GoFile)
	}
	return false
}

func sameTargetShape(left, right ProbeTarget) bool {
	return left.Kind == right.Kind && left.payloadCount() == right.payloadCount()
}

func sameComparableTarget[T comparable](left, right *T) bool {
	return left != nil && right != nil && *left == *right
}

func sameToolTarget(left, right *ToolTarget) bool {
	if left == nil || right == nil || left.Identity != right.Identity || (left.Module == nil) != (right.Module == nil) {
		return false
	}
	return left.Module == nil || *left.Module == *right.Module
}

func sameGoFileTarget(left, right *GoFileTarget) bool {
	return left != nil && right != nil && left.Module == right.Module && left.Package == right.Package && left.File == right.File && slices.Equal(left.ChildKinds, right.ChildKinds)
}

func sameGoPackageTarget(left, right *GoPackageTarget) bool {
	return left != nil && right != nil && left.Module == right.Module && left.Package == right.Package && slices.Equal(left.ChildKinds, right.ChildKinds)
}
