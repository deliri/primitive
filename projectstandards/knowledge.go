package projectstandards

import "errors"

const (
	ReasonMaximum    = 32
	BoundaryMaximum  = 32
	FeatureMaximum   = 32
	UsageMaximum     = 16
	UsageStepMaximum = 16
	ComponentMaximum = 64
	// PackageMaximum is a bounded project capacity, not a current package count.
	PackageMaximum            = 512
	GroupMaximum              = 32
	ContributionMaximum       = 32
	AssuranceReferenceMaximum = 16
	AssuranceSurfaceMaximum   = 16
)

type Boundary struct {
	Title  Name `json:"title"`
	Detail Text `json:"detail"`
}

func (b Boundary) Validate() error { return contractJoin(b.Title.Validate(), b.Detail.Validate()) }

type Reason struct {
	Title  Name `json:"title"`
	Detail Text `json:"detail"`
}

func (r Reason) Validate() error { return contractJoin(r.Title.Validate(), r.Detail.Validate()) }

type Feature struct {
	ID               Identifier    `json:"id"`
	Title            Name          `json:"title"`
	Technical        Text          `json:"technical_contract"`
	Benefit          Text          `json:"benefit"`
	ProofRequirement Text          `json:"proof_requirement"`
	Delivery         DeliveryState `json:"delivery"`
}

func (f Feature) Validate() error {
	return contractJoin(f.ID.Validate(), f.Title.Validate(), f.Technical.Validate(), f.Benefit.Validate(), f.ProofRequirement.Validate(), f.Delivery.Validate())
}

type UsageStep struct {
	Reference *CodeReference `json:"reference,omitempty"`
	Title     Name           `json:"title"`
	Detail    Text           `json:"detail"`
}

func (s UsageStep) Validate() error {
	if err := contractJoin(s.Title.Validate(), s.Detail.Validate()); err != nil {
		return err
	}
	if s.Reference != nil {
		return s.Reference.Validate()
	}
	return nil
}

type Usage struct {
	ID       Identifier  `json:"id"`
	Title    Name        `json:"title"`
	Audience Text        `json:"audience"`
	Goal     Text        `json:"goal"`
	Outcome  Text        `json:"outcome"`
	Steps    []UsageStep `json:"steps"`
}

func (u Usage) Validate() error {
	if len(u.Steps) == 0 || len(u.Steps) > UsageStepMaximum {
		return contractError(errors.New("project standards usage step count is invalid"))
	}
	if err := contractJoin(u.ID.Validate(), u.Title.Validate(), u.Audience.Validate(), u.Goal.Validate(), u.Outcome.Validate()); err != nil {
		return err
	}
	for index := range u.Steps {
		if err := u.Steps[index].Validate(); err != nil {
			return err
		}
	}
	return nil
}

type AssuranceStage uint8

const (
	AssuranceStageUnknown AssuranceStage = iota
	AssuranceStagePolicy
	AssuranceStageValidation
	AssuranceStageEffects
	AssuranceStageProof
	assuranceStageLimit
)

func assuranceStageLabels() []string { return []string{"", "policy", "validation", "effects", "proof"} }
func (s AssuranceStage) Validate() error {
	return validateEnum(uint8(s), assuranceStageLabels(), "project standards assurance stage is invalid")
}
func (s AssuranceStage) IsValid() bool  { return s.Validate() == nil }
func (s AssuranceStage) String() string { return enumString(uint8(s), assuranceStageLabels()) }
func (s AssuranceStage) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(s), assuranceStageLabels(), "project standards assurance stage is invalid")
}
func (s *AssuranceStage) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(errors.New("nil project standards assurance stage receiver"))
	}
	value, err := unmarshalEnum(data, assuranceStageLabels(), "project standards assurance stage is invalid")
	if err == nil {
		*s = AssuranceStage(value)
	}
	return err
}

type AssuranceAuthority uint8

const (
	AssuranceAuthorityUnknown AssuranceAuthority = iota
	AssuranceAuthorityProduct
	AssuranceAuthorityCore
	AssuranceAuthorityPrimitive
	AssuranceAuthorityIndependent
	assuranceAuthorityLimit
)

func assuranceAuthorityLabels() []string {
	return []string{"", "product", "core", "primitive", "independent"}
}
func (a AssuranceAuthority) Validate() error {
	return validateEnum(uint8(a), assuranceAuthorityLabels(), "project standards assurance authority is invalid")
}
func (a AssuranceAuthority) IsValid() bool  { return a.Validate() == nil }
func (a AssuranceAuthority) String() string { return enumString(uint8(a), assuranceAuthorityLabels()) }
func (a AssuranceAuthority) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(a), assuranceAuthorityLabels(), "project standards assurance authority is invalid")
}
func (a *AssuranceAuthority) UnmarshalJSON(data []byte) error {
	if a == nil {
		return jsonError(errors.New("nil project standards assurance authority receiver"))
	}
	value, err := unmarshalEnum(data, assuranceAuthorityLabels(), "project standards assurance authority is invalid")
	if err == nil {
		*a = AssuranceAuthority(value)
	}
	return err
}

type AssuranceControl struct {
	Statement  Text               `json:"statement"`
	References []CodeReference    `json:"references"`
	SurfaceIDs []Identifier       `json:"evidence_surface_ids"`
	Stage      AssuranceStage     `json:"stage"`
	Authority  AssuranceAuthority `json:"authority"`
}

func (c AssuranceControl) Validate() error {
	if len(c.References) == 0 || len(c.References) > AssuranceReferenceMaximum || len(c.SurfaceIDs) == 0 || len(c.SurfaceIDs) > AssuranceSurfaceMaximum {
		return contractError(errors.New("project standards assurance control bounds are invalid"))
	}
	if err := contractJoin(c.Stage.Validate(), c.Authority.Validate(), c.Statement.Validate()); err != nil {
		return err
	}
	for index := range c.References {
		if err := c.References[index].Validate(); err != nil {
			return err
		}
		for previous := range index {
			if codeReferenceEqual(c.References[previous], c.References[index]) {
				return conflictError(errors.New("project standards assurance reference is duplicated"))
			}
		}
	}
	return validateIdentifiers(c.SurfaceIDs)
}

type Assurance struct {
	Policy     AssuranceControl `json:"policy"`
	Validation AssuranceControl `json:"validation"`
	Effects    AssuranceControl `json:"effects"`
	Proof      AssuranceControl `json:"proof"`
}

func (a Assurance) Validate() error {
	controls := [...]AssuranceControl{a.Policy, a.Validation, a.Effects, a.Proof}
	stages := [...]AssuranceStage{AssuranceStagePolicy, AssuranceStageValidation, AssuranceStageEffects, AssuranceStageProof}
	for index := range controls {
		if err := controls[index].Validate(); err != nil {
			return err
		}
		if controls[index].Stage != stages[index] {
			return conflictError(errors.New("project standards assurance control occupies the wrong stage"))
		}
	}
	return nil
}

type ComponentKind uint8

const (
	ComponentKindUnknown ComponentKind = iota
	ComponentKindSourceFile
	ComponentKindIntegration
	ComponentKindRuntimeArea
	componentKindLimit
)

func componentKindLabels() []string {
	return []string{"", "source_file", "integration", "runtime_area"}
}
func (k ComponentKind) Validate() error {
	return validateEnum(uint8(k), componentKindLabels(), "project standards component kind is invalid")
}
func (k ComponentKind) IsValid() bool  { return k.Validate() == nil }
func (k ComponentKind) String() string { return enumString(uint8(k), componentKindLabels()) }
func (k ComponentKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(k), componentKindLabels(), "project standards component kind is invalid")
}
func (k *ComponentKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return jsonError(errors.New("nil project standards component kind receiver"))
	}
	value, err := unmarshalEnum(data, componentKindLabels(), "project standards component kind is invalid")
	if err == nil {
		*k = ComponentKind(value)
	}
	return err
}

type Component struct {
	Created          OptionalGitOrigin `json:"created"`
	AuthorPurpose    Text              `json:"purpose"`
	AuthorTitle      Name              `json:"title"`
	Path             SourcePath        `json:"path"`
	Language         Name              `json:"language"`
	Package          SourcePath        `json:"package"`
	AuthorRemoval    Text              `json:"removal"`
	AuthorReasons    []Reason          `json:"reasons"`
	AuthorOwns       []Boundary        `json:"owns"`
	AuthorDoesNotOwn []Boundary        `json:"does_not_own"`
	AuthorFeatures   []Feature         `json:"features"`
	Changed          GitOrigin         `json:"changed"`
	Kind             ComponentKind     `json:"kind"`
}

func (c Component) Validate() error {
	if err := contractJoin(c.Path.Validate(), c.Package.Validate(), c.AuthorTitle.Validate(), c.AuthorPurpose.Validate(), c.Language.Validate(), c.Kind.Validate(), c.Created.Validate(), c.Changed.Validate(), c.AuthorRemoval.Validate()); err != nil {
		return err
	}
	if !pathWithin(c.Package, c.Path, false) {
		return conflictError(errors.New("project standards component is outside its package"))
	}
	return validateKnowledgeLists(knowledgeLists{Reasons: c.AuthorReasons, Owns: c.AuthorOwns, Excludes: c.AuthorDoesNotOwn, Features: c.AuthorFeatures})
}

type ProductKnowledge struct {
	Created        OptionalGitOrigin `json:"created"`
	AuthorTitle    Name              `json:"title"`
	AuthorProblem  Text              `json:"problem"`
	AuthorPurpose  Text              `json:"purpose"`
	AuthorAudience Text              `json:"audience"`
	AuthorPromise  Text              `json:"promise"`
	SourcePath     SourcePath        `json:"source_path"`
	AuthorReasons  []Reason          `json:"reasons"`
	AuthorOwns     []Boundary        `json:"owns"`
	AuthorNonGoals []Boundary        `json:"non_goals"`
	AuthorFeatures []Feature         `json:"features"`
	Changed        GitOrigin         `json:"changed"`
}

func (p ProductKnowledge) Validate() error {
	if err := contractJoin(p.AuthorTitle.Validate(), p.AuthorProblem.Validate(), p.AuthorPurpose.Validate(), p.AuthorAudience.Validate(), p.AuthorPromise.Validate(), p.SourcePath.Validate(), p.Created.Validate(), p.Changed.Validate()); err != nil {
		return err
	}
	return validateKnowledgeLists(knowledgeLists{Reasons: p.AuthorReasons, Owns: p.AuthorOwns, Excludes: p.AuthorNonGoals, Features: p.AuthorFeatures})
}

type Inventory struct {
	CoverageBasisPoints *uint16 `json:"coverage_basis_points,omitempty"`
	GoPackages          uint32  `json:"go_packages"`
	JavaScriptUnits     uint32  `json:"javascript_units"`
	Files               uint32  `json:"files"`
	TestFiles           uint32  `json:"test_files"`
	Documents           uint32  `json:"documents"`
	TestDeclarations    uint32  `json:"test_declarations"`
	Benchmarks          uint32  `json:"benchmarks"`
	FuzzTargets         uint32  `json:"fuzz_targets"`
}

func (i Inventory) Validate() error {
	if i.Files == 0 || i.GoPackages > i.Files || i.JavaScriptUnits > i.Files || i.TestFiles > i.Files || i.Documents > i.Files {
		return contractError(errors.New("project standards inventory file accounting is invalid"))
	}
	if uint64(i.TestDeclarations) > uint64(i.TestFiles)*SourceFileDeclarationMaximum || uint64(i.Benchmarks)+uint64(i.FuzzTargets) > uint64(i.TestDeclarations) {
		return conflictError(errors.New("project standards inventory declaration accounting is invalid"))
	}
	if i.CoverageBasisPoints != nil && *i.CoverageBasisPoints > 10_000 {
		return contractError(errors.New("project standards inventory coverage is invalid"))
	}
	return nil
}

type PackageKnowledge struct {
	Created          OptionalGitOrigin `json:"created"`
	Path             SourcePath        `json:"path"`
	AuthorTitle      Name              `json:"title"`
	AuthorPurpose    Text              `json:"purpose"`
	AuthorAudience   Text              `json:"audience"`
	AuthorValue      Text              `json:"value"`
	AuthorSteward    Name              `json:"steward"`
	AuthorSubstrate  Name              `json:"substrate"`
	AuthorRuntime    Name              `json:"runtime"`
	AuthorRemoval    Text              `json:"removal"`
	AuthorProblem    Text              `json:"problem"`
	AuthorReasons    []Reason          `json:"reasons"`
	AuthorOwns       []Boundary        `json:"owns"`
	AuthorDoesNotOwn []Boundary        `json:"does_not_own"`
	AuthorUsage      []Usage           `json:"usage"`
	AuthorFeatures   []Feature         `json:"features"`
	AuthorComplexity []ComplexityClaim `json:"complexity_claims"`
	AuthorAssurance  Assurance         `json:"assurance"`
	Changed          GitOrigin         `json:"changed"`
}

func (p PackageKnowledge) Validate() error {
	if err := contractJoin(p.Path.Validate(), p.AuthorTitle.Validate(), p.AuthorProblem.Validate(), p.AuthorPurpose.Validate(), p.AuthorAudience.Validate(), p.AuthorValue.Validate(), p.AuthorSteward.Validate(), p.AuthorSubstrate.Validate(), p.AuthorRuntime.Validate(), p.Created.Validate(), p.Changed.Validate(), p.AuthorRemoval.Validate(), p.AuthorAssurance.Validate()); err != nil {
		return err
	}
	if err := validateKnowledgeLists(knowledgeLists{Reasons: p.AuthorReasons, Owns: p.AuthorOwns, Excludes: p.AuthorDoesNotOwn, Features: p.AuthorFeatures}); err != nil {
		return err
	}
	if err := validateUsage(p.AuthorUsage); err != nil {
		return err
	}
	return validateComplexityClaims(p.Path, p.AuthorComplexity)
}

type knowledgeLists struct {
	Reasons  []Reason
	Owns     []Boundary
	Excludes []Boundary
	Features []Feature
}

func validateKnowledgeLists(lists knowledgeLists) error {
	if !validKnowledgeListBounds(lists) {
		return contractError(errors.New("project standards authored knowledge list bounds are invalid"))
	}
	if err := validateReasons(lists.Reasons); err != nil {
		return err
	}
	if err := validateBoundaries(lists.Owns); err != nil {
		return err
	}
	if err := validateBoundaries(lists.Excludes); err != nil {
		return err
	}
	return validateFeatures(lists.Features)
}

func validKnowledgeListBounds(lists knowledgeLists) bool {
	return len(lists.Reasons) > 0 && len(lists.Reasons) <= ReasonMaximum && len(lists.Owns) > 0 && len(lists.Owns) <= BoundaryMaximum &&
		len(lists.Excludes) > 0 && len(lists.Excludes) <= BoundaryMaximum && len(lists.Features) > 0 && len(lists.Features) <= FeatureMaximum
}

func validateReasons(values []Reason) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
	}
	return nil
}

func validateBoundaries(values []Boundary) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
	}
	return nil
}

func validateFeatures(values []Feature) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		for previous := range index {
			if values[previous].ID == values[index].ID {
				return conflictError(errors.New("project standards feature identity is duplicated"))
			}
		}
	}
	return nil
}

func validateUsage(values []Usage) error {
	if len(values) == 0 || len(values) > UsageMaximum {
		return contractError(errors.New("project standards usage count is invalid"))
	}
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		for previous := range index {
			if values[previous].ID == values[index].ID {
				return conflictError(errors.New("project standards usage identity is duplicated"))
			}
		}
	}
	return nil
}

func validateComplexityClaims(owner SourcePath, values []ComplexityClaim) error {
	if len(values) > ComplexityClaimMaximum {
		return contractError(errors.New("project standards complexity claim count is invalid"))
	}
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if !pathWithin(owner, values[index].Operation.Path, true) {
			return conflictError(errors.New("project standards complexity operation is outside its package"))
		}
		for previous := range index {
			if values[previous].ID == values[index].ID {
				return conflictError(errors.New("project standards complexity claim identity is duplicated"))
			}
		}
	}
	return nil
}

func codeReferenceEqual(left, right CodeReference) bool {
	if left.Path != right.Path || (left.Symbol == nil) != (right.Symbol == nil) {
		return false
	}
	return left.Symbol == nil || *left.Symbol == *right.Symbol
}
