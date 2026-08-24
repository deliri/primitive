package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

type exitCode uint8

const (
	exitCodeSuccess exitCode = iota
	exitCodeFailure
)

func main() {
	code := run(context.Background(), os.Args[1:], commandStreams{
		stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr,
	})
	os.Exit(int(code))
}

type commandStreams struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func run(
	ctx context.Context,
	arguments []string,
	streams commandStreams,
) exitCode {
	if ctx == nil || streams.stdin == nil || streams.stdout == nil || streams.stderr == nil {
		return writeFailure(streams.stderr, commandError("command runtime is incomplete", nil))
	}
	invocation, err := parseInvocation(arguments)
	if err != nil {
		return writeFailure(streams.stderr, err)
	}
	return runInvocation(ctx, invocation, streams)
}

func runInvocation(ctx context.Context, invocation invocation, streams commandStreams) exitCode {
	if invocation.Mode == invocationModeSchema {
		if err := writeJSON(streams.stdout, currentSchema()); err != nil {
			return writeFailure(streams.stderr, err)
		}
		return exitCodeSuccess
	}
	return runJob(ctx, invocation, streams)
}

func runJob(ctx context.Context, invocation invocation, streams commandStreams) exitCode {
	workingDirectory, err := process.WorkingDirectory()
	if err != nil {
		return writeFailure(streams.stderr, commandError("working directory observation failed", err))
	}
	configuration, job, err := loadInputs(ctx, workingDirectory, invocation.JobPath, streams.stdin)
	if err != nil {
		return writeFailure(streams.stderr, err)
	}
	if err := validateJobConfiguration(configuration, job); err != nil {
		return writeFailure(streams.stderr, err)
	}
	client, err := configuredClient(ctx, configuration)
	if err != nil {
		return writeFailure(streams.stderr, err)
	}
	result, err := executeJob(ctx, client, configuration, job)
	if err != nil {
		return writeFailure(streams.stderr, err)
	}
	if err := writeJSON(streams.stdout, result); err != nil {
		return writeFailure(streams.stderr, err)
	}
	return exitCodeSuccess
}

func writeJSON(destination io.Writer, document core.ValidatedJSONMarshaler) error {
	if destination == nil {
		return commandError("JSON destination is missing", nil)
	}
	encoded, err := document.MarshalJSON()
	if err != nil {
		return commandError("JSON result encoding failed", err)
	}
	encoded = append(encoded, '\n')
	count, err := destination.Write(encoded)
	if err != nil {
		return commandError(jsonResultWriteFailedDetail, err)
	}
	if count != len(encoded) {
		return commandError(jsonResultWriteFailedDetail, io.ErrShortWrite)
	}
	return nil
}

func writeFailure(destination io.Writer, err error) exitCode {
	if destination != nil {
		_, _ = fmt.Fprintln(destination, err)
	}
	return exitCodeFailure
}
