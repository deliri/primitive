package release

import (
	"errors"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

const goPathEnvironmentName = "PATH"

// BuildProcessRequest supplies host execution facts around one exact build
// command. HostEnvironment must be exact; ambient inheritance is never a
// release input.
type BuildProcessRequest struct {
	Streams          process.Streams
	WorkingDirectory core.AbsolutePath
	HostEnvironment  process.Environment
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
	if err := validateBuildProcessToolBinding(request); err != nil {
		return process.Request{}, err
	}
	if err := request.WorkingDirectory.Validate(); err != nil {
		return process.Request{}, contractError(errors.New("build working directory is invalid"), err)
	}
	goToolDirectory, err := request.Tools.GoExecutable().Parent()
	if err != nil {
		return process.Request{}, contractError(errors.New("go tool directory is invalid"), err)
	}
	environment, err := prepareBuildEnvironment(
		request.HostEnvironment,
		goToolDirectory,
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
	}
	if err := prepared.Validate(); err != nil {
		return process.Request{}, contractError(errors.New("build process request is invalid"), err)
	}
	return prepared, nil
}

func validateBuildProcessToolBinding(request BuildProcessRequest) error {
	if err := request.Command.Validate(); err != nil {
		return contractError(errors.New("build process command is invalid"), err)
	}
	if err := request.Tools.Validate(); err != nil {
		return contractError(errors.New("build tools are not verified"), err)
	}
	return nil
}

func prepareBuildEnvironment(
	host process.Environment,
	goToolDirectory core.AbsolutePath,
	command BuildCommand,
) (process.Environment, error) {
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
	filtered = append(filtered, goPathEnvironmentName+"="+goToolDirectory.String())
	filtered = append(filtered, overrides...)
	prepared, err := process.ParseExactEnvironment(filtered)
	if err != nil {
		return process.Environment{}, contractError(errors.New("exact build environment is invalid"), err)
	}
	return prepared, nil
}

func buildControlledEnvironmentName(name string) bool {
	return name == goPathEnvironmentName || name == "CGO_ENABLED" || strings.HasPrefix(name, "GO")
}
