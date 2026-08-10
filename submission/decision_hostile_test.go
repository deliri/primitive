package submission

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

type reuseEvidenceFixtureRequest struct {
	Request   RequestPayload
	KeyByte   byte
	ScopeByte byte
}

type reuseEvidenceFixture struct {
	evidence receipt.EvidenceDocument
	account  receipt.AccountIdentity
	offering receipt.OfferingIdentity
}

func TestSubmissionDecisionAuthenticatesUploadAndScopedReuseArms(t *testing.T) {
	t.Parallel()

	grant := newGrantFixture(t, grantFixtureRequest{})
	reuse := newReuseEvidenceFixture(t, reuseEvidenceFixtureRequest{
		Request: grant.request, KeyByte: 0x41, ScopeByte: 0x61,
	})
	cases := []struct {
		name       string
		projection DecisionProjection
		wantKind   DecisionKind
	}{
		{
			name:       "fresh upload grant",
			projection: mustUploadDecision(t, grant.projection),
			wantKind:   DecisionUpload,
		},
		{
			name:       "same-scope accepted object reuse",
			projection: mustReuseDecision(t, reuse.evidence),
			wantKind:   DecisionReuse,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := testCase.projection.MarshalJSON()
			if err != nil {
				t.Fatalf("DecisionProjection.MarshalJSON() error = %v, want nil", err)
			}
			var document DecisionDocument
			if err := document.UnmarshalJSON(encoded); err != nil {
				t.Fatalf("DecisionDocument.UnmarshalJSON() error = %v, want nil", err)
			}
			verified, err := VerifyDecision(DecisionExpectation{
				Decision: document, Request: grant.request,
				Account: reuse.account, Offering: reuse.offering,
				ObservedAt:  temporal.InstantFromNanoseconds(testGrantIssuedAt),
				TrustedKeys: grant.trusted,
			})
			if err != nil {
				t.Fatalf("VerifyDecision() error = %v, want nil", err)
			}
			kind, err := verified.Kind()
			if err != nil || kind != testCase.wantKind {
				t.Fatalf("VerifiedDecision.Kind() = (%v, %v), want (%v, nil)",
					kind, err, testCase.wantKind)
			}
			grantProof, hasGrant := verified.Grant()
			evidenceProof, hasEvidence := verified.Evidence()
			if testCase.wantKind == DecisionUpload {
				if !hasGrant || hasEvidence || grantProof.Validate() != nil {
					t.Fatalf("upload projections = (grant %v/%v, evidence %v/%v), want only valid grant",
						grantProof, hasGrant, evidenceProof, hasEvidence)
				}
				return
			}
			if hasGrant || !hasEvidence || evidenceProof.Validate() != nil {
				t.Fatalf("reuse projections = (grant %v/%v, evidence %v/%v), want only valid evidence",
					grantProof, hasGrant, evidenceProof, hasEvidence)
			}
		})
	}
}

func TestSubmissionReuseDecisionRefusesCrossTenantAndIntegrityOracleProbes(t *testing.T) {
	t.Parallel()

	grant := newGrantFixture(t, grantFixtureRequest{})
	reuse := newReuseEvidenceFixture(t, reuseEvidenceFixtureRequest{
		Request: grant.request, KeyByte: 0x41, ScopeByte: 0x62,
	})
	document := decodeDecisionProjection(t, mustReuseDecision(t, reuse.evidence))
	other := newReuseEvidenceFixture(t, reuseEvidenceFixtureRequest{
		Request: grant.request, KeyByte: 0x41, ScopeByte: 0x72,
	})
	if got, gotErr := VerifyDecision(DecisionExpectation{
		Decision: document, Request: grant.request,
		Account: other.account, Offering: other.offering,
		ObservedAt:  temporal.InstantFromNanoseconds(testGrantIssuedAt),
		TrustedKeys: grant.trusted,
	}); !errors.Is(gotErr, core.ErrReceiptScope) || got != (VerifiedDecision{}) {
		t.Fatalf("VerifyDecision(cross-tenant reuse) = (%v, %v), want zero and errors.Is %v",
			got, gotErr, core.ErrReceiptScope)
	}

	differentRequest := grant.request
	differentRequest.Declaration = testDeclaration(t, []byte{0x7f})
	if got, gotErr := VerifyDecision(DecisionExpectation{
		Decision: document, Request: differentRequest,
		Account: reuse.account, Offering: reuse.offering,
		ObservedAt:  temporal.InstantFromNanoseconds(testGrantIssuedAt),
		TrustedKeys: grant.trusted,
	}); !errors.Is(gotErr, core.ErrControlPlaneResponseBinding) || got != (VerifiedDecision{}) {
		t.Fatalf("VerifyDecision(different declaration) = (%v, %v), want zero and errors.Is %v",
			got, gotErr, core.ErrControlPlaneResponseBinding)
	}
}

func TestSubmissionDecisionTaggedUnionRefusesAbsentAndContradictoryArms(t *testing.T) {
	t.Parallel()

	grant := newGrantFixture(t, grantFixtureRequest{})
	reuse := newReuseEvidenceFixture(t, reuseEvidenceFixtureRequest{
		Request: grant.request, KeyByte: 0x41, ScopeByte: 0x63,
	})
	grantDocument := grant.document
	evidence := reuse.evidence
	cases := []struct {
		name     string
		document DecisionDocument
	}{
		{name: "zero decision", document: DecisionDocument{}},
		{name: "upload omits grant", document: DecisionDocument{Kind: DecisionUpload}},
		{
			name: "upload carries both arms",
			document: DecisionDocument{
				Kind: DecisionUpload, Grant: &grantDocument, Evidence: &evidence,
			},
		},
		{name: "reuse omits evidence", document: DecisionDocument{Kind: DecisionReuse}},
		{
			name: "reuse carries both arms",
			document: DecisionDocument{
				Kind: DecisionReuse, Grant: &grantDocument, Evidence: &evidence,
			},
		},
		{
			name:     "kind above domain",
			document: DecisionDocument{Kind: DecisionKind(255), Evidence: &evidence},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if gotErr := testCase.document.Validate(); !errors.Is(gotErr, core.ErrControlPlaneContract) {
				t.Fatalf("DecisionDocument.Validate() error = %v, want errors.Is %v",
					gotErr, core.ErrControlPlaneContract)
			}
		})
	}
}

func TestSubmissionDecisionStrictJSONRefusesUnknownAndOversizeWithoutMutation(t *testing.T) {
	t.Parallel()

	grant := newGrantFixture(t, grantFixtureRequest{})
	reuse := newReuseEvidenceFixture(t, reuseEvidenceFixtureRequest{
		Request: grant.request, KeyByte: 0x41, ScopeByte: 0x64,
	})
	before := decodeDecisionProjection(t, mustReuseDecision(t, reuse.evidence))
	encoded, err := mustReuseDecision(t, reuse.evidence).MarshalJSON()
	if err != nil {
		t.Fatalf("DecisionProjection.MarshalJSON() error = %v, want nil", err)
	}
	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: []byte{}},
		{name: "null", data: []byte("null")},
		{name: "array", data: []byte{'[', ']'}},
		{name: "trailing object", data: append(append([]byte(nil), encoded...), '{', '}')},
		{name: "oversize", data: make([]byte, int(DecisionDocumentJSONMaximumBytes)+1)},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := before
			gotErr := got.UnmarshalJSON(testCase.data)
			if !errors.Is(gotErr, core.ErrJSONContract) || !sameReuseDecision(got, before) {
				t.Fatalf("DecisionDocument.UnmarshalJSON(%q) error = %v, want preserved and errors.Is %v",
					testCase.data, gotErr, core.ErrJSONContract)
			}
		})
	}
}

func newReuseEvidenceFixture(
	t *testing.T,
	request reuseEvidenceFixtureRequest,
) reuseEvidenceFixture {
	t.Helper()

	account := submissionLifecycleIdentity(t, request.ScopeByte, receipt.NewAccountIdentity)
	offering := submissionLifecycleIdentity(t, request.ScopeByte+1, receipt.NewOfferingIdentity)
	submission := submissionLifecycleIdentity(t, request.ScopeByte+2, receipt.NewSubmissionIdentity)
	object := submissionLifecycleIdentity(t, request.ScopeByte+3, receipt.NewObjectIdentity)
	receiptBytes := [receipt.ReceiptIDBytes]byte{}
	receiptBytes[0] = request.ScopeByte + 4
	identity, err := receipt.NewReceiptID(receiptBytes)
	if err != nil {
		t.Fatalf("receipt.NewReceiptID() error = %v, want nil", err)
	}
	_, private := testSigningKey(t, request.KeyByte)
	document, err := receipt.IssueEvidence(receipt.IssueEvidenceRequest{
		Key: private, Identity: identity, Account: account, Offering: offering,
		OccurredAt: temporal.InstantFromNanoseconds(testGrantIssuedAt),
		Body: receipt.EvidenceBody{
			Extent:     request.Request.Declaration.Extent,
			SHA256:     request.Request.Declaration.SHA256,
			CRC32C:     request.Request.Declaration.CRC32C,
			Submission: submission, Object: object,
		},
	})
	if err != nil {
		t.Fatalf("receipt.IssueEvidence() error = %v, want nil", err)
	}
	return reuseEvidenceFixture{evidence: document, account: account, offering: offering}
}

func submissionLifecycleIdentity[T core.Validatable](
	t *testing.T,
	marker byte,
	constructor func([receipt.LifecycleIdentityBytes]byte) (T, error),
) T {
	t.Helper()
	value := [receipt.LifecycleIdentityBytes]byte{}
	value[0] = marker
	identity, err := constructor(value)
	if err != nil {
		t.Fatalf("submission lifecycle constructor error = %v, want nil", err)
	}
	return identity
}

func mustUploadDecision(t *testing.T, grant GrantProjection) DecisionProjection {
	t.Helper()
	decision, err := UploadDecision(grant)
	if err != nil {
		t.Fatalf("UploadDecision() error = %v, want nil", err)
	}
	return decision
}

func mustReuseDecision(t *testing.T, evidence receipt.EvidenceDocument) DecisionProjection {
	t.Helper()
	decision, err := ReuseDecision(evidence)
	if err != nil {
		t.Fatalf("ReuseDecision() error = %v, want nil", err)
	}
	return decision
}

func decodeDecisionProjection(t *testing.T, projection DecisionProjection) DecisionDocument {
	t.Helper()
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("DecisionProjection.MarshalJSON() error = %v, want nil", err)
	}
	var document DecisionDocument
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("json.Unmarshal(DecisionProjection) error = %v, want nil", err)
	}
	return document
}

func sameReuseDecision(left, right DecisionDocument) bool {
	return left.Kind == right.Kind && left.Grant == nil && right.Grant == nil &&
		left.Evidence != nil && right.Evidence != nil && *left.Evidence == *right.Evidence
}
