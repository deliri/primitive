package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

const (
	gitStatusCommand     = "status"
	gitStatusPorcelainV1 = "--porcelain=v1"
)

// StatusRequest identifies the repository whose native porcelain stream is
// observed. The receipt reports mechanical byte counts, without product meaning.
type StatusRequest struct{ Root core.AbsolutePath }

func (r StatusRequest) Validate() error {
	if err := r.Root.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// StatusObservation retains the native reaped result privately. An absent
// observation cannot be confused with a completed status whose output is empty.
// Process derives byte and exit facts from that one native result.
type StatusObservation struct{ result process.Result }

func (o StatusObservation) Validate() error {
	if _, err := o.result.Observation(); err != nil {
		return contractError(err)
	}
	return requireSuccess(o.result)
}

func (o StatusObservation) Process() (process.ResultObservation, error) {
	if err := o.Validate(); err != nil {
		return process.ResultObservation{}, err
	}
	return o.result.Observation()
}

// ObserveStatus drains Git's native porcelain output to discard. Primitive's
// process receipt counts accepted bytes; neither a path list nor a status body
// is retained. Native read, execution and cancellation refusals return no receipt.
func (c Capability) ObserveStatus(ctx context.Context, request StatusRequest) (StatusObservation, error) {
	if err := errors.Join(contextstate.Validate(ctx), c.Validate(), request.Validate()); err != nil {
		return StatusObservation{}, contractError(err)
	}
	arguments, err := process.ParseArguments([]string{gitNoOptionalLocks, gitConfigOverride, gitNoGlobalExcludes, gitConfigOverride, gitNoFilesystemMonitor, gitStatusCommand, gitStatusPorcelainV1, gitNullTerminate})
	if err != nil {
		return StatusObservation{}, contractError(err)
	}
	result, err := process.Run(ctx, process.Request{
		Command: c.command, WorkingDirectory: request.Root, Arguments: arguments, Environment: c.environment,
		Streams:      process.Streams{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard},
		OutputPolicy: process.OutputPolicy{Mode: process.OutputModeStreaming}, WaitDelay: c.configuration.WaitDelay,
		Containment: process.Containment{Isolation: process.IsolationDirect, CancelSignal: process.CancelSignalTerminate},
	})
	if err != nil {
		return StatusObservation{}, executionError(err)
	}
	if err := requireSuccess(result); err != nil {
		return StatusObservation{}, err
	}
	observation := StatusObservation{result: result}
	if err := observation.Validate(); err != nil {
		return StatusObservation{}, err
	}
	return observation, nil
}
