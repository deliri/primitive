package controlplane_test

import (
	"crypto/ed25519"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// TestRegistrationPayloadHoldsEveryOutcomeToTheStatusRule closes the arm that
// decides whether a signed decision is self-consistent.
//
// The rule has two halves and they belong to different owners. Which status may
// travel beside an outcome is ProductStatus's, and ValidateOutcome states it for
// one offering. Whether a credential accompanies the decision is this document's,
// because only the document knows a grant hands one over and a revocation must
// not. The table below is the product of the two, computed from the delegated
// rule rather than restated, so a change to either owner has to move this test
// rather than leave it agreeing by coincidence.
//
// The refusal arm is what this exists for. A refusal that names an active status
// is a signed contradiction: the authority is simultaneously saying the
// installation may not proceed and that nothing is wrong. Read-only is the case
// that cannot be decided without the offering, which is why the offering the
// header names is the one that must reach the rule.
func TestRegistrationPayloadHoldsEveryOutcomeToTheStatusRule(t *testing.T) {
	t.Parallel()

	base := issueTestRegistration(t).document.Payload
	_, signer := testSigningKey(t, 1)
	offering := base.Header.Offering
	outcomes := [...]lease.Outcome{lease.OutcomeGrant, lease.OutcomeRefusal, lease.OutcomeRevocation}

	statuses := validProductStatuses(t)
	if len(statuses) == 0 {
		t.Fatal("valid product statuses = 0, want the closed commercial domain")
	}
	for _, status := range statuses {
		for _, outcome := range outcomes {
			t.Run(status.String()+" beside a "+outcome.String(), func(t *testing.T) {
				t.Parallel()

				payload := registrationPayloadFor(t, base, signer, status, outcome)
				// want is derived, never spelled. ValidateOutcome owns which
				// status may accompany the outcome for this offering; the
				// credential half is this document's and is stated here because
				// it is stated nowhere else.
				wantAdmitted := status.ValidateOutcome(offering, outcome) == nil
				gotErr := payload.Validate()
				if wantAdmitted && gotErr != nil {
					t.Fatalf("RegistrationPayload.Validate() error = %v, want nil for %v beside a %v", gotErr, status, outcome)
				}
				if wantAdmitted {
					return
				}
				if !errors.Is(gotErr, core.ErrControlPlaneContract) || !errors.Is(gotErr, core.ErrControlPlaneDecisionConsistency) {
					t.Fatalf("RegistrationPayload.Validate() error = %v, want errors.Is(..., %v, %v) for %v beside a %v", gotErr, core.ErrControlPlaneContract, core.ErrControlPlaneDecisionConsistency, status, outcome)
				}
			})
		}
	}
}

// TestRegistrationPayloadRefusalNamesWhyAcrossTheWholeStatusDomain states the
// refusal arm as a fact rather than as a derivation, so the two tests cannot
// drift together.
//
// The table above computes its expectation from ValidateOutcome. If that rule
// were itself weakened, both it and production would move and the table would
// stay green. This one names the four statuses a refusal may not carry, so the
// contract survives even a change to the rule it delegates to.
func TestRegistrationPayloadRefusalNamesWhyAcrossTheWholeStatusDomain(t *testing.T) {
	t.Parallel()

	base := issueTestRegistration(t).document.Payload
	_, signer := testSigningKey(t, 1)
	cases := []struct {
		name         string
		status       controlplane.ProductStatus
		wantAdmitted bool
	}{
		{name: "stopped explains a refusal", status: controlplane.ProductStatusStopped, wantAdmitted: true},
		{name: "upgrade required explains a refusal", status: controlplane.ProductStatusUpgradeRequired, wantAdmitted: true},
		{name: "active contradicts a refusal", status: controlplane.ProductStatusActive},
		{name: "payment retry contradicts a refusal", status: controlplane.ProductStatusPaymentRetry},
		{name: "revoked belongs to a revocation not a refusal", status: controlplane.ProductStatusRevoked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			payload := registrationPayloadFor(t, base, signer, tc.status, lease.OutcomeRefusal)
			gotErr := payload.Validate()
			if tc.wantAdmitted != (gotErr == nil) {
				t.Fatalf("RegistrationPayload.Validate() error = %v, want admitted %t for %v beside a refusal", gotErr, tc.wantAdmitted, tc.status)
			}
		})
	}

	// Read-only is the one status whose verdict the offering decides, so it is
	// asserted against the offering this payload actually carries rather than
	// against a remembered answer.
	payload := registrationPayloadFor(t, base, signer, controlplane.ProductStatusReadOnly, lease.OutcomeRefusal)
	wantReadOnly := controlplane.ProductStatusReadOnly.ValidateOutcome(payload.Header.Offering, lease.OutcomeRefusal) == nil
	if gotErr := payload.Validate(); wantReadOnly != (gotErr == nil) {
		t.Fatalf("read-only refusal for %v: RegistrationPayload.Validate() error = %v, want admitted %t", payload.Header.Offering, gotErr, wantReadOnly)
	}
}

// TestRegistrationDocumentsBindTheirEnvelopeToTheBodysDomain closes the
// decode-time half of signing-domain confusion.
//
// Verification recomputes the domain from the body, so a mismatched envelope
// cannot survive attest.Verify either way. What it can do without this is
// survive decoding: a document read off the wire and held, logged, or passed on
// before anyone verifies it would carry an envelope claiming one namespace over
// a body that declares another. A shape check that admits a document its own
// verifier will refuse is a shape check that has not closed.
func TestRegistrationDocumentsBindTheirEnvelopeToTheBodysDomain(t *testing.T) {
	t.Parallel()

	issued := issueTestRegistration(t)
	document := issued.document
	certificate := *document.Payload.Certificate

	foreign := controlplane.SigningDomainCheckInV1
	if foreign == document.Payload.AttestationDomain() || foreign == certificate.Body.AttestationDomain() {
		t.Fatalf("foreign domain %v collides with a document's own, want a namespace neither declares", foreign)
	}

	mismatchedResponse := document
	mismatchedResponse.Attestation.Domain = foreign
	if gotErr := mismatchedResponse.Validate(); !errors.Is(gotErr, core.ErrControlPlaneSigningDomain) {
		t.Fatalf("RegistrationDocument.Validate() with a foreign envelope domain error = %v, want %v", gotErr, core.ErrControlPlaneSigningDomain)
	}

	mismatchedCertificate := certificate
	mismatchedCertificate.Attestation.Domain = foreign
	if gotErr := mismatchedCertificate.Validate(); !errors.Is(gotErr, core.ErrControlPlaneSigningDomain) {
		t.Fatalf("InstallationCertificateDocument.Validate() with a foreign envelope domain error = %v, want %v", gotErr, core.ErrControlPlaneSigningDomain)
	}

	// The genuine documents must still pass, or the check above would be
	// satisfied by refusing everything.
	if err := document.Validate(); err != nil {
		t.Fatalf("genuine RegistrationDocument.Validate() error = %v, want nil", err)
	}
	if err := certificate.Validate(); err != nil {
		t.Fatalf("genuine InstallationCertificateDocument.Validate() error = %v, want nil", err)
	}
}

// registrationPayloadFor rebuilds one payload around the exact status and
// outcome under test, leaving every other fact the authority's.
//
// The lease is re-signed rather than edited because the outcome is part of the
// signed decision. The credential follows the outcome's own rule so that a row
// testing the status arm cannot fail for the unrelated reason that a revocation
// carried a certificate.
func registrationPayloadFor(t *testing.T, base controlplane.RegistrationPayload, signer ed25519.PrivateKey, status controlplane.ProductStatus, outcome lease.Outcome) controlplane.RegistrationPayload {
	t.Helper()

	payload := base
	payload.Header.Status = status
	payload.Lease = resignLease(t, leaseDocumentFor(t, base.Lease, outcome, signer), signer)
	if outcome == lease.OutcomeRevocation {
		payload.Certificate = nil
	}
	return payload
}

// leaseDocumentFor issues one unsigned-yet decision of the requested outcome
// under the golden lease's own header, so subject, generation and issued-at
// stay bound to the rest of the payload.
func leaseDocumentFor(t *testing.T, base lease.Document, outcome lease.Outcome, signer ed25519.PrivateKey) lease.Document {
	t.Helper()

	header, err := base.Decision.Header()
	if err != nil {
		t.Fatalf("lease.Decision.Header() error = %v, want nil", err)
	}
	grant, err := base.Decision.Grant()
	if err != nil {
		t.Fatalf("lease.Decision.Grant() error = %v, want the golden's grant", err)
	}
	decision, err := newLeaseDecision(header, grant, outcome)
	if err != nil {
		t.Fatalf("new %v decision error = %v, want nil", outcome, err)
	}
	envelope, err := attest.Sign(attest.SignRequest[lease.Domain]{Body: decision, Signer: signer})
	if err != nil {
		t.Fatalf("attest.Sign(lease decision) error = %v, want nil", err)
	}
	return lease.Document{Decision: decision, Attestation: envelope}
}

func newLeaseDecision(header lease.Header, grant lease.Grant, outcome lease.Outcome) (lease.Decision, error) {
	switch outcome {
	case lease.OutcomeGrant:
		return lease.NewGrantDecision(lease.GrantDecisionRequest{Header: header, Grant: grant})
	case lease.OutcomeRefusal:
		return lease.NewRefusalDecision(lease.RefusalDecisionRequest{
			Header: header, Refusal: lease.Refusal{ContactAfter: grant.ContactAfter},
		})
	case lease.OutcomeRevocation:
		return lease.NewRevocationDecision(lease.RevocationDecisionRequest{
			Header: header, Revocation: lease.Revocation{Reason: lease.RevocationReasonLicenceBreach},
		})
	}
	return lease.Decision{}, errors.New("unreachable outcome")
}

// validProductStatuses walks the whole byte domain so a status added to the
// enum joins the matrix without anyone remembering to add a row.
func validProductStatuses(t *testing.T) []controlplane.ProductStatus {
	t.Helper()

	var valid []controlplane.ProductStatus
	for value := range math.MaxUint8 + 1 {
		status := controlplane.ProductStatus(value)
		if status.Validate() == nil {
			valid = append(valid, status)
		}
	}
	return valid
}
