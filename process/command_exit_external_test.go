package process_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

const commandExitChildArgument = "--primitive-command-exit="

func TestExitStatusClosedDomain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		status   process.ExitStatus
		wantCode int
		wantErr  bool
	}{
		{name: "success maps to platform code zero", status: process.ExitStatusSuccess, wantCode: 0},
		{name: "failure maps to platform code one", status: process.ExitStatusFailure, wantCode: 1},
		{name: "unknown next status is refused", status: process.ExitStatus(3), wantErr: true},
		{name: "maximum representation is refused", status: process.ExitStatus(^uint8(0)), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotCode, gotErr := tc.status.Code()
			if (gotErr != nil) != tc.wantErr {
				t.Fatalf("ExitStatus.Code() error = %v, want error %t", gotErr, tc.wantErr)
			}
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrProcessContract) || gotCode == 0 {
					t.Fatalf("ExitStatus.Code() = (%d, %v), want nonzero and errors.Is(..., %v)", gotCode, gotErr, core.ErrProcessContract)
				}
				return
			}
			if gotCode != tc.wantCode {
				t.Fatalf("ExitStatus.Code() = %d, want %d", gotCode, tc.wantCode)
			}
		})
	}
}

func TestExitCommandProductionPath(t *testing.T) {
	if status, child := commandExitChildStatus(t); child {
		process.ExitCommand(status)
		t.Fatalf("process.ExitCommand(%v) returned, want process termination", status)
	}
	t.Parallel()

	cases := []struct {
		name     string
		argument string
		wantCode int
	}{
		{name: "success terminates the command with zero", argument: "success", wantCode: 0},
		{name: "failure terminates the command with one", argument: "failure", wantCode: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := runCommandExitChild(t, tc.argument)
			gotCode, gotErr := got.ExitCode()
			if gotErr != nil {
				t.Fatalf("Result.ExitCode() error = %v, want nil", gotErr)
			}
			code, codeErr := gotCode.Int()
			if codeErr != nil {
				t.Fatalf("ExitCode.Int() error = %v, want nil", codeErr)
			}
			if code != tc.wantCode {
				t.Fatalf("ExitCommand observed exit code = %d, want %d", code, tc.wantCode)
			}
		})
	}
}

func commandExitChildStatus(t *testing.T) (process.ExitStatus, bool) {
	t.Helper()

	arguments, err := process.AmbientArguments()
	if err != nil {
		t.Fatalf("process.AmbientArguments() error = %v, want nil", err)
	}
	for _, argument := range arguments {
		value, valueErr := argument.Value()
		if valueErr != nil {
			t.Fatalf("Argument.Value() error = %v, want nil", valueErr)
		}
		switch value {
		case commandExitChildArgument + "success":
			return process.ExitStatusSuccess, true
		case commandExitChildArgument + "failure":
			return process.ExitStatusFailure, true
		}
	}
	return process.ExitStatusUnknown, false
}

func runCommandExitChild(t *testing.T, status string) process.Result {
	t.Helper()

	executable, err := process.Executable()
	if err != nil {
		t.Fatalf("process.Executable() error = %v, want nil", err)
	}
	directory, err := process.WorkingDirectory()
	if err != nil {
		t.Fatalf("process.WorkingDirectory() error = %v, want nil", err)
	}
	arguments, err := process.ParseArguments([]string{
		"-test.run=^TestExitCommandProductionPath$",
		"--",
		commandExitChildArgument + status,
	})
	if err != nil {
		t.Fatalf("process.ParseArguments() error = %v, want nil", err)
	}
	environment, err := process.AmbientEnvironment()
	if err != nil {
		t.Fatalf("process.AmbientEnvironment() error = %v, want nil", err)
	}
	maximum, err := core.NewByteCount(4096)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	wait, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds() error = %v, want nil", err)
	}
	var stderr bytes.Buffer
	got, gotErr := process.Run(context.Background(), process.Request{
		Command: executable, WorkingDirectory: directory, Arguments: arguments, Environment: environment,
		Streams:     process.Streams{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: &stderr},
		OutputLimit: maximum, WaitDelay: wait,
	})
	if gotErr != nil {
		t.Fatalf("process.Run(command exit child) error = %v stderr = %q, want nil", gotErr, stderr.String())
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("process.Run(command exit child).Validate() error = %v, want nil", err)
	}
	return got
}
