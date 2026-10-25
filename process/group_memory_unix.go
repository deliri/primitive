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

const groupMemoryExecutable = "/bin/ps"
const groupMemoryFields = "pgid=,rss=,time="
const groupMemoryWaitSeconds = 3

func observeGroupUsage(ctx context.Context, request GroupMemoryRequest) (GroupUsage, error) {
	command, err := core.ParseAbsolutePath(groupMemoryExecutable)
	if err != nil {
		return GroupUsage{}, err
	}
	args, err := ParseArguments([]string{"-axo", groupMemoryFields})
	if err != nil {
		return GroupUsage{}, err
	}
	fold := groupUsageFold{target: request.Group}
	delay, err := temporal.DurationFromSeconds(groupMemoryWaitSeconds)
	if err != nil {
		return GroupUsage{}, err
	}
	result, err := Run(ctx, Request{Command: command, Arguments: args, WorkingDirectory: request.WorkingDirectory, Environment: request.Environment, Streams: Streams{Stdin: strings.NewReader(""), Stdout: &fold, Stderr: io.Discard}, OutputPolicy: OutputPolicy{Mode: OutputModeStreaming}, WaitDelay: delay})
	if err != nil {
		return GroupUsage{}, errors.Join(core.ErrProcessObservation, err)
	}
	code, err := result.ExitCode()
	if err != nil {
		return GroupUsage{}, errors.Join(core.ErrProcessObservation, err)
	}
	value, err := code.Int()
	if err != nil || value != 0 {
		return GroupUsage{}, errors.Join(core.ErrProcessObservation, err)
	}
	return fold.finish()
}
