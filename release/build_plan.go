package release

import (
	"errors"
	"go/token"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/garble"
)

const (
	mainPackageMaximumBytes       = 512
	mainPackageInvalidDiagnostic  = "main package is invalid"
	linkerSymbolMaximumBytes      = 512
	linkerValueMaximumBytes       = 512
	linkerAssignmentMaximumCount  = 16
	goTrimpathArgument            = "-trimpath"
	goDisableBuildVCSArgument     = "-buildvcs=false"
	goDisablePGOArgument          = "-pgo=off"
	goLinkerArgumentPrefix        = "-ldflags="
	goOutputArgument              = "-o"
	goLinkerStripArguments        = "-w -s"
	goLinkerAssignmentArgument    = " -X "
	goModuleArgumentPrefix        = "-mod="
	goEnvironmentCGODisabled      = "CGO_ENABLED=0"
	goEnvironmentToolchainLocal   = "GOTOOLCHAIN=local"
	goEnvironmentAMD64Baseline    = "GOAMD64=v1"
	goEnvironmentARM64Baseline    = "GOARM64=v8.0"
	goEnvironmentConfigurationOff = "GOENV=off"
	goEnvironmentFlagsEmpty       = "GOFLAGS="
	goEnvironmentExperimentEmpty  = "GOEXPERIMENT="
	goEnvironmentFIPSOff          = "GOFIPS140=off"
	goEnvironmentWorkspaceOff     = "GOWORK=off"
)

// BuildModuleMode selects the Go module graph used by one release build.
type BuildModuleMode uint8

const (
	// BuildModuleUnknown is the invalid zero module mode.
	BuildModuleUnknown BuildModuleMode = iota
	// BuildModuleReadonly admits the checked module graph without modifying it.
	BuildModuleReadonly
	// BuildModuleVendor requires the checked vendor projection.
	BuildModuleVendor
	buildModuleLimit
)

func buildModuleLabels() [buildModuleLimit]string {
	return [...]string{
		BuildModuleReadonly: "readonly",
		BuildModuleVendor:   "vendor",
	}
}

// Validate rejects an incomplete or future module mode.
func (m BuildModuleMode) Validate() error {
	if m <= BuildModuleUnknown || m >= buildModuleLimit || buildModuleLabels()[m] == "" {
		return contractError(errors.New("build module mode is outside the admitted domain"))
	}
	return nil
}

// IsValid reports membership in the closed module-mode domain.
func (m BuildModuleMode) IsValid() bool { return m.Validate() == nil }

// OffWireEnum declares BuildModuleMode as local build execution policy.
func (BuildModuleMode) OffWireEnum() {}

// String returns the exact Go -mod value, or the unknown diagnostic.
func (m BuildModuleMode) String() string {
	if !m.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return buildModuleLabels()[m]
}

// MainPackage is one bounded canonical Go import path naming a release main.
type MainPackage struct {
	value string
}

// ParseMainPackage validates one canonical Go import path.
func ParseMainPackage(value string) (MainPackage, error) {
	if err := validateGoPackagePath(value, mainPackageMaximumBytes); err != nil {
		return MainPackage{}, contractError(errors.New(mainPackageInvalidDiagnostic), err)
	}
	return MainPackage{value: value}, nil
}

// Validate rejects an unset or noncanonical main package.
func (p MainPackage) Validate() error {
	parsed, err := ParseMainPackage(p.value)
	if err != nil || parsed != p {
		return contractError(errors.New(mainPackageInvalidDiagnostic), err)
	}
	return nil
}

// String returns the canonical import path, or empty text when invalid.
func (p MainPackage) String() string {
	if p.Validate() != nil {
		return ""
	}
	return p.value
}

// LinkerAssignment is one bounded Go linker -X symbol/value pair.
type LinkerAssignment struct {
	symbol string
	value  string
}

// NewLinkerAssignment constructs one product-owned linker assignment.
func NewLinkerAssignment(symbol, value string) (LinkerAssignment, error) {
	assignment := LinkerAssignment{symbol: symbol, value: value}
	if err := assignment.Validate(); err != nil {
		return LinkerAssignment{}, err
	}
	return assignment, nil
}

// Validate proves symbol grammar, value bounds, and Primitive symbol ownership.
func (a LinkerAssignment) Validate() error {
	if err := validateLinkerSymbol(a.symbol); err != nil {
		return contractError(errors.New("linker assignment symbol is invalid"), err)
	}
	if primitiveBuildLinkSymbol(a.symbol) {
		return contractError(errors.New("linker assignment overrides a Primitive-owned symbol"))
	}
	if err := validateLinkerValue(a.value); err != nil {
		return contractError(errors.New("linker assignment value is invalid"), err)
	}
	return nil
}

// Symbol returns the validated linker symbol.
func (a LinkerAssignment) Symbol() string {
	if a.Validate() != nil {
		return ""
	}
	return a.symbol
}

// Value returns the validated linker value.
func (a LinkerAssignment) Value() string {
	if a.Validate() != nil {
		return ""
	}
	return a.value
}

// LinkerAssignments is a bounded, symbol-sorted set of linker assignments.
type LinkerAssignments struct {
	values [linkerAssignmentMaximumCount]LinkerAssignment
	count  int
}

// NewLinkerAssignments validates, sorts, and rejects duplicate symbols.
func NewLinkerAssignments(values []LinkerAssignment) (LinkerAssignments, error) {
	if len(values) > linkerAssignmentMaximumCount {
		return LinkerAssignments{}, contractError(errors.New("linker assignment count exceeds its bound"))
	}
	ordered := append([]LinkerAssignment(nil), values...)
	for _, assignment := range ordered {
		if err := assignment.Validate(); err != nil {
			return LinkerAssignments{}, err
		}
	}
	sort.Slice(ordered, func(left, right int) bool {
		return ordered[left].symbol < ordered[right].symbol
	})
	var set LinkerAssignments
	for index, assignment := range ordered {
		if index > 0 && ordered[index-1].symbol == assignment.symbol {
			return LinkerAssignments{}, contractError(errors.New("linker assignment symbol is duplicated"))
		}
		set.values[index] = assignment
	}
	set.count = len(ordered)
	return set, nil
}

// Validate proves canonical ordering, uniqueness, count, and zero padding.
func (s LinkerAssignments) Validate() error {
	if s.count < 0 || s.count > len(s.values) {
		return contractError(errors.New("linker assignment count is outside its storage bounds"))
	}
	for index, assignment := range s.values[:s.count] {
		if err := assignment.Validate(); err != nil {
			return err
		}
		if index > 0 && s.values[index-1].symbol >= assignment.symbol {
			return contractError(errors.New("linker assignments are not unique and sorted"))
		}
	}
	for _, padding := range s.values[s.count:] {
		if padding != (LinkerAssignment{}) {
			return contractError(errors.New("linker assignment padding is nonzero"))
		}
	}
	return nil
}

// At returns one validated assignment by ascending symbol order.
func (s LinkerAssignments) At(index int) (LinkerAssignment, bool) {
	if s.Validate() != nil || index < 0 || index >= s.count {
		return LinkerAssignment{}, false
	}
	return s.values[index], true
}

// BuildPlanRequest carries every compiler-owned input to the fixed release build.
type BuildPlanRequest struct {
	MainPackage       MainPackage
	OutputDirectory   core.RelativePath
	LinkerAssignments LinkerAssignments
	Version           core.ReleaseVersion
	Commit            core.BuildCommit
	Garble            garble.BuildIntent
	GoToolchain       GoToolchainIdentity
	Offering          core.Offering
	ModuleMode        BuildModuleMode
}

// Validate proves every build-plan boundary input.
func (r BuildPlanRequest) Validate() error {
	for _, err := range []error{
		r.Offering.Validate(), r.Version.Validate(), r.Commit.Validate(),
		r.MainPackage.Validate(), r.OutputDirectory.Validate(),
		r.GoToolchain.Validate(), r.ModuleMode.Validate(), r.LinkerAssignments.Validate(),
	} {
		if err != nil {
			return contractError(errors.New("build plan request is invalid"), err)
		}
	}
	if err := r.Garble.Validate(); err != nil {
		return err
	}
	tool, err := r.Garble.Tool()
	if err != nil {
		return err
	}
	if r.GoToolchain != CurrentGoToolchain() || tool != garble.CurrentTool() {
		return contractError(errors.New("build plan does not use the current release tools"))
	}
	return nil
}

// BuildCommand is one exact Garble invocation for one canonical target.
type BuildCommand struct {
	mainPackage       MainPackage
	outputDirectory   core.RelativePath
	output            core.RelativePath
	linkerAssignments LinkerAssignments
	build             core.BuildIdentity
	garble            garble.BuildIntent
	goToolchain       GoToolchainIdentity
	moduleMode        BuildModuleMode
	valid             bool
}

// Validate proves the command and its derived output filename.
func (c BuildCommand) Validate() error {
	if !c.valid {
		return contractError(errors.New("build command is unset"))
	}
	for _, err := range []error{
		c.build.Validate(), c.mainPackage.Validate(), c.outputDirectory.Validate(),
		c.output.Validate(), c.goToolchain.Validate(), c.moduleMode.Validate(),
		c.linkerAssignments.Validate(),
	} {
		if err != nil {
			return contractError(errors.New("build command is invalid"), err)
		}
	}
	if err := c.garble.Validate(); err != nil {
		return err
	}
	want, err := buildOutputPath(c.outputDirectory, c.build)
	if err != nil || want != c.output {
		return contractError(errors.New("build command output is not derived from its identity"), err)
	}
	return nil
}

// Build returns the immutable build identity.
func (c BuildCommand) Build() core.BuildIdentity { return c.build }

// GoToolchain returns the exact compiler identity required by the command.
func (c BuildCommand) GoToolchain() GoToolchainIdentity { return c.goToolchain }

// Output returns the derived artifact path.
func (c BuildCommand) Output() core.RelativePath { return c.output }

// EnvironmentOverrides lowers the target to its hermetic compatibility
// controls. The executor supplies exact host paths and caches separately.
func (c BuildCommand) EnvironmentOverrides() ([]string, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	platform := c.build.Platform()
	values := []string{
		goEnvironmentCGODisabled,
		"GOARCH=" + platform.Architecture.String(),
		"GOOS=" + platform.OperatingSystem.String(),
		goEnvironmentToolchainLocal,
	}
	switch platform.Architecture {
	case core.CPUArchitectureAMD64:
		values = append(values, goEnvironmentAMD64Baseline)
	case core.CPUArchitectureARM64:
		values = append(values, goEnvironmentARM64Baseline)
	default:
		return nil, contractError(errors.New("build architecture lacks a compatibility baseline"))
	}
	values = append(values,
		goEnvironmentConfigurationOff,
		goEnvironmentFlagsEmpty,
		goEnvironmentExperimentEmpty,
		goEnvironmentFIPSOff,
		goEnvironmentWorkspaceOff,
	)
	return values, nil
}

// ArgumentValues lowers the validated command to exact Garble and Go arguments.
func (c BuildCommand) ArgumentValues() ([]string, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	garbleArguments, err := c.garble.Arguments()
	if err != nil {
		return nil, err
	}
	arguments := make([]string, 0, 12)
	for argument := range garbleArguments {
		value, textErr := argument.Text()
		if textErr != nil {
			return nil, textErr
		}
		arguments = append(arguments, value)
	}
	arguments = append(arguments,
		goTrimpathArgument,
		goDisableBuildVCSArgument,
		goDisablePGOArgument,
		goModuleArgumentPrefix+c.moduleMode.String(),
		goLinkerArgumentPrefix+c.linkerFlags(),
		goOutputArgument,
		c.output.String(),
		c.mainPackage.String(),
	)
	return arguments, nil
}

func (c BuildCommand) linkerFlags() string {
	var flags strings.Builder
	flags.WriteString(goLinkerStripArguments)
	writeLinkerFlag(&flags, EmbeddedBuildOfferingLinkSymbol, c.build.Offering().String())
	writeLinkerFlag(&flags, EmbeddedBuildVersionLinkSymbol, c.build.Version().String())
	writeLinkerFlag(&flags, EmbeddedBuildCommitLinkSymbol, c.build.Commit().String())
	writeLinkerFlag(&flags, EmbeddedBuildPlatformLinkSymbol, c.build.Platform().String())
	for index := range c.linkerAssignments.count {
		assignment := c.linkerAssignments.values[index]
		writeLinkerFlag(&flags, assignment.symbol, assignment.value)
	}
	return flags.String()
}

func writeLinkerFlag(destination *strings.Builder, symbol, value string) {
	destination.WriteString(goLinkerAssignmentArgument)
	destination.WriteString(symbol)
	destination.WriteString("=")
	destination.WriteString(value)
}

// BuildPlan is the exact fixed-target deterministic release build projection.
type BuildPlan struct {
	request  BuildPlanRequest
	commands [TargetCount]BuildCommand
	valid    bool
}

// PrepareBuildPlan constructs one command for every canonical release target.
func PrepareBuildPlan(request BuildPlanRequest) (BuildPlan, error) {
	return prepareBuildPlan(request)
}

func prepareBuildPlan(request BuildPlanRequest) (BuildPlan, error) {
	if err := request.Validate(); err != nil {
		return BuildPlan{}, err
	}
	var commands [TargetCount]BuildCommand
	targets := Targets()
	for index := range TargetCount {
		target, _ := targets.At(index)
		build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
			Offering: request.Offering,
			Version:  request.Version,
			Commit:   request.Commit,
			Platform: target,
		})
		if err != nil {
			return BuildPlan{}, contractError(errors.New("target build identity is invalid"), err)
		}
		output, err := buildOutputPath(request.OutputDirectory, build)
		if err != nil {
			return BuildPlan{}, err
		}
		commands[index] = BuildCommand{
			build: build, mainPackage: request.MainPackage,
			outputDirectory: request.OutputDirectory, output: output,
			garble: request.Garble, goToolchain: request.GoToolchain,
			moduleMode:        request.ModuleMode,
			linkerAssignments: request.LinkerAssignments, valid: true,
		}
	}
	return BuildPlan{request: request, commands: commands, valid: true}, nil
}

// Validate reconstructs and compares the complete fixed-target projection.
func (p BuildPlan) Validate() error {
	if !p.valid {
		return contractError(errors.New("build plan is unset"))
	}
	want, err := prepareBuildPlan(p.request)
	if err != nil {
		return err
	}
	if want != p {
		return contractError(errors.New("build plan does not match its request"))
	}
	for _, command := range p.commands {
		if err := command.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// At returns one command in canonical target order.
func (p BuildPlan) At(index int) (BuildCommand, bool) {
	if p.Validate() != nil || index < 0 || index >= TargetCount {
		return BuildCommand{}, false
	}
	return p.commands[index], true
}

func buildOutputPath(directory core.RelativePath, build core.BuildIdentity) (core.RelativePath, error) {
	filename, err := newBinaryFilename(build)
	if err != nil {
		return core.RelativePath{}, err
	}
	output, err := core.ParseRelativePath(filepath.Join(directory.String(), filename.String()))
	if err != nil {
		return core.RelativePath{}, contractError(errors.New("build output path is invalid"), err)
	}
	return output, nil
}

func validateGoPackagePath(value string, maximum int) error {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) {
		return errors.New("go package path has invalid length or encoding")
	}
	if value != path.Clean(value) || strings.HasPrefix(value, "/") ||
		!strings.Contains(value, "/") {
		return errors.New("go package path is not canonical")
	}
	for element := range strings.SplitSeq(value, "/") {
		if err := validateGoPackagePathElement(element); err != nil {
			return err
		}
	}
	return nil
}

// validateGoPackagePathElement rejects the Go module-path element forms that
// are also hostile as argv. A leading hyphen would reach the Go command as a
// flag rather than as a package, and a leading or trailing dot is not an
// admitted module-path element.
func validateGoPackagePathElement(element string) error {
	if element == "" {
		return errors.New("go package path has an empty element")
	}
	if strings.HasPrefix(element, "-") {
		return errors.New("go package path element begins with a flag prefix")
	}
	if strings.HasPrefix(element, ".") || strings.HasSuffix(element, ".") {
		return errors.New("go package path element begins or ends with a dot")
	}
	for _, character := range element {
		if !validGoPackagePathRune(character) {
			return errors.New("go package path contains an unsupported character")
		}
	}
	return nil
}

func validGoPackagePathRune(character rune) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' ||
		strings.ContainsRune("/._-~", character)
}

func validateLinkerSymbol(symbol string) error {
	if symbol == "" || len(symbol) > linkerSymbolMaximumBytes || !utf8.ValidString(symbol) {
		return errors.New("linker symbol has invalid length or encoding")
	}
	separator := strings.LastIndexByte(symbol, '.')
	if separator <= 0 || separator == len(symbol)-1 {
		return errors.New("linker symbol lacks a package path or variable")
	}
	if err := validateGoPackagePath(symbol[:separator], linkerSymbolMaximumBytes); err != nil {
		return err
	}
	if !token.IsIdentifier(symbol[separator+1:]) {
		return errors.New("linker symbol variable is not a Go identifier")
	}
	return nil
}

func validateLinkerValue(value string) error {
	if value == "" || len(value) > linkerValueMaximumBytes || !utf8.ValidString(value) {
		return errors.New("linker value has invalid length or encoding")
	}
	for _, character := range value {
		if character == '=' || unicode.IsSpace(character) {
			return errors.New("linker value contains a delimiter")
		}
		if !unicode.IsPrint(character) {
			return errors.New("linker value contains a nonprintable character")
		}
	}
	return nil
}

func primitiveBuildLinkSymbol(symbol string) bool {
	switch symbol {
	case EmbeddedBuildOfferingLinkSymbol,
		EmbeddedBuildVersionLinkSymbol,
		EmbeddedBuildCommitLinkSymbol,
		EmbeddedBuildPlatformLinkSymbol:
		return true
	default:
		return false
	}
}

var _ core.OffWireEnum = BuildModuleUnknown
