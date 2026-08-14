package submissionauth

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/submission"
)

func TestSubmissionDecisionResponseLayerTriadAuthenticatesRefusesAndKeepsNeutralZero(t *testing.T) {
	t.Parallel()

	fixture := newAuthCompletionFixture(t, authCompletionFixtureRequest{authorityByte: 0x83})
	body, err := submission.UploadDecision(fixture.grantProjection)
	if err != nil {
		t.Fatalf("submission.UploadDecision(real grant projection) error = %v, want nil", err)
	}
	activation, err := controlwire.NewPolicyActivation(1)
	if err != nil {
		t.Fatalf("controlwire.NewPolicyActivation(1) error = %v, want nil", err)
	}
	request := fixture.request.request.Payload
	certificate := fixture.request.certificate.Body
	header := controlplane.ResponseHeader{
		ProviderTime: certificate.IssuedAt,
		RequestNonce: request.Nonce,
		Account:      certificate.Account,
		Installation: certificate.Subject.DeviceID,
		Revision:     request.Revision,
		Family:       controlwire.RouteFamilySubmissions,
		Status:       controlplane.ProductStatusActive,
		Offering:     request.Build.Offering(),
		Policy: controlwire.PolicyCursor{
			Revision: controlwire.PolicyRevisionID{1}, Activation: activation,
		},
	}
	expected := controlplane.ResponseExpectation{
		RequestNonce: header.RequestNonce, Account: header.Account,
		Installation: header.Installation, Revision: header.Revision, Family: header.Family, Offering: header.Offering,
	}
	issuance := SubmissionResponseIssuance{
		Signer: fixture.request.authority, Header: header, Body: body,
		Assessment: acceptedSubmissionResponseAssessment(t, header),
	}
	if err := issuance.Validate(); err != nil {
		t.Fatalf("SubmissionResponseIssuance.Validate(real decision) error = %v, want nil", err)
	}
	projection, err := IssueSubmissionResponse(issuance)
	if err != nil {
		t.Fatalf("IssueSubmissionResponse(real decision) error = %v, want nil", err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil || len(encoded) == 0 {
		t.Fatalf("submission response MarshalJSON() = (%d bytes, %v), want non-empty and nil", len(encoded), err)
	}
	var document controlplane.ResponseDocument[submission.DecisionDocument, *submission.DecisionDocument]
	if err := document.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("ResponseDocument.UnmarshalJSON(real decision) error = %v, want nil", err)
	}
	verification := SubmissionResponseVerification{
		Document: document, Expected: expected, TrustedKeys: fixture.request.trusted,
	}
	if err := verification.Validate(); err != nil {
		t.Fatalf("SubmissionResponseVerification.Validate(real decision) error = %v, want nil", err)
	}
	verified, err := VerifySubmissionResponse(verification)
	if err != nil {
		t.Fatalf("VerifySubmissionResponse(bound request and authority) error = %v, want nil", err)
	}
	gotBody, err := verified.Body()
	if err != nil {
		t.Fatalf("VerifiedResponse.Body() error = %v, want nil", err)
	}
	scope, err := certificate.Scope()
	if err != nil {
		t.Fatalf("InstallationCertificateBody.Scope() error = %v, want nil", err)
	}
	decision, err := submission.VerifyDecision(submission.DecisionExpectation{
		Decision: gotBody, Request: request, Account: scope.Account, Offering: scope.Offering,
		ObservedAt: fixture.grant.Payload.IssuedAt, TrustedKeys: fixture.request.trusted,
	})
	if err != nil {
		t.Fatalf("submission.VerifyDecision(authenticated outer body) error = %v, want nil", err)
	}
	kind, err := decision.Kind()
	grant, hasGrant := decision.Grant()
	if err != nil || kind != submission.DecisionUpload || !hasGrant || grant.Validate() != nil {
		t.Fatalf("verified submission decision = (kind %v, grant %v, present %t, error %v), want authenticated upload grant", kind, grant, hasGrant, err)
	}

	mismatched := expected
	mismatched.RequestNonce = authRequestPayload(t, request.Build, 0x72).Nonce
	rejected, err := VerifySubmissionResponse(SubmissionResponseVerification{
		Document: document, Expected: mismatched, TrustedKeys: fixture.request.trusted,
	})
	var binding controlplane.ResponseBindingError
	if !errors.Is(err, core.ErrControlPlaneResponseBinding) ||
		!errors.As(err, &binding) || binding.Field() != controlplane.ResponseHeaderFieldRequestNonce ||
		rejected.Validate() == nil {
		t.Fatalf("VerifySubmissionResponse(other request nonce) = (%v, %v, field %v), want invalid zero proof and %v/%v", rejected, err, binding.Field(), core.ErrControlPlaneResponseBinding, controlplane.ResponseHeaderFieldRequestNonce)
	}
	zeroProjection, err := IssueSubmissionResponse(SubmissionResponseIssuance{})
	if !errors.Is(err, core.ErrControlPlaneResponseDocument) || zeroProjection.Validate() == nil {
		t.Fatalf("IssueSubmissionResponse(zero) = (%v, %v), want invalid zero projection and %v", zeroProjection, err, core.ErrControlPlaneResponseDocument)
	}
	zeroVerified, err := VerifySubmissionResponse(SubmissionResponseVerification{})
	if !errors.Is(err, core.ErrControlPlaneResponseDocument) || zeroVerified.Validate() == nil {
		t.Fatalf("VerifySubmissionResponse(zero) = (%v, %v), want invalid zero proof and %v", zeroVerified, err, core.ErrControlPlaneResponseDocument)
	}
}

func acceptedSubmissionResponseAssessment(t testing.TB, header controlplane.ResponseHeader) controlwire.ProtocolAssessment {
	t.Helper()
	support, err := controlwire.PublishedProtocolSupport()
	if err != nil {
		t.Fatalf("controlwire.PublishedProtocolSupport() error = %v, want nil", err)
	}
	assessment, err := controlwire.AssessProtocol(controlwire.ProtocolAssessmentRequest{
		Support: support, Capability: controlwire.ProtocolCapability{Revision: header.Revision, Family: header.Family},
	})
	if err != nil {
		t.Fatalf("controlwire.AssessProtocol(published submission response pair) error = %v, want nil", err)
	}
	return assessment
}

func TestSubmissionResponseBoundariesRefuseEveryNeutralInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		execute func() (error, error)
		name    string
	}{
		{name: "submission issuance validation", execute: func() (error, error) { err := (SubmissionResponseIssuance{}).Validate(); return err, err }},
		{name: "submission issue", execute: func() (error, error) {
			value, err := IssueSubmissionResponse(SubmissionResponseIssuance{})
			return err, value.Validate()
		}},
		{name: "submission verification validation", execute: func() (error, error) { err := (SubmissionResponseVerification{}).Validate(); return err, err }},
		{name: "submission verify", execute: func() (error, error) {
			value, err := VerifySubmissionResponse(SubmissionResponseVerification{})
			return err, value.Validate()
		}},
		{name: "completion issuance validation", execute: func() (error, error) { err := (CompletionResponseIssuance{}).Validate(); return err, err }},
		{name: "completion issue", execute: func() (error, error) {
			value, err := IssueCompletionResponse(CompletionResponseIssuance{})
			return err, value.Validate()
		}},
		{name: "completion verification validation", execute: func() (error, error) { err := (CompletionResponseVerification{}).Validate(); return err, err }},
		{name: "completion verify", execute: func() (error, error) {
			value, err := VerifyCompletionResponse(CompletionResponseVerification{})
			return err, value.Validate()
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			operationErr, resultErr := tc.execute()
			if !errors.Is(operationErr, core.ErrControlPlaneResponseDocument) ||
				!errors.Is(resultErr, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("neutral response operation = (operation %v, result %v), want %v on both", operationErr, resultErr, core.ErrControlPlaneResponseDocument)
			}
		})
	}
}
