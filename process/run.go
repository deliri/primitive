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
	execution, err := Begin(ctx, request)
	if err != nil {
		return Result{}, err
	}
	return execution.Wait()
}

// Begin starts one contained direct child and hands back the running
// execution. The caller owns exactly one obligation from that moment: call
// Wait, on every path, so the child is reaped. Signaling and termination
// remain available while the wait is in flight, which is the entire reason
// Begin exists apart from Run.
func Begin(ctx context.Context, request Request) (*Execution, error) {
	if err := validateRun(ctx, request); err != nil {
		return nil, err
	}
	return beginValidated(ctx, request)
}

func validateRun(ctx context.Context, request Request) error {
	if err := contextstate.Validate(ctx); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	return request.Validate()
}

func beginValidated(ctx context.Context, request Request) (*Execution, error) {
	runContext, cancel := context.WithCancelCause(ctx)
	failures := &streamFailures{cancel: cancel}
	streams := newCommandStreams(request, failures)
	prepared, err := prepareCommand(runContext, request, streams)
	if err != nil {
		cancel(nil)
		return nil, err
	}
	identity := ProcessIdentity(prepared.command.Process.Pid) // #nosec G115 -- a started child's identifier fits the platform pid domain.
	if err := identity.Validate(); err != nil {
		cancel(nil)
		return nil, err
	}
	return &Execution{
		prepared:    prepared,
		streams:     streams,
		failures:    failures,
		cancel:      cancel,
		parent:      ctx,
		commandPath: request.Command,
		containment: request.Containment.orDefault(),
		identity:    identity,
	}, nil
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
	containment := request.Containment.orDefault()
	if err := applyContainment(command, containment); err != nil {
		return preparedCommand{}, err
	}
	cancellation := &atomic.Bool{}
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		err := deliverSignal(signalDelivery{
			process:     command.Process,
			identity:    ProcessIdentity(command.Process.Pid), // #nosec G115 -- a started child's identifier fits the platform pid domain.
			containment: containment,
			signal:      containment.CancelSignal,
		})
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
		return preparedCommand{}, newFailure(FailureKindStart, request.Command, err)
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
		return Result{}, newFailure(FailureKindWait, request.commandPath, resultErr)
	}
	if streamErr := request.failures.joined(); streamErr != nil {
		return result, streamErr
	}
	if request.prepared.cancellation.Load() {
		return result, newFailure(
			FailureKindWait,
			request.commandPath,
			errors.Join(request.parent.Err(), waitErr),
		)
	}
	_, exited := errors.AsType[*exec.ExitError](waitErr)
	if waitErr == nil || exited {
		return result, nil
	}
	return result, newFailure(FailureKindWait, request.commandPath, waitErr)
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
	signal, signalReported := observedTerminationSignal(state)
	return Result{
		exit:           exit,
		cpu:            cpu,
		stdinBytes:     stdinBytes,
		stdoutBytes:    stdoutBytes,
		stderrBytes:    stderrBytes,
		signal:         signal,
		signalReported: signalReported,
		set:            true,
	}, nil
}
