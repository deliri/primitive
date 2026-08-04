package release

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// CachedLatestState distinguishes explicit absence from an authenticated
// cached selection.
type CachedLatestState uint8

const (
	CachedLatestUnknown CachedLatestState = iota
	CachedLatestMissing
	CachedLatestPresent
	cachedLatestLimit
)

func (s CachedLatestState) Validate() error {
	if s <= CachedLatestUnknown || s >= cachedLatestLimit ||
		cachedLatestStateLabels()[s] == "" {
		return contractError(errors.New("cached latest state is outside the closed domain"))
	}
	return nil
}

func (s CachedLatestState) IsValid() bool { return s.Validate() == nil }
func (CachedLatestState) OffWireEnum()    {}

// String returns a stable diagnostic label.
func (s CachedLatestState) String() string {
	if s >= cachedLatestLimit || cachedLatestStateLabels()[s] == "" {
		return core.UnknownEnumDiagnostic
	}
	return cachedLatestStateLabels()[s]
}

func cachedLatestStateLabels() [cachedLatestLimit]string {
	return [...]string{"", "missing", "present"}
}

// CachedLatest is the explicit optional cache input.
type CachedLatest struct {
	latest VerifiedLatest
	state  CachedLatestState
	valid  bool
}

func MissingCachedLatest() CachedLatest {
	return CachedLatest{state: CachedLatestMissing, valid: true}
}

func NewCachedLatest(latest VerifiedLatest) (CachedLatest, error) {
	candidate := CachedLatest{latest: latest, state: CachedLatestPresent, valid: true}
	if err := candidate.Validate(); err != nil {
		return CachedLatest{}, err
	}
	return candidate, nil
}

func (c CachedLatest) Validate() error {
	if !c.valid {
		return contractError(errors.New("cached latest is unset"))
	}
	if err := c.state.Validate(); err != nil {
		return err
	}
	switch c.state {
	case CachedLatestMissing:
		if c.latest != (VerifiedLatest{}) {
			return contractError(errors.New("missing cache carries a latest proof"))
		}
	case CachedLatestPresent:
		if err := c.latest.Validate(); err != nil {
			return verificationError(err)
		}
	default:
		return contractError(errors.New("cached latest state escaped its domain"))
	}
	return nil
}

// SelectionState is the complete pure release-selection result domain.
type SelectionState uint8

const (
	SelectionUnknown SelectionState = iota
	SelectionCurrent
	SelectionAvailable
	SelectionRefreshRequired
	SelectionReassessAt
	selectionLimit
)

func (s SelectionState) Validate() error {
	if s <= SelectionUnknown || s >= selectionLimit ||
		selectionStateLabels()[s] == "" {
		return contractError(errors.New("selection state is outside the closed domain"))
	}
	return nil
}

func (s SelectionState) IsValid() bool { return s.Validate() == nil }
func (SelectionState) OffWireEnum()    {}

// String returns a stable diagnostic label.
func (s SelectionState) String() string {
	if s >= selectionLimit || selectionStateLabels()[s] == "" {
		return core.UnknownEnumDiagnostic
	}
	return selectionStateLabels()[s]
}

func selectionStateLabels() [selectionLimit]string {
	return [...]string{
		"",
		currentDiagnostic,
		"available",
		"refresh-required",
		"reassess-at",
	}
}

type EvaluateRequest struct {
	InstalledManifest VerifiedManifest
	Latest            CachedLatest
	Observation       temporal.Instant
}

// Validate proves the complete installed-selection ingress.
func (r EvaluateRequest) Validate() error {
	if err := r.InstalledManifest.Validate(); err != nil {
		return verificationError(err)
	}
	if err := r.Latest.Validate(); err != nil {
		return err
	}
	if err := r.Observation.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// CurrentRelease proves the installed immutable release remains selected.
type CurrentRelease struct {
	validUntil temporal.Instant
	version    core.ReleaseVersion
	manifest   ManifestIdentity
	artifact   ArtifactIdentity
	valid      bool
}

// CurrentSummary is the non-authoritative value projection of a current
// selection.
type CurrentSummary struct {
	ValidUntil temporal.Instant
	Version    core.ReleaseVersion
	Manifest   ManifestIdentity
	Artifact   ArtifactIdentity
}

// Validate proves every projected current-release fact.
func (s CurrentSummary) Validate() error {
	for _, err := range []error{
		s.ValidUntil.Validate(), s.Version.Validate(),
		s.Manifest.Validate(), s.Artifact.Validate(),
	} {
		if err != nil {
			return contractError(err)
		}
	}
	return nil
}

func (c CurrentRelease) Validate() error {
	if !c.valid {
		return contractError(errors.New("current release is unset"))
	}
	for _, err := range []error{
		c.manifest.Validate(), c.artifact.Validate(), c.version.Validate(), c.validUntil.Validate(),
	} {
		if err != nil {
			return contractError(err)
		}
	}
	return nil
}

// Summary returns a validated value copy without installation authority.
func (c CurrentRelease) Summary() (CurrentSummary, error) {
	if err := c.Validate(); err != nil {
		return CurrentSummary{}, err
	}
	summary := CurrentSummary{
		ValidUntil: c.validUntil, Version: c.version,
		Manifest: c.manifest, Artifact: c.artifact,
	}
	return summary, summary.Validate()
}

// AvailableRelease privately carries every authenticated fact needed to
// prepare one Upgrade handoff without re-decoding caller data.
type AvailableRelease struct {
	installedManifest VerifiedManifest
	candidateManifest VerifiedManifest
	latest            VerifiedLatest
	installedArtifact Artifact
	candidateArtifact Artifact
	assessment        LatestAssessment
	installed         core.BuildIdentity
	valid             bool
}

// AvailableSummary is the non-authoritative value projection of an available
// release. Its fields are sufficient to render and preflight the exact
// candidate without granting installation authority.
type AvailableSummary struct {
	Installed        core.BuildIdentity
	Candidate        core.BuildIdentity
	Manifest         ManifestIdentity
	ManifestDocument ManifestDocumentDigest
	Artifact         ArtifactIdentity
	Filename         BinaryFilename
	Integrity        ArtifactIntegrity
	ValidUntil       temporal.Instant
}

// Validate proves the summary's complete candidate-artifact closure.
func (s AvailableSummary) Validate() error {
	for _, err := range []error{
		s.Installed.Validate(), s.Candidate.Validate(), s.Manifest.Validate(),
		s.ManifestDocument.Validate(), s.Artifact.Validate(), s.Filename.Validate(),
		s.Integrity.Validate(), s.ValidUntil.Validate(),
	} {
		if err != nil {
			return contractError(err)
		}
	}
	if s.Installed.Offering() != s.Candidate.Offering() ||
		s.Installed.Platform() != s.Candidate.Platform() {
		return conflictError(errors.New("available summary build stream differs"))
	}
	order, err := s.Installed.Version().Compare(s.Candidate.Version())
	if err != nil || order != core.ComparisonLess {
		return conflictError(errors.New("available summary candidate is not newer"), err)
	}
	filename, err := newBinaryFilename(s.Candidate)
	if err != nil || filename != s.Filename {
		return conflictError(errors.New("available summary filename differs"), err)
	}
	return validateAvailableSummaryArtifact(s)
}

func validateAvailableSummaryArtifact(s AvailableSummary) error {
	artifact, err := NewArtifact(ArtifactRequest{
		Extent: s.Integrity.Extent(), Build: s.Candidate,
		CRC32C: s.Integrity.CRC32C(), SHA256: s.Integrity.SHA256(),
	})
	if err != nil || artifact.Identity() != s.Artifact {
		return conflictError(errors.New("available summary artifact differs"), err)
	}
	return nil
}

func (a AvailableRelease) Validate() error {
	if !a.valid {
		return verificationError(errors.New("available release is unset"))
	}
	for _, err := range []error{
		a.installed.Validate(), a.installedArtifact.Validate(),
		a.installedManifest.Validate(), a.candidateArtifact.Validate(),
		a.candidateManifest.Validate(), a.latest.Validate(), a.assessment.Validate(),
	} {
		if err != nil {
			return verificationError(err)
		}
	}
	return validateAvailableBindings(a)
}

// Summary returns a validated value copy without Upgrade authority.
func (a AvailableRelease) Summary() (AvailableSummary, error) {
	if err := a.Validate(); err != nil {
		return AvailableSummary{}, err
	}
	manifestIdentity := a.candidateManifest.Identity()
	manifestDocument := a.candidateManifest.DocumentDigest()
	filename, err := a.candidateArtifact.Filename()
	if err != nil {
		return AvailableSummary{}, err
	}
	summary := AvailableSummary{
		Installed: a.installed, Candidate: a.candidateArtifact.Build(),
		Manifest: manifestIdentity, ManifestDocument: manifestDocument,
		Artifact: a.candidateArtifact.Identity(), Filename: filename,
		Integrity:  a.candidateArtifact.Integrity(),
		ValidUntil: a.assessment.ValidUntil(),
	}
	return summary, summary.Validate()
}

func validateAvailableBindings(a AvailableRelease) error {
	if a.installedArtifact.Build() != a.installed ||
		a.candidateArtifact.Target() != a.installed.Platform() ||
		a.assessment.Freshness() != LatestFreshnessCurrent {
		return conflictError(errors.New("available release binding differs"))
	}
	if err := validateAvailableInstalled(a); err != nil {
		return err
	}
	if err := validateAvailableCandidate(a); err != nil {
		return err
	}
	return validateAvailableLatest(a)
}

func validateAvailableInstalled(a AvailableRelease) error {
	installedArtifacts := a.installedManifest.Artifacts()
	installedArtifact, ok := installedArtifacts.ForPlatform(a.installed.Platform())
	if !ok || installedArtifact != a.installedArtifact {
		return conflictError(errors.New("available installed artifact differs from its manifest"))
	}
	return nil
}

func validateAvailableCandidate(a AvailableRelease) error {
	candidateArtifacts := a.candidateManifest.Artifacts()
	candidateArtifact, ok := candidateArtifacts.ForPlatform(a.installed.Platform())
	if !ok || candidateArtifact != a.candidateArtifact {
		return conflictError(errors.New("available candidate artifact differs from its manifest"))
	}
	return nil
}

func validateAvailableLatest(a AvailableRelease) error {
	latestManifest := a.latest.Manifest()
	latestIdentity := latestManifest.Identity()
	candidateIdentity := a.candidateManifest.Identity()
	if latestIdentity != candidateIdentity {
		return conflictError(errors.New("available candidate differs from latest"))
	}
	return nil
}

type RefreshDirective struct {
	valid bool
}

func (d RefreshDirective) Validate() error {
	if !d.valid {
		return contractError(errors.New("refresh directive is unset"))
	}
	return nil
}

type ReassessDirective struct {
	At temporal.Instant
}

func (d ReassessDirective) Validate() error { return d.At.Validate() }

// Selection is a validated tagged union over the four caller actions.
type Selection struct {
	available AvailableRelease
	current   CurrentRelease
	reassess  ReassessDirective
	refresh   RefreshDirective
	state     SelectionState
	valid     bool
}

func (s Selection) Validate() error {
	if !s.valid {
		return contractError(errors.New("selection is unset"))
	}
	if err := s.state.Validate(); err != nil {
		return err
	}
	switch s.state {
	case SelectionCurrent:
		return s.validateCurrentArm()
	case SelectionAvailable:
		return s.validateAvailableArm()
	case SelectionRefreshRequired:
		return s.validateRefreshArm()
	case SelectionReassessAt:
		return s.validateReassessArm()
	default:
		return contractError(errors.New("selection state escaped its domain"))
	}
}

func (s Selection) validateCurrentArm() error {
	if s.available.valid || s.refresh.valid || s.reassess != (ReassessDirective{}) {
		return contractError(errors.New("current selection carries another arm"))
	}
	return s.current.Validate()
}

func (s Selection) validateAvailableArm() error {
	if s.current.valid || s.refresh.valid || s.reassess != (ReassessDirective{}) {
		return contractError(errors.New("available selection carries another arm"))
	}
	return s.available.Validate()
}

func (s Selection) validateRefreshArm() error {
	if s.current.valid || s.available.valid || s.reassess != (ReassessDirective{}) {
		return contractError(errors.New("refresh selection carries another arm"))
	}
	return s.refresh.Validate()
}

func (s Selection) validateReassessArm() error {
	if s.current.valid || s.available.valid || s.refresh.valid {
		return contractError(errors.New("reassess selection carries another arm"))
	}
	return s.reassess.Validate()
}

func (s Selection) State() SelectionState { return s.state }
func (s Selection) Current() (CurrentRelease, bool) {
	return s.current, s.valid && s.state == SelectionCurrent
}
func (s Selection) Available() (AvailableRelease, bool) {
	return s.available, s.valid && s.state == SelectionAvailable
}
func (s Selection) Refresh() (RefreshDirective, bool) {
	return s.refresh, s.valid && s.state == SelectionRefreshRequired
}
func (s Selection) Reassess() (ReassessDirective, bool) {
	return s.reassess, s.valid && s.state == SelectionReassessAt
}

// Evaluate reads the current binary's Core-owned embedded identity and makes
// one pure installed-versus-latest selection.
func Evaluate(request EvaluateRequest) (Selection, error) {
	installed, err := EmbeddedBuildIdentity()
	if err != nil {
		return Selection{}, conflictError(err)
	}
	return evaluateWithInstalled(request, installed)
}

func evaluateWithInstalled(request EvaluateRequest, installed core.BuildIdentity) (Selection, error) {
	if err := installed.Validate(); err != nil {
		return Selection{}, conflictError(err)
	}
	if err := request.Validate(); err != nil {
		return Selection{}, err
	}
	installedArtifact, err := closeInstalled(request.InstalledManifest, installed)
	if err != nil {
		return Selection{}, err
	}
	if request.Latest.state == CachedLatestMissing {
		return sealSelection(Selection{
			state: SelectionRefreshRequired, refresh: RefreshDirective{valid: true}, valid: true,
		})
	}
	return evaluatePresent(request, installed, installedArtifact)
}

func closeInstalled(manifest VerifiedManifest, installed core.BuildIdentity) (Artifact, error) {
	offering := manifest.Offering()
	if offering != installed.Offering() {
		return Artifact{}, conflictError(errors.New("installed offering differs from manifest"))
	}
	version := manifest.Version()
	if version != installed.Version() {
		return Artifact{}, conflictError(errors.New("installed version differs from manifest"))
	}
	artifacts := manifest.Artifacts()
	artifact, ok := artifacts.ForPlatform(installed.Platform())
	if !ok || artifact.Build() != installed {
		return Artifact{}, conflictError(errors.New("installed identity has no exact manifest artifact"))
	}
	return artifact, nil
}

func evaluatePresent(
	request EvaluateRequest,
	installed core.BuildIdentity,
	installedArtifact Artifact,
) (Selection, error) {
	fact := request.Latest.latest.Fact()
	if fact.Offering() != installed.Offering() {
		return Selection{}, conflictError(errors.New("latest offering differs from installed offering"))
	}
	assessment, err := AssessLatest(AssessLatestRequest{
		Latest: request.Latest.latest, Observation: request.Observation,
	})
	if err != nil {
		return Selection{}, err
	}
	switch assessment.Freshness() {
	case LatestFreshnessExpired:
		return sealSelection(Selection{
			state: SelectionRefreshRequired, refresh: RefreshDirective{valid: true}, valid: true,
		})
	case LatestFreshnessNotYetValid:
		at, ok := assessment.Boundary()
		if !ok {
			return Selection{}, verificationError(errors.New("not-yet-valid assessment lacks boundary"))
		}
		return sealSelection(Selection{
			state: SelectionReassessAt, reassess: ReassessDirective{At: at}, valid: true,
		})
	case LatestFreshnessCurrent:
		return compareSelected(request, installed, installedArtifact, assessment)
	default:
		return Selection{}, verificationError(errors.New("freshness escaped its domain"))
	}
}

func compareSelected(
	request EvaluateRequest,
	installed core.BuildIdentity,
	installedArtifact Artifact,
	assessment LatestAssessment,
) (Selection, error) {
	candidate := request.Latest.latest.Manifest()
	version := candidate.Version()
	order, err := installed.Version().Compare(version)
	if err != nil {
		return Selection{}, contractError(err)
	}
	switch order {
	case core.ComparisonGreater:
		return Selection{}, rollbackError(errors.New("installed version is newer than latest"))
	case core.ComparisonEqual:
		return selectCurrent(request.InstalledManifest, installedArtifact, candidate, assessment)
	case core.ComparisonLess:
		return selectAvailable(request, installed, installedArtifact, assessment)
	default:
		return Selection{}, contractError(errors.New(versionComparisonDomainDiagnostic))
	}
}

func selectCurrent(
	installed VerifiedManifest,
	installedArtifact Artifact,
	candidate VerifiedManifest,
	assessment LatestAssessment,
) (Selection, error) {
	installedID := installed.Identity()
	candidateID := candidate.Identity()
	artifacts := candidate.Artifacts()
	artifact, ok := artifacts.ForPlatform(installedArtifact.Target())
	if !ok || installedID != candidateID || artifact.Identity() != installedArtifact.Identity() {
		return Selection{}, conflictError(errors.New("equal version differs immutably"))
	}
	version := candidate.Version()
	current := CurrentRelease{
		manifest: installedID, artifact: artifact.Identity(), version: version,
		validUntil: assessment.ValidUntil(), valid: true,
	}
	return sealSelection(Selection{state: SelectionCurrent, current: current, valid: true})
}

func selectAvailable(
	request EvaluateRequest,
	installed core.BuildIdentity,
	installedArtifact Artifact,
	assessment LatestAssessment,
) (Selection, error) {
	candidate := request.Latest.latest.Manifest()
	artifacts := candidate.Artifacts()
	artifact, ok := artifacts.ForPlatform(installed.Platform())
	if !ok {
		return Selection{}, verificationError(errors.New("candidate lacks installed platform"))
	}
	available := AvailableRelease{
		installed: installed, installedArtifact: installedArtifact,
		installedManifest: request.InstalledManifest,
		candidateArtifact: artifact, candidateManifest: candidate,
		latest: request.Latest.latest, assessment: assessment, valid: true,
	}
	return sealSelection(Selection{state: SelectionAvailable, available: available, valid: true})
}

func sealSelection(candidate Selection) (Selection, error) {
	if err := candidate.Validate(); err != nil {
		return Selection{}, err
	}
	return candidate, nil
}

// PreparedRelease is the authenticated handoff consumed by Upgrade.
type PreparedRelease struct {
	candidateManifest VerifiedManifest
	installedManifest VerifiedManifest
	latest            VerifiedLatest
	artifact          Artifact
	observation       temporal.Instant
	assessment        LatestAssessment
	valid             bool
}

func (p PreparedRelease) Validate() error {
	if !p.valid {
		return verificationError(errors.New("prepared release is unset"))
	}
	if err := p.validateValues(); err != nil {
		return err
	}
	if err := p.validateFreshness(); err != nil {
		return err
	}
	return p.validateBinding()
}

func (p PreparedRelease) validateValues() error {
	for _, err := range []error{
		p.candidateManifest.Validate(), p.installedManifest.Validate(),
		p.latest.Validate(), p.artifact.Validate(), p.observation.Validate(),
		p.assessment.Validate(),
	} {
		if err != nil {
			return verificationError(err)
		}
	}
	return nil
}

func (p PreparedRelease) validateFreshness() error {
	reassessed, err := AssessLatest(AssessLatestRequest{
		Latest: p.latest, Observation: p.observation,
	})
	if err != nil || reassessed != p.assessment ||
		p.assessment.Freshness() != LatestFreshnessCurrent {
		return verificationError(errors.New("prepared release freshness proof differs"), err)
	}
	return nil
}

func (p PreparedRelease) validateBinding() error {
	latestManifest := p.latest.Manifest()
	latestIdentity := latestManifest.Identity()
	candidateIdentity := p.candidateManifest.Identity()
	artifacts := p.candidateManifest.Artifacts()
	candidateArtifact, ok := artifacts.ForPlatform(p.artifact.Target())
	if latestIdentity != candidateIdentity || !ok || candidateArtifact != p.artifact {
		return verificationError(errors.New("prepared release handoff differs from latest"))
	}
	return nil
}

func (p PreparedRelease) Artifact() (Artifact, error) {
	if err := p.Validate(); err != nil {
		return Artifact{}, err
	}
	return p.artifact, nil
}

func (p PreparedRelease) CandidateManifest() (VerifiedManifest, error) {
	if err := p.Validate(); err != nil {
		return VerifiedManifest{}, err
	}
	return p.candidateManifest, nil
}

// InstalledManifest returns the exact authenticated installed manifest.
func (p PreparedRelease) InstalledManifest() (VerifiedManifest, error) {
	if err := p.Validate(); err != nil {
		return VerifiedManifest{}, err
	}
	return p.installedManifest, nil
}

// Latest returns the exact authenticated selection that authorized preparation.
func (p PreparedRelease) Latest() (VerifiedLatest, error) {
	if err := p.Validate(); err != nil {
		return VerifiedLatest{}, err
	}
	return p.latest, nil
}

// Observation returns the exact observation used for final freshness proof.
func (p PreparedRelease) Observation() (temporal.Instant, error) {
	if err := p.Validate(); err != nil {
		return temporal.Instant{}, err
	}
	return p.observation, nil
}

// Assessment returns the exact freshness proof at Observation.
func (p PreparedRelease) Assessment() (LatestAssessment, error) {
	if err := p.Validate(); err != nil {
		return LatestAssessment{}, err
	}
	return p.assessment, nil
}

// Preparation is a validated ready/refresh/reassess union.
type Preparation struct {
	ready    PreparedRelease
	reassess ReassessDirective
	refresh  RefreshDirective
	state    SelectionState
	valid    bool
}

func (p Preparation) Validate() error {
	if !p.valid {
		return contractError(errors.New("preparation is unset"))
	}
	switch p.state {
	case SelectionAvailable:
		return p.validateReadyArm()
	case SelectionRefreshRequired:
		return p.validateRefreshArm()
	case SelectionReassessAt:
		return p.validateReassessArm()
	default:
		return contractError(errors.New("preparation state is unsupported"))
	}
}

func (p Preparation) validateReadyArm() error {
	if p.refresh.valid || p.reassess != (ReassessDirective{}) {
		return contractError(errors.New("ready preparation carries another arm"))
	}
	return p.ready.Validate()
}

func (p Preparation) validateRefreshArm() error {
	if p.ready.valid || p.reassess != (ReassessDirective{}) {
		return contractError(errors.New("refresh preparation carries another arm"))
	}
	return p.refresh.Validate()
}

func (p Preparation) validateReassessArm() error {
	if p.ready.valid || p.refresh.valid {
		return contractError(errors.New("reassess preparation carries another arm"))
	}
	return p.reassess.Validate()
}

func (p Preparation) Ready() (PreparedRelease, bool) {
	return p.ready, p.valid && p.state == SelectionAvailable
}
func (p Preparation) Refresh() (RefreshDirective, bool) {
	return p.refresh, p.valid && p.state == SelectionRefreshRequired
}
func (p Preparation) Reassess() (ReassessDirective, bool) {
	return p.reassess, p.valid && p.state == SelectionReassessAt
}

func (a AvailableRelease) PrepareAt(observation temporal.Instant) (Preparation, error) {
	if err := a.Validate(); err != nil {
		return Preparation{}, err
	}
	assessment, err := AssessLatest(AssessLatestRequest{Latest: a.latest, Observation: observation})
	if err != nil {
		return Preparation{}, err
	}
	switch assessment.Freshness() {
	case LatestFreshnessExpired:
		return sealPreparation(Preparation{
			state: SelectionRefreshRequired, refresh: RefreshDirective{valid: true}, valid: true,
		})
	case LatestFreshnessNotYetValid:
		at, ok := assessment.Boundary()
		if !ok {
			return Preparation{}, verificationError(errors.New("not-yet-valid preparation lacks boundary"))
		}
		return sealPreparation(Preparation{
			state: SelectionReassessAt, reassess: ReassessDirective{At: at}, valid: true,
		})
	case LatestFreshnessCurrent:
		ready := PreparedRelease{
			candidateManifest: a.candidateManifest,
			installedManifest: a.installedManifest, latest: a.latest,
			artifact: a.candidateArtifact, observation: observation,
			assessment: assessment, valid: true,
		}
		return sealPreparation(Preparation{state: SelectionAvailable, ready: ready, valid: true})
	default:
		return Preparation{}, verificationError(errors.New("preparation freshness escaped its domain"))
	}
}

func sealPreparation(candidate Preparation) (Preparation, error) {
	if err := candidate.Validate(); err != nil {
		return Preparation{}, err
	}
	return candidate, nil
}

var (
	_ core.OffWireEnum = CachedLatestUnknown
	_ core.OffWireEnum = SelectionUnknown
)
