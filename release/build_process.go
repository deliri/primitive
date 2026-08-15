package release

import (
	"errors"
	"os"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	goPathEnvironmentName            = "PATH"
	goEnvironmentNameCGO             = "CGO_ENABLED"
	goEnvironmentNameOS              = "GOOS"
	goEnvironmentNameArchitecture    = "GOARCH"
	goEnvironmentNameToolchain       = "GOTOOLCHAIN"
	goEnvironmentNameAMD64           = "GOAMD64"
	goEnvironmentNameARM64           = "GOARM64"
	goEnvironmentNameConfiguration   = "GOENV"
	goEnvironmentNameFlags           = "GOFLAGS"
	goEnvironmentNameExperiment      = "GOEXPERIMENT"
	goEnvironmentNameFIPS            = "GOFIPS140"
	goEnvironmentNameWorkspace       = "GOWORK"
	goToolDirectoryInvalidDiagnostic = "go tool directory is invalid"
)

// BuildProcessRequest supplies host execution facts around one exact build
// command. HostEnvironment must be exact; ambient inheritance is never a
// release input.
type BuildProcessRequest struct {
	Streams          process.Streams
	WorkingDirectory core.AbsolutePath
	HostEnvironment  process.Environment
	Repository       VerifiedRepository
	Tools            VerifiedBuildTools
	Command          BuildCommand
	OutputLimit      core.ByteCount
	WaitDelay        temporal.Duration
}

// Validate proves every host and command fact before process lowering.
func (r BuildProcessRequest) Validate() error {
	_, err := prepareBuildProcess(r)
	return err
}

// PrepareBuildProcess lowers one command into the generic typed process
// boundary without starting it.
func PrepareBuildProcess(request BuildProcessRequest) (process.Request, error) {
	if err := request.Validate(); err != nil {
		return process.Request{}, err
	}
	return prepareBuildProcess(request)
}

func prepareBuildProcess(request BuildProcessRequest) (process.Request, error) {
	if err := validateBuildProcessCapabilityBinding(request); err != nil {
		return process.Request{}, err
	}
	if err := request.WorkingDirectory.Validate(); err != nil {
		return process.Request{}, contractError(errors.New("build working directory is invalid"), err)
	}
	if request.WorkingDirectory != request.Repository.Root() {
		return process.Request{}, contractError(errors.New("build working directory differs from the verified repository"))
	}
	searchPath, err := verifiedBuildSearchPath(request.Tools, request.Repository)
	if err != nil {
		return process.Request{}, err
	}
	environment, err := prepareBuildEnvironment(
		request.HostEnvironment,
		searchPath,
		request.Command,
	)
	if err != nil {
		return process.Request{}, err
	}
	values, err := request.Command.ArgumentValues()
	if err != nil {
		return process.Request{}, err
	}
	arguments, err := process.ParseArguments(values)
	if err != nil {
		return process.Request{}, contractError(errors.New("build arguments are invalid"), err)
	}
	prepared := process.Request{
		Streams: request.Streams, Command: request.Tools.GarbleExecutable(),
		WorkingDirectory: request.WorkingDirectory, Arguments: arguments,
		Environment: environment, OutputLimit: request.OutputLimit,
		WaitDelay: request.WaitDelay,
		Containment: process.Containment{
			Isolation:    process.IsolationDirect,
			CancelSignal: process.CancelSignalKill,
		},
	}
	if err := prepared.Validate(); err != nil {
		return process.Request{}, contractError(errors.New("build process request is invalid"), err)
	}
	return prepared, nil
}

func validateBuildProcessCapabilityBinding(request BuildProcessRequest) error {
	if err := request.Command.Validate(); err != nil {
		return contractError(errors.New("build process command is invalid"), err)
	}
	if err := request.Repository.Validate(); err != nil {
		return contractError(errors.New("build repository is not verified"), err)
	}
	if request.Command.Build().Commit() != request.Repository.Commit() {
		return contractError(errors.New("build command commit differs from the verified repository"))
	}
	if err := request.Tools.Validate(); err != nil {
		return contractError(errors.New("build tools are not verified"), err)
	}
	return nil
}

func prepareBuildEnvironment(
	host process.Environment,
	searchPath string,
	command BuildCommand,
) (process.Environment, error) {
	if searchPath == "" {
		return process.Environment{}, contractError(errors.New("build search path is empty"))
	}
	values, err := host.Strings()
	if err != nil {
		return process.Environment{}, contractError(errors.New("host build environment is invalid"), err)
	}
	if values == nil {
		return process.Environment{}, contractError(errors.New("host build environment inherits ambient state"))
	}
	filtered := make([]string, 0, len(values)+11)
	for _, value := range values {
		name, _, _ := strings.Cut(value, "=")
		if buildControlledEnvironmentName(name) {
			continue
		}
		filtered = append(filtered, value)
	}
	overrides, err := command.EnvironmentOverrides()
	if err != nil {
		return process.Environment{}, err
	}
	filtered = append(filtered, goPathEnvironmentName+"="+searchPath)
	filtered = append(filtered, overrides...)
	prepared, err := process.ParseExactEnvironment(filtered)
	if err != nil {
		return process.Environment{}, contractError(errors.New("exact build environment is invalid"), err)
	}
	return prepared, nil
}

// composeExactSearchPath projects one exact PATH value from already-validated
// directories. Consecutive duplicates collapse so Go and Git in one directory
// do not invent a second search entry.
func verifiedBuildSearchPath(tools VerifiedBuildTools, repository VerifiedRepository) (string, error) {
	goToolDirectory, err := tools.GoExecutable().Parent()
	if err != nil {
		return "", contractError(errors.New(goToolDirectoryInvalidDiagnostic), err)
	}
	gitToolDirectory, err := repository.GitExecutable().Parent()
	if err != nil {
		return "", contractError(errors.New("git tool directory is invalid"), err)
	}
	return composeExactSearchPath([]core.AbsolutePath{goToolDirectory, gitToolDirectory})
}

func composeExactSearchPath(directories []core.AbsolutePath) (string, error) {
	if len(directories) == 0 {
		return "", contractError(errors.New("build search path has no directories"))
	}
	parts := make([]string, 0, len(directories))
	for _, directory := range directories {
		if err := directory.Validate(); err != nil {
			return "", contractError(errors.New("build search path directory is invalid"), err)
		}
		next := directory.String()
		if len(parts) > 0 && parts[len(parts)-1] == next {
			continue
		}
		parts = append(parts, next)
	}
	return strings.Join(parts, string(os.PathListSeparator)), nil
}

func buildControlledEnvironmentNames() []string {
	return []string{
		goPathEnvironmentName,
		goEnvironmentNameCGO,
		goEnvironmentNameOS,
		goEnvironmentNameArchitecture,
		goEnvironmentNameToolchain,
		goEnvironmentNameAMD64,
		goEnvironmentNameARM64,
		goEnvironmentNameConfiguration,
		goEnvironmentNameFlags,
		goEnvironmentNameExperiment,
		goEnvironmentNameFIPS,
		goEnvironmentNameWorkspace,
	}
}

func buildControlledEnvironmentName(name string) bool {
	for _, controlled := range buildControlledEnvironmentNames() {
		if controlled == name {
			return true
		}
	}
	return false
}
