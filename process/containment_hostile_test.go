package process_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"testing"

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
		if got := isolation.Validate() == nil; got != admitted {
			t.Fatalf("Isolation(%d).Validate() admitted = %t, want %t", raw, got, admitted)
		}
		if got := isolation.IsValid(); got != admitted {
			t.Fatalf("Isolation(%d).IsValid() = %t, want %t", raw, got, admitted)
		}
		if !admitted && isolation.String() != core.UnknownEnumDiagnostic {
			t.Fatalf("Isolation(%d).String() = %q, want %q", raw, isolation.String(), core.UnknownEnumDiagnostic)
		}
	}
	if process.IsolationUnknown.Validate() == nil {
		t.Fatalf("IsolationUnknown.Validate() = nil, want the zero value rejected")
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
		if got := signal.Validate() == nil; got != admitted {
			t.Fatalf("CancelSignal(%d).Validate() admitted = %t, want %t", raw, got, admitted)
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
		if got := liveness.Validate() == nil; got != admitted {
			t.Fatalf("Liveness(%d).Validate() admitted = %t, want %t", raw, got, admitted)
		}
	}
}

func TestContainmentValidatesBothMembers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		containment process.Containment
		wantErr     bool
	}{
		{
			name:        "zero containment names no isolation",
			containment: process.Containment{},
			wantErr:     true,
		},
		{
			name: "isolation without a cancel signal is incomplete",
			containment: process.Containment{
				Isolation: process.IsolationDirect,
			},
			wantErr: true,
		},
		{
			name: "a cancel signal without isolation is incomplete",
			containment: process.Containment{
				CancelSignal: process.CancelSignalKill,
			},
			wantErr: true,
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
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.containment.Validate()
			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Fatalf("Containment.Validate() error = %v, wantErr %t", err, tc.wantErr)
			}
			if tc.wantErr && !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("Containment.Validate() error = %v, want %v", err, core.ErrProcessContract)
			}
		})
	}
}

func TestProcessIdentityRejectsNonPositiveValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		identity process.ProcessIdentity
		wantErr  bool
	}{
		{name: "zero is not a real identity", identity: 0, wantErr: true},
		{name: "negative one is not a real identity", identity: -1, wantErr: true},
		{name: "the smallest negative identity is rejected", identity: math.MinInt32, wantErr: true},
		{name: "one is the smallest real identity", identity: 1},
		{name: "a typical pid is admitted", identity: 4321},
		{name: "the largest identity is admitted", identity: math.MaxInt32},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := tc.identity.Int()
			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Fatalf("ProcessIdentity(%d).Int() error = %v, wantErr %t", tc.identity, err, tc.wantErr)
			}
			if tc.wantErr && !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("ProcessIdentity(%d).Int() error = %v, want %v", tc.identity, err, core.ErrProcessContract)
			}
		})
	}
}

func TestRunRefusesAnIncompleteContainment(t *testing.T) {
	t.Parallel()

	request := processRequest(t, "silent", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
	})
	request.Containment = process.Containment{}
	_, err := process.Run(t.Context(), request)
	if !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("process.Run(incomplete containment) error = %v, want %v", err, core.ErrProcessContract)
	}
}
