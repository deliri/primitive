package gotoolchain

import (
	"bytes"
	"context"
	jsontext "encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/standard"
)

// Capability is one resolved cmd/go execution boundary.
type Capability struct {
	command       core.AbsolutePath
	environment   process.Environment
	configuration Configuration
}

// Open resolves cmd/go and captures the exact child environment once.
func Open(ctx context.Context, configuration Configuration) (Capability, error) {
	if err := configuration.Validate(); err != nil {
		return Capability{}, err
	}
	name, err := core.ParsePathComponent("go")
	if err != nil {
		return Capability{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	command, err := process.Resolve(ctx, name)
	if err != nil {
		return Capability{}, errors.Join(core.ErrGoToolchainExecution, err)
	}
	environment, err := hostfacts.AmbientEnvironment()
	if err != nil {
		return Capability{}, errors.Join(core.ErrGoToolchainExecution, err)
	}
	if configuration.Workspace == WorkspaceModeDisabled {
		environment, err = disableWorkspace(environment)
		if err != nil {
			return Capability{}, err
		}
	}
	capability := Capability{command: command, environment: environment, configuration: configuration}
	if err := capability.Validate(); err != nil {
		return Capability{}, err
	}
	return capability, nil
}

func (c Capability) Validate() error {
	return errors.Join(c.command.Validate(), c.environment.Validate(), c.configuration.Validate())
}

// ObserveModule returns the exact main module selected by cmd/go.
func (c Capability) ObserveModule(ctx context.Context, request ObservationRequest) (gomodule.Path, error) {
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return gomodule.Path{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	output, _, err := c.execute(ctx, request.WorkingDirectory, goListSubcommand, "-m", "-f={{.Path}}")
	if err != nil {
		return gomodule.Path{}, err
	}
	value, ok := strings.CutSuffix(string(output), "\n")
	if !ok || strings.ContainsAny(value, "\r\n") {
		return gomodule.Path{}, outputError("module observation is not one canonical line", nil)
	}
	path, err := gomodule.ParsePath(value)
	if err != nil {
		return gomodule.Path{}, outputError("module observation is invalid", err)
	}
	return path, nil
}

// ObserveBuildContext returns cmd/go's selected platform, version, and cgo fact.
func (c Capability) ObserveBuildContext(ctx context.Context, request ObservationRequest) (BuildContext, error) {
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return BuildContext{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	output, _, err := c.execute(ctx, request.WorkingDirectory, "env", standard.GoJSONOutputArgument, "GOOS", "GOARCH", "GOVERSION", "CGO_ENABLED")
	if err != nil {
		return BuildContext{}, err
	}
	return decodeBuildContext(output)
}

// ListPackages returns a bounded canonical package catalog from cmd/go.
func (c Capability) ListPackages(ctx context.Context, request ListRequest) (PackageCatalog, error) {
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return PackageCatalog{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	arguments := []string{goListSubcommand, standard.GoJSONOutputArgument}
	if request.Dependencies {
		arguments = append(arguments, "-deps")
	}
	arguments = append(arguments, "--", request.Pattern)
	output, _, err := c.execute(ctx, request.WorkingDirectory, arguments...)
	if err != nil {
		return PackageCatalog{}, err
	}
	return decodePackageCatalog(output, c.configuration.Limits.PackageMaximum)
}

// CompilePackage compiles one package and its tests without executing them.
func (c Capability) CompilePackage(ctx context.Context, request CompileRequest) (Compilation, error) {
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return Compilation{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	discard, err := process.DiscardDeviceArgument()
	if err != nil {
		return Compilation{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	discardValue, err := discard.Value()
	if err != nil {
		return Compilation{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	_, result, err := c.execute(ctx, request.WorkingDirectory, standard.GoTestText, "-c", "-o", discardValue, request.Pattern)
	if err != nil {
		return Compilation{}, err
	}
	compilation := Compilation{Result: result}
	return compilation, compilation.Validate()
}

func (c Capability) execute(ctx context.Context, directory core.AbsolutePath, values ...string) ([]byte, process.Result, error) {
	arguments, err := process.ParseArguments(values)
	if err != nil {
		return nil, process.Result{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	request := process.Request{
		Streams: process.Streams{Stdin: bytes.NewReader(nil), Stdout: &stdout, Stderr: &stderr},
		Command: c.command, WorkingDirectory: directory, Arguments: arguments, Environment: c.environment,
		OutputLimit: c.configuration.Limits.OutputBytes, WaitDelay: c.configuration.Limits.WaitDelay,
		Containment: process.Containment{Isolation: process.IsolationGroup, CancelSignal: process.CancelSignalTerminate},
	}
	result, runErr := runToolchainGroup(ctx, request)
	if runErr != nil {
		return nil, result, errors.Join(core.ErrGoToolchainExecution, runErr)
	}
	exit, err := result.ExitCode()
	if err != nil {
		return nil, result, errors.Join(core.ErrGoToolchainExecution, err)
	}
	success, err := exit.Success()
	if err != nil {
		return nil, result, errors.Join(core.ErrGoToolchainExecution, err)
	}
	if !success {
		diagnostic := strings.TrimSpace(stderr.String())
		return nil, result, executionError(diagnostic)
	}
	return bytes.Clone(stdout.Bytes()), result, nil
}

func runToolchainGroup(ctx context.Context, request process.Request) (process.Result, error) {
	execution, err := process.Begin(ctx, request)
	if err != nil {
		return process.Result{}, err
	}
	result, waitErr := execution.Wait()
	if waitErr == nil {
		return result, nil
	}
	return result, errors.Join(waitErr, execution.Sweep())
}

type buildContextWire struct {
	GOOS       string `json:"GOOS"`
	GOARCH     string `json:"GOARCH"`
	GOVERSION  string `json:"GOVERSION"`
	CGOEnabled string `json:"CGO_ENABLED"`
}

func decodeBuildContext(data []byte) (BuildContext, error) {
	wire, err := core.DecodeStrictJSONStructure[buildContextWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return BuildContext{}, outputError("build context JSON is invalid", err)
	}
	var platform core.Platform
	if err := platform.UnmarshalText([]byte(wire.GOOS + "-" + wire.GOARCH)); err != nil {
		return BuildContext{}, outputError("build context platform is invalid", err)
	}
	version, err := ParseToolchainVersion(wire.GOVERSION)
	if err != nil {
		return BuildContext{}, outputError("build context toolchain is invalid", err)
	}
	if wire.CGOEnabled != "0" && wire.CGOEnabled != "1" {
		return BuildContext{}, outputError("build context cgo value is invalid", nil)
	}
	context := BuildContext{Platform: platform, Toolchain: version, CGOEnabled: wire.CGOEnabled == "1"}
	return context, context.Validate()
}

type packageWire struct {
	Error      *struct{}   `json:"Error"`
	Module     *moduleWire `json:"Module"`
	ImportPath string      `json:"ImportPath"`
	Name       string      `json:"Name"`
	Standard   bool        `json:"Standard"`
	Incomplete bool        `json:"Incomplete"`
}

type moduleWire struct {
	Path string `json:"Path"`
}

func decodePackageCatalog(data []byte, maximum uint32) (PackageCatalog, error) {
	if maximum == 0 || maximum > PackageMaximumCount {
		return PackageCatalog{}, outputError("package stream count bound is invalid", nil)
	}
	limit := int(maximum)
	decoder := jsontext.NewDecoder(bytes.NewReader(data))
	packages := make([]Package, 0)
	for len(packages) < limit {
		var wire packageWire
		err := json.UnmarshalDecode(decoder, &wire)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return PackageCatalog{}, outputError("package stream JSON is invalid", err)
		}
		observed, err := packageFromWire(wire)
		if err != nil {
			return PackageCatalog{}, err
		}
		packages = append(packages, observed)
	}
	if len(packages) == limit {
		var extra packageWire
		if err := json.UnmarshalDecode(decoder, &extra); !errors.Is(err, io.EOF) {
			return PackageCatalog{}, outputError("package stream exceeds its count bound", err)
		}
	}
	slices.SortFunc(packages, func(first, second Package) int {
		return strings.Compare(first.ImportPath.String(), second.ImportPath.String())
	})
	catalog := PackageCatalog{Packages: packages}
	return catalog, catalog.Validate()
}

func packageFromWire(wire packageWire) (Package, error) {
	if wire.Incomplete || wire.Error != nil {
		return Package{}, outputError("cmd/go reported an incomplete package", nil)
	}
	path, err := gomodule.ParseImportPath(wire.ImportPath)
	if err != nil {
		return Package{}, outputError("package import path is invalid", err)
	}
	name, err := ParsePackageName(wire.Name)
	if err != nil {
		return Package{}, outputError("package name is invalid", err)
	}
	observed := Package{ImportPath: path, Name: name, Standard: wire.Standard}
	if wire.Module != nil {
		module, parseErr := gomodule.ParsePath(wire.Module.Path)
		if parseErr != nil {
			return Package{}, outputError("package module is invalid", parseErr)
		}
		observed.Module = &module
	}
	return observed, observed.Validate()
}

func disableWorkspace(environment process.Environment) (process.Environment, error) {
	name, err := process.NewEnvironmentName("GOWORK")
	if err != nil {
		return process.Environment{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	value, err := process.NewEnvironmentValue("off")
	if err != nil {
		return process.Environment{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	variables := slices.Clone(environment.Variables)
	replacement := process.EnvironmentVariable{Name: name, Value: value}
	found := false
	for index := range variables {
		if variables[index].Name == name {
			variables[index] = replacement
			found = true
		}
	}
	if !found {
		variables = append(variables, replacement)
	}
	result := process.Environment{Mode: process.EnvironmentModeExact, Variables: variables}
	if err := result.Validate(); err != nil {
		return process.Environment{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	return result, nil
}

func contractError(message string) error {
	return errors.Join(core.ErrGoToolchainContract, errors.New(message))
}

func outputError(message string, cause error) error {
	return errors.Join(core.ErrGoToolchainOutput, errors.New(message), cause)
}

func executionError(diagnostic string) error {
	if diagnostic == "" {
		return errors.Join(core.ErrGoToolchainExecution, errors.New("cmd/go failed without a diagnostic"))
	}
	return errors.Join(core.ErrGoToolchainExecution, fmt.Errorf("cmd/go: %s", diagnostic))
}
