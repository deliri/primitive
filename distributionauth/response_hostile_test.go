package distributionauth

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
)

func TestPublicationResponseLayerTriadAuthenticatesRefusesAndKeepsNeutralZero(t *testing.T) {
	t.Parallel()

	fixture := newPublicationAuthFixture(t, publicationAuthFixtureRequest{authorityByte: 0x84})
	request := fixture.document.Request.Payload
	certificate := fixture.document.Certificate.Body
	activation, err := controlwire.NewPolicyActivation(1)
	if err != nil {
		t.Fatalf("controlwire.NewPolicyActivation(1) error = %v, want nil", err)
	}
	header := controlplane.ResponseHeader{
		ProviderTime: certificate.IssuedAt,
		RequestNonce: request.Nonce,
		Account:      certificate.Account,
		Installation: certificate.Subject.DeviceID,
		Revision:     request.Revision,
		Family:       controlwire.RouteFamilyReleasePublications,
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
	issuance := PublicationResponseIssuance{
		Signer: fixture.installation.AuthorityPrivate, Header: header, Body: fixture.grantProjection,
		Assessment: acceptedDistributionResponseAssessment(t, header),
	}
	if err := issuance.Validate(); err != nil {
		t.Fatalf("PublicationResponseIssuance.Validate(real grant) error = %v, want nil", err)
	}
	projection, err := IssuePublicationResponse(issuance)
	if err != nil {
		t.Fatalf("IssuePublicationResponse(real grant) error = %v, want nil", err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil || len(encoded) == 0 {
		t.Fatalf("publication response MarshalJSON() = (%d bytes, %v), want non-empty and nil", len(encoded), err)
	}
	var document controlplane.ResponseDocument[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument]
	if err := document.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("ResponseDocument.UnmarshalJSON(real publication grant) error = %v, want nil", err)
	}
	verification := PublicationResponseVerification{
		Document: document, Expected: expected, TrustedKeys: fixture.authority,
	}
	if err := verification.Validate(); err != nil {
		t.Fatalf("PublicationResponseVerification.Validate(real grant) error = %v, want nil", err)
	}
	verified, err := VerifyPublicationResponse(verification)
	if err != nil {
		t.Fatalf("VerifyPublicationResponse(bound request and authority) error = %v, want nil", err)
	}
	body, err := verified.Body()
	if err != nil {
		t.Fatalf("VerifiedResponse.Body() error = %v, want nil", err)
	}
	inner, err := distribution.VerifyPublicationGrant(distribution.PublicationGrantExpectation{
		Request: request, Document: body, ObservedAt: certificate.IssuedAt, TrustedKeys: fixture.authority,
	})
	if err != nil || inner.Validate() != nil {
		t.Fatalf("distribution.VerifyPublicationGrant(authenticated outer body) = (%v, %v), want valid and nil", inner, err)
	}

	mismatched := expected
	mismatched.RequestNonce = distributionAuthNonce(t, 0x72)
	rejected, err := VerifyPublicationResponse(PublicationResponseVerification{
		Document: document, Expected: mismatched, TrustedKeys: fixture.authority,
	})
	var binding controlplane.ResponseBindingError
	if !errors.Is(err, core.ErrControlPlaneResponseBinding) ||
		!errors.As(err, &binding) || binding.Field() != controlplane.ResponseHeaderFieldRequestNonce ||
		rejected.Validate() == nil {
		t.Fatalf("VerifyPublicationResponse(other request nonce) = (%v, %v, field %v), want invalid zero proof and %v/%v", rejected, err, binding.Field(), core.ErrControlPlaneResponseBinding, controlplane.ResponseHeaderFieldRequestNonce)
	}
	zeroProjection, err := IssuePublicationResponse(PublicationResponseIssuance{})
	if !errors.Is(err, core.ErrControlPlaneResponseDocument) || zeroProjection.Validate() == nil {
		t.Fatalf("IssuePublicationResponse(zero) = (%v, %v), want invalid zero projection and %v", zeroProjection, err, core.ErrControlPlaneResponseDocument)
	}
	zeroVerified, err := VerifyPublicationResponse(PublicationResponseVerification{})
	if !errors.Is(err, core.ErrControlPlaneResponseDocument) || zeroVerified.Validate() == nil {
		t.Fatalf("VerifyPublicationResponse(zero) = (%v, %v), want invalid zero proof and %v", zeroVerified, err, core.ErrControlPlaneResponseDocument)
	}
}

func acceptedDistributionResponseAssessment(t testing.TB, header controlplane.ResponseHeader) controlwire.ProtocolAssessment {
	t.Helper()
	support, err := controlwire.PublishedProtocolSupport()
	if err != nil {
		t.Fatalf("controlwire.PublishedProtocolSupport() error = %v, want nil", err)
	}
	assessment, err := controlwire.AssessProtocol(controlwire.ProtocolAssessmentRequest{
		Support: support, Capability: controlwire.ProtocolCapability{Revision: header.Revision, Family: header.Family},
	})
	if err != nil {
		t.Fatalf("controlwire.AssessProtocol(published distribution response pair) error = %v, want nil", err)
	}
	return assessment
}

func TestDistributionResponseBoundariesRefuseEveryNeutralInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		execute func() (error, error)
		name    string
	}{
		{name: "material issuance validation", execute: func() (error, error) { err := (MaterialResponseIssuance{}).Validate(); return err, err }},
		{name: "material issue", execute: func() (error, error) {
			value, err := IssueMaterialResponse(MaterialResponseIssuance{})
			return err, value.Validate()
		}},
		{name: "material verification validation", execute: func() (error, error) { err := (MaterialResponseVerification{}).Validate(); return err, err }},
		{name: "material verify", execute: func() (error, error) {
			value, err := VerifyMaterialResponse(MaterialResponseVerification{})
			return err, value.Validate()
		}},
		{name: "publication issuance validation", execute: func() (error, error) { err := (PublicationResponseIssuance{}).Validate(); return err, err }},
		{name: "publication issue", execute: func() (error, error) {
			value, err := IssuePublicationResponse(PublicationResponseIssuance{})
			return err, value.Validate()
		}},
		{name: "publication verification validation", execute: func() (error, error) { err := (PublicationResponseVerification{}).Validate(); return err, err }},
		{name: "publication verify", execute: func() (error, error) {
			value, err := VerifyPublicationResponse(PublicationResponseVerification{})
			return err, value.Validate()
		}},
		{name: "publication completion issuance validation", execute: func() (error, error) { err := (PublicationCompletionResponseIssuance{}).Validate(); return err, err }},
		{name: "publication completion issue", execute: func() (error, error) {
			value, err := IssuePublicationCompletionResponse(PublicationCompletionResponseIssuance{})
			return err, value.Validate()
		}},
		{name: "publication completion verification validation", execute: func() (error, error) {
			err := (PublicationCompletionResponseVerification{}).Validate()
			return err, err
		}},
		{name: "publication completion verify", execute: func() (error, error) {
			value, err := VerifyPublicationCompletionResponse(PublicationCompletionResponseVerification{})
			return err, value.Validate()
		}},
		{name: "update issuance validation", execute: func() (error, error) { err := (UpdateResponseIssuance{}).Validate(); return err, err }},
		{name: "update issue", execute: func() (error, error) {
			value, err := IssueUpdateResponse(UpdateResponseIssuance{})
			return err, value.Validate()
		}},
		{name: "update verification validation", execute: func() (error, error) { err := (UpdateResponseVerification{}).Validate(); return err, err }},
		{name: "update verify", execute: func() (error, error) {
			value, err := VerifyUpdateResponse(UpdateResponseVerification{})
			return err, value.Validate()
		}},
		{name: "upgrade issuance validation", execute: func() (error, error) { err := (UpgradeResponseIssuance{}).Validate(); return err, err }},
		{name: "upgrade issue", execute: func() (error, error) {
			value, err := IssueUpgradeResponse(UpgradeResponseIssuance{})
			return err, value.Validate()
		}},
		{name: "upgrade verification validation", execute: func() (error, error) { err := (UpgradeResponseVerification{}).Validate(); return err, err }},
		{name: "upgrade verify", execute: func() (error, error) {
			value, err := VerifyUpgradeResponse(UpgradeResponseVerification{})
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
