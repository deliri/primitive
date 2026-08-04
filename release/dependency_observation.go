package release

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	buildDependencyObservationMaximumBytes = 64 << 20
	buildPackageObservationMaximumCount    = 65_536
	goListSubcommand                       = "list"
	goListDependenciesFlag                 = "-deps"
	goListJSONFlag                         = "-json"
)

// BuildDependencyObservationRequest binds dependency observation to the exact
// verified tool, build plan, target environments, and repository root.
type BuildDependencyObservationRequest struct {
	Stderr           io.Writer
	WorkingDirectory core.AbsolutePath
	HostEnvironment  process.Environment
	Tools            VerifiedBuildTools
	Plan             BuildPlan
	WaitDelay        temporal.Duration
}

func (r BuildDependencyObservationRequest) Validate() error {
	if r.Stderr == nil {
		return contractError(errors.New("build dependency stderr is nil"))
	}
	for _, err := range [...]error{
		r.WorkingDirectory.Validate(), r.HostEnvironment.Validate(), r.Tools.Validate(),
		r.Plan.Validate(), r.WaitDelay.Validate(),
	} {
		if err != nil {
			return contractError(errors.New("build dependency observation request is invalid"), err)
		}
	}
	if r.WaitDelay.IsZero() {
		return contractError(errors.New("build dependency wait delay is zero"))
	}
	return r.Plan.request.ModuleMode.validateChecksumObservable()
}

type goListModuleWire struct {
	Replace *goListModuleWire
	Path    string
	Version string
	Sum     string
	Main    bool
}

type goListErrorWire struct {
	Err string
}

type goListPackageWire struct {
	Module     *goListModuleWire
	Error      *goListErrorWire
	ImportPath string
	Standard   bool
	Incomplete bool
}

type dependencyObservation struct {
	main    GoModulePath
	modules [BuildDependencyMaximumCount]BuildDependency
	count   int
}

type dependencyProcessOutcome struct {
	err    error
	result process.Result
}

// ObserveBuildDependencies streams cmd/go's exact package closure for every
// canonical target and returns the fixed, path-sorted module union.
func ObserveBuildDependencies(
	ctx context.Context,
	request BuildDependencyObservationRequest,
) (BuildDependencies, error) {
	if ctx == nil {
		return BuildDependencies{}, contractError(core.ErrNilContext)
	}
	if err := ctx.Err(); err != nil {
		return BuildDependencies{}, contractError(err)
	}
	if err := request.Validate(); err != nil {
		return BuildDependencies{}, err
	}
	combined, observed := &dependencyObservation{}, &dependencyObservation{}
	for index := range TargetCount {
		command, ok := request.Plan.At(index)
		if !ok {
			return BuildDependencies{}, contractError(errors.New("build dependency command slot is invalid"))
		}
		if err := observeBuildCommandDependencies(ctx, request, command, observed); err != nil {
			return BuildDependencies{}, err
		}
		if err := combined.merge(observed); err != nil {
			return BuildDependencies{}, err
		}
	}
	modules := append([]BuildDependency(nil), combined.modules[:combined.count]...)
	return newBuildDependencies(combined.main, request.Tools.GoToolchain(), modules)
}

// observeBuildCommandDependencies fills observed with one target's closure. The
// fixed observation storage is reused across targets, so every failure path
// leaves it zeroed rather than partially populated.
func observeBuildCommandDependencies(
	ctx context.Context,
	request BuildDependencyObservationRequest,
	command BuildCommand,
	observed *dependencyObservation,
) error {
	if err := observeBuildCommandStream(ctx, request, command, observed); err != nil {
		*observed = dependencyObservation{}
		return err
	}
	return nil
}

func observeBuildCommandStream(
	ctx context.Context,
	request BuildDependencyObservationRequest,
	command BuildCommand,
	observed *dependencyObservation,
) error {
	reader, writer := io.Pipe()
	childContext, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan dependencyProcessOutcome, 1)
	go func() {
		result, err := runBuildDependencyProcess(childContext, request, command, writer)
		_ = writer.CloseWithError(err)
		done <- dependencyProcessOutcome{result: result, err: err}
	}()
	decodeErr := decodeBuildDependencies(reader, observed)
	if decodeErr != nil {
		cancel()
		_ = reader.CloseWithError(decodeErr)
	}
	outcome := <-done
	closeErr := reader.Close()
	processErr := outcome.err
	if processErr == nil {
		processErr = validateDependencyProcessExit(outcome.result)
	}
	if err := errors.Join(decodeErr, processErr, closeErr); err != nil {
		return contractError(errors.New("observe build dependencies"), err)
	}
	return nil
}

func validateDependencyProcessExit(result process.Result) error {
	exit, err := result.ExitCode()
	if err != nil {
		return contractError(errors.New("build dependency exit is unavailable"), err)
	}
	success, err := exit.Success()
	if err != nil || !success {
		return contractError(errors.New("build dependency process failed"), err)
	}
	return nil
}

func runBuildDependencyProcess(
	ctx context.Context,
	request BuildDependencyObservationRequest,
	command BuildCommand,
	stdout io.Writer,
) (process.Result, error) {
	prepared, err := prepareBuildDependencyProcess(request, command, stdout)
	if err != nil {
		return process.Result{}, err
	}
	return process.Run(ctx, prepared)
}

func prepareBuildDependencyProcess(
	request BuildDependencyObservationRequest,
	command BuildCommand,
	stdout io.Writer,
) (process.Request, error) {
	goDirectory, err := request.Tools.GoExecutable().Parent()
	if err != nil {
		return process.Request{}, contractError(errors.New("go tool directory is invalid"), err)
	}
	environment, err := prepareBuildEnvironment(request.HostEnvironment, goDirectory, command)
	if err != nil {
		return process.Request{}, err
	}
	selectors, err := command.closureSelectorArguments()
	if err != nil {
		return process.Request{}, err
	}
	values := append([]string{goListSubcommand, goListDependenciesFlag, goListJSONFlag}, selectors...)
	arguments, err := process.ParseArguments(append(values, command.mainPackage.String()))
	if err != nil {
		return process.Request{}, contractError(errors.New("build dependency arguments are invalid"), err)
	}
	maximum, err := core.NewByteCount(buildDependencyObservationMaximumBytes)
	if err != nil {
		return process.Request{}, err
	}
	prepared := process.Request{
		Streams: process.Streams{Stdin: bytes.NewReader(nil), Stdout: stdout, Stderr: request.Stderr},
		Command: request.Tools.GoExecutable(), WorkingDirectory: request.WorkingDirectory,
		Arguments: arguments, Environment: environment, OutputLimit: maximum, WaitDelay: request.WaitDelay,
	}
	if err := prepared.Validate(); err != nil {
		return process.Request{}, contractError(errors.New("build dependency process is invalid"), err)
	}
	return prepared, nil
}

// decodeBuildDependencies streams one cmd/go package closure into observed and
// leaves observed zeroed when any package in the stream is rejected.
func decodeBuildDependencies(source io.Reader, observed *dependencyObservation) error {
	*observed = dependencyObservation{}
	if err := decodeBuildDependencyStream(source, observed); err != nil {
		*observed = dependencyObservation{}
		return err
	}
	return nil
}

func decodeBuildDependencyStream(source io.Reader, observed *dependencyObservation) error {
	decoder := json.NewDecoder(source)
	for packageCount := 0; ; packageCount++ {
		if packageCount >= buildPackageObservationMaximumCount {
			return contractError(errors.New("build package count exceeds its bound"))
		}
		var wire goListPackageWire
		err := decoder.Decode(&wire)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return contractError(errors.New("decode build dependency package"), err)
		}
		if err := observed.addPackage(wire); err != nil {
			return err
		}
	}
	if err := observed.main.Validate(); err != nil {
		return contractError(errors.New("build dependency main module is absent"), err)
	}
	return nil
}

func (o *dependencyObservation) addPackage(wire goListPackageWire) error {
	if wire.ImportPath == "" || wire.Incomplete || wire.Error != nil {
		return contractError(errors.New("go package observation is incomplete"))
	}
	if wire.Module == nil {
		return validateStandardPackageObservation(wire.Standard)
	}
	if wire.Standard || wire.Module.Replace != nil {
		return contractError(errors.New("go package module is substituted or inconsistent"))
	}
	return o.addPackageModule(*wire.Module)
}

func validateStandardPackageObservation(standard bool) error {
	if !standard {
		return contractError(errors.New("non-standard package has no module"))
	}
	return nil
}

func (o *dependencyObservation) addPackageModule(moduleWire goListModuleWire) error {
	if moduleWire.Main {
		main, err := parseGoModulePath(moduleWire.Path)
		if err != nil {
			return err
		}
		if o.main != (GoModulePath{}) && o.main != main {
			return contractError(errors.New("go package closure has multiple main modules"))
		}
		o.main = main
		return nil
	}
	module, err := newBuildDependency(moduleWire.Path, moduleWire.Version, moduleWire.Sum)
	if err != nil {
		return err
	}
	return o.addModule(module)
}

func (o *dependencyObservation) addModule(module BuildDependency) error {
	index := sort.Search(o.count, func(index int) bool {
		return o.modules[index].path.value >= module.path.value
	})
	if index < o.count && o.modules[index].path == module.path {
		if o.modules[index] != module {
			return contractError(errors.New("go module facts conflict across packages"))
		}
		return nil
	}
	if o.count >= len(o.modules) {
		return contractError(errors.New("build dependency count exceeds its bound"))
	}
	copy(o.modules[index+1:o.count+1], o.modules[index:o.count])
	o.modules[index] = module
	o.count++
	return nil
}

func (o *dependencyObservation) merge(other *dependencyObservation) error {
	if o.main == (GoModulePath{}) {
		o.main = other.main
	}
	if o.main != other.main {
		return contractError(errors.New("target dependency closures have different main modules"))
	}
	for _, module := range other.modules[:other.count] {
		if err := o.addModule(module); err != nil {
			return err
		}
	}
	return nil
}

var _ core.Validatable = BuildDependencyObservationRequest{}
