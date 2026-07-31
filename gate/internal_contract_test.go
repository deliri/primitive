package gate

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// TestCapabilityRejectsAuthenticStateItDoesNotOwn is the forgery ratchet. Both
// Gate capabilities are private carriers, so the only way to build one over
// the wrong authentic state is from inside the package. Each must reject
// itself rather than trust the state it was handed: a permit that closes over
// a denying assessment must not authorize work, and a denial that closes over
// a permitting assessment must not be usable as a refusal.
func TestCapabilityRejectsAuthenticStateItDoesNotOwn(t *testing.T) {
	t.Parallel()

	t.Run("permit over an expired assessment refuses to authorize", func(t *testing.T) {
		t.Parallel()

		assessment := fixtureInternalAssessment(t, internalGoodUntilNanoseconds)
		if assessment.State() != lease.StateExpired {
			t.Fatalf(
				"fixture lease.Assessment.State() = %v, want %v",
				assessment.State(), lease.StateExpired,
			)
		}
		forged := NewWorkPermit{assessment: assessment}
		requireBoundaryRejection(t, forged.Validate(), ContractBoundaryNewWorkPermit)
		got, gotErr := forged.Assessment()
		if got != (lease.Assessment{}) {
			t.Fatalf("forged NewWorkPermit.Assessment() = %+v, want the zero assessment", got)
		}
		requireBoundaryRejection(t, gotErr, ContractBoundaryNewWorkPermit)
	})

	t.Run("denial over a current assessment refuses to refuse", func(t *testing.T) {
		t.Parallel()

		assessment := fixtureInternalAssessment(t, internalNotBeforeNanoseconds)
		if assessment.State() != lease.StateCurrent {
			t.Fatalf(
				"fixture lease.Assessment.State() = %v, want %v",
				assessment.State(), lease.StateCurrent,
			)
		}
		forged := denialError{assessment: assessment}
		requireBoundaryRejection(t, forged.Validate(), ContractBoundaryDenial)
		gotAssessment, gotAssessmentErr := forged.Assessment()
		if gotAssessment != (lease.Assessment{}) {
			t.Fatalf(
				"forged DenialError.Assessment() = %+v, want the zero assessment",
				gotAssessment,
			)
		}
		requireBoundaryRejection(t, gotAssessmentErr, ContractBoundaryDenial)
		gotState, gotStateErr := forged.State()
		if gotState != lease.StateUnknown {
			t.Fatalf("forged DenialError.State() = %v, want unknown", gotState)
		}
		requireBoundaryRejection(t, gotStateErr, ContractBoundaryDenial)
		gotContact, gotContactErr := forged.ContactState()
		if gotContact != lease.ContactStateUnknown {
			t.Fatalf("forged DenialError.ContactState() = %v, want unknown", gotContact)
		}
		requireBoundaryRejection(t, gotContactErr, ContractBoundaryDenial)
	})
}

// TestAuthorizeNewWorkCarriesTheExactAssessmentIntoItsResult proves the
// capability closes over the caller's exact assessment rather than a
// re-derived or default one.
func TestAuthorizeNewWorkCarriesTheExactAssessmentIntoItsResult(t *testing.T) {
	t.Parallel()

	permitting := fixtureInternalAssessment(t, internalNotBeforeNanoseconds)
	gotPermit, gotErr := AuthorizeNewWork(AuthorizeRequest{Assessment: permitting})
	if gotErr != nil {
		t.Fatalf("AuthorizeNewWork(current) error = %v, want nil", gotErr)
	}
	if gotPermit.assessment != permitting {
		t.Fatalf(
			"NewWorkPermit carries state/contact %v/%v, want %v/%v",
			gotPermit.assessment.State(), gotPermit.assessment.ContactState(),
			permitting.State(), permitting.ContactState(),
		)
	}

	denying := fixtureInternalAssessment(t, internalGoodUntilNanoseconds)
	_, denialErr := AuthorizeNewWork(AuthorizeRequest{Assessment: denying})
	var gotDenial DenialError
	if !errors.As(denialErr, &gotDenial) {
		t.Fatalf("AuthorizeNewWork(expired) error = %v, want a typed DenialError", denialErr)
	}
	gotAssessment, gotAssessmentErr := gotDenial.Assessment()
	if gotAssessmentErr != nil || gotAssessment != denying {
		t.Fatalf(
			"DenialError.Assessment() = (%v/%v,%v), want (%v/%v,nil)",
			gotAssessment.State(), gotAssessment.ContactState(), gotAssessmentErr,
			denying.State(), denying.ContactState(),
		)
	}
}

func requireBoundaryRejection(t *testing.T, got error, want ContractBoundary) {
	t.Helper()

	if !errors.Is(got, core.ErrGateContract) {
		t.Fatalf("error = %v, want ErrGateContract", got)
	}
	if errors.Is(got, core.ErrGateDenied) {
		t.Fatalf("error = %v, want no denial identity", got)
	}
	var diagnostic ContractError
	if !errors.As(got, &diagnostic) {
		t.Fatalf("error = %v, want a typed ContractError", got)
	}
	if diagnostic.Boundary() != want {
		t.Fatalf("ContractError.Boundary() = %v, want %v", diagnostic.Boundary(), want)
	}
}
