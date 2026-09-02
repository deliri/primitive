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
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/process"
)

func TestWorkingDirectoryFeedsARealRun(t *testing.T) {
	t.Parallel()

	directory, err := hostfacts.WorkingDirectory()
	if err != nil {
		t.Fatalf("hostfacts.WorkingDirectory() error = %v, want nil", err)
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
	execution, err := process.Begin(ctx, request)
	if err != nil {
		t.Fatalf("process.Begin(group cancellation) error = %v, want nil", err)
	}
	done := make(chan runOutcome, 1)
	go func() {
		result, err := execution.Wait()
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

// TestSweepEndsAReapedGroupsSurvivors proves the one moment Sweep exists for:
// the leader exited, WaitDelay released the wait while a descendant held the
// inherited pipe, and the survivor must die by the group address because
// Deliver and Terminate are already refused.
func TestSweepEndsAReapedGroupsSurvivors(t *testing.T) {
	t.Parallel()

	var announced bytes.Buffer
	request := processRequest(t, "linger-block-pid", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: &announced, Stderr: io.Discard,
	})
	request.Containment = process.Containment{
		Isolation:    process.IsolationGroup,
		CancelSignal: process.CancelSignalKill,
	}
	execution, err := process.Begin(t.Context(), request)
	if err != nil {
		t.Fatalf("process.Begin(linger-block-pid) error = %v, want nil", err)
	}
	_, waitErr := execution.Wait()
	if !errors.Is(waitErr, exec.ErrWaitDelay) {
		t.Fatalf("Execution.Wait() error = %v, want errors.Is exec.ErrWaitDelay: the fixture lost its pipe holder", waitErr)
	}
	descendant, err := strconv.Atoi(strings.TrimPrefix(announced.String(), "ready:"))
	if err != nil {
		t.Fatalf("descendant announcement = %q, want ready:<pid> (%v)", announced.String(), err)
	}
	identity := process.ProcessIdentity(descendant) // #nosec G115 -- an announced child's identifier fits the platform pid domain.
	liveness, err := process.Alive(identity)
	if err != nil {
		t.Fatalf("process.Alive(survivor) error = %v, want nil", err)
	}
	if liveness != process.LivenessAlive {
		t.Fatalf("process.Alive(survivor) = %v, want %v: the fixture lost its survivor", liveness, process.LivenessAlive)
	}
	if err := execution.Terminate(); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("Terminate(after reap) error = %v, want errors.Is %v", err, core.ErrProcessContract)
	}
	if err := execution.Sweep(); err != nil {
		t.Fatalf("Execution.Sweep() error = %v, want nil", err)
	}
	for attempt := 1; ; attempt++ {
		liveness, err := process.Alive(identity)
		if err != nil {
			t.Fatalf("process.Alive(swept survivor) error = %v, want nil", err)
		}
		if liveness == process.LivenessGone {
			break
		}
		if attempt == 50 {
			t.Fatalf("survivor %d still %v after %d probes, want %v: the sweep did not reach the group",
				descendant, liveness, attempt, process.LivenessGone)
		}
		<-time.After(processTestProbeInterval)
	}
}

// TestSweepStopsARunningGroupAndToleratesRepetition proves Sweep is legal
// while the wait is in flight, where it is the absence-tolerant force stop a
// drain path wants, and that sweeping the same group again once it is gone is
// a successful no-op rather than an error.
func TestSweepStopsARunningGroupAndToleratesRepetition(t *testing.T) {
	t.Parallel()

	ready := make(chan struct{})
	request := processRequest(t, "wait", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: &readyWriter{ready: ready}, Stderr: io.Discard,
	})
	request.Containment = process.Containment{
		Isolation:    process.IsolationGroup,
		CancelSignal: process.CancelSignalKill,
	}
	execution, err := process.Begin(t.Context(), request)
	if err != nil {
		t.Fatalf("process.Begin(wait) error = %v, want nil", err)
	}
	select {
	case <-ready:
	case <-time.After(processTestBackstop):
		t.Fatalf("child readiness wait reached %s, want readiness first", processTestBackstop)
	}
	if err := execution.Sweep(); err != nil {
		t.Fatalf("Execution.Sweep(running group) error = %v, want nil", err)
	}
	result, waitErr := execution.Wait()
	if waitErr != nil {
		t.Fatalf("Execution.Wait(swept) error = %v, want nil", waitErr)
	}
	exit, err := result.ExitCode()
	if err != nil {
		t.Fatalf("swept ExitCode() error = %v, want nil", err)
	}
	signaled, err := exit.Signaled()
	if err != nil {
		t.Fatalf("swept ExitCode().Signaled() error = %v, want nil", err)
	}
	if !signaled {
		t.Fatalf("swept child Signaled() = false, want true")
	}
	if err := execution.Sweep(); err != nil {
		t.Fatalf("Execution.Sweep(group already gone) error = %v, want nil", err)
	}
}

// TestSweepRefusesADirectChild pins the group-only contract: a directly
// contained child has no group address, and its stored number after reap is
// exactly the recycled-identity delivery ProcessIdentity forbids.
func TestSweepRefusesADirectChild(t *testing.T) {
	t.Parallel()

	request := processRequest(t, "silent", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
	})
	execution, err := process.Begin(t.Context(), request)
	if err != nil {
		t.Fatalf("process.Begin(silent) error = %v, want nil", err)
	}
	if _, err := execution.Wait(); err != nil {
		t.Fatalf("Execution.Wait() error = %v, want nil", err)
	}
	if err := execution.Sweep(); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("Sweep(direct child) error = %v, want errors.Is %v", err, core.ErrProcessContract)
	}
}

// TestSweepRefusesAnExecutionNotStartedByBegin holds the door to the same
// unstarted-handle refusal every other supervision door shares.
func TestSweepRefusesAnExecutionNotStartedByBegin(t *testing.T) {
	t.Parallel()

	if err := new(process.Execution).Sweep(); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("Sweep(zero execution) error = %v, want errors.Is %v", err, core.ErrProcessContract)
	}
}

// TestSelfNamesTheCallingProcess proves Self answers with this exact
// process: the platform's own report is the oracle, the identity validates,
// and the liveness door agrees the identity names a running process.
func TestSelfNamesTheCallingProcess(t *testing.T) {
	t.Parallel()

	identity, err := process.Self()
	if err != nil {
		t.Fatalf("process.Self() error = %v, want nil", err)
	}
	pid, err := identity.Int()
	if err != nil {
		t.Fatalf("Self().Int() error = %v, want nil", err)
	}
	if want := os.Getpid(); pid != want {
		t.Fatalf("process.Self() = %d, want the platform's own %d", pid, want)
	}
	liveness, err := process.Alive(identity)
	if err != nil {
		t.Fatalf("process.Alive(Self()) error = %v, want nil", err)
	}
	if liveness != process.LivenessAlive {
		t.Fatalf("process.Alive(Self()) = %v, want %v", liveness, process.LivenessAlive)
	}
}
