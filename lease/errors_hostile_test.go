package lease

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// TestSealedFailureConstructorsRequireProvingFacts ratchets the distinction
// between a general Lease contract rejection and the specialized identities a
// caller acts on. Unset, equal, reordered, and tolerated facts must never claim
// that a scope mismatch or clock contradiction occurred.
func TestSealedFailureConstructorsRequireProvingFacts(t *testing.T) {
	t.Parallel()

	subject := fixtureInternalHeader(t).Subject
	scopeRejections := []error{
		newScopeMismatch(Subject{}, subject),
		newScopeMismatch(subject, Subject{}),
		newScopeMismatch(subject, subject),
	}
	for _, rejection := range scopeRejections {
		var mismatch ScopeMismatch
		if !errors.Is(rejection, core.ErrLeaseContract) ||
			errors.Is(rejection, core.ErrLeaseScope) ||
			errors.As(rejection, &mismatch) {
			t.Errorf("newScopeMismatch() error = %v, want an untyped contract rejection", rejection)
		}
	}
	if errors.Is(scopeMismatch{}, core.ErrLeaseScope) {
		t.Errorf("zero internal scope mismatch carries %v", core.ErrLeaseScope)
	}

	unset := temporal.Instant{}
	zero := fixtureInternalInstant(0)
	tolerance := fixtureInternalInstant(ClockRollbackToleranceNanoseconds)
	clockRejections := []error{
		newClockContradiction(unset, tolerance),
		newClockContradiction(zero, unset),
		newClockContradiction(zero, zero),
		newClockContradiction(tolerance, zero),
		newClockContradiction(zero, tolerance),
	}
	for _, rejection := range clockRejections {
		var contradiction ClockContradiction
		if !errors.Is(rejection, core.ErrLeaseContract) ||
			errors.Is(rejection, core.ErrLeaseClock) ||
			errors.As(rejection, &contradiction) {
			t.Errorf("newClockContradiction() error = %v, want an untyped contract rejection", rejection)
		}
	}
	if errors.Is(clockContradiction{}, core.ErrLeaseClock) {
		t.Errorf("zero internal clock contradiction carries %v", core.ErrLeaseClock)
	}
}

// TestSealedFailureFactsSurviveConstruction proves admitted reports preserve
// the exact facts while carrying both their specialized and Lease-parent
// identities.
func TestSealedFailureFactsSurviveConstruction(t *testing.T) {
	t.Parallel()

	expected := fixtureInternalHeader(t).Subject
	actual := expected
	actual.Offering = core.Offering{Token: "lease-foreign-fixture"}
	scopeErr := newScopeMismatch(expected, actual)
	var mismatch ScopeMismatch
	if !errors.Is(scopeErr, core.ErrLeaseScope) ||
		!errors.Is(scopeErr, core.ErrLeaseContract) ||
		!errors.As(scopeErr, &mismatch) {
		t.Fatalf("newScopeMismatch() error = %v, want typed scope and contract identities", scopeErr)
	}
	gotExpected, gotActual, factErr := mismatch.Subjects()
	if factErr != nil || gotExpected != expected || gotActual != actual {
		t.Fatalf("ScopeMismatch.Subjects() = (%v, %v, %v), want exact facts and nil", gotExpected, gotActual, factErr)
	}

	observed := fixtureInternalInstant(0)
	trusted := fixtureInternalInstant(ClockRollbackToleranceNanoseconds + 1)
	clockErr := newClockContradiction(observed, trusted)
	var contradiction ClockContradiction
	if !errors.Is(clockErr, core.ErrLeaseClock) ||
		!errors.Is(clockErr, core.ErrLeaseContract) ||
		!errors.As(clockErr, &contradiction) {
		t.Fatalf("newClockContradiction() error = %v, want typed clock and contract identities", clockErr)
	}
	gotObserved, gotTrusted, instantErr := contradiction.Instants()
	if instantErr != nil || gotObserved != observed || gotTrusted != trusted {
		t.Fatalf("ClockContradiction.Instants() = (%v, %v, %v), want exact facts and nil", gotObserved, gotTrusted, instantErr)
	}
}
