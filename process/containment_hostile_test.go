package process_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestIsolationAdmitsOnlyTheClosedDomain(t *testing.T) {
	t.Parallel()

	valid := []process.Isolation{process.IsolationDirect, process.IsolationGroup}
	for raw := 0; raw <= math.MaxUint8; raw++ {
		isolation := process.Isolation(raw)
		admitted := false
		for _, member := range valid {
			if isolation == member {
				admitted = true
			}
		}
		gotErr := isolation.Validate()
		if admitted && gotErr != nil || !admitted && !errors.Is(gotErr, core.ErrProcessContract) {
			t.Fatalf("Isolation(%d).Validate() error = %v, want admitted %t or errors.Is %v", raw, gotErr, admitted, core.ErrProcessContract)
		}
		if got := isolation.IsValid(); got != admitted {
			t.Fatalf("Isolation(%d).IsValid() = %t, want %t", raw, got, admitted)
		}
		if !admitted && isolation.String() != core.UnknownEnumDiagnostic {
			t.Fatalf("Isolation(%d).String() = %q, want %q", raw, isolation.String(), core.UnknownEnumDiagnostic)
		}
	}
	if gotErr := process.IsolationUnknown.Validate(); !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf("IsolationUnknown.Validate() error = %v, want errors.Is %v", gotErr, core.ErrProcessContract)
	}
}

func TestCancelSignalAdmitsOnlyTheClosedDomain(t *testing.T) {
	t.Parallel()

	valid := []process.CancelSignal{
		process.CancelSignalKill,
		process.CancelSignalQuit,
		process.CancelSignalInterrupt,
		process.CancelSignalTerminate,
	}
	for raw := 0; raw <= math.MaxUint8; raw++ {
		signal := process.CancelSignal(raw)
		admitted := false
		for _, member := range valid {
			if signal == member {
				admitted = true
			}
		}
		gotErr := signal.Validate()
		if admitted && gotErr != nil || !admitted && !errors.Is(gotErr, core.ErrProcessContract) {
			t.Fatalf("CancelSignal(%d).Validate() error = %v, want admitted %t or errors.Is %v", raw, gotErr, admitted, core.ErrProcessContract)
		}
		if !admitted && signal.String() != core.UnknownEnumDiagnostic {
			t.Fatalf("CancelSignal(%d).String() = %q, want %q", raw, signal.String(), core.UnknownEnumDiagnostic)
		}
	}
}

func TestLivenessAdmitsOnlyTheClosedDomain(t *testing.T) {
	t.Parallel()

	valid := []process.Liveness{process.LivenessAlive, process.LivenessGone}
	for raw := 0; raw <= math.MaxUint8; raw++ {
		liveness := process.Liveness(raw)
		admitted := false
		for _, member := range valid {
			if liveness == member {
				admitted = true
			}
		}
		gotErr := liveness.Validate()
		if admitted && gotErr != nil || !admitted && !errors.Is(gotErr, core.ErrProcessContract) {
			t.Fatalf("Liveness(%d).Validate() error = %v, want admitted %t or errors.Is %v", raw, gotErr, admitted, core.ErrProcessContract)
		}
	}
}

func TestContainmentValidatesBothMembers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		containment process.Containment
		wantErr     error
	}{
		{
			name:        "zero containment names no isolation",
			containment: process.Containment{},
			wantErr:     core.ErrProcessContract,
		},
		{
			name: "isolation without a cancel signal is incomplete",
			containment: process.Containment{
				Isolation: process.IsolationDirect,
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "a cancel signal without isolation is incomplete",
			containment: process.Containment{
				CancelSignal: process.CancelSignalKill,
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "direct isolation with a kill signal is complete",
			containment: process.Containment{
				Isolation:    process.IsolationDirect,
				CancelSignal: process.CancelSignalKill,
			},
		},
		{
			name: "group isolation with a quit signal is complete",
			containment: process.Containment{
				Isolation:    process.IsolationGroup,
				CancelSignal: process.CancelSignalQuit,
			},
		},
		{
			name: "an out-of-domain isolation is rejected even with a valid signal",
			containment: process.Containment{
				Isolation:    process.Isolation(math.MaxUint8),
				CancelSignal: process.CancelSignalKill,
			},
			wantErr: core.ErrProcessContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.containment.Validate()
			if tc.wantErr == nil && err != nil {
				t.Fatalf("Containment.Validate() error = %v, want nil", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("Containment.Validate() error = %v, want errors.Is %v", err, tc.wantErr)
			}
		})
	}
}

func TestProcessIdentityRejectsNonPositiveValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		identity process.ProcessIdentity
		wantErr  error
	}{
		{name: "zero is not a real identity", identity: 0, wantErr: core.ErrProcessContract},
		{name: "negative one is not a real identity", identity: -1, wantErr: core.ErrProcessContract},
		{name: "the smallest negative identity is rejected", identity: math.MinInt32, wantErr: core.ErrProcessContract},
		{name: "one is the smallest real identity", identity: 1},
		{name: "a typical pid is admitted", identity: 4321},
		{name: "the largest identity is admitted", identity: math.MaxInt32},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.identity.Int()
			if tc.wantErr == nil && (err != nil || got != int(tc.identity)) {
				t.Fatalf("ProcessIdentity(%d).Int() = (%d, %v), want (%d, nil)",
					tc.identity, got, err, tc.identity)
			}
			if tc.wantErr != nil && (got != 0 || !errors.Is(err, tc.wantErr)) {
				t.Fatalf("ProcessIdentity(%d).Int() = (%d, %v), want zero and errors.Is %v",
					tc.identity, got, err, tc.wantErr)
			}
		})
	}
}

func TestRunDefaultsAZeroContainmentToDirectKill(t *testing.T) {
	t.Parallel()

	// A caller that names no containment is the common probe case: run one
	// command, read its output. The zero value must be accepted and run as a
	// direct child, not rejected, so existing callers are never forced to spell
	// out the conservative default.
	request := processRequest(t, "silent", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
	})
	request.Containment = process.Containment{}
	if err := request.Validate(); err != nil {
		t.Fatalf("Request.Validate(zero containment) error = %v, want nil", err)
	}
	result, err := process.Run(t.Context(), request)
	if err != nil {
		t.Fatalf("process.Run(zero containment) error = %v, want nil", err)
	}
	exit, err := result.ExitCode()
	if err != nil {
		t.Fatalf("ExitCode() error = %v, want nil", err)
	}
	if success, err := exit.Success(); err != nil || !success {
		t.Fatalf("zero-containment child success = %t (err %v), want true", success, err)
	}
}

// TestZeroContainmentCancellationActuallyKillsTheChild proves the default
// behaves as the direct-kill it names, not merely that it is accepted: a
// lingering child under a fully zero containment must die on cancellation and
// the run must report the cancellation, inside the backstop. It goes red if
// the default resolves to a containment whose cancel delivers nothing.
func TestZeroContainmentCancellationActuallyKillsTheChild(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})
	request := processRequest(t, "wait", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: &readyWriter{ready: ready}, Stderr: io.Discard,
	})
	request.Containment = process.Containment{}
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
			t.Fatalf("zero-containment cancelled Run error = %v, want %v and %v",
				got.err, context.Canceled, core.ErrProcessWait)
		}
	case <-time.After(processTestBackstop):
		t.Fatalf("zero-containment cancelled Run reached %s, want the defaulted kill to reap the child", processTestBackstop)
	}
}

func TestRunRefusesAHalfNamedContainment(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		containment process.Containment
	}{
		{
			name:        "isolation without a cancel signal",
			containment: process.Containment{Isolation: process.IsolationGroup},
		},
		{
			name:        "cancel signal without isolation",
			containment: process.Containment{CancelSignal: process.CancelSignalQuit},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := processRequest(t, "silent", process.Streams{
				Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
			})
			request.Containment = tc.containment
			if _, err := process.Run(t.Context(), request); !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("process.Run(half containment) error = %v, want %v", err, core.ErrProcessContract)
			}
		})
	}
}
