package runnercontrol

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	BenchmarkAcceptanceSeconds uint64 = 30
	FuzzAcceptanceSeconds      uint64 = 30
)

type GoProfileKind uint8

const (
	GoProfileUnknown GoProfileKind = iota
	GoProfileFocused
	GoProfileAcceptance
	GoProfileRace
	GoProfileBenchmark
	GoProfileDiagnostic
	GoProfileFuzz
	goProfileLimit
)

func (k GoProfileKind) Validate() error {
	if k <= GoProfileUnknown || k >= goProfileLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (k GoProfileKind) IsValid() bool { return k.Validate() == nil }

func (k GoProfileKind) String() string {
	if !k.IsValid() {
		return invalidEnumString()
	}
	return []string{"", "focused", "acceptance", "race", "benchmark", "diagnostic", "fuzz"}[k]
}

func (k GoProfileKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(k.String())
}

func (k *GoProfileKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	for candidate := GoProfileUnknown + 1; candidate < goProfileLimit; candidate++ {
		if candidate.String() == value {
			*k = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
}

type CoverageMode uint8

const (
	CoverageModeUnknown CoverageMode = iota
	CoverageSet
	CoverageCount
	CoverageAtomic
	coverageModeLimit
)

func (m CoverageMode) Validate() error {
	if m <= CoverageModeUnknown || m >= coverageModeLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}
func (m CoverageMode) IsValid() bool { return m.Validate() == nil }
func (m CoverageMode) String() string {
	if !m.IsValid() {
		return invalidEnumString()
	}
	return []string{"", "set", "count", "atomic"}[m]
}

func (m CoverageMode) MarshalJSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(m.String())
}

func (m *CoverageMode) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	for candidate := CoverageModeUnknown + 1; candidate < coverageModeLimit; candidate++ {
		if candidate.String() == value {
			*m = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
}

type DiagnosticArtifacts struct {
	CPU    *core.RelativePath `json:"cpu,omitempty"`
	Memory *core.RelativePath `json:"memory,omitempty"`
	Block  *core.RelativePath `json:"block,omitempty"`
	Mutex  *core.RelativePath `json:"mutex,omitempty"`
	Trace  *core.RelativePath `json:"trace,omitempty"`
}

func (a DiagnosticArtifacts) Validate() error {
	count := 0
	for _, path := range []*core.RelativePath{a.CPU, a.Memory, a.Block, a.Mutex, a.Trace} {
		if path != nil {
			count++
			if err := path.Validate(); err != nil {
				return err
			}
		}
	}
	if count == 0 {
		return core.ErrPrimitiveContract
	}
	return nil
}

type GoExperimentPlan struct {
	Profile              GoProfileKind                 `json:"profile"`
	Kind                 projectstandards.ProbeKind    `json:"kind"`
	Package              projectstandards.SourcePath   `json:"package"`
	Selector             *projectstandards.Name        `json:"selector,omitempty"`
	Timeout              temporal.Duration             `json:"timeout"`
	ShuffleSeed          uint64                        `json:"shuffle_seed"`
	Parallel             uint16                        `json:"parallel"`
	PackageParallel      uint16                        `json:"package_parallel"`
	RepeatCount          uint16                        `json:"repeat_count"`
	CPU                  []uint16                      `json:"cpu"`
	Tags                 []projectstandards.Identifier `json:"tags"`
	Coverage             *CoverageMode                 `json:"coverage_mode,omitempty"`
	CoveragePath         *core.RelativePath            `json:"coverage_path,omitempty"`
	BenchmarkDuration    temporal.Duration             `json:"benchmark_duration"`
	FuzzDuration         temporal.Duration             `json:"fuzz_duration"`
	FuzzMinimizeDuration temporal.Duration             `json:"fuzz_minimize_duration"`
	Diagnostics          *DiagnosticArtifacts          `json:"diagnostics,omitempty"`
}

func (p GoExperimentPlan) Validate() error {
	if err := p.validateOwnedValues(); err != nil {
		return err
	}
	if p.Timeout.IsZero() || p.ShuffleSeed == 0 || p.Parallel == 0 || p.PackageParallel == 0 || p.RepeatCount != ExecutionRepeatCount || len(p.CPU) == 0 {
		return core.ErrPrimitiveContract
	}
	if err := p.validateCollections(); err != nil {
		return err
	}
	return errors.Join(p.validateCoverage(), p.validateVariant())
}

func (p GoExperimentPlan) validateOwnedValues() error {
	return errors.Join(p.Profile.Validate(), p.Kind.Validate(), p.Package.Validate(), p.Timeout.Validate(), p.BenchmarkDuration.Validate(), p.FuzzDuration.Validate(), p.FuzzMinimizeDuration.Validate())
}

func (p GoExperimentPlan) validateCollections() error {
	for index, value := range p.CPU {
		if value == 0 || (index > 0 && p.CPU[index-1] >= value) {
			return core.ErrPrimitiveContract
		}
	}
	return validateCanonicalIdentifiers(p.Tags)
}

func (p GoExperimentPlan) validateCoverage() error {
	if (p.Coverage == nil) != (p.CoveragePath == nil) {
		return core.ErrPrimitiveContract
	}
	if p.Coverage != nil {
		if err := errors.Join(p.Coverage.Validate(), p.CoveragePath.Validate()); err != nil {
			return err
		}
	}
	return nil
}
func (p GoExperimentPlan) validateVariant() error {
	if err := p.validateProfileShape(); err != nil {
		return err
	}
	return p.validateProfileExclusivity()
}

func (p GoExperimentPlan) validateProfileShape() error {
	switch p.Profile {
	case GoProfileFocused:
		return p.validateFocused()
	case GoProfileAcceptance:
		return requireGoKind(p.Kind, projectstandards.ProbeKindGoTest)
	case GoProfileRace:
		return requireGoKind(p.Kind, projectstandards.ProbeKindGoRace)
	case GoProfileBenchmark:
		return p.validateBenchmark()
	case GoProfileDiagnostic:
		return p.validateDiagnostic()
	case GoProfileFuzz:
		return p.validateFuzz()
	default:
		return core.ErrPrimitiveContract
	}

}

func (p GoExperimentPlan) validateFocused() error {
	if p.Kind != projectstandards.ProbeKindGoTest || p.Selector == nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

func requireGoKind(got, want projectstandards.ProbeKind) error {
	if got != want {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (p GoExperimentPlan) validateBenchmark() error {
	accepted, err := temporal.DurationFromSeconds(BenchmarkAcceptanceSeconds)
	if err != nil {
		return err
	}
	if p.Kind != projectstandards.ProbeKindGoBenchmark || p.Selector == nil {
		return core.ErrPrimitiveContract
	}
	if p.BenchmarkDuration != accepted || p.Parallel != 1 || p.Diagnostics == nil {
		return core.ErrPrimitiveContract
	}
	if p.Diagnostics.CPU == nil || p.Diagnostics.Memory == nil {
		return errors.Join(core.ErrPrimitiveContract, errors.New("benchmark evidence requires CPU and memory profiles"))
	}
	return p.Diagnostics.Validate()
}

func (p GoExperimentPlan) validateDiagnostic() error {
	if p.Kind != projectstandards.ProbeKindGoDiagnosticProfile || p.Diagnostics == nil {
		return core.ErrPrimitiveContract
	}
	return p.Diagnostics.Validate()
}

func (p GoExperimentPlan) validateFuzz() error {
	if p.Kind != projectstandards.ProbeKindGoFuzz || p.Selector == nil || p.Parallel != 1 {
		return core.ErrPrimitiveContract
	}
	accepted, err := temporal.DurationFromSeconds(FuzzAcceptanceSeconds)
	if err != nil {
		return err
	}
	if p.FuzzDuration != accepted || p.FuzzMinimizeDuration.IsZero() {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (p GoExperimentPlan) validateProfileExclusivity() error {
	if p.Selector != nil {
		if err := p.Selector.Validate(); err != nil {
			return err
		}
	}
	if !p.durationFieldsMatchProfile() {
		return core.ErrPrimitiveContract
	}
	if p.Profile != GoProfileDiagnostic && p.Profile != GoProfileBenchmark && p.Diagnostics != nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (p GoExperimentPlan) durationFieldsMatchProfile() bool {
	if p.Profile != GoProfileBenchmark && !p.BenchmarkDuration.IsZero() {
		return false
	}
	return p.Profile == GoProfileFuzz || (p.FuzzDuration.IsZero() && p.FuzzMinimizeDuration.IsZero())
}

type GoExecutionEnvironment struct {
	Home       core.AbsolutePath `json:"home"`
	Cache      core.AbsolutePath `json:"cache"`
	Temporary  core.AbsolutePath `json:"temporary"`
	GOMAXPROCS uint16            `json:"gomaxprocs"`
	CGOEnabled bool              `json:"cgo_enabled"`
}

func (e GoExecutionEnvironment) Validate() error {
	if e.GOMAXPROCS == 0 {
		return core.ErrPrimitiveContract
	}
	return errors.Join(e.Home.Validate(), e.Cache.Validate(), e.Temporary.Validate())
}

type GoPlanRequest struct {
	Command           core.AbsolutePath
	WorkingDirectory  core.AbsolutePath
	WorkspaceRoot     core.AbsolutePath
	ArtifactDirectory core.RelativePath
	Environment       GoExecutionEnvironment
	Experiment        GoExperimentPlan
	OutputLimit       core.ByteCount
	ArtifactLimit     core.ByteCount
	ExpectedUnits     uint32
	WaitDelay         temporal.Duration
	Subject           SubjectExecution
}

func (r GoPlanRequest) Validate() error {
	if err := errors.Join(r.Command.Validate(), r.WorkingDirectory.Validate(), r.WorkspaceRoot.Validate(), r.ArtifactDirectory.Validate(), r.Environment.Validate(), r.Experiment.Validate(), r.OutputLimit.Validate(), r.ArtifactLimit.Validate(), r.WaitDelay.Validate()); err != nil {
		return err
	}
	if r.ExpectedUnits == 0 || r.ExpectedUnits > ExecutionAccountingUnitMaximum {
		return core.ErrPrimitiveContract
	}
	if r.Experiment.Profile == GoProfileRace && !r.Environment.CGOEnabled {
		return errors.Join(core.ErrPrimitiveContract, errors.New("go race execution requires the admitted cgo toolchain context"))
	}
	_, workingErr := r.WorkingDirectory.RelativeTo(r.Subject.SourceRoot)
	_, artifactErr := r.WorkspaceRoot.JoinRelative(r.ArtifactDirectory)
	return errors.Join(workingErr, artifactErr)
}

type ExperimentExecution struct {
	Process     process.Plan          `json:"process"`
	Workspace   WritableWorkspace     `json:"workspace"`
	Subject     SubjectExecution      `json:"subject"`
	Artifacts   []ArtifactExpectation `json:"artifacts"`
	Observation ObservationPolicy     `json:"observation"`
	Budget      ExecutionBudget       `json:"budget"`
}

func (p ExperimentExecution) Validate() error {
	return errors.Join(p.Process.Validate(), p.Workspace.Validate(), p.Subject.Validate(p.Process, p.Workspace), p.Workspace.ValidateEnvironment(p.Process.Environment), validateArtifactExpectations(p.Artifacts), p.Observation.Validate(), p.Budget.Validate())
}

func CompileGoPlan(request GoPlanRequest) (ExperimentExecution, error) {
	if err := request.Validate(); err != nil {
		return ExperimentExecution{}, err
	}
	artifacts, paths, err := compileGoArtifacts(request)
	if err != nil {
		return ExperimentExecution{}, err
	}
	arguments, err := request.Experiment.arguments(paths)
	if err != nil {
		return ExperimentExecution{}, err
	}
	parsedArguments, err := process.ParseArguments(arguments)
	if err != nil {
		return ExperimentExecution{}, err
	}
	environment, err := goEnvironment(request.Environment)
	if err != nil {
		return ExperimentExecution{}, err
	}
	budget, err := NewExecutionBudget(request.Experiment.Timeout, request.ExpectedUnits, request.Experiment.PackageParallel)
	if err != nil {
		return ExperimentExecution{}, err
	}
	plan := process.Plan{SchemaVersion: process.ExecutionPlanSchemaVersion, Command: request.Command, WorkingDirectory: request.WorkingDirectory, Arguments: parsedArguments, Environment: environment, OutputLimit: request.OutputLimit, WaitDelay: request.WaitDelay, Containment: process.Containment{Isolation: process.IsolationGroup, CancelSignal: process.CancelSignalTerminate}}
	filtered := request.Experiment.Profile == GoProfileFocused || request.Experiment.Profile == GoProfileBenchmark || request.Experiment.Profile == GoProfileFuzz
	workspace := WritableWorkspace{Root: request.WorkspaceRoot, Home: request.Environment.Home, Output: paths.output, Cache: request.Environment.Cache, Temporary: request.Environment.Temporary}
	compiled := ExperimentExecution{Process: plan, Workspace: workspace, Subject: request.Subject, Artifacts: artifacts, Observation: ObservationPolicy{Format: ObservationGoTestJSON, ExpectedUnits: request.ExpectedUnits, Filtered: filtered}, Budget: budget}
	return compiled, compiled.Validate()
}

type compiledGoArtifactPaths struct {
	output      core.AbsolutePath
	coverage    *core.AbsolutePath
	diagnostics diagnosticArtifactPaths
}

type diagnosticArtifactPaths struct {
	CPU    *core.AbsolutePath
	Memory *core.AbsolutePath
	Block  *core.AbsolutePath
	Mutex  *core.AbsolutePath
	Trace  *core.AbsolutePath
}

func (p GoExperimentPlan) arguments(paths compiledGoArtifactPaths) ([]string, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	timeout, err := p.Timeout.Stdlib()
	if err != nil {
		return nil, err
	}
	arguments := []string{"test", p.Package.String(), "-json", "-count=" + strconv.FormatUint(uint64(p.RepeatCount), 10), "-fullpath", "-outputdir=" + paths.output.String(), "-timeout=" + timeout.String(), "-shuffle=" + strconv.FormatUint(p.ShuffleSeed, 10), "-p=" + strconv.FormatUint(uint64(p.PackageParallel), 10), "-parallel=" + strconv.FormatUint(uint64(p.Parallel), 10), "-cpu=" + joinCPU(p.CPU)}
	selector := p.selectorArgument()
	arguments, err = p.appendProfileArguments(arguments, selector)
	if err != nil {
		return nil, err
	}
	return p.appendOptionalArguments(arguments, paths), nil
}

func (p GoExperimentPlan) selectorArgument() string {
	if p.Selector == nil {
		return ""
	}
	return "^" + regexp.QuoteMeta(p.Selector.String()) + "$"
}

func (p GoExperimentPlan) appendProfileArguments(arguments []string, selector string) ([]string, error) {
	switch p.Profile {
	case GoProfileFocused:
		arguments = append(arguments, "-run="+selector)
	case GoProfileAcceptance, GoProfileRace, GoProfileDiagnostic:
		if selector != "" {
			arguments = append(arguments, "-run="+selector)
		}
	case GoProfileBenchmark:
		duration, err := p.BenchmarkDuration.Stdlib()
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, "-run=^$", "-bench="+selector, "-benchmem", "-benchtime="+duration.String())
	case GoProfileFuzz:
		fuzz, err := p.FuzzDuration.Stdlib()
		if err != nil {
			return nil, err
		}
		minimize, err := p.FuzzMinimizeDuration.Stdlib()
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, "-run=^$", "-fuzz="+selector, "-fuzztime="+fuzz.String(), "-fuzzminimizetime="+minimize.String())
	default:
		return nil, core.ErrPrimitiveContract
	}
	return arguments, nil
}

func (p GoExperimentPlan) appendOptionalArguments(arguments []string, paths compiledGoArtifactPaths) []string {
	if p.Profile == GoProfileRace {
		arguments = append(arguments, "-race")
	}
	if len(p.Tags) > 0 {
		values := make([]string, len(p.Tags))
		for index := range p.Tags {
			values[index] = p.Tags[index].String()
		}
		arguments = append(arguments, "-tags="+strings.Join(values, ","))
	}
	if p.Coverage != nil {
		arguments = append(arguments, "-covermode="+p.Coverage.String(), "-coverprofile="+paths.coverage.String())
	}
	if p.Diagnostics != nil {
		arguments = appendDiagnostic(arguments, paths.diagnostics)
	}
	return arguments
}
func joinCPU(values []uint16) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.FormatUint(uint64(value), 10)
	}
	return strings.Join(parts, ",")
}
func appendDiagnostic(arguments []string, a diagnosticArtifactPaths) []string {
	pairs := []struct {
		flag string
		path *core.AbsolutePath
	}{{"-cpuprofile=", a.CPU}, {"-memprofile=", a.Memory}, {"-blockprofile=", a.Block}, {"-mutexprofile=", a.Mutex}, {"-trace=", a.Trace}}
	for _, pair := range pairs {
		if pair.path != nil {
			arguments = append(arguments, pair.flag+pair.path.String())
		}
	}
	return arguments
}

func compileGoArtifacts(request GoPlanRequest) ([]ArtifactExpectation, compiledGoArtifactPaths, error) {
	output, err := request.WorkspaceRoot.JoinRelative(request.ArtifactDirectory)
	if err != nil {
		return nil, compiledGoArtifactPaths{}, err
	}
	paths := compiledGoArtifactPaths{output: output}
	artifacts := make([]ArtifactExpectation, 0, 6)
	if request.Experiment.CoveragePath != nil {
		expectation, absolute, compileErr := compileGoArtifact(request, *request.Experiment.CoveragePath, ArtifactCoverage)
		if compileErr != nil {
			return nil, compiledGoArtifactPaths{}, compileErr
		}
		artifacts = append(artifacts, expectation)
		paths.coverage = &absolute
	}
	if request.Experiment.Diagnostics != nil {
		var err error
		artifacts, paths.diagnostics, err = compileDiagnosticArtifacts(request, artifacts, *request.Experiment.Diagnostics)
		if err != nil {
			return nil, compiledGoArtifactPaths{}, err
		}
	}
	slices.SortFunc(artifacts, func(left, right ArtifactExpectation) int {
		return strings.Compare(left.Path.String(), right.Path.String())
	})
	return artifacts, paths, nil
}

func compileDiagnosticArtifacts(request GoPlanRequest, artifacts []ArtifactExpectation, diagnostics DiagnosticArtifacts) ([]ArtifactExpectation, diagnosticArtifactPaths, error) {
	paths := diagnosticArtifactPaths{}
	values := []struct {
		relative *core.RelativePath
		kind     ArtifactKind
		absolute **core.AbsolutePath
	}{{diagnostics.CPU, ArtifactCPUProfile, &paths.CPU}, {diagnostics.Memory, ArtifactMemoryProfile, &paths.Memory}, {diagnostics.Block, ArtifactBlockProfile, &paths.Block}, {diagnostics.Mutex, ArtifactMutexProfile, &paths.Mutex}, {diagnostics.Trace, ArtifactTrace, &paths.Trace}}
	for _, value := range values {
		if value.relative == nil {
			continue
		}
		expectation, absolute, err := compileGoArtifact(request, *value.relative, value.kind)
		if err != nil {
			return nil, diagnosticArtifactPaths{}, err
		}
		artifacts = append(artifacts, expectation)
		*value.absolute = &absolute
	}
	return artifacts, paths, nil
}

func compileGoArtifact(request GoPlanRequest, name core.RelativePath, kind ArtifactKind) (ArtifactExpectation, core.AbsolutePath, error) {
	component, err := core.ParsePathComponent(name.String())
	if err != nil {
		return ArtifactExpectation{}, core.AbsolutePath{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	path, err := request.ArtifactDirectory.Join(component)
	if err != nil {
		return ArtifactExpectation{}, core.AbsolutePath{}, err
	}
	absolute, err := request.WorkspaceRoot.JoinRelative(path)
	if err != nil {
		return ArtifactExpectation{}, core.AbsolutePath{}, err
	}
	protocolPath, err := projectstandards.ParseSourcePath(path.String())
	if err != nil {
		return ArtifactExpectation{}, core.AbsolutePath{}, err
	}
	expectation := ArtifactExpectation{Kind: kind, Path: protocolPath, MediaType: core.HTTPMediaTypeOctetStream(), MaximumBytes: request.ArtifactLimit, Required: true}
	return expectation, absolute, expectation.Validate()
}
func goEnvironment(e GoExecutionEnvironment) (process.Environment, error) {
	cgo := "0"
	if e.CGOEnabled {
		cgo = "1"
	}
	values := []string{core.EnvironmentHomeName + "=" + e.Home.String(), "GOCACHE=" + e.Cache.String(), core.EnvironmentTemporaryName + "=" + e.Temporary.String(), core.EnvironmentCacheName + "=" + e.Cache.String(), "GOMAXPROCS=" + fmt.Sprintf("%d", e.GOMAXPROCS), "CGO_ENABLED=" + cgo}
	return process.ParseExactEnvironment(values)
}

var (
	_ core.Validatable = GoProfileUnknown
	_ json.Unmarshaler = (*GoProfileKind)(nil)
	_ core.Validatable = CoverageModeUnknown
	_ json.Unmarshaler = (*CoverageMode)(nil)
	_ core.Validatable = DiagnosticArtifacts{}
	_ core.Validatable = GoExperimentPlan{}
	_ core.Validatable = GoExecutionEnvironment{}
	_ core.Validatable = GoPlanRequest{}
	_ core.Validatable = ExperimentExecution{}
)
