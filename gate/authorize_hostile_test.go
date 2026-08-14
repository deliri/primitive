package gate

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// TestLeaseStateDispositionExhaustsUnderlyingDomain proves the fixed new-work
// disposition over the complete underlying byte domain, including every value
// a future Lease revision could introduce.
func TestLeaseStateDispositionExhaustsUnderlyingDomain(t *testing.T) {
	t.Parallel()

	var permitted, denied, closed int
	for raw := 0; raw <= math.MaxUint8; raw++ {
		state := lease.State(raw)
		got, err := dispositionForState(state)
		switch state {
		case lease.StateCurrent, lease.StateContinuity:
			if err != nil || got != authorizationDispositionPermit {
				t.Fatalf(
					"dispositionForState(lease.State(%d)) = (%v,%v), want (%v,nil)",
					raw, got, err, authorizationDispositionPermit,
				)
			}
			permitted++
		case lease.StateNotYetValid, lease.StateExpired,
			lease.StateRefused, lease.StateRevoked:
			if err != nil || got != authorizationDispositionDeny {
				t.Fatalf(
					"dispositionForState(lease.State(%d)) = (%v,%v), want (%v,nil)",
					raw, got, err, authorizationDispositionDeny,
				)
			}
			denied++
		default:
			if got != authorizationDispositionUnknown ||
				!errors.Is(err, core.ErrGateContract) ||
				!errors.Is(err, core.ErrLeaseContract) {
				t.Fatalf(
					"dispositionForState(lease.State(%d)) = (%v,%v), want unknown/gate+lease contract",
					raw, got, err,
				)
			}
			closed++
		}
	}
	if permitted != 2 || denied != 4 || closed != math.MaxUint8+1-6 {
		t.Fatalf(
			"disposition census = (permit=%d, deny=%d, closed=%d), want (2, 4, %d)",
			permitted, denied, closed, math.MaxUint8+1-6,
		)
	}
}

// TestDispositionRoutingRejectsUnhandledAdmittedState is the fail-closed
// ratchet for the switch that follows validation. If a future Lease revision
// adds an admitted state that Gate does not classify, the default arm must
// fail closed with Gate identity rather than fall through to a permit.
func TestDispositionRoutingRejectsUnhandledAdmittedState(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint8; raw++ {
		state := lease.State(raw)
		if !state.IsValid() {
			continue
		}
		got, err := dispositionForState(state)
		if err != nil {
			t.Fatalf(
				"dispositionForState(admitted lease.State(%d)) error = %v, want nil",
				raw, err,
			)
		}
		if got != authorizationDispositionPermit && got != authorizationDispositionDeny {
			t.Fatalf(
				"dispositionForState(admitted lease.State(%d)) = %v, want a classified disposition",
				raw, got,
			)
		}
	}
	got, err := dispositionForState(lease.StateUnknown)
	if got != authorizationDispositionUnknown || !errors.Is(err, core.ErrGateContract) {
		t.Fatalf(
			"dispositionForState(StateUnknown) = (%v,%v), want (unknown, ErrGateContract)",
			got, err,
		)
	}
}

// TestUnsetCapabilitiesFailClosedWithTypedBoundary proves each Gate-owned zero
// value rejects itself at its own named boundary.
func TestUnsetCapabilitiesFailClosedWithTypedBoundary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		validate     func() error
		name         string
		wantBoundary ContractBoundary
	}{
		{
			name:         "unset authorize request",
			validate:     AuthorizeRequest{}.Validate,
			wantBoundary: ContractBoundaryAuthorizeRequest,
		},
		{
			name:         "unset new-work permit",
			validate:     NewWorkPermit{}.Validate,
			wantBoundary: ContractBoundaryNewWorkPermit,
		},
		{
			name:         "unset denial",
			validate:     denialError{}.Validate,
			wantBoundary: ContractBoundaryDenial,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.validate()
			if !errors.Is(err, core.ErrGateContract) {
				t.Fatalf("Validate() error = %v, want ErrGateContract", err)
			}
			if !errors.Is(err, core.ErrLeaseContract) {
				t.Fatalf("Validate() error = %v, want the exact Lease rejection preserved", err)
			}
			var diagnostic ContractError
			if !errors.As(err, &diagnostic) ||
				diagnostic.Boundary() != tc.wantBoundary {
				t.Fatalf(
					"Validate() diagnostic = (%v,%v), want boundary %v",
					diagnostic, err, tc.wantBoundary,
				)
			}
		})
	}
}

// TestContractBoundaryExhaustsUnderlyingDomain proves the diagnostic domain is
// closed and that its label table can never render an unadmitted value.
func TestContractBoundaryExhaustsUnderlyingDomain(t *testing.T) {
	t.Parallel()

	want := [...]ContractBoundary{
		ContractBoundaryAuthorizeRequest,
		ContractBoundaryNewWorkPermit,
		ContractBoundaryDenial,
	}
	seen := make(map[string]ContractBoundary, len(want))
	for raw := 0; raw <= math.MaxUint8; raw++ {
		boundary := ContractBoundary(raw)
		found := false
		for _, admitted := range want {
			if boundary == admitted {
				found = true
				break
			}
		}
		if boundary.IsValid() != found {
			t.Fatalf(
				"ContractBoundary(%d).IsValid() = %t, want %t",
				raw, boundary.IsValid(), found,
			)
		}
		if !found {
			if gotErr := boundary.Validate(); !errors.Is(gotErr, core.ErrGateContract) {
				t.Fatalf("ContractBoundary(%d).Validate() error = %v, want errors.Is %v", raw, gotErr, core.ErrGateContract)
			}
			if boundary.String() != "" {
				t.Fatalf(
					"ContractBoundary(%d).String() = %q, want empty",
					raw, boundary.String(),
				)
			}
			continue
		}
		if err := boundary.Validate(); err != nil {
			t.Fatalf("ContractBoundary(%d).Validate() error = %v", raw, err)
		}
		label := boundary.String()
		if label == "" {
			t.Fatalf("ContractBoundary(%d).String() is empty", raw)
		}
		if prior, duplicate := seen[label]; duplicate {
			t.Fatalf(
				"ContractBoundary(%d).String() = %q, already used by %v",
				raw, label, prior,
			)
		}
		seen[label] = boundary
	}
	if len(seen) != len(want) {
		t.Fatalf("distinct boundary labels = %d, want %d", len(seen), len(want))
	}
}

// TestContractDiagnosticsCarryIdentityWithoutRenderingPrivateFacts proves the
// operator-facing text is exactly the stable Core diagnostic. Gate must never
// render the subject, generation, instants, or state it is holding.
func TestContractDiagnosticsCarryIdentityWithoutRenderingPrivateFacts(t *testing.T) {
	t.Parallel()

	for _, boundary := range [...]ContractBoundary{
		ContractBoundaryAuthorizeRequest,
		ContractBoundaryNewWorkPermit,
		ContractBoundaryDenial,
	} {
		diagnostic := ContractError{boundary: boundary}
		if err := diagnostic.Validate(); err != nil {
			t.Fatalf("ContractError{%v}.Validate() error = %v, want nil", boundary, err)
		}
		if !errors.Is(diagnostic, core.ErrGateContract) {
			t.Fatalf("errors.Is(ContractError{%v}, ErrGateContract) = false, want true", boundary)
		}
		if errors.Is(diagnostic, core.ErrGateDenied) {
			t.Fatalf("errors.Is(ContractError{%v}, ErrGateDenied) = true, want false", boundary)
		}
		if got, want := diagnostic.Error(), core.ErrGateContract.Error(); got != want {
			t.Fatalf("ContractError{%v}.Error() = %q, want %q", boundary, got, want)
		}
		if got := diagnostic.Boundary(); got != boundary {
			t.Fatalf("ContractError.Boundary() = %v, want %v", got, boundary)
		}
	}

	unset := ContractError{}
	if gotErr := unset.Validate(); !errors.Is(gotErr, core.ErrGateContract) {
		t.Fatalf("ContractError{}.Validate() error = %v, want errors.Is %v", gotErr, core.ErrGateContract)
	}

	var unsetDenial DenialError
	var unsetDenialError error = unsetDenial
	if unsetDenialError != nil {
		t.Fatalf("error(zero DenialError) = %v, want nil", unsetDenialError)
	}
	if errors.Is(unsetDenialError, core.ErrGateDenied) {
		t.Fatal("errors.Is(zero DenialError, ErrGateDenied) = true, want false")
	}
}
