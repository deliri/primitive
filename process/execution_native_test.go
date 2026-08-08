// These proofs encode unix signal facts: quit is deliverable, a killed child
// reports its termination signal, and group cancellation reaps a tree. On
// Windows the same requests are refused before the child exists, so the
// windows counterparts are a separately tagged proof surface.
//go:build unix

package process_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
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
	// After the child is reaped its identity names no process, so the honest
	// expectation is gone. A reused pid is possible on a busy host, so gone is
	// polled a bounded number of times rather than asserted once; a probe that
	// still answers alive after every retry means the gone branch regressed,
	// because the kernel reusing this exact pid across every probe is not a
	// real event. An error is never admissible.
	for attempt := 1; ; attempt++ {
		liveness, err := process.Alive(identity)
		if err != nil {
			t.Fatalf("process.Alive(reaped child) error = %v, want nil", err)
		}
		if liveness == process.LivenessGone {
			break
		}
		if attempt == 5 {
			t.Fatalf("process.Alive(reaped child) = %v after %d probes, want %v",
				liveness, attempt, process.LivenessGone)
		}
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
	var announced bytes.Buffer
	request := processRequest(t, "linger-wait-pid", process.Streams{
		Stdin:  bytes.NewReader(nil),
		Stdout: io.MultiWriter(&announced, &readyWriter{ready: ready}),
		Stderr: io.Discard,
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

	// The wedge above proves the run returned; this proves the tree died. The
	// leader announced the descendant it spawned, so the group address is held
	// to its whole claim: if the minus sign in the group delivery regressed to
	// a direct signal, the leader would die, the run would still return inside
	// the backstop once WaitDelay closed the held pipe, and only this
	// observation would go red.
	descendant, err := strconv.Atoi(strings.TrimPrefix(announced.String(), "ready:"))
	if err != nil {
		t.Fatalf("descendant announcement = %q, want ready:<pid> (%v)", announced.String(), err)
	}
	identity := process.ProcessIdentity(descendant) // #nosec G115 -- an announced child's identifier fits the platform pid domain.
	for attempt := 1; ; attempt++ {
		liveness, err := process.Alive(identity)
		if err != nil {
			t.Fatalf("process.Alive(descendant) error = %v, want nil", err)
		}
		if liveness == process.LivenessGone {
			break
		}
		if attempt == 50 {
			t.Fatalf("descendant %d still %v after %d probes, want %v: the group cancellation did not reach it",
				descendant, liveness, attempt, process.LivenessGone)
		}
		<-time.After(processTestProbeInterval)
	}
}

// TestSupervisionRefusesADeliveryAfterTheChildIsReaped pins the lifetime edge
// of the handle. Once Wait has returned, the stored number may already name a
// recycled process or, under group isolation, a recycled group, and signaling
// it would be exactly the stored-number-later delivery the ProcessIdentity
// contract forbids. Deliver and Terminate must refuse rather than address
// whoever holds the number now.
func TestSupervisionRefusesADeliveryAfterTheChildIsReaped(t *testing.T) {
	t.Parallel()

	ready := make(chan struct{})
	request := processRequest(t, "wait", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: &readyWriter{ready: ready}, Stderr: io.Discard,
	})
	execution, err := process.Begin(t.Context(), request)
	if err != nil {
		t.Fatalf("process.Begin(wait) error = %v, want nil", err)
	}
	select {
	case <-ready:
	case <-time.After(processTestBackstop):
		t.Fatalf("child readiness wait reached %s, want readiness first", processTestBackstop)
	}
	if err := execution.Terminate(); err != nil {
		t.Fatalf("Execution.Terminate() error = %v, want nil", err)
	}
	if _, err := execution.Wait(); err != nil {
		t.Fatalf("Execution.Wait(terminated) error = %v, want nil", err)
	}

	if err := execution.Deliver(process.CancelSignalKill); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("Deliver(after reap) error = %v, want errors.Is %v", err, core.ErrProcessContract)
	}
	if err := execution.Terminate(); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("Terminate(after reap) error = %v, want errors.Is %v", err, core.ErrProcessContract)
	}
}
