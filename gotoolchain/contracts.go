package gotoolchain

import (
	"errors"
	"go/token"
	"go/version"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/packages"
)

const (
	// DefaultOutputBytes is the default caller budget for cmd/go output.
	DefaultOutputBytes = 4 << 20
	// DefaultPackageMaximum is the default caller budget for package metadata.
	DefaultPackageMaximum        = 4096
	toolchainVersionMaximumBytes = 64
)

// WorkspaceMode controls whether cmd/go may consult an ambient go.work file.
type WorkspaceMode uint8

const (
	WorkspaceModeUnknown WorkspaceMode = iota
	// WorkspaceModeAmbient preserves cmd/go's admitted ambient workspace.
	WorkspaceModeAmbient
	// WorkspaceModeDisabled sets GOWORK=off in the exact child environment.
	WorkspaceModeDisabled
)

func (m WorkspaceMode) Validate() error {
	if m != WorkspaceModeAmbient && m != WorkspaceModeDisabled {
		return contractError("workspace mode is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether m belongs to the closed workspace domain.
func (m WorkspaceMode) IsValid() bool { return m.Validate() == nil }

// OffWireEnum marks WorkspaceMode as a compiler-only enum.
func (WorkspaceMode) OffWireEnum() {}

// String returns the stable execution identity of a valid workspace mode.
func (m WorkspaceMode) String() string {
	if !m.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return [...]string{
		WorkspaceModeAmbient:  "ambient",
		WorkspaceModeDisabled: "workspace_disabled",
	}[m]
}

var _ core.OffWireEnum = WorkspaceModeUnknown

// Limits bounds one cmd/go execution and its package projection.
type Limits struct {
	OutputBytes    core.ByteCount
	WaitDelay      temporal.Duration
	PackageMaximum uint64
}

// DefaultLimits returns Primitive's compiler-owned cmd/go execution budget.
func DefaultLimits() (Limits, error) {
	output, err := core.NewByteCount(DefaultOutputBytes)
	if err != nil {
		return Limits{}, err
	}
	wait, err := temporal.DurationFromSeconds(30)
	if err != nil {
		return Limits{}, err
	}
	limits := Limits{OutputBytes: output, WaitDelay: wait, PackageMaximum: DefaultPackageMaximum}
	return limits, limits.Validate()
}

func (l Limits) Validate() error {
	if err := errors.Join(l.OutputBytes.Validate(), l.WaitDelay.Validate()); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	bytes, err := l.OutputBytes.Int64()
	if err != nil || bytes == 0 || l.PackageMaximum == 0 {
		return contractError("toolchain limits are absent or not representable")
	}
	return nil
}

// Configuration selects workspace posture, execution limits
// and optional caller-owned environment. Nil Environment captures the ambient
// environment at Open; a supplied environment must be exact and is copied.
type Configuration struct {
	Environment *process.Environment
	Limits      Limits
	Workspace   WorkspaceMode
}

func (c Configuration) Validate() error {
	if err := errors.Join(c.Workspace.Validate(), c.Limits.Validate()); err != nil {
		return err
	}
	if c.Environment != nil {
		if err := c.Environment.Validate(); err != nil {
			return errors.Join(core.ErrGoToolchainContract, err)
		}
		if c.Environment.Mode != process.EnvironmentModeExact {
			return contractError("supplied compiler environment must be exact")
		}
	}
	return nil
}

// ToolchainVersion is one bounded cmd/go version token.
type ToolchainVersion struct{ value string }

// ParseToolchainVersion admits one canonical cmd/go version token.
func ParseToolchainVersion(value string) (ToolchainVersion, error) {
	version := ToolchainVersion{value: value}
	if err := version.Validate(); err != nil {
		return ToolchainVersion{}, err
	}
	return version, nil
}

func (v ToolchainVersion) Validate() error {
	if len(v.value) > toolchainVersionMaximumBytes || !strings.HasPrefix(v.value, goVersionPrefix) || !version.IsValid(v.value) {
		return contractError("toolchain version is not canonical")
	}
	for component := range strings.SplitSeq(v.value[len(goVersionPrefix):], ".") {
		if err := validateVersionComponent(component); err != nil {
			return err
		}
	}
	return nil
}

func validateVersionComponent(component string) error {
	if component == "" {
		return contractError("toolchain version contains an empty component")
	}
	for _, value := range component {
		if value < '0' || value > '9' {
			return contractError("toolchain version contains a nonnumeric component")
		}
	}
	return nil
}

func (v ToolchainVersion) String() string {
	if v.Validate() != nil {
		return ""
	}
	return v.value
}

// BuildContext is the cmd/go-selected host build context.
type BuildContext struct {
	Toolchain  ToolchainVersion
	Platform   core.Platform
	CGOEnabled bool
}

func (c BuildContext) Validate() error {
	return errors.Join(c.Platform.Validate(), c.Toolchain.Validate())
}

// PackageName is one compiler-admitted Go package clause name.
type PackageName struct{ value string }

// ParsePackageName admits one Go package clause identifier.
func ParsePackageName(value string) (PackageName, error) {
	name := PackageName{value: value}
	if err := name.Validate(); err != nil {
		return PackageName{}, err
	}
	return name, nil
}

func (n PackageName) Validate() error {
	if !token.IsIdentifier(n.value) || token.Lookup(n.value).IsKeyword() || n.value == "_" || n.value == "init" {
		return contractError("package name is not a Go identifier")
	}
	return nil
}

func (n PackageName) String() string {
	if n.Validate() != nil {
		return ""
	}
	return n.value
}

// Package is one complete package observation from cmd/go.
type Package struct {
	Module     *gomodule.Path
	ImportPath gomodule.ImportPath
	Name       PackageName
	Standard   bool
}

func (p Package) Validate() error {
	if err := errors.Join(p.ImportPath.Validate(), p.Name.Validate()); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	if p.Standard {
		if p.Module != nil {
			return contractError("standard package carries a module")
		}
		return nil
	}
	if p.Module == nil {
		return contractError("nonstandard package has no module")
	}
	return p.Module.Validate()
}

// PackageCatalog is one bounded deterministic go-list projection.
type PackageCatalog struct{ Packages []Package }

func (c PackageCatalog) Validate() error {
	if len(c.Packages) == 0 {
		return contractError("package catalog count is outside the admitted domain")
	}
	previous := ""
	for index := range c.Packages {
		if err := c.Packages[index].Validate(); err != nil {
			return err
		}
		current := c.Packages[index].ImportPath.String()
		if index > 0 && previous >= current {
			return contractError("package catalog is duplicate or noncanonical")
		}
		previous = current
	}
	return nil
}

// ObservationRequest identifies one module root for a cmd/go observation.
type ObservationRequest struct{ WorkingDirectory core.AbsolutePath }

func (r ObservationRequest) Validate() error {
	if err := r.WorkingDirectory.Validate(); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	return nil
}

// ListRequest declares one bounded package selection.
type ListRequest struct {
	WorkingDirectory core.AbsolutePath
	Pattern          string
	Dependencies     bool
}

func (r ListRequest) Validate() error {
	if err := r.WorkingDirectory.Validate(); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	if r.Pattern == "" || strings.HasPrefix(r.Pattern, "-") || strings.ContainsAny(r.Pattern, "\x00\r\n\t ") {
		return contractError("package pattern is absent or ambiguous")
	}
	return nil
}

// CompileRequest declares one package that cmd/go must compile without running tests.
type CompileRequest struct {
	WorkingDirectory core.AbsolutePath
	Pattern          string
}

func (r CompileRequest) Validate() error {
	return ListRequest{WorkingDirectory: r.WorkingDirectory, Pattern: r.Pattern}.Validate()
}

// AnalysisRequest identifies one exact package whose compiler-owned syntax,
// object, selection, and type facts are required. Package is an import path,
// not a cmd/go pattern. IncludeTests returns the package's ordinary,
// internal-test, and external-test compilation units; synthetic test-main
// wiring is not source understanding and is excluded.
type AnalysisRequest struct {
	WorkingDirectory core.AbsolutePath
	Package          gomodule.ImportPath
	IncludeTests     bool
}

func (r AnalysisRequest) Validate() error {
	if err := errors.Join(r.WorkingDirectory.Validate(), r.Package.Validate()); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	return nil
}

// PackageAnalysis is the ephemeral compiler graph for one exact package.
// Units are Go-team package objects, not a persisted protocol or product
// model. Callers project the facts they own and then release this graph.
type PackageAnalysis struct {
	WorkingDirectory core.AbsolutePath
	Package          gomodule.ImportPath
	Units            []*packages.Package
	// Metadata contains only the requested units selected by cmd/go. It is
	// available after a checker failure so callers can retain import selection
	// without launching another compiler command. It is never type evidence.
	Metadata     []*packages.Package
	IncludeTests bool
	Incomplete   bool
}

// Validate rejects unrelated units and any partial compilation presented as complete.
// Ill-typed units retain available compiler facts only with explicit diagnostics
// and an incomplete parent analysis; callers must preserve that distinction.
func (a PackageAnalysis) Validate() error {
	if err := a.ValidateMetadata(); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	if !a.Incomplete && len(a.Units) != len(a.Metadata) {
		return contractError("package analysis unit count is inconsistent with its request")
	}
	for _, metadata := range a.Metadata {
		if !a.Incomplete && len(metadata.Errors) != 0 {
			return contractError("package analysis hides compiler metadata errors")
		}
	}
	return a.validateUnits()
}

// ValidateMetadata admits compiler-selected membership, including units with
// diagnostics. It does not claim that those units parsed or type checked.
func (a PackageAnalysis) ValidateMetadata() error {
	if err := errors.Join(a.WorkingDirectory.Validate(), a.Package.Validate()); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	if len(a.Metadata) == 0 || !a.IncludeTests && len(a.Metadata) != 1 {
		return contractError("package analysis has no selected metadata")
	}
	return validateSelectedMetadata(a.Metadata, a.Package, a.IncludeTests)
}

func validateSelectedMetadata(metadata []*packages.Package, requested gomodule.ImportPath, includeTests bool) error {
	previous := ""
	foundPackage := false
	for _, unit := range metadata {
		if unit == nil || !analysisMetadataMatches(unit, requested.String(), includeTests) || unit.ID <= previous {
			return contractError("package analysis metadata is foreign, duplicated, or unordered")
		}
		previous = unit.ID
		foundPackage = foundPackage || unit.PkgPath == requested.String()
	}
	if !foundPackage {
		return contractError("package analysis metadata has no requested package")
	}
	return nil
}

func (a PackageAnalysis) validateUnits() error {
	foundPackage := false
	previous := ""
	for _, unit := range a.Units {
		if err := validateAnalysisUnit(unit, a.Incomplete); err != nil {
			return err
		}
		if !analysisMetadataMatches(unit, a.Package.String(), a.IncludeTests) || unit.ID <= previous {
			return contractError("package analysis units are foreign, duplicated, or unordered")
		}
		previous = unit.ID
		foundPackage = foundPackage || unit.PkgPath == a.Package.String()
	}
	if !foundPackage && !a.Incomplete {
		return contractError("package analysis does not contain its requested package")
	}
	return nil
}

func validateAnalysisUnit(unit *packages.Package, incomplete bool) error {
	if unit == nil || unit.Types == nil || unit.TypesInfo == nil || unit.Fset == nil {
		return contractError("package analysis contains an incomplete compilation unit")
	}
	if err := validateAnalysisUnitErrors(unit, incomplete); err != nil {
		return err
	}
	if len(unit.Syntax) != len(unit.CompiledGoFiles) {
		return contractError("package analysis syntax and file membership disagree")
	}
	return nil
}

func validateAnalysisUnitErrors(unit *packages.Package, incomplete bool) error {
	if unit.IllTyped != (len(unit.Errors) != 0) || unit.IllTyped && !incomplete {
		return contractError("package analysis compiler errors lack explicit partial ownership")
	}
	if !unit.IllTyped && len(unit.TypeErrors) != 0 {
		return contractError("package analysis hides type checker errors")
	}
	return nil
}

// Compilation is the validated exact process evidence from one successful compile.
type Compilation struct{ Result process.Result }

func (c Compilation) Validate() error {
	if err := c.Result.Validate(); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	exit, err := c.Result.ExitCode()
	if err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	success, err := exit.Success()
	if err != nil || !success {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	return nil
}
