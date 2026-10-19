//go:build unix

package process

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const groupObservationExecutable = "/bin/ps"
const groupMemoryFields = "pgid=,rss="
const groupCPUFields = "pgid=,pcpu="
const groupObservationWaitSeconds = 3

func observeGroupMemory(ctx context.Context, request GroupObservationRequest) (core.ByteLength, error) {
	fold := groupMemoryFold{target: request.Group}
	if err := observeGroupRows(ctx, request, groupMemoryFields, &fold); err != nil {
		return core.ByteLength{}, err
	}
	return fold.finish()
}

func observeGroupCPU(ctx context.Context, request GroupObservationRequest) (uint64, error) {
	fold := groupCPUFold{target: request.Group}
	if err := observeGroupRows(ctx, request, groupCPUFields, &fold); err != nil {
		return 0, err
	}
	return fold.finish()
}

// observeGroupRows streams one native ps table into fold. A failed or nonzero
// native observation is reported before the caller reads any partial fold.
func observeGroupRows(ctx context.Context, request GroupObservationRequest, fields string, fold io.Writer) error {
	command, err := core.ParseAbsolutePath(groupObservationExecutable)
	if err != nil {
		return err
	}
	args, err := ParseArguments([]string{"-axo", fields})
	if err != nil {
		return err
	}
	delay, err := temporal.DurationFromSeconds(groupObservationWaitSeconds)
	if err != nil {
		return err
	}
	result, err := Run(ctx, Request{Command: command, Arguments: args, WorkingDirectory: request.WorkingDirectory, Environment: request.Environment, Streams: Streams{Stdin: strings.NewReader(""), Stdout: fold, Stderr: io.Discard}, OutputPolicy: OutputPolicy{Mode: OutputModeStreaming}, WaitDelay: delay})
	if err != nil {
		return errors.Join(core.ErrProcessObservation, err)
	}
	code, err := result.ExitCode()
	if err != nil {
		return errors.Join(core.ErrProcessObservation, err)
	}
	value, err := code.Int()
	if err != nil || value != 0 {
		return errors.Join(core.ErrProcessObservation, err)
	}
	return nil
}
