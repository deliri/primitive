package controlplane_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
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
// Every status is crossed with every outcome for every offering, so the table
// is the whole domain rather than a sample of it.
func TestValidateOutcomeClosesEveryStatusAndOutcomePair(t *testing.T) {
	t.Parallel()

	offerings := []core.Offering{core.OfferingBug, core.OfferingWitness, core.OfferingPeachfuzz}
	outcomes := []lease.Outcome{lease.OutcomeGrant, lease.OutcomeRefusal, lease.OutcomeRevocation}

	for _, offering := range offerings {
		t.Run(offering.String(), func(t *testing.T) {
			t.Parallel()

			for _, status := range allProductStatuses() {
				for _, outcome := range outcomes {
					want := wantAdmitted(status, outcome, offering)
					err := status.ValidateOutcome(offering, outcome)
					if got := err == nil; got != want {
						t.Errorf("ProductStatus(%v).ValidateOutcome(%v, %v) admitted = %t, want %t (error = %v)",
							status, offering, outcome, got, want, err)
					}
				}
			}
		})
	}
}

// wantAdmitted states the rule independently of the implementation, in the
// product's own words rather than by calling the code under test.
//
// A grant needs a paying status. A revocation must say revoked and nothing
// else. A refusal must name a reason a customer can act on: stopped and
// upgrade-required always do, and read-only does only where it describes a real
// state, which is an offering that retains something to read. Peachfuzz
// produces new work and keeps no licensed readable state, so read-only is not a
// state it can be in.
func wantAdmitted(status controlplane.ProductStatus, outcome lease.Outcome, offering core.Offering) bool {
	switch outcome {
	case lease.OutcomeGrant:
		return status == controlplane.ProductStatusActive ||
			status == controlplane.ProductStatusPaymentRetry
	case lease.OutcomeRevocation:
		return status == controlplane.ProductStatusRevoked
	case lease.OutcomeRefusal:
		if status == controlplane.ProductStatusStopped ||
			status == controlplane.ProductStatusUpgradeRequired {
			return true
		}
		return status == controlplane.ProductStatusReadOnly &&
			(offering == core.OfferingBug || offering == core.OfferingWitness)
	}
	return false
}

// TestReadOnlyRefusalIsOfferingDependent names the one asymmetry in the matrix
// on its own, because it is a product rule rather than an arithmetic one and a
// reader should not have to derive it from a loop.
func TestReadOnlyRefusalIsOfferingDependent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		offering     core.Offering
		wantAdmitted bool
	}{
		{name: "bug retains readable evidence", offering: core.OfferingBug, wantAdmitted: true},
		{name: "witness retains readable evidence", offering: core.OfferingWitness, wantAdmitted: true},
		{name: "peachfuzz only produces new work", offering: core.OfferingPeachfuzz, wantAdmitted: false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := controlplane.ProductStatusReadOnly.ValidateOutcome(testCase.offering, lease.OutcomeRefusal)
			if got := err == nil; got != testCase.wantAdmitted {
				t.Fatalf("read-only refusal for %v admitted = %t, want %t (error = %v)",
					testCase.offering, got, testCase.wantAdmitted, err)
			}
		})
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
		if status.AdmitsOutcome(lease.OutcomeGrant) {
			t.Fatalf("ProductStatus(%d).AdmitsOutcome(grant) = true, want false", value)
		}
		if err := status.ValidateOutcome(core.OfferingBug, lease.OutcomeRefusal); err == nil {
			t.Fatalf("ProductStatus(%d).ValidateOutcome() error = nil, want a refusal", value)
		}
	}
}

// TestValidateOutcomeRefusesAnUnnamedOfferingOrOutcome closes the other two
// inputs. The rule reads all three, so an unset offering or outcome must refuse
// rather than fall through to whatever the status alone would have decided.
func TestValidateOutcomeRefusesAnUnnamedOfferingOrOutcome(t *testing.T) {
	t.Parallel()

	if err := controlplane.ProductStatusActive.ValidateOutcome(
		core.OfferingUnknown, lease.OutcomeGrant,
	); err == nil {
		t.Errorf("ValidateOutcome() with an unset offering error = nil, want a refusal")
	}
	if err := controlplane.ProductStatusActive.ValidateOutcome(
		core.OfferingBug, lease.Outcome(0),
	); err == nil {
		t.Errorf("ValidateOutcome() with an unset outcome error = nil, want a refusal")
	}
	if controlplane.ProductStatusActive.AdmitsOutcome(lease.Outcome(0)) {
		t.Errorf("AdmitsOutcome() with an unset outcome = true, want false")
	}
}
