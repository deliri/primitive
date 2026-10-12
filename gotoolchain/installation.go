package gotoolchain

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/process"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// ModuleVersion names one canonical Go module version, including pseudo versions.
type ModuleVersion struct{ value string }

func ParseModuleVersion(value string) (ModuleVersion, error) {
	if !semver.IsValid(value) || module.CanonicalVersion(value) != value {
		return ModuleVersion{}, core.ErrGoToolchainContract
	}
	return ModuleVersion{value: value}, nil
}
func (v ModuleVersion) String() string        { return v.value }
func (v ModuleVersion) Validate() error       { _, err := ParseModuleVersion(v.value); return err }
func (ModuleVersion) goToolchainSealedValue() {}

type ModuleDownloadRequest struct {
	WorkingDirectory core.AbsolutePath
	Module           gomodule.Path
	Version          ModuleVersion
}

func (r ModuleDownloadRequest) Validate() error {
	if err := errors.Join(r.WorkingDirectory.Validate(), r.Module.Validate(), r.Version.Validate()); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	if err := module.Check(r.Module.String(), r.Version.String()); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	return nil
}
func (ModuleDownloadRequest) goToolchainProtocolFact() {}

type ModuleDownloadObservation struct {
	Directory core.AbsolutePath
	Process   process.ResultObservation
}

func (o ModuleDownloadObservation) Validate() error {
	if err := errors.Join(o.Directory.Validate(), o.Process.Validate()); err != nil {
		return errors.Join(core.ErrGoToolchainOutput, err)
	}
	if o.Process.ExitCode != 0 {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (ModuleDownloadObservation) goToolchainProtocolFact() {}

// CommandExitError preserves the completed native exit observation beside a
// refused command. It never admits a source directory or installation fact.
type CommandExitError struct{ Process process.ResultObservation }

func (e CommandExitError) Error() string {
	return fmt.Sprintf("Go command exited %d", e.Process.ExitCode)
}
func (e CommandExitError) Validate() error {
	if err := e.Process.Validate(); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	if e.Process.ExitCode == 0 {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (e CommandExitError) Unwrap() error          { return core.ErrGoToolchainExecution }
func (CommandExitError) goToolchainInternalFlow() {}

type installationEnvironmentWire struct {
	GOBIN  *string
	GOPATH *string
}

func (installationEnvironmentWire) goToolchainInternalFlow() {}

type moduleDownloadOriginWire struct {
	VCS       string
	URL       string
	Subdir    string
	Hash      string
	TagPrefix string
	TagSum    string
	Ref       string
	RepoSum   string
}

func (moduleDownloadOriginWire) goToolchainInternalFlow() {}

type moduleDownloadWire struct {
	Path     string
	Version  string
	Query    string
	Error    string
	Info     string
	GoMod    string
	Zip      string
	Dir      string
	Sum      string
	GoModSum string
	Origin   *moduleDownloadOriginWire
	Reuse    bool
}

func (moduleDownloadWire) goToolchainInternalFlow() {}

// ObserveInstallationDirectory asks cmd/go for its selected GOBIN and GOPATH.
// Go's install rule selects GOBIN or the first GOPATH entry's bin directory.
func (c Capability) ObserveInstallationDirectory(ctx context.Context, request ObservationRequest) (core.AbsolutePath, error) {
	if err := errors.Join(contextstate.Validate(ctx), c.Validate(), request.Validate()); err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	var directory core.AbsolutePath
	err := c.withCompletedCommandOutput(ctx, request.WorkingDirectory, []string{"env", "-json", "GOBIN", "GOPATH"}, func(source io.Reader, _ process.ResultObservation) error {
		var wire installationEnvironmentWire
		if err := json.UnmarshalRead(source, &wire, json.RejectUnknownMembers(true)); err != nil {
			return errors.Join(core.ErrGoToolchainOutput, err)
		}
		if wire.GOBIN == nil || wire.GOPATH == nil {
			return core.ErrGoToolchainOutput
		}
		value := *wire.GOBIN
		if value == "" {
			first, _, _ := strings.Cut(*wire.GOPATH, string(os.PathListSeparator))
			if first == "" {
				return core.ErrGoToolchainOutput
			}
			value = filepath.Join(first, "bin")
		}
		var err error
		directory, err = core.ParseAbsolutePath(value)
		if err != nil {
			return errors.Join(core.ErrGoToolchainOutput, err)
		}
		return contextstate.Validate(ctx)
	})
	if err != nil {
		return core.AbsolutePath{}, err
	}
	return directory, nil
}

// DownloadModule executes one exact module download and binds its returned
// source directory to the requested module and canonical version.
func (c Capability) DownloadModule(ctx context.Context, request ModuleDownloadRequest) (ModuleDownloadObservation, error) {
	if err := errors.Join(contextstate.Validate(ctx), c.Validate(), request.Validate()); err != nil {
		return ModuleDownloadObservation{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	var observation ModuleDownloadObservation
	err := c.withCompletedCommandOutput(ctx, request.WorkingDirectory, []string{"mod", "download", "-json", request.Module.String() + "@" + request.Version.String()}, func(source io.Reader, receipt process.ResultObservation) error {
		var wire moduleDownloadWire
		if err := json.UnmarshalRead(source, &wire, json.RejectUnknownMembers(true)); err != nil {
			return errors.Join(core.ErrGoToolchainOutput, err)
		}
		if wire.Error != "" || wire.Path != request.Module.String() || wire.Version != request.Version.String() {
			return core.ErrGoToolchainOutput
		}
		directory, err := core.ParseAbsolutePath(wire.Dir)
		if err != nil {
			return errors.Join(core.ErrGoToolchainOutput, err)
		}
		observation = ModuleDownloadObservation{Directory: directory, Process: receipt}
		return errors.Join(observation.Validate(), contextstate.Validate(ctx))
	})
	if err != nil {
		return ModuleDownloadObservation{}, err
	}
	return observation, nil
}

// The native file is borrowed only after Wait and file extent inspection. The
// process and file receipts agree on exact accepted stdout bytes. Scratch grows
// with output; no process-output ceiling or whole transcript buffer is used.
// Native JSON v2 scalar/whitespace retention remains a separate decoder boundary.
func (c Capability) withCompletedCommandOutput(ctx context.Context, directory core.AbsolutePath, values []string, consume func(io.Reader, process.ResultObservation) error) error {
	parent, err := hostfacts.TemporaryDirectory()
	if err != nil {
		return err
	}
	arguments, err := process.ParseArguments(values)
	if err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	return filestore.WithScratchScope(ctx, filestore.ScratchScopeRequest{Parent: parent, Use: func(owner context.Context, root *os.Root) error {
		path, err := core.ParseRelativePath("go-output")
		if err != nil {
			return err
		}
		scope, err := filestore.WithScratchReplayScope(owner, filestore.ScratchReplayScopeRequest{Scratch: filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o600}, Use: func(owner context.Context, stdout filestore.ScratchReplayFile) error {
			result, runErr := runToolchainGroup(ctx, process.Request{Command: c.command, WorkingDirectory: directory, Arguments: arguments, Environment: c.environment, Streams: process.Streams{Stdin: bytes.NewReader(nil), Stdout: stdout, Stderr: io.Discard}, OutputPolicy: process.OutputPolicy{Mode: process.OutputModeStreaming}, WaitDelay: c.configuration.Limits.WaitDelay, Containment: process.Containment{Isolation: process.IsolationGroup, CancelSignal: process.CancelSignalTerminate}})
			if runErr != nil {
				return errors.Join(core.ErrGoToolchainExecution, runErr)
			}
			receipt, err := result.Observation()
			if err != nil {
				return errors.Join(core.ErrGoToolchainExecution, err)
			}
			if receipt.ExitCode != 0 {
				return CommandExitError{Process: receipt}
			}
			extent, err := filestore.InspectOpenFile(owner, filestore.HandleInspectionRequest{File: stdout})
			if err != nil {
				return err
			}
			lengthValue, err := extent.SizeBytes()
			if err != nil {
				return err
			}
			length, err := lengthValue.Int64()
			if err != nil {
				return err
			}
			if receipt.StdoutBytes.Uint64() != uint64(length) {
				return core.ErrGoToolchainOutput
			}
			return consume(io.NewSectionReader(stdout, 0, length), receipt)
		}})
		return errors.Join(err, scope.Validate(), scope.OperationError(), scope.CleanupError())
	}})
}
