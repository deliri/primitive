package standard

import (
	"errors"
	"slices"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

const (
	// PackageSourceFileMaximum is a resource ceiling for one package projection,
	// not a claim about how many files any current package contains.
	PackageSourceFileMaximum = 65_535
	// SourceFileDeclarationMaximum bounds test, benchmark, and fuzz declarations
	// admitted from one source file.
	SourceFileDeclarationMaximum = 1_024
	// SourceEffectSiteMaximum bounds exact call sites retained for one file.
	SourceEffectSiteMaximum = 2_048
)

// SourceLanguage identifies the compiler-visible source family of one file.
type SourceLanguage uint8

const (
	SourceLanguageUnknown SourceLanguage = iota
	SourceLanguageGo
	SourceLanguageJavaScript
	SourceLanguageMarkdown
	SourceLanguageOther
	sourceLanguageLimit
)

func sourceLanguageLabels() []string {
	return []string{"", "go", "javascript", "markdown", "other"}
}

func (l SourceLanguage) Validate() error {
	return validateEnum(uint8(l), sourceLanguageLabels(), sourceLanguageInvalidDiagnostic)
}

func (l SourceLanguage) IsValid() bool  { return l.Validate() == nil }
func (l SourceLanguage) String() string { return enumString(uint8(l), sourceLanguageLabels()) }
func (l SourceLanguage) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(l), sourceLanguageLabels(), sourceLanguageInvalidDiagnostic)
}
func (l *SourceLanguage) UnmarshalJSON(data []byte) error {
	if l == nil {
		return jsonError(errors.New("nil standard source language receiver"))
	}
	value, err := unmarshalEnum(data, sourceLanguageLabels(), sourceLanguageInvalidDiagnostic)
	if err == nil {
		*l = SourceLanguage(value)
	}
	return err
}

// SourceFileKind identifies the role a file has in its package.
type SourceFileKind uint8

const (
	SourceFileKindUnknown SourceFileKind = iota
	SourceFileKindProduction
	SourceFileKindTest
	SourceFileKindDocumentation
	SourceFileKindConfiguration
	SourceFileKindAsset
	sourceFileKindLimit
)

func sourceFileKindLabels() []string {
	return []string{"", "production", "test_source", "documentation_source", "configuration", "asset"}
}

func (k SourceFileKind) Validate() error {
	return validateEnum(uint8(k), sourceFileKindLabels(), sourceFileKindInvalidDiagnostic)
}

func (k SourceFileKind) IsValid() bool  { return k.Validate() == nil }
func (k SourceFileKind) String() string { return enumString(uint8(k), sourceFileKindLabels()) }
func (k SourceFileKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(k), sourceFileKindLabels(), sourceFileKindInvalidDiagnostic)
}
func (k *SourceFileKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return jsonError(errors.New("nil standard source file kind receiver"))
	}
	value, err := unmarshalEnum(data, sourceFileKindLabels(), sourceFileKindInvalidDiagnostic)
	if err == nil {
		*k = SourceFileKind(value)
	}
	return err
}

// PrimitiveEffectPosture records the exact relationship between a source file,
// product policy, and Primitive-owned real-world effects.
type PrimitiveEffectPosture uint8

const (
	PrimitiveEffectPostureUnknown PrimitiveEffectPosture = iota
	// PrimitiveEffectNotApplicable means the file cannot execute policy or effects.
	PrimitiveEffectNotApplicable
	// PrimitiveEffectPurePolicy means the file contains policy with no real-world effect.
	PrimitiveEffectPurePolicy
	// PrimitiveEffectMediated means every observed effect crosses a Primitive capability.
	PrimitiveEffectMediated
	// PrimitiveEffectImplementation means the file implements the named Primitive capability.
	PrimitiveEffectImplementation
	// PrimitiveEffectDirectObserved means a real-world effect bypasses its named Primitive owner.
	PrimitiveEffectDirectObserved
	// PrimitiveEffectUnresolved means the scan could not classify every effect site.
	PrimitiveEffectUnresolved
	primitiveEffectPostureLimit
)

func primitiveEffectPostureLabels() []string {
	return []string{"", "not_applicable", "pure_policy", "primitive_mediated", primitiveImplementationText, "direct_effect_observed", canonicalUnresolvedText}
}

func (p PrimitiveEffectPosture) Validate() error {
	return validateEnum(uint8(p), primitiveEffectPostureLabels(), primitiveEffectPostureInvalidDiagnostic)
}

func (p PrimitiveEffectPosture) IsValid() bool { return p.Validate() == nil }
func (p PrimitiveEffectPosture) String() string {
	return enumString(uint8(p), primitiveEffectPostureLabels())
}
func (p PrimitiveEffectPosture) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(p), primitiveEffectPostureLabels(), primitiveEffectPostureInvalidDiagnostic)
}
func (p *PrimitiveEffectPosture) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil standard Primitive effect posture receiver"))
	}
	value, err := unmarshalEnum(data, primitiveEffectPostureLabels(), primitiveEffectPostureInvalidDiagnostic)
	if err == nil {
		*p = PrimitiveEffectPosture(value)
	}
	return err
}

// PrimitiveCapabilityUse names one compiler-owned Primitive package used by a
// file, or the package that a direct-effect observation should have used.
type PrimitiveCapabilityUse struct {
	Package core.PackageIdentity `json:"package"`
}

// Validate proves that Package resolves through Primitive's compiled catalog.
func (u PrimitiveCapabilityUse) Validate() error {
	_, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeTest, u.Package))
	if err != nil {
		return contractError(err)
	}
	return nil
}

// SourceFileDeclarations retains declarations attributable to exactly one
// source file. TestDeclarations includes tests, benchmarks, and fuzz targets.
type SourceFileDeclarations struct {
	Symbols          []SourceDeclaration `json:"symbols"`
	TestDeclarations uint16              `json:"test_declarations"`
	Benchmarks       uint16              `json:"benchmarks"`
	FuzzTargets      uint16              `json:"fuzz_targets"`
}

// SourceEffectSite is one exact syntax observation behind an effect fact.
// Capability may be absent only when the site is unresolved. Selector may be
// absent for an unresolved whole-import effect such as a blank import.
type SourceEffectSite struct {
	Capability *PrimitiveCapabilityUse `json:"capability,omitempty"`
	Selector   *Identifier             `json:"selector,omitempty"`
	ImportPath SourcePath              `json:"import_path"`
	Line       uint32                  `json:"line"`
	Column     uint32                  `json:"column"`
}

func (s SourceEffectSite) Validate() error {
	if err := s.ImportPath.Validate(); err != nil {
		return err
	}
	if s.Line == 0 || s.Column == 0 {
		return contractError(errors.New("standard effect site coordinate is absent"))
	}
	if s.Capability != nil {
		if err := s.Capability.Validate(); err != nil {
			return err
		}
	}
	if s.Selector != nil {
		return s.Selector.Validate()
	}
	return nil
}

func (d SourceFileDeclarations) Validate() error {
	if d.TestDeclarations > SourceFileDeclarationMaximum || len(d.Symbols) > SourceDeclarationMaximum {
		return contractError(errors.New("standard source file declaration count exceeds its bound"))
	}
	if uint32(d.Benchmarks)+uint32(d.FuzzTargets) > uint32(d.TestDeclarations) {
		return conflictError(errors.New("standard source file declaration accounting does not close"))
	}
	for index := range d.Symbols {
		if err := d.Symbols[index].Validate(); err != nil {
			return err
		}
		if index > 0 && (d.Symbols[index-1].Line > d.Symbols[index].Line ||
			d.Symbols[index-1].Line == d.Symbols[index].Line && d.Symbols[index-1].Column >= d.Symbols[index].Column) {
			return conflictError(errors.New("standard source declarations are duplicated or not in source order"))
		}
	}
	return nil
}

// SourceFileEffects is the bounded effect analysis for one exact source file.
type SourceFileEffects struct {
	Capabilities   []PrimitiveCapabilityUse `json:"capabilities"`
	Direct         []SourceEffectSite       `json:"direct"`
	Mediated       []SourceEffectSite       `json:"mediated"`
	Implementation []SourceEffectSite       `json:"implementation"`
	Unresolved     []SourceEffectSite       `json:"unresolved"`
	Posture        PrimitiveEffectPosture   `json:"posture"`
}

func (e SourceFileEffects) Validate() error {
	if err := e.Posture.Validate(); err != nil {
		return err
	}
	if len(e.Capabilities) > core.PrimitivePackageCount {
		return contractError(errors.New("standard source file capability count exceeds the Primitive catalog"))
	}
	if err := e.validateCapabilities(); err != nil {
		return err
	}
	if err := e.validateSites(); err != nil {
		return err
	}
	return e.validatePostureShape()
}

func (e SourceFileEffects) validateCapabilities() error {
	for index := range e.Capabilities {
		if err := e.Capabilities[index].Validate(); err != nil {
			return err
		}
		for previous := range index {
			if e.Capabilities[previous].Package == e.Capabilities[index].Package {
				return conflictError(errors.New("standard source file capability is duplicated"))
			}
		}
	}
	return nil
}

func (e SourceFileEffects) validateSites() error {
	total := len(e.Direct) + len(e.Mediated) + len(e.Implementation) + len(e.Unresolved)
	if total > SourceEffectSiteMaximum {
		return contractError(errors.New("standard source effect site count exceeds its bound"))
	}
	for _, group := range [][]SourceEffectSite{e.Direct, e.Mediated, e.Implementation, e.Unresolved} {
		for index := range group {
			if err := group[index].Validate(); err != nil {
				return err
			}
		}
	}
	for _, site := range append(append(append([]SourceEffectSite{}, e.Direct...), e.Mediated...), e.Implementation...) {
		if site.Capability == nil || site.Selector == nil {
			return conflictError(errors.New("standard resolved effect site lacks capability or selector"))
		}
	}
	return e.validateCapabilityClosure()
}

func (e SourceFileEffects) validateCapabilityClosure() error {
	seen := make([]core.PackageIdentity, 0, len(e.Capabilities))
	for _, group := range [][]SourceEffectSite{e.Direct, e.Mediated, e.Implementation, e.Unresolved} {
		for _, site := range group {
			if site.Capability == nil || slices.Contains(seen, site.Capability.Package) {
				continue
			}
			seen = append(seen, site.Capability.Package)
		}
	}
	if len(seen) != len(e.Capabilities) {
		return conflictError(errors.New("standard capability summary differs from source sites"))
	}
	for _, use := range e.Capabilities {
		if !slices.Contains(seen, use.Package) {
			return conflictError(errors.New("standard capability summary has no source site"))
		}
	}
	return nil
}

// DerivedPosture collapses the orthogonal evidence only for presentation.
// Direct bypass evidence has precedence over unresolved evidence, so an
// uncertain sibling call can never hide a known boundary violation.
func (e SourceFileEffects) DerivedPosture() PrimitiveEffectPosture {
	if len(e.Direct) > 0 {
		return PrimitiveEffectDirectObserved
	}
	if len(e.Unresolved) > 0 {
		return PrimitiveEffectUnresolved
	}
	if len(e.Implementation) > 0 {
		return PrimitiveEffectImplementation
	}
	if len(e.Mediated) > 0 {
		return PrimitiveEffectMediated
	}
	if e.Posture == PrimitiveEffectNotApplicable {
		return PrimitiveEffectNotApplicable
	}
	return PrimitiveEffectPurePolicy
}

func (e SourceFileEffects) validatePostureShape() error {
	if e.Posture != e.DerivedPosture() {
		return conflictError(errors.New("standard effect posture contradicts orthogonal source facts"))
	}
	switch e.Posture {
	case PrimitiveEffectNotApplicable, PrimitiveEffectPurePolicy:
		if len(e.Capabilities) != 0 || len(e.Direct)+len(e.Mediated)+len(e.Implementation)+len(e.Unresolved) != 0 {
			return conflictError(errors.New("standard effect-free posture carries effect facts"))
		}
	case PrimitiveEffectMediated, PrimitiveEffectImplementation, PrimitiveEffectDirectObserved:
		if len(e.Capabilities) == 0 {
			return conflictError(errors.New("standard resolved effect posture has incomplete capability accounting"))
		}
	case PrimitiveEffectUnresolved:
		if len(e.Unresolved) == 0 {
			return conflictError(errors.New("standard unresolved effect posture has no unresolved site"))
		}
	default:
		return contractError(errors.New("standard validated effect posture has no shape"))
	}
	return nil
}

// SourceFile is the exact generated structure and effect observation for one file.
type SourceFile struct {
	Imports      *SourceFileImports     `json:"imports,omitempty"`
	Path         SourcePath             `json:"path"`
	Package      SourcePath             `json:"package"`
	Declarations SourceFileDeclarations `json:"declarations"`
	Effects      SourceFileEffects      `json:"effects"`
	Language     SourceLanguage         `json:"language"`
	Kind         SourceFileKind         `json:"kind"`
	Generated    bool                   `json:"generated"`
}

func (f SourceFile) Validate() error {
	if err := contractJoin(f.Path.Validate(), f.Package.Validate(), f.Language.Validate(), f.Kind.Validate(), f.Declarations.Validate(), f.Effects.Validate()); err != nil {
		return err
	}
	if f.Imports != nil {
		if err := f.Imports.Validate(); err != nil {
			return err
		}
	}
	if !pathWithin(f.Package, f.Path, false) {
		return conflictError(errors.New("standard source file is outside its package"))
	}
	if err := f.validateKind(); err != nil {
		return err
	}
	if err := f.validateCapabilityScope(); err != nil {
		return err
	}
	return nil
}

// ExecutesPolicyThroughPrimitive reports the narrow positive contract: this
// product file has effects and every observed effect is Primitive-mediated.
func (f SourceFile) ExecutesPolicyThroughPrimitive() bool {
	return f.Validate() == nil && f.Effects.Posture == PrimitiveEffectMediated
}

func (f SourceFile) validateKind() error {
	if f.Kind != SourceFileKindTest && !sourceFileTestDeclarationsEmpty(f.Declarations) {
		return conflictError(errors.New("standard non-test file carries test declarations"))
	}
	if f.Kind == SourceFileKindDocumentation && f.Language != SourceLanguageMarkdown {
		return conflictError(errors.New("standard documentation file is not Markdown"))
	}
	if (f.Kind == SourceFileKindDocumentation || f.Kind == SourceFileKindAsset) && f.Effects.Posture != PrimitiveEffectNotApplicable {
		return conflictError(errors.New("standard inert file carries an executable effect posture"))
	}
	return nil
}

func sourceFileTestDeclarationsEmpty(declarations SourceFileDeclarations) bool {
	return declarations.TestDeclarations == 0 && declarations.Benchmarks == 0 && declarations.FuzzTargets == 0
}

func (f SourceFile) validateCapabilityScope() error {
	scope := capabilities.ScopeProduction
	if f.Kind == SourceFileKindTest {
		scope = capabilities.ScopeTest
	}
	for _, use := range f.Effects.Capabilities {
		if _, err := capabilities.Resolve(capabilities.ForPackage(scope, use.Package)); err != nil {
			return contractError(err)
		}
	}
	return nil
}

// PackageFileCatalog is an optional exact source scan for one package. Nil on
// Code means not observed; an empty present catalog is never folded into zero.
type PackageFileCatalog struct {
	Architecture *PackageArchitectureFacts `json:"architecture,omitempty"`
	Package      SourcePath                `json:"package"`
	Files        []SourceFile              `json:"files"`
}

func (c PackageFileCatalog) Validate() error {
	if err := c.Package.Validate(); err != nil {
		return err
	}
	if len(c.Files) == 0 || len(c.Files) > PackageSourceFileMaximum {
		return contractError(errors.New("standard package file catalog count is invalid"))
	}
	if err := c.validateFiles(); err != nil {
		return err
	}
	return c.validateArchitecture()
}

func (c PackageFileCatalog) validateFiles() error {
	for index := range c.Files {
		if err := c.Files[index].Validate(); err != nil {
			return err
		}
		if c.Files[index].Package != c.Package {
			return conflictError(errors.New("standard source file package differs from its catalog"))
		}
		if index > 0 && c.Files[index-1].Path.String() >= c.Files[index].Path.String() {
			return conflictError(errors.New("standard source files are duplicated or not in canonical order"))
		}
	}
	return nil
}

func (c PackageFileCatalog) validateArchitecture() error {
	if c.Architecture == nil {
		return nil
	}
	if err := c.Architecture.Validate(); err != nil {
		return err
	}
	if derived := derivePackageArchitecture(c.Files); !architectureFactsEqual(*c.Architecture, derived) {
		return conflictError(errors.New("standard package architecture facts contradict source observations"))
	}
	return nil
}

// ValidateComplete proves that every Go file carries an import observation.
// Validate permits a structurally valid partial observation; generators and
// verifiers claiming complete source inspection use this stricter door.
func (c PackageFileCatalog) ValidateComplete() error {
	if err := c.Validate(); err != nil {
		return err
	}
	for index := range c.Files {
		file := c.Files[index]
		if file.Language == SourceLanguageGo && file.Imports == nil {
			return conflictError(errors.New("standard complete Go file catalog omits import observations"))
		}
	}
	if c.Architecture == nil {
		return conflictError(errors.New("standard complete catalog omits package architecture facts"))
	}
	return nil
}

// ValidateOwnership proves the per-site classification against authored
// package capability ownership. Mixed files remain valid: only implementation
// and direct sites participate in this invariant.
func (c PackageFileCatalog) ValidateOwnership(module gomodule.Path, ownership []CapabilityOwnership) error {
	if err := c.ValidateComplete(); err != nil {
		return err
	}
	if _, err := ResolvePackageRelationship(module, c.Package, ownership); err != nil {
		return err
	}
	for _, file := range c.Files {
		if err := validateImplementationOwnership(file.Effects.Implementation, ownership); err != nil {
			return err
		}
		if file.Kind == SourceFileKindProduction {
			if err := validateDirectOwnership(file.Effects.Direct, ownership); err != nil {
				return err
			}
		}
	}
	return nil
}

// ValidateArchitecture closes complete source observations against the
// package owner's hand-authored role and capability ownership. Source
// discovery may remain partial and unreviewed; any caller claiming an
// architecture result uses this stricter acceptance boundary.
func (c PackageFileCatalog) ValidateArchitecture(module gomodule.Path, knowledge PackageKnowledge) error {
	if err := knowledge.Validate(); err != nil {
		return err
	}
	if c.Package != knowledge.Path {
		return conflictError(errors.New("standard source catalog package differs from authored knowledge"))
	}
	if err := c.ValidateOwnership(module, knowledge.AuthorCapabilityOwnership); err != nil {
		return err
	}
	if c.Architecture == nil {
		return conflictError(errors.New("standard architecture acceptance lacks generated facts"))
	}
	return ValidatePackageRoleArchitecture(knowledge.AuthorRole, *c.Architecture)
}

func validateImplementationOwnership(sites []SourceEffectSite, ownership []CapabilityOwnership) error {
	for _, site := range sites {
		owned, err := capabilityOwnerIsDeclared(site, ownership)
		if err != nil {
			return err
		}
		if !owned {
			return conflictError(errors.New("standard implementation site lacks package capability ownership"))
		}
	}
	return nil
}

func validateDirectOwnership(sites []SourceEffectSite, ownership []CapabilityOwnership) error {
	if len(sites) != 0 {
		return conflictError(errors.New("standard production source bypasses a Primitive effect owner"))
	}
	return nil
}

func capabilityOwnerIsDeclared(site SourceEffectSite, ownership []CapabilityOwnership) (bool, error) {
	if site.Capability == nil {
		return false, conflictError(errors.New("standard classified effect site lacks capability owner"))
	}
	for _, declared := range ownership {
		effect, err := declared.Capability.Effect()
		if err != nil {
			return false, err
		}
		match, err := capabilities.Resolve(capabilities.ForEffect(capabilities.ScopeProduction, effect))
		if err != nil {
			return false, err
		}
		if match.Capability.Package == site.Capability.Package {
			return true, nil
		}
	}
	return false, nil
}
