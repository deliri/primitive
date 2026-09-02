package projectstandards

import (
	"errors"
	"go/token"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

const (
	SourceDeclarationMaximum      = 2_048
	PackageContractMaximum        = 4_096
	PackageAuthenticationMaximum  = 1_024
	PackageWireDocumentMaximum    = 2_048
	PackageDependencyMaximum      = 4_096
	PackageCapabilityTraitMaximum = core.PrimitivePackageCount
	validateMethodName            = "Validate"
	marshalJSONMethodName         = "MarshalJSON"
	unmarshalJSONMethodName       = "UnmarshalJSON"
)

// SourceDeclarationKind is the compiler-visible shape of one Go declaration.
type SourceDeclarationKind uint8

const (
	SourceDeclarationKindUnknown SourceDeclarationKind = iota
	SourceDeclarationKindConstant
	SourceDeclarationKindVariable
	SourceDeclarationKindType
	SourceDeclarationKindFunction
	SourceDeclarationKindMethod
	sourceDeclarationKindLimit
)

func sourceDeclarationKindLabels() []string {
	return []string{"", "constant", "variable", "type", "function", "method"}
}

func (k SourceDeclarationKind) Validate() error {
	return validateEnum(uint8(k), sourceDeclarationKindLabels(), "project standards source declaration kind is invalid")
}

func (k SourceDeclarationKind) IsValid() bool { return k.Validate() == nil }
func (k SourceDeclarationKind) String() string {
	return enumString(uint8(k), sourceDeclarationKindLabels())
}
func (k SourceDeclarationKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(k), sourceDeclarationKindLabels(), "project standards source declaration kind is invalid")
}
func (k *SourceDeclarationKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return jsonError(errors.New("nil project standards source declaration kind receiver"))
	}
	value, err := unmarshalEnum(data, sourceDeclarationKindLabels(), "project standards source declaration kind is invalid")
	if err == nil {
		*k = SourceDeclarationKind(value)
	}
	return err
}

// SourceDeclaration is one exact declaration observed from a Go syntax tree.
// Receiver is populated only for methods. AttestBound records a structural
// dependency on Primitive attest material inside a declared type.
type SourceDeclaration struct {
	Name        Identifier            `json:"name"`
	Receiver    *Identifier           `json:"receiver,omitempty"`
	Kind        SourceDeclarationKind `json:"kind"`
	Line        uint32                `json:"line"`
	Column      uint32                `json:"column"`
	Exported    bool                  `json:"exported"`
	AttestBound bool                  `json:"attest_bound"`
}

func (d SourceDeclaration) Validate() error {
	if err := contractJoin(d.Name.Validate(), d.Kind.Validate()); err != nil {
		return err
	}
	if !validGoName(d.Name.String()) || d.Line == 0 || d.Column == 0 {
		return contractError(errors.New("project standards source declaration identity is invalid"))
	}
	hasReceiver := d.Receiver != nil
	if hasReceiver {
		if err := d.Receiver.Validate(); err != nil || !validGoName(d.Receiver.String()) {
			return contractError(errors.New("project standards source declaration receiver is invalid"))
		}
	}
	if hasReceiver != (d.Kind == SourceDeclarationKindMethod) {
		return conflictError(errors.New("project standards declaration receiver contradicts its kind"))
	}
	if d.AttestBound && d.Kind != SourceDeclarationKindType {
		return conflictError(errors.New("project standards non-type declaration claims an attestation binding"))
	}
	runeValue, _ := utf8.DecodeRuneInString(d.Name.String())
	if d.Exported != unicode.IsUpper(runeValue) {
		return conflictError(errors.New("project standards declaration export fact contradicts its name"))
	}
	return nil
}

func validGoName(value string) bool {
	return token.IsIdentifier(value) && !token.Lookup(value).IsKeyword() && value != "_"
}

// DeclarationReference binds one package-level trait to the exact declaration
// and source coordinate that produced it.
type DeclarationReference struct {
	Path     SourcePath  `json:"path"`
	Name     Identifier  `json:"name"`
	Receiver *Identifier `json:"receiver,omitempty"`
	Line     uint32      `json:"line"`
	Column   uint32      `json:"column"`
}

func (r DeclarationReference) Validate() error {
	if err := contractJoin(r.Path.Validate(), r.Name.Validate()); err != nil {
		return err
	}
	if r.Line == 0 || r.Column == 0 || !validGoName(r.Name.String()) {
		return contractError(errors.New("project standards declaration reference is invalid"))
	}
	if r.Receiver != nil {
		if err := r.Receiver.Validate(); err != nil || !validGoName(r.Receiver.String()) {
			return contractError(errors.New("project standards declaration reference receiver is invalid"))
		}
	}
	return nil
}

// PackageDependency is one unique import edge observed in package source.
type PackageDependency struct {
	Path          SourcePath       `json:"path"`
	ProjectModule *SourcePath      `json:"project_module,omitempty"`
	Kind          SourceImportKind `json:"kind"`
}

func (d PackageDependency) Validate() error {
	importFact := SourceImport{Path: d.Path, Kind: d.Kind}
	if d.ProjectModule != nil {
		module := *d.ProjectModule
		importFact.ProjectModule = &module
	}
	return importFact.Validate()
}

// PackageArchitectureFacts are source-derived traits. They describe compiled
// structure and never substitute for the package owner's authored role.
type PackageArchitectureFacts struct {
	ContractsOwned          []DeclarationReference   `json:"contracts_owned"`
	AuthenticationBindings  []DeclarationReference   `json:"authentication_bindings"`
	CapabilitiesConsumed    []PrimitiveCapabilityUse `json:"capabilities_consumed"`
	CapabilitiesImplemented []PrimitiveCapabilityUse `json:"capabilities_implemented"`
	WireDocuments           []DeclarationReference   `json:"wire_documents"`
	Dependencies            []PackageDependency      `json:"dependencies"`
}

func (f PackageArchitectureFacts) Validate() error {
	if len(f.ContractsOwned) > PackageContractMaximum || len(f.AuthenticationBindings) > PackageAuthenticationMaximum ||
		len(f.CapabilitiesConsumed) > PackageCapabilityTraitMaximum || len(f.CapabilitiesImplemented) > PackageCapabilityTraitMaximum ||
		len(f.WireDocuments) > PackageWireDocumentMaximum || len(f.Dependencies) > PackageDependencyMaximum {
		return contractError(errors.New("project standards package architecture fact count exceeds its bound"))
	}
	if err := validateDeclarationReferences(f.ContractsOwned); err != nil {
		return err
	}
	if err := validateDeclarationReferences(f.AuthenticationBindings); err != nil {
		return err
	}
	if err := validateCapabilityUses(f.CapabilitiesConsumed); err != nil {
		return err
	}
	if err := validateCapabilityUses(f.CapabilitiesImplemented); err != nil {
		return err
	}
	if err := validateDeclarationReferences(f.WireDocuments); err != nil {
		return err
	}
	return validateDependencies(f.Dependencies)
}

func validateDeclarationReferences(values []DeclarationReference) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if index > 0 && declarationReferenceKey(values[index-1]) >= declarationReferenceKey(values[index]) {
			return conflictError(errors.New("project standards declaration references are duplicated or not canonical"))
		}
	}
	return nil
}

func validateCapabilityUses(values []PrimitiveCapabilityUse) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if index > 0 && values[index-1].Package >= values[index].Package {
			return conflictError(errors.New("project standards capability traits are duplicated or not canonical"))
		}
	}
	return nil
}

func validateDependencies(values []PackageDependency) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if index > 0 && values[index-1].Path.String() >= values[index].Path.String() {
			return conflictError(errors.New("project standards package dependencies are duplicated or not canonical"))
		}
	}
	return nil
}

func declarationReferenceKey(value DeclarationReference) string {
	receiver := ""
	if value.Receiver != nil {
		receiver = value.Receiver.String()
	}
	return value.Path.String() + "\x00" + value.Name.String() + "\x00" + receiver
}

func derivePackageArchitecture(files []SourceFile) PackageArchitectureFacts {
	facts := PackageArchitectureFacts{}
	for _, file := range files {
		facts.observeDeclarations(file)
		facts.observeEffects(file.Effects)
		facts.observeImports(file.Imports)
	}
	facts.observeWireDocuments(files)
	facts.sort()
	return facts
}

func (f *PackageArchitectureFacts) observeDeclarations(file SourceFile) {
	for _, declaration := range file.Declarations.Symbols {
		reference := declarationReference(file.Path, declaration)
		if declaration.Exported && (declaration.Kind == SourceDeclarationKindType || declaration.Kind == SourceDeclarationKindConstant) {
			f.ContractsOwned = append(f.ContractsOwned, reference)
		}
		if declaration.AttestBound {
			f.AuthenticationBindings = append(f.AuthenticationBindings, reference)
		}
	}
}

func declarationReference(path SourcePath, declaration SourceDeclaration) DeclarationReference {
	return DeclarationReference{Path: path, Name: declaration.Name, Receiver: copyIdentifier(declaration.Receiver), Line: declaration.Line, Column: declaration.Column}
}

func copyIdentifier(source *Identifier) *Identifier {
	if source == nil {
		return nil
	}
	value := *source
	return &value
}

func (f *PackageArchitectureFacts) observeWireDocuments(files []SourceFile) {
	for _, file := range files {
		for _, declaration := range file.Declarations.Symbols {
			if declaration.Kind == SourceDeclarationKindType && hasWireMethodSet(files, declaration.Name) {
				f.WireDocuments = append(f.WireDocuments, declarationReference(file.Path, declaration))
			}
		}
	}
}

func hasWireMethodSet(files []SourceFile, receiver Identifier) bool {
	validate, marshal, unmarshal := false, false, false
	for _, file := range files {
		for _, declaration := range file.Declarations.Symbols {
			if declaration.Kind != SourceDeclarationKindMethod || declaration.Receiver == nil || *declaration.Receiver != receiver {
				continue
			}
			switch declaration.Name.String() {
			case validateMethodName:
				validate = true
			case marshalJSONMethodName:
				marshal = true
			case unmarshalJSONMethodName:
				unmarshal = true
			}
		}
	}
	return validate && marshal && unmarshal
}

func (f *PackageArchitectureFacts) observeEffects(effects SourceFileEffects) {
	for _, site := range effects.Mediated {
		f.appendConsumed(site)
	}
	for _, site := range effects.Implementation {
		f.appendImplemented(site)
	}
}

func (f *PackageArchitectureFacts) appendConsumed(site SourceEffectSite) {
	if site.Capability != nil && !slices.Contains(f.CapabilitiesConsumed, *site.Capability) {
		f.CapabilitiesConsumed = append(f.CapabilitiesConsumed, *site.Capability)
	}
}

func (f *PackageArchitectureFacts) appendImplemented(site SourceEffectSite) {
	if site.Capability != nil && !slices.Contains(f.CapabilitiesImplemented, *site.Capability) {
		f.CapabilitiesImplemented = append(f.CapabilitiesImplemented, *site.Capability)
	}
}

func (f *PackageArchitectureFacts) observeImports(imports *SourceFileImports) {
	if imports == nil {
		return
	}
	for _, imported := range imports.Values {
		dependency := PackageDependency{Path: imported.Path, Kind: imported.Kind}
		if imported.ProjectModule != nil {
			dependency.ProjectModule = copySourcePath(imported.ProjectModule)
		}
		if !slices.ContainsFunc(f.Dependencies, func(current PackageDependency) bool { return packageDependencyEqual(current, dependency) }) {
			f.Dependencies = append(f.Dependencies, dependency)
		}
	}
}

func copySourcePath(source *SourcePath) *SourcePath {
	if source == nil {
		return nil
	}
	value := *source
	return &value
}

func (f *PackageArchitectureFacts) sort() {
	slices.SortFunc(f.ContractsOwned, compareDeclarationReference)
	slices.SortFunc(f.AuthenticationBindings, compareDeclarationReference)
	slices.SortFunc(f.CapabilitiesConsumed, compareCapabilityUse)
	slices.SortFunc(f.CapabilitiesImplemented, compareCapabilityUse)
	slices.SortFunc(f.WireDocuments, compareDeclarationReference)
	slices.SortFunc(f.Dependencies, func(left, right PackageDependency) int {
		return strings.Compare(left.Path.String(), right.Path.String())
	})
}

func compareDeclarationReference(left, right DeclarationReference) int {
	return strings.Compare(declarationReferenceKey(left), declarationReferenceKey(right))
}

func compareCapabilityUse(left, right PrimitiveCapabilityUse) int {
	if left.Package < right.Package {
		return -1
	}
	if left.Package > right.Package {
		return 1
	}
	return 0
}

func architectureFactsEqual(left, right PackageArchitectureFacts) bool {
	return slices.EqualFunc(left.ContractsOwned, right.ContractsOwned, declarationReferenceEqual) &&
		slices.EqualFunc(left.AuthenticationBindings, right.AuthenticationBindings, declarationReferenceEqual) &&
		slices.Equal(left.CapabilitiesConsumed, right.CapabilitiesConsumed) &&
		slices.Equal(left.CapabilitiesImplemented, right.CapabilitiesImplemented) &&
		slices.EqualFunc(left.WireDocuments, right.WireDocuments, declarationReferenceEqual) &&
		slices.EqualFunc(left.Dependencies, right.Dependencies, packageDependencyEqual)
}

func declarationReferenceEqual(left, right DeclarationReference) bool {
	return declarationReferenceKey(left) == declarationReferenceKey(right) && left.Line == right.Line && left.Column == right.Column
}

func packageDependencyEqual(left, right PackageDependency) bool {
	if left.Path != right.Path || left.Kind != right.Kind || (left.ProjectModule == nil) != (right.ProjectModule == nil) {
		return false
	}
	return left.ProjectModule == nil || *left.ProjectModule == *right.ProjectModule
}

// DerivePackageArchitecture returns the canonical package traits proven by an
// already-bounded source-file window. Forge calls this after inspecting one
// package; callers cannot author a competing summary.
func DerivePackageArchitecture(files []SourceFile) (PackageArchitectureFacts, error) {
	for index := range files {
		if err := files[index].Validate(); err != nil {
			return PackageArchitectureFacts{}, err
		}
	}
	facts := derivePackageArchitecture(files)
	if err := facts.Validate(); err != nil {
		return PackageArchitectureFacts{}, err
	}
	return facts, nil
}
