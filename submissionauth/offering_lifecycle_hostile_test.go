package submissionauth

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/submission"
)

// TestBlindSubmissionLifecycleLayerTriad proves Bug, Witness, and Peachfuzz
// traverse one declaration/request/credential/decision/grant contract. The
// positive oracle holds selected evidence constant while changing only typed
// offering identity. The negative oracle then recombines independently valid
// same-key documents whose sole build difference is that identity.
func TestBlindSubmissionLifecycleLayerTriad(t *testing.T) {
	t.Parallel()

	offerings := [...]core.Offering{
		core.OfferingBug,
		core.OfferingWitness,
		core.OfferingPeachfuzz,
	}
	var commonDeclaration submission.Declaration
	for index, offering := range offerings {
		fixture := newAuthCompletionFixture(t, authCompletionFixtureRequest{
			offering: offering, authorityByte: byte(index) + 0x31,
			deviceByte: byte(index) + 0x51, nonceByte: byte(index) + 0x71,
		})
		authenticated, err := fixture.verifiedRequest.Document()
		if err != nil || authenticated != fixture.request.document {
			t.Fatalf("authenticated request for %v = (%+v, %v), want exact credentialed document and nil",
				offering, authenticated, err)
		}
		declaration := authenticated.Request.Payload.Declaration
		if index == 0 {
			commonDeclaration = declaration
		} else if declaration != commonDeclaration {
			t.Fatalf("declaration for %v = %+v, want offering-blind %+v",
				offering, declaration, commonDeclaration)
		}

		decisionDocument := blindSubmissionDecisionDocument(t, fixture)
		scope, err := authenticated.Certificate.Body.Scope()
		if err != nil {
			t.Fatalf("certificate scope for %v error = %v, want nil", offering, err)
		}
		decision, err := submission.VerifyDecision(submission.DecisionExpectation{
			Decision: decisionDocument, Request: authenticated.Request.Payload,
			Account: scope.Account, Offering: scope.Offering,
			ObservedAt: fixture.grant.Payload.IssuedAt, TrustedKeys: fixture.request.trusted,
		})
		if err != nil {
			t.Fatalf("VerifyDecision(%v blind path) error = %v, want nil", offering, err)
		}
		kind, kindErr := decision.Kind()
		grant, hasGrant := decision.Grant()
		if kindErr != nil || kind != submission.DecisionUpload || !hasGrant {
			t.Fatalf("decision for %v = (kind %v, grant %v, present %t, error %v), want upload with one grant",
				offering, kind, grant, hasGrant, kindErr)
		}
		payload, err := grant.Payload()
		if err != nil {
			t.Fatalf("verified grant payload for %v error = %v, want nil", offering, err)
		}
		requestCommitment, err := submission.CommitRequest(authenticated.Request.Payload)
		if err != nil {
			t.Fatalf("CommitRequest(%v authenticated payload) error = %v, want nil", offering, err)
		}
		capability, err := grant.Capability()
		if err != nil {
			t.Fatalf("verified grant capability for %v error = %v, want nil", offering, err)
		}
		capabilityCommitment, err := capability.Commitment()
		if err != nil {
			t.Fatalf("capability commitment for %v error = %v, want nil", offering, err)
		}
		if payload.Request != requestCommitment || payload.Capability != capabilityCommitment {
			t.Fatalf("grant commitments for %v do not close the exact request and bearer", offering)
		}
	}

	t.Run("offering_is_load_bearing_with_identical_keys_and_evidence", func(t *testing.T) {
		t.Parallel()

		bug := newAuthCompletionFixture(t, authCompletionFixtureRequest{
			offering: core.OfferingBug, authorityByte: 0x61, deviceByte: 0x62, nonceByte: 0x63,
		})
		witness := newAuthCompletionFixture(t, authCompletionFixtureRequest{
			offering: core.OfferingWitness, authorityByte: 0x61, deviceByte: 0x62, nonceByte: 0x63,
		})
		proveOnlyOfferingDiffers(t, bug.request.request.Payload, witness.request.request.Payload)

		recombined, err := Assemble(RequestAssembly{
			Request: bug.request.request, Certificate: witness.request.certificate,
		})
		if !errors.Is(err, core.ErrControlPlaneResponseBinding) || recombined != (RequestDocument{}) {
			t.Fatalf("Assemble(request and same-key foreign-offering certificate) = (%v, %v), want zero and errors.Is %v",
				recombined, err, core.ErrControlPlaneResponseBinding)
		}

		bugDecision := blindSubmissionDecisionDocument(t, bug)
		witnessScope, scopeErr := witness.request.certificate.Body.Scope()
		if scopeErr != nil {
			t.Fatalf("Witness certificate scope error = %v, want nil", scopeErr)
		}
		rejected, err := submission.VerifyDecision(submission.DecisionExpectation{
			Decision: bugDecision, Request: witness.request.request.Payload,
			Account: witnessScope.Account, Offering: witnessScope.Offering,
			ObservedAt: bug.grant.Payload.IssuedAt, TrustedKeys: witness.request.trusted,
		})
		if !errors.Is(err, core.ErrControlPlaneResponseBinding) || rejected != (submission.VerifiedDecision{}) {
			t.Fatalf("VerifyDecision(same-key foreign offering) = (%v, %v), want zero and errors.Is %v",
				rejected, err, core.ErrControlPlaneResponseBinding)
		}
	})

	t.Run("zero_inputs_never_acquire_request_or_decision_authority", func(t *testing.T) {
		t.Parallel()

		request, requestErr := Verify(Verification{})
		if !errors.Is(requestErr, core.ErrControlPlaneContract) || request != (Verified{}) {
			t.Fatalf("Verify(zero) = (%v, %v), want zero and errors.Is %v",
				request, requestErr, core.ErrControlPlaneContract)
		}
		decision, decisionErr := submission.VerifyDecision(submission.DecisionExpectation{})
		if !errors.Is(decisionErr, core.ErrControlPlaneContract) || decision != (submission.VerifiedDecision{}) {
			t.Fatalf("VerifyDecision(zero) = (%v, %v), want zero and errors.Is %v",
				decision, decisionErr, core.ErrControlPlaneContract)
		}
	})
}

func blindSubmissionDecisionDocument(
	t testing.TB,
	fixture authCompletionFixture,
) submission.DecisionDocument {
	t.Helper()

	body, err := submission.UploadDecision(fixture.grantProjection)
	if err != nil {
		t.Fatalf("UploadDecision() error = %v, want nil", err)
	}
	request := fixture.request.request.Payload
	certificate := fixture.request.certificate.Body
	header := controlplane.ResponseHeader{
		ProviderTime: certificate.IssuedAt, RequestNonce: request.Nonce,
		Account: certificate.Account, Installation: certificate.Subject.DeviceID,
		Revision: request.Revision, Family: controlwire.RouteFamilySubmissions,
		Status: controlplane.ProductStatusActive, Offering: request.Build.Offering(),
		Policy: blindSubmissionPolicyCursor(t),
	}
	projection, err := IssueSubmissionResponse(SubmissionResponseIssuance{
		Signer: fixture.request.authority, Header: header, Body: body,
		Assessment: acceptedSubmissionResponseAssessment(t, header),
	})
	if err != nil {
		t.Fatalf("IssueSubmissionResponse() error = %v, want nil", err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("submission response MarshalJSON() error = %v, want nil", err)
	}
	var document controlplane.ResponseDocument[submission.DecisionDocument, *submission.DecisionDocument]
	if err := document.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("submission response UnmarshalJSON() error = %v, want nil", err)
	}
	verified, err := VerifySubmissionResponse(SubmissionResponseVerification{
		Document: document,
		Expected: controlplane.ResponseExpectation{
			RequestNonce: header.RequestNonce, Account: header.Account,
			Installation: header.Installation, Revision: header.Revision,
			Family: header.Family, Offering: header.Offering,
		},
		TrustedKeys: fixture.request.trusted,
	})
	if err != nil {
		t.Fatalf("VerifySubmissionResponse() error = %v, want nil", err)
	}
	bodyDocument, err := verified.Body()
	if err != nil {
		t.Fatalf("verified submission response body error = %v, want nil", err)
	}
	return bodyDocument
}

func blindSubmissionPolicyCursor(t testing.TB) controlwire.PolicyCursor {
	t.Helper()

	activation, err := controlwire.NewPolicyActivation(1)
	if err != nil {
		t.Fatalf("NewPolicyActivation(1) error = %v, want nil", err)
	}
	return controlwire.PolicyCursor{
		Revision: controlwire.PolicyRevisionID{1}, Activation: activation,
	}
}

func proveOnlyOfferingDiffers(
	t testing.TB,
	left submission.RequestPayload,
	right submission.RequestPayload,
) {
	t.Helper()

	leftBuild, rightBuild := left.Build, right.Build
	if left.Declaration != right.Declaration || left.Manifest != right.Manifest ||
		left.Nonce != right.Nonce || left.Revision != right.Revision ||
		leftBuild.Version() != rightBuild.Version() ||
		leftBuild.Commit() != rightBuild.Commit() ||
		leftBuild.Platform() != rightBuild.Platform() ||
		leftBuild.Offering() == rightBuild.Offering() {
		t.Fatalf("same-key foreign-offering fixture changed a fact other than offering")
	}
}
