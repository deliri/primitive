package sourceclaim

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// Boundary states one named area a claim owns or explicitly does not own.
type Boundary struct {
	ID     ID   `json:"id"`
	Detail Text `json:"detail"`
}

func (b Boundary) Validate() error {
	return contractJoin(b.ID.Validate(), b.Detail.Validate())
}

// Narrative carries the concise in-struct statement and, when the reasoning
// is larger, the exact repository document that owns the full explanation.
// The applying source tool observes and proves the referenced bytes; the claim
// does not copy a digest that would become a second truth source.
type Narrative struct {
	Detail  *core.SourcePath `json:"detail,omitempty"`
	Summary Text             `json:"summary"`
}

func (n Narrative) Validate() error {
	if err := n.Summary.Validate(); err != nil {
		return err
	}
	if n.Detail != nil {
		return n.Detail.Validate()
	}
	return nil
}

// RequirementMode closes the distinction between mechanically observable
// source facts and judgments that still require a human.
type RequirementMode uint8

const (
	RequirementUnknown RequirementMode = iota
	RequirementCompiler
	RequirementExecutionEvidence
	RequirementHumanReview
	requirementModeLimit
)

func requirementModeTokens() [requirementModeLimit]string {
	return [...]string{"", "compiler", "execution_evidence", "human_review"}
}

// ExecutionKind closes the independent run phases a source claim may require.
// The runner and its time-based product view remain outside this package.
type ExecutionKind uint8

const (
	ExecutionUnknown ExecutionKind = iota
	ExecutionTest
	ExecutionRace
	ExecutionBenchmark
	ExecutionFuzz
	ExecutionTool
	executionKindLimit
)

func (k ExecutionKind) Validate() error {
	if !k.IsValid() {
		return contractError(errors.New("source claim execution kind is invalid"))
	}
	return nil
}

func (k ExecutionKind) IsValid() bool {
	return k > ExecutionUnknown && k < executionKindLimit && executionKindTokens()[k] != ""
}

// ExecutionRequirement names one independently executed phase and source
// subject. It does not name a runner, database, retry policy, or acceptance
// rule; those belong to an independent evidence authority.
type ExecutionRequirement struct {
	Subject core.SourceSubject `json:"subject"`
	Target  Reference          `json:"target"`
	Kind    ExecutionKind      `json:"kind"`
}

func (r ExecutionRequirement) Validate() error {
	return contractJoin(r.Subject.Validate(), r.Target.Validate(), r.Kind.Validate())
}

func (m RequirementMode) Validate() error {
	if !m.IsValid() {
		return contractError(errors.New("source claim requirement mode is invalid"))
	}
	return nil
}

func (m RequirementMode) IsValid() bool {
	return m > RequirementUnknown && m < requirementModeLimit && requirementModeTokens()[m] != ""
}

func (m RequirementMode) String() string {
	if m >= requirementModeLimit {
		return ""
	}
	return requirementModeTokens()[m]
}

// CompilerPredicate closes the mechanical source facts an offline source tool
// may establish through the shared agreement.
type CompilerPredicate uint8

const (
	CompilerPredicateUnknown CompilerPredicate = iota
	CompilerSubjectPresent
	CompilerSubjectAbsent
	CompilerDeclarationPresent
	CompilerDeclarationAbsent
	CompilerImportPresent
	CompilerImportAbsent
	CompilerEffectPresent
	CompilerEffectAbsent
	CompilerBuildSelected
	CompilerBuildExcluded
	CompilerMembershipComplete
	compilerPredicateLimit
)

func compilerPredicateTokens() [compilerPredicateLimit]string {
	return [...]string{"", "subject_present", "subject_absent", "declaration_present", "declaration_absent", "import_present", "import_absent", "effect_present", "effect_absent", "build_selected", "build_excluded", "membership_complete"}
}

func (p CompilerPredicate) Validate() error {
	if !p.IsValid() {
		return contractError(errors.New("source claim compiler predicate is invalid"))
	}
	return nil
}

func (p CompilerPredicate) IsValid() bool {
	return p > CompilerPredicateUnknown && p < compilerPredicateLimit && compilerPredicateTokens()[p] != ""
}

func (p CompilerPredicate) String() string {
	if p >= compilerPredicateLimit {
		return ""
	}
	return compilerPredicateTokens()[p]
}

func (p CompilerPredicate) needsReference() bool {
	return p >= CompilerDeclarationPresent && p <= CompilerBuildExcluded
}

// CompilerRequirement selects one typed mechanical fact without encoding an
// executable query or copying production logic into prose.
type CompilerRequirement struct {
	Target    core.SourceSubject `json:"target"`
	Reference *Reference         `json:"reference,omitempty"`
	Predicate CompilerPredicate  `json:"predicate"`
}

func (r CompilerRequirement) Validate() error {
	if err := contractJoin(r.Target.Validate(), r.Predicate.Validate()); err != nil {
		return err
	}
	if r.Predicate.needsReference() != (r.Reference != nil) {
		return contractError(errors.New("source claim compiler reference shape does not match its predicate"))
	}
	if r.Reference != nil {
		return r.Reference.Validate()
	}
	return nil
}

// Requirement states how one independently checkable part of a claim must be
// decided. Compiler data is present only for compiler mode.
type Requirement struct {
	Compiler  *CompilerRequirement  `json:"compiler,omitempty"`
	Execution *ExecutionRequirement `json:"execution,omitempty"`
	ID        ID                    `json:"id"`
	Statement Text                  `json:"statement"`
	Mode      RequirementMode       `json:"mode"`
}

func (r Requirement) Validate() error {
	if err := contractJoin(r.ID.Validate(), r.Statement.Validate(), r.Mode.Validate()); err != nil {
		return err
	}
	if r.Mode == RequirementHumanReview {
		if r.Compiler != nil || r.Execution != nil {
			return contractError(errors.New("human review requirement carries compiler data"))
		}
		return nil
	}
	if r.Mode == RequirementExecutionEvidence {
		if r.Compiler != nil || r.Execution == nil {
			return contractError(errors.New("execution evidence requirement has invalid typed data"))
		}
		return r.Execution.Validate()
	}
	if r.Compiler == nil || r.Execution != nil {
		return contractError(errors.New("compiler requirement omits compiler data"))
	}
	return r.Compiler.Validate()
}

// Claim is one atomic human explanation. Each Claim states one problem and
// one chosen solution rather than collapsing a package into one story.
type Claim struct {
	ID           ID                    `json:"id"`
	Author       core.Ed25519PublicKey `json:"author"`
	Subject      core.SourceSubject    `json:"subject"`
	Title        Text                  `json:"title"`
	Problem      Narrative             `json:"problem"`
	Solution     Narrative             `json:"solution"`
	Benefit      Narrative             `json:"benefit"`
	Removal      Narrative             `json:"removal"`
	Owns         []Boundary            `json:"owns"`
	DoesNotOwn   []Boundary            `json:"does_not_own"`
	Requirements []Requirement         `json:"requirements"`
}

func (c Claim) Validate() error {
	if err := contractJoin(c.ID.Validate(), c.Author.Validate(), c.Subject.Validate(), c.Title.Validate(), c.Problem.Validate(), c.Solution.Validate(), c.Benefit.Validate(), c.Removal.Validate()); err != nil {
		return err
	}
	if len(c.Owns) == 0 || len(c.DoesNotOwn) == 0 || len(c.Requirements) == 0 {
		return contractError(errors.New("source claim omits ownership or proof requirements"))
	}
	if err := validateBoundaries(c.Owns, c.DoesNotOwn); err != nil {
		return err
	}
	return validateRequirements(c.Requirements)
}

func validateBoundaries(owns, excludes []Boundary) error {
	if err := validateBoundaryList(owns); err != nil {
		return err
	}
	if err := validateBoundaryList(excludes); err != nil {
		return err
	}
	return rejectBoundaryOverlap(owns, excludes)
}

func validateBoundaryList(values []Boundary) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if index > 0 && values[index-1].ID.String() >= values[index].ID.String() {
			return conflictError(errors.New("source claim boundaries are duplicated or not canonical"))
		}
	}
	return nil
}

func rejectBoundaryOverlap(owns, excludes []Boundary) error {
	left, right := 0, 0
	for left < len(owns) && right < len(excludes) {
		owned := owns[left].ID.String()
		excluded := excludes[right].ID.String()
		if owned == excluded {
			return conflictError(errors.New("source claim boundary identity is both owned and excluded"))
		}
		if owned < excluded {
			left++
		} else {
			right++
		}
	}
	return nil
}

func validateRequirements(values []Requirement) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if index > 0 && values[index-1].ID.String() >= values[index].ID.String() {
			return conflictError(errors.New("source claim requirements are duplicated or not canonical"))
		}
	}
	return nil
}

func contractJoin(values ...error) error {
	for _, value := range values {
		if value != nil {
			return contractError(errors.Join(values...))
		}
	}
	return nil
}
