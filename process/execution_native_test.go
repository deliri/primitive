package process_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestWorkingDirectoryFeedsARealRun(t *testing.T) {
	t.Parallel()

	directory, err := process.WorkingDirectory()
	if err != nil {
		t.Fatalf("process.WorkingDirectory() error = %v, want nil", err)
	}
	if err := directory.Validate(); err != nil {
		t.Fatalf("WorkingDirectory().Validate() error = %v, want nil", err)
	}
	var stdout bytes.Buffer
	request := processRequest(t, "working-directory", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: &stdout, Stderr: io.Discard,
	})
	request.WorkingDirectory = directory
	if _, err := process.Run(t.Context(), request); err != nil {
		t.Fatalf("process.Run(working-directory) error = %v, want nil", err)
	}
	if got := stdout.String(); got != directory.String() {
		t.Fatalf("child working directory = %q, want %q", got, directory.String())
	}
}

func TestBeginExposesTheStartedChildIdentityAndReapsItOnWait(t *testing.T) {
	t.Parallel()

	ready := make(chan struct{})
	request := processRequest(t, "wait", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: &readyWriter{ready: ready}, Stderr: io.Discard,
	})
	execution, err := process.Begin(t.Context(), request)
	if err != nil {
		t.Fatalf("process.Begin(wait) error = %v, want nil", err)
	}
	identity, err := execution.Identity()
	if err != nil {
		t.Fatalf("Execution.Identity() error = %v, want nil", err)
	}
	select {
	case <-ready:
	case <-time.After(processTestBackstop):
		t.Fatalf("child readiness wait reached %s, want readiness first", processTestBackstop)
	}

	liveness, err := process.Alive(identity)
	if err != nil {
		t.Fatalf("process.Alive(running child) error = %v, want nil", err)
	}
	if liveness != process.LivenessAlive {
		t.Fatalf("process.Alive(running child) = %v, want %v", liveness, process.LivenessAlive)
	}

	// Terminate is the supervisor's own action, not a context deadline, so the
	// reaped child is a successful observation of a signaled exit rather than a
	// wait failure. The signal it died from is recorded in the Result.
	if err := execution.Terminate(); err != nil {
		t.Fatalf("Execution.Terminate() error = %v, want nil", err)
	}
	result, waitErr := execution.Wait()
	if waitErr != nil {
		t.Fatalf("Execution.Wait(terminated) error = %v, want nil", waitErr)
	}
	exit, err := result.ExitCode()
	if err != nil {
		t.Fatalf("terminated ExitCode() error = %v, want nil", err)
	}
	signaled, err := exit.Signaled()
	if err != nil {
		t.Fatalf("terminated ExitCode().Signaled() error = %v, want nil", err)
	}
	if !signaled {
		t.Fatalf("terminated child Signaled() = false, want true")
	}
	if _, err := result.TerminationSignal(); err != nil {
		t.Fatalf("terminated child TerminationSignal() error = %v, want a reported signal", err)
	}
}

func TestExecutionWaitIsExactlyOnce(t *testing.T) {
	t.Parallel()

	request := processRequest(t, "silent", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
	})
	execution, err := process.Begin(t.Context(), request)
	if err != nil {
		t.Fatalf("process.Begin(silent) error = %v, want nil", err)
	}
	if _, err := execution.Wait(); err != nil {
		t.Fatalf("first Execution.Wait() error = %v, want nil", err)
	}
	if _, err := execution.Wait(); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("second Execution.Wait() error = %v, want %v", err, core.ErrProcessContract)
	}
}

func TestNormalExitReportsNoTerminationSignal(t *testing.T) {
	t.Parallel()

	request := processRequest(t, "exit:0", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
	})
	result, err := process.Run(t.Context(), request)
	if err != nil {
		t.Fatalf("process.Run(exit:0) error = %v, want nil", err)
	}
	if _, err := result.TerminationSignal(); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("normal exit TerminationSignal() error = %v, want %v", err, core.ErrProcessContract)
	}
}

func TestAliveReportsAGoneIdentity(t *testing.T) {
	t.Parallel()

	request := processRequest(t, "silent", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
	})
	execution, err := process.Begin(t.Context(), request)
	if err != nil {
		t.Fatalf("process.Begin(silent) error = %v, want nil", err)
	}
	identity, err := execution.Identity()
	if err != nil {
		t.Fatalf("Execution.Identity() error = %v, want nil", err)
	}
	if _, err := execution.Wait(); err != nil {
		t.Fatalf("Execution.Wait() error = %v, want nil", err)
	}
	// After the child is reaped its identity names no process. A reused pid is
	// a real possibility on a busy host, so this asserts the two admitted
	// answers rather than gone alone: what must never happen is an error.
	liveness, err := process.Alive(identity)
	if err != nil {
		t.Fatalf("process.Alive(reaped child) error = %v, want nil", err)
	}
	if !liveness.IsValid() {
		t.Fatalf("process.Alive(reaped child) = %v, want a valid liveness", liveness)
	}
}

func TestAliveRefusesAnInvalidIdentity(t *testing.T) {
	t.Parallel()

	if _, err := process.Alive(0); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("process.Alive(0) error = %v, want %v", err, core.ErrProcessContract)
	}
}

func TestGroupCancellationReapsTheWholeTree(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})
	request := processRequest(t, "linger-wait", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: &readyWriter{ready: ready}, Stderr: io.Discard,
	})
	request.Containment = process.Containment{
		Isolation:    process.IsolationGroup,
		CancelSignal: process.CancelSignalQuit,
	}
	done := make(chan runOutcome, 1)
	go func() {
		result, err := process.Run(ctx, request)
		done <- runOutcome{result: result, err: err}
	}()

	select {
	case <-ready:
		cancel()
	case <-time.After(processTestBackstop):
		t.Fatalf("child readiness wait reached %s, want readiness first", processTestBackstop)
	}

	select {
	case got := <-done:
		if !errors.Is(got.err, context.Canceled) || !errors.Is(got.err, core.ErrProcessWait) {
			t.Fatalf("group-cancelled Run error = %v, want %v and %v", got.err, context.Canceled, core.ErrProcessWait)
		}
	case <-time.After(processTestBackstop):
		t.Fatalf("group-cancelled Run reached %s, want a reaped tree; a leaked group would wedge here", processTestBackstop)
	}
}
