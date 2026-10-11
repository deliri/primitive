package gotoolchain

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// PackageVisitRequest lends a synchronous consumer to one cmd/go selection.
// Each observation is delivered in cmd/go order. Primitive retains no catalog.
type PackageVisitRequest struct {
	Selection ListRequest
	Visit     func(context.Context, Package) error
}

func (r PackageVisitRequest) Validate() error {
	if r.Visit == nil {
		return core.ErrGoToolchainContract
	}
	return r.Selection.Validate()
}

// VisitPackages decodes directly from the contained child's stdout. The source
// and consumer own termination; encoded extent and package count impose no quota.
// The callback must consume each observation before returning. Go owns the
// current JSON value's scalar storage and decoder buffers.
func (c Capability) VisitPackages(ctx context.Context, request PackageVisitRequest) (err error) {
	if ctx == nil {
		return core.ErrGoToolchainContract
	}
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	values := []string{goListSubcommand, "-json=ImportPath,Name,Standard,Module,Incomplete,Error"}
	if request.Selection.Dependencies {
		values = append(values, goDependenciesArgument)
	}
	values = append(values, "--", request.Selection.Pattern)
	arguments, err := process.ParseArguments(values)
	if err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	reader, writer := io.Pipe()
	completed := make(chan error, 1)
	go func() {
		result, runErr := runToolchainGroup(work, process.Request{
			Command: c.command, WorkingDirectory: request.Selection.WorkingDirectory, Arguments: arguments, Environment: c.environment,
			Streams:      process.Streams{Stdin: bytes.NewReader(nil), Stdout: writer, Stderr: io.Discard},
			OutputPolicy: process.OutputPolicy{Mode: process.OutputModeStreaming}, WaitDelay: c.configuration.Limits.WaitDelay,
			Containment: process.Containment{Isolation: process.IsolationGroup, CancelSignal: process.CancelSignalTerminate},
		})
		if runErr == nil {
			exit, exitErr := result.ExitCode()
			success, successErr := exit.Success()
			if exitErr != nil || successErr != nil || !success {
				runErr = errors.Join(core.ErrGoToolchainExecution, exitErr, successErr)
			}
		}
		completed <- errors.Join(runErr, writer.CloseWithError(runErr))
	}()
	defer func() {
		cancel()
		err = errors.Join(err, reader.Close(), <-completed)
	}()
	return visitPackageStream(work, reader, request.Visit)
}

func visitPackageStream(ctx context.Context, source io.Reader, visit func(context.Context, Package) error) error {
	if ctx == nil || core.ReaderIsNil(source) || visit == nil {
		return core.ErrGoToolchainContract
	}
	decoder := jsontext.NewDecoder(source)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var wire packageWire
		err := json.UnmarshalDecode(decoder, &wire)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return errors.Join(core.ErrGoToolchainOutput, err)
		}
		observed, err := packageFromWire(wire)
		if err != nil {
			return err
		}
		if err := visit(ctx, observed); err != nil {
			return err
		}
	}
}
