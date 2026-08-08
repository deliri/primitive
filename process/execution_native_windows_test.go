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
	"time"

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
			if _, err := process.Run(t.Context(), request); !errors.Is(err, core.ErrProcessUnsupported) {
				t.Fatalf("process.Run(%+v) error = %v, want errors.Is %v",
					tc.containment, err, core.ErrProcessUnsupported)
			}
		})
	}
}

// TestWindowsKilledChildReportsAnExitAndNoSignal is the counterpart of the
// unix supervision proof: Terminate stops the child through its held handle,
// the wait seals an observation, and the platform honestly reports no
// termination signal because Windows has none to report.
func TestWindowsKilledChildReportsAnExitAndNoSignal(t *testing.T) {
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
	if err := execution.Terminate(); err != nil {
		t.Fatalf("Execution.Terminate() error = %v, want nil", err)
	}
	result, err := execution.Wait()
	if err != nil {
		t.Fatalf("Execution.Wait(terminated) error = %v, want nil", err)
	}
	exit, err := result.ExitCode()
	if err != nil {
		t.Fatalf("terminated ExitCode() error = %v, want nil", err)
	}
	if success, err := exit.Success(); err != nil || success {
		t.Fatalf("terminated child success = (%t, %v), want (false, nil)", success, err)
	}
	if signaled, err := exit.Signaled(); err != nil || signaled {
		t.Fatalf("terminated child Signaled() = (%t, %v), want (false, nil): windows reports no signal", signaled, err)
	}
	if _, err := result.TerminationSignal(); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("terminated child TerminationSignal() error = %v, want %v", err, core.ErrProcessContract)
	}

	for attempt := 1; ; attempt++ {
		liveness, err := process.Alive(identity)
		if err != nil {
			t.Fatalf("process.Alive(reaped child) error = %v, want nil", err)
		}
		if liveness == process.LivenessGone {
			break
		}
		if attempt == 50 {
			t.Fatalf("process.Alive(reaped child) = %v after %d probes, want %v", liveness, attempt, process.LivenessGone)
		}
		<-time.After(processTestProbeInterval)
	}
}
