package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync/atomic"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Run executes, streams, and reaps one direct child. A nonzero or signaled
// child exit is a successful observation in Result, not an infrastructure
// error.
func Run(ctx context.Context, request Request) (Result, error) {
	if err := validateRun(ctx, request); err != nil {
		return Result{}, err
	}
	return runValidated(ctx, request)
}

func validateRun(ctx context.Context, request Request) error {
	if err := contextstate.Validate(ctx); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	return request.Validate()
}

func runValidated(ctx context.Context, request Request) (Result, error) {
	runContext, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	failures := &streamFailures{cancel: cancel}
	streams := newCommandStreams(request, failures)
	prepared, err := prepareCommand(runContext, request, streams)
	if err != nil {
		return Result{}, err
	}
	return waitCommand(waitRequest{
		parent:      ctx,
		commandPath: request.Command,
		prepared:    prepared,
		streams:     streams,
		failures:    failures,
	})
}

type preparedCommand struct {
	command      *exec.Cmd
	cancellation *atomic.Bool
}

func prepareCommand(
	ctx context.Context,
	request Request,
	streams commandStreams,
) (preparedCommand, error) {
	command := exec.CommandContext( // #nosec G204 -- validated typed argv is this package's sole execution boundary.
		ctx,
		request.Command.String(),
		request.projectArguments()...,
	)
	command.Dir = request.WorkingDirectory.String()
	command.Env = request.Environment.project()
	command.Stdin = streams.stdin
	command.Stdout = streams.stdout
	command.Stderr = streams.stderr
	cancellation := &atomic.Bool{}
	command.Cancel = func() error {
		err := command.Process.Kill()
		if err == nil {
			cancellation.Store(true)
		}
		return err
	}
	waitDelay, err := request.WaitDelay.Stdlib()
	if err != nil {
		return preparedCommand{}, errors.Join(core.ErrProcessContract, err)
	}
	command.WaitDelay = waitDelay
	if err := command.Start(); err != nil {
		return preparedCommand{}, Failure{
			Kind: FailureKindStart, Command: request.Command, Cause: err,
		}
	}
	return preparedCommand{
		command: command, cancellation: cancellation,
	}, nil
}

type waitRequest struct {
	streams     commandStreams
	parent      context.Context
	prepared    preparedCommand
	failures    *streamFailures
	commandPath core.AbsolutePath
}

func waitCommand(request waitRequest) (Result, error) {
	waitErr := request.prepared.command.Wait()
	result, resultErr := newResult(
		request.prepared.command.ProcessState,
		request.streams,
	)
	if resultErr != nil {
		return Result{}, Failure{
			Kind: FailureKindWait, Command: request.commandPath, Cause: resultErr,
		}
	}
	if streamErr := request.failures.joined(); streamErr != nil {
		return result, streamErr
	}
	if request.prepared.cancellation.Load() {
		return result, Failure{
			Kind:    FailureKindWait,
			Command: request.commandPath,
			Cause:   errors.Join(request.parent.Err(), waitErr),
		}
	}
	_, exited := errors.AsType[*exec.ExitError](waitErr)
	if waitErr == nil || exited {
		return result, nil
	}
	return result, Failure{
		Kind: FailureKindWait, Command: request.commandPath, Cause: waitErr,
	}
}

func newResult(
	state *os.ProcessState,
	streams commandStreams,
) (Result, error) {
	exit, err := newExitCode(state.ExitCode())
	if err != nil {
		return Result{}, err
	}
	user, err := temporal.NewDuration(state.UserTime())
	if err != nil {
		return Result{}, err
	}
	system, err := temporal.NewDuration(state.SystemTime())
	if err != nil {
		return Result{}, err
	}
	cpu, err := user.Add(system)
	if err != nil {
		return Result{}, err
	}
	stdinBytes, err := core.NewByteLength(streams.stdin.count)
	if err != nil {
		return Result{}, err
	}
	stdoutBytes, err := core.NewByteLength(streams.stdout.count)
	if err != nil {
		return Result{}, err
	}
	stderrBytes, err := core.NewByteLength(streams.stderr.count)
	if err != nil {
		return Result{}, err
	}
	return Result{
		exit:        exit,
		cpu:         cpu,
		stdinBytes:  stdinBytes,
		stdoutBytes: stdoutBytes,
		stderrBytes: stderrBytes,
		set:         true,
	}, nil
}
