package gate_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gate"
	"github.com/deliri/primitive/v2026/lease"
)

type decisionKind uint8

const (
	decisionKindUnknown decisionKind = iota
	decisionKindGrant
	decisionKindRefusal
	decisionKindRevocation
)

// TestAuthorizeNewWorkBoundaryPressure drives the real signed path across both
// sides of every grant boundary the Lease timeline owns. The permitting window
// is exactly [NotBefore, GoodUntil); every instant outside it, and every
// non-grant outcome, must deny.
func TestAuthorizeNewWorkBoundaryPressure(t *testing.T) {
	t.Parallel()

	authority := fixtureAuthority(t, 11)
	subject := fixtureSubject(t, 12)
	grant := fixtureVerified(t, authority, fixtureGrantDecision(t, subject), subject)
	refusal := fixtureVerified(t, authority, fixtureRefusalDecision(t, subject), subject)
	revocation := fixtureVerified(t, authority, fixtureRevocationDecision(t, subject), subject)

	cases := []struct {
		name        string
		at          int64
		kind        decisionKind
		wantState   lease.State
		wantContact lease.ContactState
		wantPermit  bool
	}{
		{
			name: "issuance instant precedes the window and denies",
			kind: decisionKindGrant, at: fixtureIssuedAtNanoseconds,
			wantState: lease.StateNotYetValid, wantContact: lease.ContactStateNotDue,
		},
		{
			name: "one nanosecond before not-before denies",
			kind: decisionKindGrant, at: fixtureNotBeforeNanoseconds - 1,
			wantState: lease.StateNotYetValid, wantContact: lease.ContactStateNotDue,
		},
		{
			name: "exact not-before instant permits",
			kind: decisionKindGrant, at: fixtureNotBeforeNanoseconds,
			wantState: lease.StateCurrent, wantContact: lease.ContactStateNotDue,
			wantPermit: true,
		},
		{
			name: "one nanosecond after not-before permits",
			kind: decisionKindGrant, at: fixtureNotBeforeNanoseconds + 1,
			wantState: lease.StateCurrent, wantContact: lease.ContactStateNotDue,
			wantPermit: true,
		},
		{
			name: "one nanosecond before contact-after permits with contact not due",
			kind: decisionKindGrant, at: fixtureContactAfterNanoseconds - 1,
			wantState: lease.StateCurrent, wantContact: lease.ContactStateNotDue,
			wantPermit: true,
		},
		{
			name: "exact contact-after instant permits with contact due",
			kind: decisionKindGrant, at: fixtureContactAfterNanoseconds,
			wantState: lease.StateCurrent, wantContact: lease.ContactStateDue,
			wantPermit: true,
		},
		{
			name: "one nanosecond after contact-after permits with contact due",
			kind: decisionKindGrant, at: fixtureContactAfterNanoseconds + 1,
			wantState: lease.StateCurrent, wantContact: lease.ContactStateDue,
			wantPermit: true,
		},
		{
			name: "one nanosecond before not-after stays current and permits",
			kind: decisionKindGrant, at: fixtureNotAfterNanoseconds - 1,
			wantState: lease.StateCurrent, wantContact: lease.ContactStateDue,
			wantPermit: true,
		},
		{
			name: "exact not-after instant enters continuity and still permits",
			kind: decisionKindGrant, at: fixtureNotAfterNanoseconds,
			wantState: lease.StateContinuity, wantContact: lease.ContactStateDue,
			wantPermit: true,
		},
		{
			name: "one nanosecond after not-after permits under continuity",
			kind: decisionKindGrant, at: fixtureNotAfterNanoseconds + 1,
			wantState: lease.StateContinuity, wantContact: lease.ContactStateDue,
			wantPermit: true,
		},
		{
			name: "one nanosecond before good-until is the last permitting instant",
			kind: decisionKindGrant, at: fixtureGoodUntilNanoseconds - 1,
			wantState: lease.StateContinuity, wantContact: lease.ContactStateDue,
			wantPermit: true,
		},
		{
			name: "exact good-until instant expires and denies",
			kind: decisionKindGrant, at: fixtureGoodUntilNanoseconds,
			wantState: lease.StateExpired, wantContact: lease.ContactStateDue,
		},
		{
			name: "one nanosecond after good-until denies",
			kind: decisionKindGrant, at: fixtureGoodUntilNanoseconds + 1,
			wantState: lease.StateExpired, wantContact: lease.ContactStateDue,
		},
		{
			name: "far future instant denies without wrapping into a permit",
			kind: decisionKindGrant, at: fixtureGoodUntilNanoseconds * 1_000_000,
			wantState: lease.StateExpired, wantContact: lease.ContactStateDue,
		},
		{
			name: "refusal before its contact instant denies with contact not due",
			kind: decisionKindRefusal, at: fixtureContactAfterNanoseconds - 1,
			wantState: lease.StateRefused, wantContact: lease.ContactStateNotDue,
		},
		{
			name: "refusal at its contact instant denies with contact due",
			kind: decisionKindRefusal, at: fixtureContactAfterNanoseconds,
			wantState: lease.StateRefused, wantContact: lease.ContactStateDue,
		},
		{
			name: "refusal long after its contact instant still denies",
			kind: decisionKindRefusal, at: fixtureGoodUntilNanoseconds,
			wantState: lease.StateRefused, wantContact: lease.ContactStateDue,
		},
		{
			name: "revocation denies inside the original grant window",
			kind: decisionKindRevocation, at: fixtureNotBeforeNanoseconds,
			wantState: lease.StateRevoked, wantContact: lease.ContactStateProhibited,
		},
		{
			name: "revocation denies and prohibits contact at the issuance instant",
			kind: decisionKindRevocation, at: fixtureIssuedAtNanoseconds,
			wantState: lease.StateRevoked, wantContact: lease.ContactStateProhibited,
		},
		{
			name: "revocation denies after the original good-until instant",
			kind: decisionKindRevocation, at: fixtureGoodUntilNanoseconds + 1,
			wantState: lease.StateRevoked, wantContact: lease.ContactStateProhibited,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var verified lease.Verified
			switch tc.kind {
			case decisionKindGrant:
				verified = grant
			case decisionKindRefusal:
				verified = refusal
			case decisionKindRevocation:
				verified = revocation
			default:
				t.Fatalf("decision kind = %d, want an admitted fixture kind", tc.kind)
			}
			assessment := fixtureAssessment(t, verified, tc.at)
			if assessment.State() != tc.wantState {
				t.Fatalf(
					"lease.Assessment.State() = %v, want %v",
					assessment.State(), tc.wantState,
				)
			}
			if assessment.ContactState() != tc.wantContact {
				t.Fatalf(
					"lease.Assessment.ContactState() = %v, want %v",
					assessment.ContactState(), tc.wantContact,
				)
			}

			gotPermit, gotErr := gate.AuthorizeNewWork(gate.AuthorizeRequest{
				Assessment: assessment,
			})
			if tc.wantPermit {
				requirePermit(t, gotPermit, gotErr, assessment)
				return
			}
			requireDenial(t, gotPermit, gotErr, assessment, tc.wantState, tc.wantContact)
		})
	}
}

// TestAuthorizeNewWorkLayerTriad proves the classifier layer's positive,
// negative, and neutral results through the real signed path: a permitting
// state yields a usable capability, a denying state yields a typed denial that
// carries the exact assessment, and an unset request yields neither.
func TestAuthorizeNewWorkLayerTriad(t *testing.T) {
	t.Parallel()

	authority := fixtureAuthority(t, 21)
	subject := fixtureSubject(t, 22)
	grant := fixtureVerified(t, authority, fixtureGrantDecision(t, subject), subject)

	t.Run("positive current assessment yields a usable permit", func(t *testing.T) {
		t.Parallel()

		assessment := fixtureAssessment(t, grant, fixtureNotBeforeNanoseconds)
		gotPermit, gotErr := gate.AuthorizeNewWork(gate.AuthorizeRequest{
			Assessment: assessment,
		})
		requirePermit(t, gotPermit, gotErr, assessment)
	})

	t.Run("negative expired assessment yields a typed denial", func(t *testing.T) {
		t.Parallel()

		assessment := fixtureAssessment(t, grant, fixtureGoodUntilNanoseconds)
		gotPermit, gotErr := gate.AuthorizeNewWork(gate.AuthorizeRequest{
			Assessment: assessment,
		})
		requireDenial(
			t, gotPermit, gotErr, assessment,
			lease.StateExpired, lease.ContactStateDue,
		)
	})

	t.Run("neutral unset request yields neither permit nor denial", func(t *testing.T) {
		t.Parallel()

		gotPermit, gotErr := gate.AuthorizeNewWork(gate.AuthorizeRequest{})
		if gotPermit != (gate.NewWorkPermit{}) {
			t.Fatalf("AuthorizeNewWork(unset) permit = %+v, want the zero permit", gotPermit)
		}
		var gotDenial gate.DenialError
		if errors.As(gotErr, &gotDenial) {
			t.Fatalf("AuthorizeNewWork(unset) produced a denial, want a contract rejection")
		}
		if errors.Is(gotErr, core.ErrGateDenied) {
			t.Fatalf("AuthorizeNewWork(unset) error = %v, want no denial identity", gotErr)
		}
		requireContractBoundary(t, gotErr, gate.ContractBoundaryAuthorizeRequest)
	})
}

// TestZeroCapabilitiesCarryNoAuthority proves the exported zero values are
// inert: neither can be used as a permit or as an authentic denial.
func TestZeroCapabilitiesCarryNoAuthority(t *testing.T) {
	t.Parallel()

	t.Run("zero permit rejects use and returns no assessment", func(t *testing.T) {
		t.Parallel()

		var permit gate.NewWorkPermit
		requireContractBoundary(t, permit.Validate(), gate.ContractBoundaryNewWorkPermit)
		gotAssessment, gotErr := permit.Assessment()
		if gotAssessment != (lease.Assessment{}) {
			t.Fatalf("NewWorkPermit{}.Assessment() = %+v, want the zero assessment", gotAssessment)
		}
		requireContractBoundary(t, gotErr, gate.ContractBoundaryNewWorkPermit)
	})

	t.Run("zero denial rejects every accessor", func(t *testing.T) {
		t.Parallel()

		var denial gate.DenialError
		if denial != nil {
			t.Fatalf("zero DenialError = nonnil, want nil")
		}
		var denialErr error = denial
		if denialErr != nil {
			t.Fatalf("error(zero DenialError) = %v, want nil", denialErr)
		}
		if errors.Is(denialErr, core.ErrGateDenied) {
			t.Fatal("errors.Is(error(zero DenialError), ErrGateDenied) = true, want false")
		}
	})
}

// TestDenialAndContractIdentitiesStayDistinguishable pins the exact recovery
// contract a CLI depends on. A denial means the customer must renew; a
// contract rejection means the caller handed Gate an unusable value. A caller
// must never be able to read one as the other.
func TestDenialAndContractIdentitiesStayDistinguishable(t *testing.T) {
	t.Parallel()

	authority := fixtureAuthority(t, 31)
	subject := fixtureSubject(t, 32)
	grant := fixtureVerified(t, authority, fixtureGrantDecision(t, subject), subject)
	assessment := fixtureAssessment(t, grant, fixtureGoodUntilNanoseconds)

	_, denialErr := gate.AuthorizeNewWork(gate.AuthorizeRequest{Assessment: assessment})
	_, contractErr := gate.AuthorizeNewWork(gate.AuthorizeRequest{})

	if !errors.Is(denialErr, core.ErrGateDenied) {
		t.Fatalf("denial error = %v, want ErrGateDenied", denialErr)
	}
	if errors.Is(contractErr, core.ErrGateDenied) {
		t.Fatalf("contract error = %v, want no denial identity", contractErr)
	}
	var gotContractDiagnostic gate.ContractError
	if errors.As(denialErr, &gotContractDiagnostic) {
		t.Fatalf(
			"denial error carries contract diagnostic %v, want none",
			gotContractDiagnostic.Boundary(),
		)
	}
	if !errors.As(contractErr, &gotContractDiagnostic) {
		t.Fatalf("contract error = %v, want a typed ContractError", contractErr)
	}
	if got, want := denialErr.Error(), core.ErrGateDenied.Error(); got != want {
		t.Fatalf("denial error text = %q, want %q", got, want)
	}
	if got, want := contractErr.Error(), core.ErrGateContract.Error(); !strings.Contains(got, want) {
		t.Fatalf("contract error text = %q, want it to carry %q", got, want)
	}
}

func requirePermit(
	t *testing.T,
	got gate.NewWorkPermit,
	gotErr error,
	want lease.Assessment,
) {
	t.Helper()

	if gotErr != nil {
		t.Fatalf("AuthorizeNewWork() error = %v, want nil", gotErr)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("NewWorkPermit.Validate() error = %v, want nil", err)
	}
	gotAssessment, gotAssessmentErr := got.Assessment()
	if gotAssessmentErr != nil {
		t.Fatalf("NewWorkPermit.Assessment() error = %v, want nil", gotAssessmentErr)
	}
	if gotAssessment != want {
		t.Fatalf(
			"NewWorkPermit.Assessment() state/contact = %v/%v, want %v/%v",
			gotAssessment.State(), gotAssessment.ContactState(),
			want.State(), want.ContactState(),
		)
	}
}

func requireDenial(
	t *testing.T,
	gotPermit gate.NewWorkPermit,
	gotErr error,
	wantAssessment lease.Assessment,
	wantState lease.State,
	wantContact lease.ContactState,
) {
	t.Helper()

	if gotPermit != (gate.NewWorkPermit{}) {
		t.Fatalf("AuthorizeNewWork() permit = %+v, want the zero permit", gotPermit)
	}
	if !errors.Is(gotErr, core.ErrGateDenied) {
		t.Fatalf("AuthorizeNewWork() error = %v, want ErrGateDenied", gotErr)
	}
	var gotDenial gate.DenialError
	if !errors.As(gotErr, &gotDenial) {
		t.Fatalf("AuthorizeNewWork() error = %v, want a typed DenialError", gotErr)
	}
	gotState, gotStateErr := gotDenial.State()
	if gotStateErr != nil || gotState != wantState {
		t.Fatalf(
			"DenialError.State() = (%v,%v), want (%v,nil)",
			gotState, gotStateErr, wantState,
		)
	}
	gotContact, gotContactErr := gotDenial.ContactState()
	if gotContactErr != nil || gotContact != wantContact {
		t.Fatalf(
			"DenialError.ContactState() = (%v,%v), want (%v,nil)",
			gotContact, gotContactErr, wantContact,
		)
	}
	gotAssessment, gotAssessmentErr := gotDenial.Assessment()
	if gotAssessmentErr != nil || gotAssessment != wantAssessment {
		t.Fatalf(
			"DenialError.Assessment() error = %v and state/contact = %v/%v, want nil and %v/%v",
			gotAssessmentErr, gotAssessment.State(), gotAssessment.ContactState(),
			wantAssessment.State(), wantAssessment.ContactState(),
		)
	}
	if got, want := gotErr.Error(), core.ErrGateDenied.Error(); got != want {
		t.Fatalf("denial error text = %q, want exactly %q", got, want)
	}
}

func requireContractBoundary(
	t *testing.T,
	gotErr error,
	want gate.ContractBoundary,
) {
	t.Helper()

	if !errors.Is(gotErr, core.ErrGateContract) {
		t.Fatalf("error = %v, want ErrGateContract", gotErr)
	}
	var gotDiagnostic gate.ContractError
	if !errors.As(gotErr, &gotDiagnostic) {
		t.Fatalf("error = %v, want a typed ContractError", gotErr)
	}
	if gotDiagnostic.Boundary() != want {
		t.Fatalf(
			"ContractError.Boundary() = %v, want %v",
			gotDiagnostic.Boundary(), want,
		)
	}
}
