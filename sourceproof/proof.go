package sourceproof

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/sourceclaim"
)

// State closes every independently checked requirement outcome.
type State uint8

const (
	StateUnknown State = iota
	StateProven
	StateContradicted
	StateUnproven
	StateStale
	StateUnavailable
	StateHumanReviewRequired
	stateLimit
)

func (s State) Validate() error {
	if !s.IsValid() {
		return contractError(errors.New("source proof state is invalid"))
	}
	return nil
}

func (s State) IsValid() bool {
	return s > StateUnknown && s < stateLimit && stateTokens()[s] != ""
}

func stateTokens() [stateLimit]string {
	return [...]string{"", "proven", "contradicted", "unproven", "stale", "unavailable", "human_review_required"}
}

func (s State) String() string {
	if s >= stateLimit {
		return ""
	}
	return stateTokens()[s]
}

// EvidenceKind distinguishes compiler observations, independently retained
// execution receipts, and human review receipts without treating them as
// interchangeable proof.
type EvidenceKind uint8

const (
	EvidenceUnknown EvidenceKind = iota
	EvidenceSourceObservation
	EvidenceTestReceipt
	EvidenceRaceReceipt
	EvidenceBenchmarkReceipt
	EvidenceFuzzReceipt
	EvidenceToolReceipt
	EvidenceHumanReviewReceipt
	evidenceKindLimit
)

func (k EvidenceKind) Validate() error {
	if !k.IsValid() {
		return contractError(errors.New("source proof evidence kind is invalid"))
	}
	return nil
}

func (k EvidenceKind) IsValid() bool {
	return k > EvidenceUnknown && k < evidenceKindLimit && evidenceKindTokens()[k] != ""
}

func evidenceKindTokens() [evidenceKindLimit]string {
	return [...]string{"", "source_observation", "test_receipt", "race_receipt", "benchmark_receipt", "fuzz_receipt", "tool_receipt", "human_review_receipt"}
}

func (k EvidenceKind) String() string {
	if k >= evidenceKindLimit {
		return ""
	}
	return evidenceKindTokens()[k]
}

// EvidenceReference binds one independently retained artifact by digest.
type EvidenceReference struct {
	Subject   core.SourceSubject    `json:"subject"`
	Digest    core.SHA256Digest     `json:"digest"`
	Snapshot  core.SourceSnapshot   `json:"snapshot"`
	Authority core.Ed25519PublicKey `json:"authority"`
	Kind      EvidenceKind          `json:"kind"`
}

func (r EvidenceReference) Validate() error {
	return contractJoin(r.Subject.Validate(), r.Digest.Validate(), r.Snapshot.Validate(), r.Authority.Validate(), r.Kind.Validate())
}

// RequirementResult retains the exact outcome for one typed claim
// requirement. Evidence identity is preserved instead of reduced to a count.
type RequirementResult struct {
	Requirement sourceclaim.ID      `json:"requirement"`
	Evidence    []EvidenceReference `json:"evidence"`
	State       State               `json:"state"`
}

func (r RequirementResult) Validate() error {
	if err := contractJoin(r.Requirement.Validate(), r.State.Validate()); err != nil {
		return err
	}
	if err := validateEvidence(r.Evidence); err != nil {
		return err
	}
	return r.validateIntrinsicStateEvidence()
}

func (r RequirementResult) ValidateAgainst(requirement sourceclaim.Requirement, snapshot core.SourceSnapshot, subject core.SourceSubject) error {
	if err := contractJoin(r.Validate(), requirement.Validate()); err != nil {
		return err
	}
	if r.Requirement != requirement.ID {
		return conflictError(errors.New("source proof requirement identity differs from its claim"))
	}
	if err := validateEvidence(r.Evidence); err != nil {
		return err
	}
	if err := r.validateSnapshots(snapshot); err != nil {
		return err
	}
	return r.validateMode(requirement, subject)
}

func (r RequirementResult) validateSnapshots(snapshot core.SourceSnapshot) error {
	for _, evidence := range r.Evidence {
		if r.State == StateStale && evidence.Snapshot == snapshot {
			return conflictError(errors.New("source proof stale state cites current-snapshot evidence"))
		}
		if r.State != StateStale && evidence.Snapshot != snapshot {
			return conflictError(errors.New("source proof current state cites stale evidence"))
		}
	}
	return nil
}

func (r RequirementResult) validateMode(requirement sourceclaim.Requirement, claimSubject core.SourceSubject) error {
	if err := r.validateStateEvidence(requirement.Mode); err != nil {
		return err
	}
	if r.State == StateHumanReviewRequired {
		return nil
	}
	switch requirement.Mode {
	case sourceclaim.RequirementCompiler:
		return r.validateCompilerMode(*requirement.Compiler)
	case sourceclaim.RequirementExecutionEvidence:
		return r.validateExecutionMode(*requirement.Execution)
	case sourceclaim.RequirementHumanReview:
		return r.validateHumanMode(claimSubject)
	default:
		return contractError(errors.New("source proof requirement mode is invalid"))
	}
}

func (r RequirementResult) validateStateEvidence(mode sourceclaim.RequirementMode) error {
	if r.State == StateHumanReviewRequired {
		if mode != sourceclaim.RequirementHumanReview {
			return conflictError(errors.New("source proof human-review state disagrees with its requirement"))
		}
	}
	return nil
}

func (r RequirementResult) validateIntrinsicStateEvidence() error {
	if r.State == StateHumanReviewRequired && len(r.Evidence) != 0 {
		return conflictError(errors.New("source proof human-review-required state carries evidence"))
	}
	if (r.State == StateProven || r.State == StateContradicted || r.State == StateStale) && len(r.Evidence) == 0 {
		return contractError(errors.New("source proof decisive state has no evidence"))
	}
	return nil
}

func (r RequirementResult) validateCompilerMode(requirement sourceclaim.CompilerRequirement) error {
	for _, evidence := range r.Evidence {
		if evidence.Kind != EvidenceSourceObservation {
			return conflictError(errors.New("compiler requirement cites non-source evidence"))
		}
		if evidence.Subject != requirement.Target {
			return conflictError(errors.New("compiler requirement cites the wrong source subject"))
		}
	}
	return nil
}

func (r RequirementResult) validateExecutionMode(requirement sourceclaim.ExecutionRequirement) error {
	wantKind, err := evidenceKindForExecution(requirement.Kind)
	if err != nil {
		return err
	}
	for _, evidence := range r.Evidence {
		if evidence.Kind != wantKind {
			return conflictError(errors.New("execution requirement cites the wrong receipt kind"))
		}
		if evidence.Subject != requirement.Subject {
			return conflictError(errors.New("execution requirement cites the wrong source subject"))
		}
	}
	return nil
}

func (r RequirementResult) validateHumanMode(claimSubject core.SourceSubject) error {
	for _, evidence := range r.Evidence {
		if evidence.Kind != EvidenceHumanReviewReceipt {
			return conflictError(errors.New("human requirement cites non-review evidence"))
		}
		if evidence.Subject != claimSubject {
			return conflictError(errors.New("human review receipt cites the wrong source subject"))
		}
	}
	return nil
}

// Result binds one complete atomic claim to all of its requirement outcomes.
type Result struct {
	Claim        sourceclaim.ID        `json:"claim"`
	Subject      core.SourceSubject    `json:"subject"`
	ClaimDigest  core.SHA256Digest     `json:"claim_digest"`
	Snapshot     core.SourceSnapshot   `json:"snapshot"`
	Verifier     core.Ed25519PublicKey `json:"verifier"`
	Requirements []RequirementResult   `json:"requirements"`
}

// Validate proves the proof record's intrinsic structure. ValidateAgainst
// adds the independently supplied authored claim and proves structural
// agreement; product acceptance remains outside this package.
func (r Result) Validate() error {
	if err := contractJoin(r.Claim.Validate(), r.Subject.Validate(), r.ClaimDigest.Validate(), r.Snapshot.Validate(), r.Verifier.Validate()); err != nil {
		return err
	}
	if len(r.Requirements) == 0 {
		return contractError(errors.New("source proof has no requirement results"))
	}
	for index := range r.Requirements {
		if err := r.Requirements[index].Validate(); err != nil {
			return err
		}
		if index > 0 && r.Requirements[index-1].Requirement.String() >= r.Requirements[index].Requirement.String() {
			return conflictError(errors.New("source proof requirements are duplicated or not canonical"))
		}
	}
	return nil
}

// ValidateAgainst refuses self-issued proof and proves that the complete
// ordered requirement sequence binds to the independently supplied claim.
func (r Result) ValidateAgainst(claim sourceclaim.Claim) error {
	if err := contractJoin(claim.Validate(), r.Validate()); err != nil {
		return err
	}
	if r.Claim != claim.ID || r.Subject != claim.Subject {
		return conflictError(errors.New("source proof claim identity or subject differs from authored claim"))
	}
	if r.Verifier == claim.Author {
		return conflictError(errors.New("source claim author cannot issue its own proof"))
	}
	wantDigest, err := claim.Digest()
	if err != nil {
		return err
	}
	if r.ClaimDigest != wantDigest {
		return conflictError(errors.New("source proof digest differs from canonical authored claim"))
	}
	if len(r.Requirements) != len(claim.Requirements) {
		return conflictError(errors.New("source proof requirement accounting does not close"))
	}
	for index := range r.Requirements {
		if err := r.Requirements[index].ValidateAgainst(claim.Requirements[index], r.Snapshot, claim.Subject); err != nil {
			return err
		}
	}
	return nil
}

func evidenceKindForExecution(run sourceclaim.ExecutionKind) (EvidenceKind, error) {
	switch run {
	case sourceclaim.ExecutionTest:
		return EvidenceTestReceipt, nil
	case sourceclaim.ExecutionRace:
		return EvidenceRaceReceipt, nil
	case sourceclaim.ExecutionBenchmark:
		return EvidenceBenchmarkReceipt, nil
	case sourceclaim.ExecutionFuzz:
		return EvidenceFuzzReceipt, nil
	case sourceclaim.ExecutionTool:
		return EvidenceToolReceipt, nil
	default:
		return EvidenceUnknown, contractError(errors.New("source proof execution kind is invalid"))
	}
}

func validateEvidence(values []EvidenceReference) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if index == 0 {
			continue
		}
		order, err := compareEvidenceReferences(values[index-1], values[index])
		if err != nil {
			return err
		}
		if order >= 0 {
			return conflictError(errors.New("source proof evidence is duplicated or not canonical"))
		}
	}
	return nil
}

func compareEvidenceReferences(left, right EvidenceReference) (int, error) {
	if order := core.CompareSourceSubjects(left.Subject, right.Subject); order != 0 {
		return order, nil
	}
	leftDigest, leftDigestErr := left.Digest.Hex()
	rightDigest, rightDigestErr := right.Digest.Hex()
	if leftDigestErr != nil || rightDigestErr != nil {
		return 0, contractError(errors.Join(leftDigestErr, rightDigestErr))
	}
	if leftDigest != rightDigest {
		return compareString(leftDigest, rightDigest), nil
	}
	if left.Snapshot != right.Snapshot {
		return compareString(left.Snapshot.String(), right.Snapshot.String()), nil
	}
	if left.Kind != right.Kind {
		return compareUint8(uint8(left.Kind), uint8(right.Kind)), nil
	}
	leftAuthority, leftAuthorityErr := left.Authority.Hex()
	rightAuthority, rightAuthorityErr := right.Authority.Hex()
	if leftAuthorityErr != nil || rightAuthorityErr != nil {
		return 0, contractError(errors.Join(leftAuthorityErr, rightAuthorityErr))
	}
	return compareString(leftAuthority, rightAuthority), nil
}

func compareString(left, right string) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func compareUint8(left, right uint8) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}
