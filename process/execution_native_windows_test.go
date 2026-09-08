// These proofs encode the Windows facts the unix suite deliberately excludes:
// quit and group containment are refused before a child exists, a killed
// child reports an exit code and no termination signal, and a reaped identity
// answers gone. The Darwin host cross-compiles this file; execution is a
// Windows runner's evidence.
//go:build windows

package process_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// TestWindowsRefusesContainmentItCannotDeliver pins the fail-closed admission
// boundary: a cancel signal Windows cannot deliver and a group isolation its
// delivery leaf cannot address are both refused before the child exists,
// never silently downgraded after it does.
func TestWindowsRefusesContainmentItCannotDeliver(t *testing.T) {
	t.Parallel()

	refusals := []struct {
		name        string
		containment process.Containment
	}{
		{
			name: "quit is not deliverable on windows",
			containment: process.Containment{
				Isolation:    process.IsolationDirect,
				CancelSignal: process.CancelSignalQuit,
			},
		},
		{
			name: "group cancellation is not deliverable on windows",
			containment: process.Containment{
				Isolation:    process.IsolationGroup,
				CancelSignal: process.CancelSignalKill,
			},
		},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := processRequest(t, "silent", process.Streams{
				Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
			})
			request.Containment = tc.containment
			if result, err := process.Run(t.Context(), request); result != (process.Result{}) || !errors.Is(err, core.ErrProcessUnsupported) {
				t.Fatalf("process.Run(%+v) error = %v, want errors.Is %v",
					tc.containment, err, core.ErrProcessUnsupported)
			}
		})
	}
}

// Both public kill doors must reap an owned child and retain Windows facts.
// Completed-object liveness is tested separately with a retained native handle;
// probing an unowned PID after Wait would race operating-system PID reuse.
func TestWindowsKilledChildReportsAnExitAndNoSignal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		stop func(*process.Execution) error
	}{
		{name: "terminate retains exit and refuses invented signal", stop: (*process.Execution).Terminate},
		{name: "deliver kill retains exit and refuses invented signal", stop: func(e *process.Execution) error { return e.Deliver(process.CancelSignalKill) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ready := make(chan struct{})
			var output bytes.Buffer
			request := processRequest(t, "wait", process.Streams{
				Stdin: bytes.NewReader(nil), Stdout: io.MultiWriter(&output, &readyWriter{ready: ready}), Stderr: io.Discard,
			})
			execution, err := process.Begin(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			waited := false
			t.Cleanup(func() {
				if !waited {
					stopErr := execution.Terminate()
					_, waitErr := execution.Wait()
					if err := errors.Join(stopErr, waitErr); err != nil {
						t.Error(err)
					}
				}
			})
			identity, err := execution.Identity()
			if err != nil || identity == 0 {
				t.Fatalf("started identity=%v, %v; want nonzero native identity", identity, err)
			}
			select {
			case <-ready:
			case <-processTestDeadline(t, processTestBackstop):
				t.Fatalf("child readiness reached %s backstop", processTestBackstop)
			}
			if err := tc.stop(execution); err != nil {
				t.Fatal(err)
			}
			result, err := execution.Wait()
			waited = true
			if err != nil {
				t.Fatal(err)
			}
			facts, err := result.Observation()
			if err != nil || facts.ExitCode <= core.ProcessExitCodeSuccess || facts.ExitCode > core.ProcessExitCodeMaximum || facts.TerminationSignal != nil || facts.PeakMemoryBytes != nil || facts.StdinBytes.Uint64() != 0 || facts.StdoutBytes.Uint64() != uint64(output.Len()) || facts.StderrBytes.Uint64() != 0 {
				t.Fatalf("terminated facts=%+v, %v; want nonzero unsigned exit, exact streams, no signal or RSS", facts, err)
			}
			signal, err := result.TerminationSignal()
			if signal != 0 || !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("termination signal=%v, %v; want zero and contract refusal", signal, err)
			}
			again, err := execution.Wait()
			if again != (process.Result{}) || !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("second wait=%+v, %v; want zero and contract refusal", again, err)
			}
			if err := tc.stop(execution); !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("post-reap stop=%v; want refusal before addressing recycled identity", err)
			}
		})
	}
}
