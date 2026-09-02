package process_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

const commandExitChildArgument = "--primitive-command-exit="

func TestCommandExitCodePortableDomain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		wantCode int
		status   process.CommandExitCode
		wantErr  bool
	}{
		{name: "success maps to platform code zero", status: process.CommandExitCodeSuccess, wantCode: 0},
		{name: "failure maps to platform code one", status: process.CommandExitCodeFailure, wantCode: 1},
		{name: "maximum portable code is accepted", status: process.CommandExitCodeMaximum, wantCode: int(process.CommandExitCodeMaximum)},
		{name: "one above portable maximum is refused", status: process.CommandExitCodeMaximum + 1, wantErr: true},
		{name: "maximum representation is refused", status: process.CommandExitCode(^uint8(0)), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotCode, gotErr := tc.status.Code()
			if (gotErr != nil) != tc.wantErr {
				t.Fatalf("CommandExitCode.Code() error = %v, want error %t", gotErr, tc.wantErr)
			}
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrProcessContract) || gotCode == 0 {
					t.Fatalf("CommandExitCode.Code() = (%d, %v), want nonzero and errors.Is(..., %v)", gotCode, gotErr, core.ErrProcessContract)
				}
				return
			}
			if gotCode != tc.wantCode {
				t.Fatalf("CommandExitCode.Code() = %d, want %d", gotCode, tc.wantCode)
			}
		})
	}
}

func TestExitCommandProductionPath(t *testing.T) {
	t.Parallel()

	if status, child := commandExitChildStatus(t); child {
		process.ExitCommand(status)
		t.Fatalf("process.ExitCommand(%v) returned, want process termination", status)
	}

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

func commandExitChildStatus(t *testing.T) (process.CommandExitCode, bool) {
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
			return process.CommandExitCodeSuccess, true
		case commandExitChildArgument + "failure":
			return process.CommandExitCodeFailure, true
		}
	}
	return process.CommandExitCodeFailure, false
}

func runCommandExitChild(t *testing.T, status string) process.Result {
	t.Helper()

	executable, err := hostfacts.Executable()
	if err != nil {
		t.Fatalf("hostfacts.Executable() error = %v, want nil", err)
	}
	directory, err := hostfacts.WorkingDirectory()
	if err != nil {
		t.Fatalf("hostfacts.WorkingDirectory() error = %v, want nil", err)
	}
	arguments, err := process.ParseArguments([]string{
		"-test.run=^TestExitCommandProductionPath$",
		"--",
		commandExitChildArgument + status,
	})
	if err != nil {
		t.Fatalf("process.ParseArguments() error = %v, want nil", err)
	}
	environment, err := hostfacts.AmbientEnvironment()
	if err != nil {
		t.Fatalf("hostfacts.AmbientEnvironment() error = %v, want nil", err)
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
