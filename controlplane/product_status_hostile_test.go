package controlplane_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/lease"
)

// allProductStatuses is every status an authority may send. Written out rather
// than walked from the iota, so a status added without a decision here shows up
// as an unhandled case in the tables below instead of passing silently.
func allProductStatuses() []controlplane.ProductStatus {
	return []controlplane.ProductStatus{
		controlplane.ProductStatusActive,
		controlplane.ProductStatusPaymentRetry,
		controlplane.ProductStatusReadOnly,
		controlplane.ProductStatusStopped,
		controlplane.ProductStatusUpgradeRequired,
		controlplane.ProductStatusRevoked,
	}
}

// TestValidateOutcomeClosesEveryStatusAndOutcomePair is the exhaustive matrix.
//
// This rule decides whether a customer's machine may start new work, so the
// case that matters is not a malformed status. It is an authentic status that
// stops work arriving beside a decision that permits it, which is what a
// partially forged or partially stale response looks like.
//
// Every status is crossed with every outcome, and the rule is deliberately
// offering blind, so the table is the whole domain rather than a sample of it.
func TestValidateOutcomeClosesEveryStatusAndOutcomePair(t *testing.T) {
	t.Parallel()

	outcomes := []lease.Outcome{lease.OutcomeGrant, lease.OutcomeRefusal, lease.OutcomeRevocation}

	for _, status := range allProductStatuses() {
		for _, outcome := range outcomes {
			want := wantAdmitted(status, outcome)
			err := status.ValidateOutcome(outcome)
			if got := err == nil; got != want {
				t.Errorf("ProductStatus(%v).ValidateOutcome(%v) admitted = %t, want %t (error = %v)",
					status, outcome, got, want, err)
			}
		}
	}
}

// wantAdmitted states the rule independently of the implementation, in the
// product's own words rather than by calling the code under test.
//
// A grant needs a paying status. A revocation must say revoked and nothing
// else. A refusal must name a reason a customer can act on: stopped,
// upgrade-required, or read-only. What read-only means to a given product is
// product meaning the authority never learns; a product for which it describes
// nothing simply never issues it, and that policy lives with the issuer, not in
// this shared validator.
func wantAdmitted(status controlplane.ProductStatus, outcome lease.Outcome) bool {
	switch outcome {
	case lease.OutcomeGrant:
		return status == controlplane.ProductStatusActive ||
			status == controlplane.ProductStatusPaymentRetry
	case lease.OutcomeRevocation:
		return status == controlplane.ProductStatusRevoked
	case lease.OutcomeRefusal:
		return status == controlplane.ProductStatusStopped ||
			status == controlplane.ProductStatusUpgradeRequired ||
			status == controlplane.ProductStatusReadOnly
	}
	return false
}

// TestReadOnlyRefusalIsOfferingBlind names the deleted asymmetry on its own:
// read-only refusals used to be admitted per offering, which put product
// meaning inside this shared validator and made a fourth offering silently
// deniable. The rule is now offering blind. Whether read-only describes
// anything for a given product is the issuer's policy, exercised by never
// sending the status, never by this validator knowing product names.
func TestReadOnlyRefusalIsOfferingBlind(t *testing.T) {
	t.Parallel()

	if err := controlplane.ProductStatusReadOnly.ValidateOutcome(lease.OutcomeRefusal); err != nil {
		t.Fatalf("read-only refusal error = %v, want admitted for every product alike", err)
	}
}

// TestProductStatusRefusesEveryValueOutsideTheClosedSet walks the whole byte
// space. An unnamed status must not be admitted, must not render text, and must
// not decide anything: a status a client cannot name is one it cannot obey.
func TestProductStatusRefusesEveryValueOutsideTheClosedSet(t *testing.T) {
	t.Parallel()

	for value := 0; value <= maximumByteOrdinal; value++ {
		status := controlplane.ProductStatus(value)
		if status.IsValid() {
			continue
		}
		if err := status.Validate(); err == nil {
			t.Fatalf("ProductStatus(%d).Validate() error = nil, want a refusal", value)
		}
		if got := status.String(); got != "" {
			t.Fatalf("ProductStatus(%d).String() = %q, want empty text", value, got)
		}
		if status.AdmitsGrant() {
			t.Fatalf("ProductStatus(%d).AdmitsGrant() = true, want false", value)
		}
		if err := status.ValidateOutcome(lease.OutcomeRefusal); err == nil {
			t.Fatalf("ProductStatus(%d).ValidateOutcome() error = nil, want a refusal", value)
		}
	}
}

// TestValidateOutcomeRefusesAnUnnamedOutcome closes the second input. The rule
// reads both, so an unset outcome must refuse rather than fall through to
// whatever the status alone would have decided.
func TestValidateOutcomeRefusesAnUnnamedOutcome(t *testing.T) {
	t.Parallel()

	if err := controlplane.ProductStatusActive.ValidateOutcome(lease.Outcome(0)); err == nil {
		t.Errorf("ValidateOutcome() with an unset outcome error = nil, want a refusal")
	}
}
