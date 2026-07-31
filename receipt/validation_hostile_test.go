package receipt

import (
	"crypto/ed25519"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestEvidenceOwnedValidationHostileMatrix(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 200)
	document := issueFixture(t, fixture)
	other := newReceiptFixture(t, 210)
	otherDocument := issueFixture(t, other)
	// wantErr is the oracle. Deriving the expected outcome from the case name
	// makes every future rename a silent behavior change.
	cases := []struct {
		value   core.Validatable
		wantErr error
		name    string
	}{
		{name: "exact header is admitted", value: document.Payload.Header},
		{name: "header without identity is rejected", value: func() Header { v := document.Payload.Header; v.Identity = ReceiptID{}; return v }(), wantErr: core.ErrReceiptContract},
		{name: "header without account is rejected", value: func() Header { v := document.Payload.Header; v.Account = AccountIdentity{}; return v }(), wantErr: core.ErrReceiptContract},
		{name: "header without offering is rejected", value: func() Header { v := document.Payload.Header; v.Offering = OfferingIdentity{}; return v }(), wantErr: core.ErrReceiptContract},
		{name: "header without revision is rejected", value: func() Header { v := document.Payload.Header; v.Revision = RevisionUnknown; return v }(), wantErr: core.ErrReceiptContract},
		{name: "header without occurrence is rejected", value: func() Header { v := document.Payload.Header; v.OccurredAt = temporal.Instant{}; return v }(), wantErr: core.ErrReceiptContract},
		{name: "exact expectation is admitted", value: fixture.expectation},
		{name: "expectation without account is rejected", value: func() EvidenceExpectation { v := fixture.expectation; v.Account = AccountIdentity{}; return v }(), wantErr: core.ErrReceiptContract},
		{name: "expectation without offering is rejected", value: func() EvidenceExpectation { v := fixture.expectation; v.Offering = OfferingIdentity{}; return v }(), wantErr: core.ErrReceiptContract},
		{name: "expectation without body is rejected", value: EvidenceExpectation{Account: fixture.account, Offering: fixture.offering}, wantErr: core.ErrReceiptContract},
		{name: "exact scope is admitted", value: Scope{Account: fixture.account, Offering: fixture.offering}},
		{name: "scope without account is rejected", value: Scope{Offering: fixture.offering}, wantErr: core.ErrReceiptContract},
		{name: "scope without offering is rejected", value: Scope{Account: fixture.account}, wantErr: core.ErrReceiptContract},
		{name: "valid scope mismatch is admitted", value: scopeMismatch{field: ScopeFieldObject}},
		{name: "unset scope mismatch is rejected", value: scopeMismatch{}, wantErr: core.ErrReceiptContract},
		{name: "future scope mismatch is rejected", value: scopeMismatch{field: ScopeField(math.MaxUint8)}, wantErr: core.ErrReceiptContract},
		{name: "payload without body is rejected", value: EvidencePayload{Header: document.Payload.Header}, wantErr: core.ErrReceiptContract},
		{name: "payload without header is rejected", value: EvidencePayload{Body: fixture.body}, wantErr: core.ErrReceiptContract},
		{name: "document without payload is rejected", value: EvidenceDocument{Attestation: document.Attestation}, wantErr: core.ErrReceiptContract},
		{name: "document without attestation is rejected", value: EvidenceDocument{Payload: document.Payload}, wantErr: core.ErrReceiptContract},
		{name: "document with wrong attestation is structurally admitted", value: func() EvidenceDocument { v := document; v.Attestation = otherDocument.Attestation; return v }()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.value.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestEvidenceRequestIngressHostileMatrix(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 220)
	document := issueFixture(t, fixture)
	issue := IssueEvidenceRequest{
		Identity: fixture.receipt, Account: fixture.account, Offering: fixture.offering,
		OccurredAt: fixture.occurredAt, Body: fixture.body, Key: fixture.private,
	}
	invalidKey := append(ed25519.PrivateKey(nil), fixture.private[:len(fixture.private)-1]...)
	issueCases := []struct {
		wantErr error
		name    string
		request IssueEvidenceRequest
	}{
		{name: "complete request signs", request: issue},
		{name: "unset identity is rejected", request: func() IssueEvidenceRequest { v := issue; v.Identity = ReceiptID{}; return v }(), wantErr: core.ErrReceiptContract},
		{name: "unset account is rejected", request: func() IssueEvidenceRequest { v := issue; v.Account = AccountIdentity{}; return v }(), wantErr: core.ErrReceiptContract},
		{name: "unset offering is rejected", request: func() IssueEvidenceRequest { v := issue; v.Offering = OfferingIdentity{}; return v }(), wantErr: core.ErrReceiptContract},
		{name: "unset occurrence is rejected", request: func() IssueEvidenceRequest { v := issue; v.OccurredAt = temporal.Instant{}; return v }(), wantErr: core.ErrReceiptContract},
		{name: "unset body is rejected", request: func() IssueEvidenceRequest { v := issue; v.Body = EvidenceBody{}; return v }(), wantErr: core.ErrReceiptContract},
		{name: "nil key is refused by Attest", request: func() IssueEvidenceRequest { v := issue; v.Key = nil; return v }(), wantErr: core.ErrAttestContract},
		{name: "short key is refused by Attest", request: func() IssueEvidenceRequest { v := issue; v.Key = invalidKey; return v }(), wantErr: core.ErrAttestContract},
	}
	for _, tc := range issueCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := IssueEvidence(tc.request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("IssueEvidence() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && got != (EvidenceDocument{}) {
				t.Fatalf("IssueEvidence(rejected) result = %v, want zero", got)
			}
		})
	}

	verify := VerifyEvidenceRequest{
		Document: document, TrustedKeys: fixture.trusted, Expected: fixture.expectation,
	}
	verifyCases := []struct {
		wantErr error
		name    string
		request VerifyEvidenceRequest
	}{
		{name: "complete request verifies", request: verify},
		{name: "unset document is verification failure", request: func() VerifyEvidenceRequest { v := verify; v.Document = EvidenceDocument{}; return v }(), wantErr: core.ErrReceiptVerification},
		{name: "unset trust is verification failure", request: func() VerifyEvidenceRequest { v := verify; v.TrustedKeys = attest.TrustedKeys{}; return v }(), wantErr: core.ErrReceiptVerification},
		{name: "unset expectation is contract failure", request: func() VerifyEvidenceRequest { v := verify; v.Expected = EvidenceExpectation{}; return v }(), wantErr: core.ErrReceiptContract},
	}
	for _, tc := range verifyCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := VerifyEvidence(tc.request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("VerifyEvidence() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && got != (VerifiedEvidence{}) {
				t.Fatalf("VerifyEvidence(rejected) result = %v, want zero", got)
			}
		})
	}
}
