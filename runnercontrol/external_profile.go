package runnercontrol

import (
	"errors"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/temporal"
)

const JavaScriptRepeatCount uint16 = 1

type ExternalExecutionEnvironment struct {
	Home      core.AbsolutePath
	Cache     core.AbsolutePath
	Temporary core.AbsolutePath
}

func (e ExternalExecutionEnvironment) Validate() error {
	return errors.Join(e.Home.Validate(), e.Cache.Validate(), e.Temporary.Validate())
}

// ExternalPlanBase contains only the effect coordinates shared by the closed
// JavaScript, smoke, and tool profile compilers. It contains no argument bag.
type ExternalPlanBase struct {
	Command          core.AbsolutePath
	WorkingDirectory core.AbsolutePath
	WorkspaceRoot    core.AbsolutePath
	OutputDirectory  core.RelativePath
	Environment      ExternalExecutionEnvironment
	OutputLimit      core.ByteCount
	WaitDelay        temporal.Duration
	Subject          SubjectExecution
	Egress           EgressPolicy
}

func (b ExternalPlanBase) Validate() error {
	if err := errors.Join(b.Command.Validate(), b.WorkingDirectory.Validate(), b.WorkspaceRoot.Validate(), b.OutputDirectory.Validate(), b.Environment.Validate(), b.OutputLimit.Validate(), b.WaitDelay.Validate(), b.Egress.Validate()); err != nil {
		return err
	}
	egressIdentity, err := b.Egress.Digest()
	if err != nil || egressIdentity != b.Subject.EgressPolicyIdentity {
		return errors.Join(core.ErrPrimitiveContract, errors.New("external profile egress policy does not bind subject execution"), err)
	}
	if b.WaitDelay.IsZero() {
		return errors.Join(core.ErrPrimitiveContract, errors.New("external profile wait delay is zero"))
	}
	if _, err := b.WorkingDirectory.RelativeTo(b.Subject.SourceRoot); err != nil {
		return errors.Join(core.ErrPrimitiveContract, errors.New("external profile working directory escapes the verified source root"), err)
	}
	output, err := b.WorkspaceRoot.JoinRelative(b.OutputDirectory)
	if err != nil || output == b.WorkspaceRoot {
		return errors.Join(core.ErrPrimitiveContract, errors.New("external profile output directory is not isolated inside its workspace"), err)
	}
	return nil
}

type JavaScriptTestPlan struct {
	File        projectstandards.SourcePath
	Timeout     temporal.Duration
	Concurrency uint16
	RepeatCount uint16
	Report      core.RelativePath
}

func (p JavaScriptTestPlan) Validate() error {
	if err := errors.Join(p.File.Validate(), p.Timeout.Validate(), p.Report.Validate()); err != nil {
		return err
	}
	if p.Timeout.IsZero() || p.Concurrency == 0 || p.RepeatCount != JavaScriptRepeatCount {
		return core.ErrPrimitiveContract
	}
	file := p.File.String()
	if !strings.HasSuffix(file, ".js") && !strings.HasSuffix(file, ".jsx") && !strings.HasSuffix(file, ".ts") && !strings.HasSuffix(file, ".tsx") {
		return errors.Join(core.ErrPrimitiveContract, errors.New("JavaScript profile target is not a JavaScript or TypeScript file"))
	}
	if _, err := core.ParsePathComponent(p.Report.String()); err != nil {
		return errors.Join(core.ErrPrimitiveContract, errors.New("JavaScript JUnit report must be one compiler-owned filename"), err)
	}
	return nil
}

type JavaScriptPlanRequest struct {
	Base          ExternalPlanBase
	Test          JavaScriptTestPlan
	ArtifactLimit core.ByteCount
	ExpectedUnits uint32
}

func (r JavaScriptPlanRequest) Validate() error {
	if err := errors.Join(r.Base.Validate(), r.Test.Validate(), r.ArtifactLimit.Validate()); err != nil {
		return err
	}
	if r.ExpectedUnits == 0 || r.ExpectedUnits > ExecutionAccountingUnitMaximum {
		return core.ErrPrimitiveContract
	}
	if r.Base.Egress.Mode != EgressDenied {
		return errors.Join(core.ErrPrimitiveContract, errors.New("JavaScript test profile must deny subject network egress"))
	}
	return nil
}

type SmokePlan struct {
	Suite   projectstandards.Identifier
	Timeout temporal.Duration
}

func (p SmokePlan) Validate() error {
	if err := errors.Join(p.Suite.Validate(), p.Timeout.Validate()); err != nil {
		return err
	}
	if p.Timeout.IsZero() {
		return core.ErrPrimitiveContract
	}
	return nil
}

type SmokePlanRequest struct {
	Base  ExternalPlanBase
	Smoke SmokePlan
}

func (r SmokePlanRequest) Validate() error {
	if err := errors.Join(r.Base.Validate(), r.Smoke.Validate()); err != nil {
		return err
	}
	if r.Base.Egress.Mode != EgressDenied && r.Base.Egress.Mode != EgressPinned {
		return errors.Join(core.ErrPrimitiveContract, errors.New("smoke profile egress mode is outside the closed policy"))
	}
	return nil
}

type ToolPlan struct {
	Timeout temporal.Duration
}

func (p ToolPlan) Validate() error {
	if err := p.Timeout.Validate(); err != nil {
		return err
	}
	if p.Timeout.IsZero() {
		return core.ErrPrimitiveContract
	}
	return nil
}

type ToolPlanRequest struct {
	Base ExternalPlanBase
	Tool ToolPlan
}

func (r ToolPlanRequest) Validate() error {
	if err := errors.Join(r.Base.Validate(), r.Tool.Validate()); err != nil {
		return err
	}
	if r.Base.Egress.Mode != EgressDenied {
		return errors.Join(core.ErrPrimitiveContract, errors.New("tool profile must deny subject network egress"))
	}
	return nil
}

func CompileJavaScriptPlan(request JavaScriptPlanRequest) (ExperimentExecution, error) {
	if err := request.Validate(); err != nil {
		return ExperimentExecution{}, err
	}
	output, report, artifact, err := compileJavaScriptReport(request)
	if err != nil {
		return ExperimentExecution{}, err
	}
	arguments := []string{
		"test", request.Test.File.String(),
		"--reporter=junit",
		"--reporter-outfile=" + report.String(),
		"--max-concurrency=" + strconv.FormatUint(uint64(request.Test.Concurrency), 10),
	}
	return compileExternalExecution(externalCompilation{
		base: request.Base, timeout: request.Test.Timeout, arguments: arguments,
		parallel: request.Test.Concurrency, expectedUnits: request.ExpectedUnits,
		workspaceOutput: output, artifacts: []ArtifactExpectation{artifact},
		observation: ObservationPolicy{Format: ObservationJUnitXML, ExpectedUnits: request.ExpectedUnits, Filtered: true},
	})
}

func CompileSmokePlan(request SmokePlanRequest) (ExperimentExecution, error) {
	if err := request.Validate(); err != nil {
		return ExperimentExecution{}, err
	}
	output, err := request.Base.WorkspaceRoot.JoinRelative(request.Base.OutputDirectory)
	if err != nil {
		return ExperimentExecution{}, err
	}
	return compileExternalExecution(externalCompilation{
		base: request.Base, timeout: request.Smoke.Timeout,
		arguments: []string{"--suite=" + request.Smoke.Suite.String()}, parallel: 1, expectedUnits: 1,
		workspaceOutput: output, artifacts: []ArtifactExpectation{}, observation: ObservationPolicy{Format: ObservationOpaque},
	})
}

func CompileToolPlan(request ToolPlanRequest) (ExperimentExecution, error) {
	if err := request.Validate(); err != nil {
		return ExperimentExecution{}, err
	}
	output, err := request.Base.WorkspaceRoot.JoinRelative(request.Base.OutputDirectory)
	if err != nil {
		return ExperimentExecution{}, err
	}
	return compileExternalExecution(externalCompilation{
		base: request.Base, timeout: request.Tool.Timeout, arguments: []string{}, parallel: 1, expectedUnits: 1,
		workspaceOutput: output, artifacts: []ArtifactExpectation{}, observation: ObservationPolicy{Format: ObservationOpaque},
	})
}

type externalCompilation struct {
	base            ExternalPlanBase
	timeout         temporal.Duration
	arguments       []string
	parallel        uint16
	expectedUnits   uint32
	workspaceOutput core.AbsolutePath
	artifacts       []ArtifactExpectation
	observation     ObservationPolicy
}

func compileExternalExecution(compilation externalCompilation) (ExperimentExecution, error) {
	arguments, argumentsErr := process.ParseArguments(compilation.arguments)
	environment, environmentErr := externalEnvironment(compilation.base.Environment)
	budget, budgetErr := NewExecutionBudget(compilation.timeout, compilation.expectedUnits, compilation.parallel)
	if err := errors.Join(argumentsErr, environmentErr, budgetErr); err != nil {
		return ExperimentExecution{}, err
	}
	plan := process.Plan{
		SchemaVersion: process.ExecutionPlanSchemaVersion,
		Command:       compilation.base.Command, WorkingDirectory: compilation.base.WorkingDirectory,
		Arguments: arguments, Environment: environment, OutputLimit: compilation.base.OutputLimit,
		WaitDelay:   compilation.base.WaitDelay,
		Containment: process.Containment{Isolation: process.IsolationGroup, CancelSignal: process.CancelSignalTerminate},
	}
	workspace := WritableWorkspace{
		Root: compilation.base.WorkspaceRoot, Home: compilation.base.Environment.Home,
		Output: compilation.workspaceOutput, Cache: compilation.base.Environment.Cache,
		Temporary: compilation.base.Environment.Temporary,
	}
	result := ExperimentExecution{Process: plan, Workspace: workspace, Subject: compilation.base.Subject, Artifacts: compilation.artifacts, Observation: compilation.observation, Budget: budget}
	return result, result.Validate()
}

func compileJavaScriptReport(request JavaScriptPlanRequest) (core.AbsolutePath, core.AbsolutePath, ArtifactExpectation, error) {
	output, outputErr := request.Base.WorkspaceRoot.JoinRelative(request.Base.OutputDirectory)
	name, nameErr := core.ParsePathComponent(request.Test.Report.String())
	relative, relativeErr := request.Base.OutputDirectory.Join(name)
	report, reportErr := request.Base.WorkspaceRoot.JoinRelative(relative)
	protocolPath, protocolErr := projectstandards.ParseSourcePath(relative.String())
	mediaType, mediaErr := core.ParseHTTPMediaType("application/xml")
	if err := errors.Join(outputErr, nameErr, relativeErr, reportErr, protocolErr, mediaErr); err != nil {
		return core.AbsolutePath{}, core.AbsolutePath{}, ArtifactExpectation{}, err
	}
	artifact := ArtifactExpectation{Kind: ArtifactReport, Path: protocolPath, MediaType: mediaType, MaximumBytes: request.ArtifactLimit, Required: true}
	return output, report, artifact, artifact.Validate()
}

func externalEnvironment(environment ExternalExecutionEnvironment) (process.Environment, error) {
	values := []string{
		core.EnvironmentHomeName + "=" + environment.Home.String(),
		core.EnvironmentTemporaryName + "=" + environment.Temporary.String(),
		core.EnvironmentCacheName + "=" + environment.Cache.String(),
		"BUN_INSTALL_CACHE_DIR=" + environment.Cache.String(),
	}
	return process.ParseExactEnvironment(values)
}

var (
	_ core.Validatable = ExternalExecutionEnvironment{}
	_ core.Validatable = ExternalPlanBase{}
	_ core.Validatable = JavaScriptTestPlan{}
	_ core.Validatable = JavaScriptPlanRequest{}
	_ core.Validatable = SmokePlan{}
	_ core.Validatable = SmokePlanRequest{}
	_ core.Validatable = ToolPlan{}
	_ core.Validatable = ToolPlanRequest{}
)
