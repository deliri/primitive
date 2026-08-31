package projectstandards

import (
	"errors"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

const ProbeKindMaximum = 16

type ProbeRole uint8

const (
	ProbeRoleUnknown ProbeRole = iota
	ProbeRoleSelection
	ProbeRoleExperiment
	probeRoleLimit
)

func probeRoleLabels() []string { return []string{"", "selection", "experiment"} }
func (r ProbeRole) Validate() error {
	return validateEnum(uint8(r), probeRoleLabels(), "project standards probe role is invalid")
}
func (r ProbeRole) IsValid() bool  { return r.Validate() == nil }
func (r ProbeRole) String() string { return enumString(uint8(r), probeRoleLabels()) }
func (r ProbeRole) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(r), probeRoleLabels(), "project standards probe role is invalid")
}
func (r *ProbeRole) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("nil project standards probe role receiver"))
	}
	value, err := unmarshalEnum(data, probeRoleLabels(), "project standards probe role is invalid")
	if err == nil {
		*r = ProbeRole(value)
	}
	return err
}

type ProbeKind uint8

const (
	ProbeKindUnknown ProbeKind = iota
	ProbeKindGoFileSelection
	ProbeKindGoPackageSelection
	ProbeKindCISelection
	ProbeKindGoTest
	ProbeKindGoRace
	ProbeKindGoBenchmark
	ProbeKindGoFuzz
	ProbeKindGoDiagnosticProfile
	ProbeKindJavaScriptTest
	ProbeKindSmoke
	ProbeKindTool
	probeKindLimit
)

func probeKindLabels() []string {
	return []string{"", "go_file_selection", "go_package_selection", "ci_selection", "go_test", "go_race", "go_benchmark", "go_fuzz", "go_diagnostic_profile", "javascript_test", "smoke", "tool"}
}
func (k ProbeKind) Validate() error {
	return validateEnum(uint8(k), probeKindLabels(), "project standards probe kind is invalid")
}
func (k ProbeKind) IsValid() bool  { return k.Validate() == nil }
func (k ProbeKind) String() string { return enumString(uint8(k), probeKindLabels()) }
func (k ProbeKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(k), probeKindLabels(), "project standards probe kind is invalid")
}
func (k *ProbeKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return jsonError(errors.New("nil project standards probe kind receiver"))
	}
	value, err := unmarshalEnum(data, probeKindLabels(), "project standards probe kind is invalid")
	if err == nil {
		*k = ProbeKind(value)
	}
	return err
}
func (k ProbeKind) Role() (ProbeRole, error) {
	if err := k.Validate(); err != nil {
		return ProbeRoleUnknown, err
	}
	if k <= ProbeKindCISelection {
		return ProbeRoleSelection, nil
	}
	return ProbeRoleExperiment, nil
}

type ProbeTargetKind uint8

const (
	ProbeTargetUnknown ProbeTargetKind = iota
	ProbeTargetGoDeclaration
	ProbeTargetGoFile
	ProbeTargetGoPackage
	ProbeTargetJavaScriptFile
	ProbeTargetSmokeSuite
	ProbeTargetTool
	ProbeTargetCIPlan
	probeTargetLimit
)

func probeTargetLabels() []string {
	return []string{"", "go_declaration", "go_file", "go_package", "javascript_file", "smoke_suite", "tool", "ci_plan"}
}
func (k ProbeTargetKind) Validate() error {
	return validateEnum(uint8(k), probeTargetLabels(), "project standards probe target kind is invalid")
}
func (k ProbeTargetKind) IsValid() bool  { return k.Validate() == nil }
func (k ProbeTargetKind) String() string { return enumString(uint8(k), probeTargetLabels()) }
func (k ProbeTargetKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(k), probeTargetLabels(), "project standards probe target kind is invalid")
}
func (k *ProbeTargetKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return jsonError(errors.New("nil project standards probe target receiver"))
	}
	value, err := unmarshalEnum(data, probeTargetLabels(), "project standards probe target kind is invalid")
	if err == nil {
		*k = ProbeTargetKind(value)
	}
	return err
}

type GoDeclarationTarget struct {
	Module  Identifier `json:"module"`
	Package SourcePath `json:"package"`
	File    SourcePath `json:"file"`
	Symbol  Name       `json:"symbol"`
}

func (t GoDeclarationTarget) Validate() error {
	return contractJoin(t.Module.Validate(), t.Package.Validate(), t.File.Validate(), t.Symbol.Validate())
}

type GoFileTarget struct {
	Module     Identifier  `json:"module"`
	Package    SourcePath  `json:"package"`
	File       SourcePath  `json:"file"`
	ChildKinds []ProbeKind `json:"child_kinds"`
}

func (t GoFileTarget) Validate() error {
	if err := contractJoin(t.Module.Validate(), t.Package.Validate(), t.File.Validate()); err != nil {
		return err
	}
	if !strings.HasSuffix(t.File.String(), "_test.go") {
		return contractError(errors.New("project standards Go file selection requires a _test.go source file"))
	}
	return validateGoSelectionChildKinds(t.ChildKinds, "project standards Go file child kinds are invalid")
}

type GoPackageTarget struct {
	Module     Identifier  `json:"module"`
	Package    SourcePath  `json:"package"`
	ChildKinds []ProbeKind `json:"child_kinds"`
}

func (t GoPackageTarget) Validate() error {
	if err := contractJoin(t.Module.Validate(), t.Package.Validate()); err != nil {
		return err
	}
	return validateGoSelectionChildKinds(t.ChildKinds, "project standards Go package child kinds are invalid")
}

type JavaScriptFileTarget struct {
	Workspace Identifier `json:"workspace"`
	File      SourcePath `json:"file"`
}

func (t JavaScriptFileTarget) Validate() error {
	return contractJoin(t.Workspace.Validate(), t.File.Validate())
}

type NamedTarget struct {
	Identity Identifier `json:"identity"`
}

func (t NamedTarget) Validate() error { return t.Identity.Validate() }

type ToolTarget struct {
	Identity Identifier  `json:"identity"`
	Module   *Identifier `json:"module,omitempty"`
}

func (t ToolTarget) Validate() error {
	if err := t.Identity.Validate(); err != nil {
		return err
	}
	if t.Module != nil {
		return t.Module.Validate()
	}
	return nil
}

type ProbeTarget struct {
	Kind          ProbeTargetKind       `json:"kind"`
	GoDeclaration *GoDeclarationTarget  `json:"go_declaration,omitempty"`
	GoFile        *GoFileTarget         `json:"go_file,omitempty"`
	GoPackage     *GoPackageTarget      `json:"go_package,omitempty"`
	JavaScript    *JavaScriptFileTarget `json:"javascript_file,omitempty"`
	Smoke         *NamedTarget          `json:"smoke_suite,omitempty"`
	Tool          *ToolTarget           `json:"tool,omitempty"`
	CI            *NamedTarget          `json:"ci_plan,omitempty"`
}

func (t ProbeTarget) Validate() error {
	if err := t.Kind.Validate(); err != nil {
		return err
	}
	if t.payloadCount() != 1 {
		return contractError(errors.New("project standards probe target must carry exactly one variant"))
	}
	return t.validateVariant()
}

func (t ProbeTarget) validateVariant() error {
	switch t.Kind {
	case ProbeTargetGoDeclaration:
		return validateTargetVariant(t.GoDeclaration)
	case ProbeTargetGoFile:
		return validateTargetVariant(t.GoFile)
	case ProbeTargetGoPackage:
		return validateTargetVariant(t.GoPackage)
	case ProbeTargetJavaScriptFile:
		return validateTargetVariant(t.JavaScript)
	case ProbeTargetSmokeSuite:
		return validateTargetVariant(t.Smoke)
	case ProbeTargetTool:
		return validateTargetVariant(t.Tool)
	case ProbeTargetCIPlan:
		return validateTargetVariant(t.CI)
	case ProbeTargetUnknown:
		return contractError(errors.New("project standards probe target escaped its domain"))
	default:
		return contractError(errors.New("project standards probe target escaped its domain"))
	}
}

func validateTargetVariant[T interface{ Validate() error }](value *T) error {
	if value == nil {
		return conflictError(errors.New("project standards probe target variant differs from kind"))
	}
	return (*value).Validate()
}

func (t ProbeTarget) payloadCount() int {
	count := 0
	for _, present := range [...]bool{t.GoDeclaration != nil, t.GoFile != nil, t.GoPackage != nil, t.JavaScript != nil, t.Smoke != nil, t.Tool != nil, t.CI != nil} {
		if present {
			count++
		}
	}
	return count
}

type EnvironmentRequirement struct {
	MachineClass Identifier        `json:"machine_class"`
	Fingerprint  core.SHA256Digest `json:"requirement_fingerprint"`
}

// AdmittedEnvironment is the exact environment control selected after the
// request's constraints are authorized against an observed machine sheet.
// RequirementFingerprint commits to the constraint that was satisfied;
// MachineFingerprint identifies the actual admitted machine generation.
type AdmittedEnvironment struct {
	MachineClass           Identifier          `json:"machine_class"`
	RequirementFingerprint core.SHA256Digest   `json:"requirement_fingerprint"`
	EnvironmentFingerprint core.SHA256Digest   `json:"environment_fingerprint"`
	MachineGeneration      MachineGenerationID `json:"machine_generation_id"`
	MachineSheetDigest     core.SHA256Digest   `json:"machine_sheet_digest"`
}

func (e AdmittedEnvironment) Validate() error {
	return contractJoin(e.MachineClass.Validate(), e.RequirementFingerprint.Validate(), e.EnvironmentFingerprint.Validate(), e.MachineGeneration.Validate(), e.MachineSheetDigest.Validate())
}

// Satisfies reports whether this exact admitted environment was selected
// under requirement. It deliberately does not equate a constraint digest
// with the independently observed machine fingerprint.
func (e AdmittedEnvironment) Satisfies(requirement EnvironmentRequirement) bool {
	return e.Validate() == nil && requirement.Validate() == nil &&
		e.MachineClass == requirement.MachineClass && e.RequirementFingerprint == requirement.Fingerprint
}

// SelectionParent binds a child experiment to the exact selection expansion
// that admitted it. It prevents an experiment from appearing beneath a file
// or CI request that never discovered that child.
type SelectionParent struct {
	Request         RequestIdentity   `json:"request_id"`
	Kind            ProbeKind         `json:"kind"`
	Target          ProbeTarget       `json:"target"`
	ExpansionDigest core.SHA256Digest `json:"expansion_digest"`
}

func (p SelectionParent) Validate() error {
	if err := contractJoin(p.Request.Validate(), p.Kind.Validate(), p.Target.Validate(), p.ExpansionDigest.Validate()); err != nil {
		return err
	}
	role, err := p.Kind.Role()
	if err != nil {
		return err
	}
	if role != ProbeRoleSelection || !targetAdmitsKind(p.Target.Kind, p.Kind) {
		return conflictError(errors.New("project standards selection parent kind and target disagree"))
	}
	return nil
}

func (r EnvironmentRequirement) Validate() error {
	return contractJoin(r.MachineClass.Validate(), r.Fingerprint.Validate())
}

type RequestedProbe struct {
	Origin      OriginIdentity         `json:"origin"`
	Subject     SubjectIdentity        `json:"subject"`
	Source      SourceCoordinate       `json:"source"`
	Target      ProbeTarget            `json:"target"`
	Kinds       []ProbeKind            `json:"requested_kinds"`
	Profile     ProfileIdentity        `json:"profile"`
	Constraints EnvironmentRequirement `json:"requested_environment"`
}

func (r RequestedProbe) Validate() error {
	if err := contractJoin(r.Origin.Validate(), r.Subject.Validate(), r.Source.Validate(), r.Target.Validate(), r.Profile.Validate(), r.Constraints.Validate()); err != nil {
		return err
	}
	if r.Subject.Repository.value != r.Source.Repository.value {
		return conflictError(errors.New("project standards requested probe source repository differs from subject"))
	}
	return validateRequestedKinds(r.Target, r.Kinds)
}

func validateRequestedKinds(target ProbeTarget, kinds []ProbeKind) error {
	if len(kinds) == 0 || len(kinds) > ProbeKindMaximum {
		return contractError(errors.New("project standards requested probe kinds are invalid"))
	}
	for index := range kinds {
		if err := validateRequestedKindAt(target, kinds, index); err != nil {
			return err
		}
	}
	if !requestedKindsMatchSelectionTarget(target, kinds) {
		return conflictError(errors.New("project standards requested probe kinds differ from selection child kinds"))
	}
	return nil
}

func validateRequestedKindAt(target ProbeTarget, kinds []ProbeKind, index int) error {
	if err := kinds[index].Validate(); err != nil {
		return err
	}
	if duplicateProbeKind(kinds, index) {
		return conflictError(errors.New("project standards requested probe kind is duplicated"))
	}
	if index > 0 && kinds[index-1] >= kinds[index] {
		return conflictError(errors.New("project standards requested probe kinds are not canonical"))
	}
	if !requestedTargetAdmitsKind(target.Kind, kinds[index]) {
		return conflictError(errors.New("project standards requested probe kind differs from target"))
	}
	return nil
}

type ProbeIdentity struct {
	Origin      OriginIdentity      `json:"origin"`
	Subject     SubjectIdentity     `json:"subject"`
	Source      SourceCoordinate    `json:"source"`
	Role        ProbeRole           `json:"role"`
	Kind        ProbeKind           `json:"kind"`
	Target      ProbeTarget         `json:"target"`
	Profile     ProfileIdentity     `json:"profile"`
	Environment AdmittedEnvironment `json:"admitted_environment"`
	Parent      *SelectionParent    `json:"selection_parent,omitempty"`
}

// AdmitRequestedProbe compiles one independently selected environment into
// the exact probe identity permitted by the requested target. It is the only
// constructor that maps request-kind sets to their selection or experiment
// identity, so control services cannot duplicate that closed switch.
func AdmitRequestedProbe(request RequestedProbe, environment AdmittedEnvironment) (ProbeIdentity, error) {
	if err := contractJoin(request.Validate(), environment.Validate()); err != nil {
		return ProbeIdentity{}, err
	}
	if !environment.Satisfies(request.Constraints) {
		return ProbeIdentity{}, conflictError(errors.New("project standards admitted environment does not satisfy the requested requirement"))
	}
	kind, err := admittedKindForRequest(request)
	if err != nil {
		return ProbeIdentity{}, err
	}
	role, err := kind.Role()
	if err != nil {
		return ProbeIdentity{}, err
	}
	probe := ProbeIdentity{
		Origin: request.Origin, Subject: request.Subject, Source: request.Source,
		Role: role, Kind: kind, Target: request.Target, Profile: request.Profile,
		Environment: environment,
	}
	return probe, probe.Validate()
}

func (i ProbeIdentity) Validate() error {
	if err := i.validateFields(); err != nil {
		return err
	}
	if i.Subject.Repository != i.Source.Repository {
		return conflictError(errors.New("project standards probe identity source repository differs from subject"))
	}
	return i.validateRelationship()
}

func (i ProbeIdentity) validateFields() error {
	return contractJoin(i.Origin.Validate(), i.Subject.Validate(), i.Source.Validate(), i.Role.Validate(), i.Kind.Validate(), i.Target.Validate(), i.Profile.Validate(), i.Environment.Validate())
}

func (i ProbeIdentity) validateRelationship() error {
	role, err := i.Kind.Role()
	if err != nil {
		return err
	}
	if role != i.Role || !targetAdmitsKind(i.Target.Kind, i.Kind) {
		return conflictError(errors.New("project standards probe role, kind, and target disagree"))
	}
	if i.Role == ProbeRoleSelection && i.Parent != nil {
		return conflictError(errors.New("project standards selection probe cannot descend from another selection"))
	}
	if i.Parent != nil {
		return i.validateParent()
	}
	return nil
}

func (i ProbeIdentity) validateParent() error {
	if err := i.Parent.Validate(); err != nil {
		return err
	}
	if !experimentDescendsFromSelection(i, *i.Parent) {
		return conflictError(errors.New("project standards experiment does not descend from its selection parent"))
	}
	return nil
}

func experimentDescendsFromSelection(child ProbeIdentity, parent SelectionParent) bool {
	if child.Role != ProbeRoleExperiment {
		return false
	}
	if parent.Target.Kind == ProbeTargetGoFile {
		return goDeclarationDescendsFromFile(child, parent.Target.GoFile)
	}
	if parent.Target.Kind == ProbeTargetGoPackage {
		return goExperimentDescendsFromPackage(child, parent.Target.GoPackage)
	}
	if parent.Target.Kind == ProbeTargetCIPlan {
		return child.Kind > ProbeKindCISelection && child.Kind < probeKindLimit
	}
	return false
}

func goExperimentDescendsFromPackage(child ProbeIdentity, parent *GoPackageTarget) bool {
	if parent == nil || !probeKindPresent(parent.ChildKinds, child.Kind) {
		return false
	}
	if child.Target.Kind == ProbeTargetGoDeclaration && child.Target.GoDeclaration != nil {
		target := child.Target.GoDeclaration
		return target.Module == parent.Module && target.Package == parent.Package
	}
	if child.Target.Kind == ProbeTargetGoPackage && child.Target.GoPackage != nil {
		target := child.Target.GoPackage
		return target.Module == parent.Module && target.Package == parent.Package
	}
	return false
}

func goDeclarationDescendsFromFile(child ProbeIdentity, parent *GoFileTarget) bool {
	if parent == nil || child.Target.Kind != ProbeTargetGoDeclaration || child.Target.GoDeclaration == nil {
		return false
	}
	target := child.Target.GoDeclaration
	return target.Module == parent.Module && target.Package == parent.Package && target.File == parent.File && probeKindPresent(parent.ChildKinds, child.Kind)
}

func probeKindPresent(values []ProbeKind, want ProbeKind) bool {
	for index := range values {
		if values[index] == want {
			return true
		}
	}
	return false
}

func contractJoin(values ...error) error {
	if err := errors.Join(values...); err != nil {
		return contractError(err)
	}
	return nil
}

func duplicateProbeKind(values []ProbeKind, index int) bool {
	for previous := 0; previous < index; previous++ {
		if values[previous] == values[index] {
			return true
		}
	}
	return false
}

func goDeclarationExperiment(kind ProbeKind) bool {
	return kind >= ProbeKindGoTest && kind <= ProbeKindGoDiagnosticProfile
}

func validateGoSelectionChildKinds(values []ProbeKind, diagnostic string) error {
	if len(values) == 0 || len(values) > ProbeKindMaximum {
		return contractError(errors.New(diagnostic))
	}
	for index := range values {
		if !goDeclarationExperiment(values[index]) {
			return conflictError(errors.New("project standards Go selection child kind is not a Go experiment"))
		}
		if duplicateProbeKind(values, index) || (index > 0 && values[index-1] >= values[index]) {
			return conflictError(errors.New("project standards Go selection child kinds are not canonical"))
		}
	}
	return nil
}

func requestedKindsMatchSelectionTarget(target ProbeTarget, kinds []ProbeKind) bool {
	if target.Kind == ProbeTargetGoFile && target.GoFile != nil {
		return slicesEqualProbeKinds(target.GoFile.ChildKinds, kinds)
	}
	if target.Kind == ProbeTargetGoPackage && target.GoPackage != nil {
		return slicesEqualProbeKinds(target.GoPackage.ChildKinds, kinds)
	}
	return true
}

func slicesEqualProbeKinds(left, right []ProbeKind) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func targetAdmitsKind(target ProbeTargetKind, kind ProbeKind) bool {
	if target == ProbeTargetGoDeclaration {
		return goDeclarationExperiment(kind)
	}
	if target == ProbeTargetGoFile {
		return kind == ProbeKindGoFileSelection
	}
	if target == ProbeTargetGoPackage {
		return kind == ProbeKindGoPackageSelection || kind == ProbeKindGoRace || kind == ProbeKindGoDiagnosticProfile
	}
	if target == ProbeTargetJavaScriptFile {
		return kind == ProbeKindJavaScriptTest
	}
	if target == ProbeTargetSmokeSuite {
		return kind == ProbeKindSmoke
	}
	if target == ProbeTargetTool {
		return kind == ProbeKindTool
	}
	if target == ProbeTargetCIPlan {
		return kind == ProbeKindCISelection
	}
	return false
}

func requestedTargetAdmitsKind(target ProbeTargetKind, kind ProbeKind) bool {
	if target == ProbeTargetGoFile || target == ProbeTargetGoPackage {
		return goDeclarationExperiment(kind)
	}
	return targetAdmitsKind(target, kind)
}
