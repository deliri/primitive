package paymentauth

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/payment"
	"github.com/deliri/primitive/v2026/receipt"
)

type paymentResponseFixture struct {
	body     payment.CatalogDocument
	header   controlplane.ResponseHeader
	expected controlplane.ResponseExpectation
	trusted  attest.TrustedKeys
	signer   ed25519.PrivateKey
}

func TestPaymentResponseLayerTriadAuthenticatesRefusesAndKeepsNeutralZero(t *testing.T) {
	t.Parallel()

	fixture := newPaymentResponseFixture(t)
	projection, err := IssueResponse(ResponseIssuance{
		Signer: fixture.signer, Header: fixture.header, Body: fixture.body,
		Assessment: acceptedPaymentResponseAssessment(t, fixture.header),
	})
	if err != nil {
		t.Fatalf("IssueResponse(real signed payment catalog) error = %v, want nil", err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil || len(encoded) == 0 {
		t.Fatalf("ResponseProjection.MarshalJSON() = (%d bytes, %v), want non-empty authenticated response and nil", len(encoded), err)
	}
	var document controlplane.ResponseDocument[payment.CatalogDocument, *payment.CatalogDocument]
	if err := document.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("ResponseDocument.UnmarshalJSON(real payment catalog) error = %v, want nil", err)
	}
	verification := ResponseVerification{
		Document: document, Expected: fixture.expected, TrustedKeys: fixture.trusted,
	}
	if err := verification.Validate(); err != nil {
		t.Fatalf("ResponseVerification.Validate(real signed payment catalog) error = %v, want nil", err)
	}
	verified, err := VerifyResponse(verification)
	if err != nil {
		t.Fatalf("VerifyResponse(bound request and authority) error = %v, want nil", err)
	}
	gotBody, err := verified.Body()
	if err != nil {
		t.Fatalf("VerifiedResponse.Body() error = %v, want nil", err)
	}
	gotJSON, gotErr := gotBody.MarshalJSON()
	wantJSON, wantErr := fixture.body.MarshalJSON()
	if gotErr != nil || wantErr != nil || !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("verified payment catalog = (%d bytes, %v), want exact signed body (%d bytes, %v)", len(gotJSON), gotErr, len(wantJSON), wantErr)
	}

	mismatched := fixture.expected
	mismatched.RequestNonce = paymentQueryNonce(t, 0x72)
	rejected, err := VerifyResponse(ResponseVerification{
		Document: document, Expected: mismatched, TrustedKeys: fixture.trusted,
	})
	var binding controlplane.ResponseBindingError
	if !errors.Is(err, core.ErrControlPlaneResponseBinding) ||
		!errors.As(err, &binding) || binding.Field() != controlplane.ResponseHeaderFieldRequestNonce ||
		rejected.Validate() == nil {
		t.Fatalf("VerifyResponse(other request nonce) = (%v, %v, field %v), want invalid zero proof and %v/%v", rejected, err, binding.Field(), core.ErrControlPlaneResponseBinding, controlplane.ResponseHeaderFieldRequestNonce)
	}

	issuance := ResponseIssuance{}
	if err := issuance.Validate(); !errors.Is(err, core.ErrControlPlaneResponseDocument) ||
		!errors.Is(err, core.ErrControlPlaneResponseHeader) {
		t.Fatalf("ResponseIssuance.Validate(zero) error = %v, want %v/%v", err, core.ErrControlPlaneResponseDocument, core.ErrControlPlaneResponseHeader)
	}
	zeroProjection, err := IssueResponse(issuance)
	if !errors.Is(err, core.ErrControlPlaneResponseDocument) || zeroProjection.Validate() == nil {
		t.Fatalf("IssueResponse(zero) = (%v, %v), want invalid zero projection and %v", zeroProjection, err, core.ErrControlPlaneResponseDocument)
	}
	zeroVerified, err := VerifyResponse(ResponseVerification{})
	if !errors.Is(err, core.ErrControlPlaneResponseDocument) || zeroVerified.Validate() == nil {
		t.Fatalf("VerifyResponse(zero) = (%v, %v), want invalid zero proof and %v", zeroVerified, err, core.ErrControlPlaneResponseDocument)
	}
}

func newPaymentResponseFixture(t testing.TB) paymentResponseFixture {
	t.Helper()

	const authorityMarker byte = 0x82
	request := newPaymentQueryFixture(t, paymentQueryFixtureRequest{authorityByte: authorityMarker})
	generation, err := receipt.NewGeneration(1)
	if err != nil {
		t.Fatalf("receipt.NewGeneration(1) error = %v, want nil", err)
	}
	cursor, err := receipt.NewCursorDigest(core.SHA256Of([]byte{1}))
	if err != nil {
		t.Fatalf("receipt.NewCursorDigest(nonzero) error = %v, want nil", err)
	}
	chain, err := receipt.NewChainHash(core.SHA256Of([]byte{2}))
	if err != nil {
		t.Fatalf("receipt.NewChainHash(nonzero) error = %v, want nil", err)
	}
	watermark, err := receipt.NewWatermark(receipt.WatermarkRequest{
		Generation: generation, Scope: request.payload.Query.Scope,
		CursorDigest: cursor, ChainHash: chain,
	})
	if err != nil {
		t.Fatalf("receipt.NewWatermark(real payment scope) error = %v, want nil", err)
	}
	seed := paymentQuerySeed(authorityMarker)
	signer := ed25519.NewKeyFromSeed(seed[:])
	body, err := payment.IssueCatalog(payment.CatalogIssuance{Signer: signer, Payload: payment.CatalogPayload{
		Entries: []payment.Document{}, Watermark: watermark,
		ObservedAt: request.document.Certificate.Body.IssuedAt,
		Scope:      request.payload.Query.Scope, Continuation: payment.End(),
	}})
	if err != nil {
		t.Fatalf("payment.IssueCatalog(real empty page) error = %v, want nil", err)
	}
	activation, err := controlwire.NewPolicyActivation(1)
	if err != nil {
		t.Fatalf("controlwire.NewPolicyActivation(1) error = %v, want nil", err)
	}
	header := controlplane.ResponseHeader{
		ProviderTime: request.document.Certificate.Body.IssuedAt,
		RequestNonce: request.payload.Nonce,
		Account:      request.document.Certificate.Body.Account,
		Installation: request.document.Certificate.Body.Subject.DeviceID,
		Revision:     request.payload.Revision,
		Family:       controlwire.RouteFamilyPayments,
		Status:       controlplane.ProductStatusActive,
		Offering:     request.payload.Build.Offering(),
		Policy: controlwire.PolicyCursor{
			Revision: controlwire.PolicyRevisionID{1}, Activation: activation,
		},
	}
	expected := controlplane.ResponseExpectation{
		RequestNonce: header.RequestNonce, Account: header.Account,
		Installation: header.Installation, Revision: header.Revision, Family: header.Family, Offering: header.Offering,
	}
	return paymentResponseFixture{
		body: body, header: header, expected: expected, trusted: request.trusted, signer: signer,
	}
}

func acceptedPaymentResponseAssessment(t testing.TB, header controlplane.ResponseHeader) controlwire.ProtocolAssessment {
	t.Helper()
	support, err := controlwire.PublishedProtocolSupport()
	if err != nil {
		t.Fatalf("controlwire.PublishedProtocolSupport() error = %v, want nil", err)
	}
	assessment, err := controlwire.AssessProtocol(controlwire.ProtocolAssessmentRequest{
		Support: support, Capability: controlwire.ProtocolCapability{Revision: header.Revision, Family: header.Family},
	})
	if err != nil {
		t.Fatalf("controlwire.AssessProtocol(published payment response pair) error = %v, want nil", err)
	}
	return assessment
}
