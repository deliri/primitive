package submission

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
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
	declaration Declaration
	trusted     attest.TrustedKeys
	evidence    receipt.EvidenceDocument
	account     receipt.AccountIdentity
	offering    receipt.OfferingIdentity
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
			projection: mustReuseDecision(t, reuse),
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
	document := decodeDecisionProjection(t, mustReuseDecision(t, reuse))
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

func TestReuseDecisionAuthorityBoundaryRefusesEveryForeignOrUnauthenticatedCandidate(t *testing.T) {
	t.Parallel()

	grant := newGrantFixture(t, grantFixtureRequest{})
	reuse := newReuseEvidenceFixture(t, reuseEvidenceFixtureRequest{
		Request: grant.request, KeyByte: 0x41, ScopeByte: 0x66,
	})
	other := newReuseEvidenceFixture(t, reuseEvidenceFixtureRequest{
		Request: grant.request, KeyByte: 0x41, ScopeByte: 0x76,
	})
	foreignOffering, err := receipt.OfferingIdentityFor(core.OfferingBug)
	if err != nil {
		t.Fatalf("receipt.OfferingIdentityFor(%v) error = %v, want nil", core.OfferingBug, err)
	}
	foreignPublic, _ := testSigningKey(t, 0x42)
	foreignTrust, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{foreignPublic},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys(foreign) error = %v, want nil", err)
	}
	foreignAuthority := newReuseEvidenceFixture(t, reuseEvidenceFixtureRequest{
		Request: grant.request, KeyByte: 0x42, ScopeByte: 0x66,
	})
	shorter := reuse.declaration
	shorter.Extent = testByteLength(t, reuse.declaration.Extent.Uint64()-1)
	longer := reuse.declaration
	longer.Extent = testByteLength(t, reuse.declaration.Extent.Uint64()+1)
	differentSHA := reuse.declaration
	differentSHA.SHA256 = core.SHA256Of([]byte("foreign digest"))
	differentCRC := reuse.declaration
	differentCRC.CRC32C = core.NewCRC32C(1)
	tampered := reuse.evidence
	tampered.Payload.Header.Account = other.account
	cases := []struct {
		wantErr   error
		name      string
		request   ReuseDecisionRequest
		wantField receipt.ScopeField
	}{
		{name: "zero request", wantErr: core.ErrControlPlaneContract},
		{name: "zero evidence", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(reuse)
			value.Evidence = receipt.EvidenceDocument{}
			return value
		}(), wantErr: core.ErrControlPlaneContract},
		{name: "zero declaration", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(reuse)
			value.Declaration = Declaration{}
			return value
		}(), wantErr: core.ErrControlPlaneContract},
		{name: "zero account", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(reuse)
			value.Account = receipt.AccountIdentity{}
			return value
		}(), wantErr: core.ErrControlPlaneContract},
		{name: "zero offering", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(reuse)
			value.Offering = receipt.OfferingIdentity{}
			return value
		}(), wantErr: core.ErrControlPlaneContract},
		{name: "zero trust", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(reuse)
			value.TrustedKeys = attest.TrustedKeys{}
			return value
		}(), wantErr: core.ErrControlPlaneContract},
		{name: "foreign account with identical digests", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(reuse)
			value.Account = other.account
			return value
		}(), wantErr: core.ErrReceiptScope, wantField: receipt.ScopeFieldAccount},
		{name: "foreign account with foreign digest", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(reuse)
			value.Account = other.account
			value.Declaration = differentSHA
			return value
		}(), wantErr: core.ErrReceiptScope, wantField: receipt.ScopeFieldAccount},
		{name: "foreign offering with identical digests", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(reuse)
			value.Offering = foreignOffering
			return value
		}(), wantErr: core.ErrReceiptScope, wantField: receipt.ScopeFieldOffering},
		{name: "foreign offering with foreign digest", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(reuse)
			value.Offering = foreignOffering
			value.Declaration = differentCRC
			return value
		}(), wantErr: core.ErrReceiptScope, wantField: receipt.ScopeFieldOffering},
		{name: "extent one below", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(reuse)
			value.Declaration = shorter
			return value
		}(), wantErr: core.ErrControlPlaneResponseBinding},
		{name: "extent one above", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(reuse)
			value.Declaration = longer
			return value
		}(), wantErr: core.ErrControlPlaneResponseBinding},
		{name: "foreign SHA-256", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(reuse)
			value.Declaration = differentSHA
			return value
		}(), wantErr: core.ErrControlPlaneResponseBinding},
		{name: "foreign CRC32C", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(reuse)
			value.Declaration = differentCRC
			return value
		}(), wantErr: core.ErrControlPlaneResponseBinding},
		{name: "foreign signing authority", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(foreignAuthority)
			value.TrustedKeys = reuse.trusted
			return value
		}(), wantErr: core.ErrReceiptVerification},
		{name: "tampered signed account", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(reuse)
			value.Evidence = tampered
			value.Account = other.account
			return value
		}(), wantErr: core.ErrReceiptVerification},
		{name: "unrelated trust set", request: func() ReuseDecisionRequest {
			value := reuseDecisionRequest(reuse)
			value.TrustedKeys = foreignTrust
			return value
		}(), wantErr: core.ErrReceiptVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			projection, gotErr := ReuseDecision(tc.request)
			if !errors.Is(gotErr, tc.wantErr) || projection != (DecisionProjection{}) {
				t.Fatalf("ReuseDecision(%s) = (%v, %v), want invalid zero projection and errors.Is %v",
					tc.name, projection, gotErr, tc.wantErr)
			}
			if !errors.Is(tc.wantErr, core.ErrControlPlaneContract) &&
				!errors.Is(gotErr, core.ErrControlPlaneResponseBinding) {
				t.Fatalf("ReuseDecision(%s) error = %v, want errors.Is %v",
					tc.name, gotErr, core.ErrControlPlaneResponseBinding)
			}
			if tc.wantField == receipt.ScopeFieldUnknown {
				return
			}
			var mismatch receipt.ScopeMismatch
			if !errors.As(gotErr, &mismatch) {
				t.Fatalf("ReuseDecision(%s) error = %v, want receipt.ScopeMismatch", tc.name, gotErr)
			}
			field, fieldErr := mismatch.Field()
			if fieldErr != nil || field != tc.wantField {
				t.Fatalf("ReuseDecision(%s) scope field = (%v, %v), want (%v, nil)",
					tc.name, field, fieldErr, tc.wantField)
			}
		})
	}
}

func TestReuseDecisionAuthorityBoundaryAdmitsTenExactSameScopeCandidates(t *testing.T) {
	t.Parallel()

	for extent := 1; extent <= 10; extent++ {
		extent := extent
		t.Run("exact extent "+strconv.Itoa(extent), func(t *testing.T) {
			t.Parallel()
			grant := newGrantFixture(t, grantFixtureRequest{content: bytes.Repeat([]byte{byte(extent)}, extent)})
			reuse := newReuseEvidenceFixture(t, reuseEvidenceFixtureRequest{
				Request: grant.request, KeyByte: 0x41, ScopeByte: byte(0x80 + extent),
			})
			projection, gotErr := ReuseDecision(reuseDecisionRequest(reuse))
			if gotErr != nil || projection.Validate() != nil {
				t.Fatalf("ReuseDecision(exact extent %d) = (%v, %v), want valid projection and nil",
					extent, projection, gotErr)
			}
			document := decodeDecisionProjection(t, projection)
			verified, gotErr := VerifyDecision(DecisionExpectation{
				Decision: document, Request: grant.request, Account: reuse.account,
				Offering: reuse.offering, ObservedAt: temporal.InstantFromNanoseconds(testGrantIssuedAt),
				TrustedKeys: reuse.trusted,
			})
			if gotErr != nil || verified.Validate() != nil {
				t.Fatalf("VerifyDecision(exact extent %d) = (%v, %v), want valid proof and nil",
					extent, verified, gotErr)
			}
		})
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
	before := decodeDecisionProjection(t, mustReuseDecision(t, reuse))
	encoded, err := mustReuseDecision(t, reuse).MarshalJSON()
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
	t testing.TB,
	request reuseEvidenceFixtureRequest,
) reuseEvidenceFixture {
	t.Helper()

	account := submissionLifecycleIdentity(t, request.ScopeByte, receipt.NewAccountIdentity)
	offering, offeringErr := receipt.OfferingIdentityFor(core.OfferingWitness)
	if offeringErr != nil {
		t.Fatalf("receipt.OfferingIdentityFor(%v) error = %v, want nil", core.OfferingWitness, offeringErr)
	}
	submission := submissionLifecycleIdentity(t, request.ScopeByte+2, receipt.NewSubmissionIdentity)
	object := submissionLifecycleIdentity(t, request.ScopeByte+3, receipt.NewObjectIdentity)
	receiptBytes := [receipt.ReceiptIDBytes]byte{}
	receiptBytes[0] = request.ScopeByte + 4
	identity, err := receipt.NewReceiptID(receiptBytes)
	if err != nil {
		t.Fatalf("receipt.NewReceiptID() error = %v, want nil", err)
	}
	public, private := testSigningKey(t, request.KeyByte)
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{public},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}
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
	return reuseEvidenceFixture{
		evidence: document, declaration: request.Request.Declaration,
		trusted: trusted, account: account, offering: offering,
	}
}

func submissionLifecycleIdentity[T core.Validatable](
	t testing.TB,
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

func reuseDecisionRequest(fixture reuseEvidenceFixture) ReuseDecisionRequest {
	return ReuseDecisionRequest{
		Evidence: fixture.evidence, Declaration: fixture.declaration,
		Account: fixture.account, Offering: fixture.offering, TrustedKeys: fixture.trusted,
	}
}

func mustReuseDecision(t *testing.T, fixture reuseEvidenceFixture) DecisionProjection {
	t.Helper()
	decision, err := ReuseDecision(reuseDecisionRequest(fixture))
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
