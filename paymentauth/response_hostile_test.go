package paymentauth

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/currency"
	"github.com/deliri/primitive/v2026/payment"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

type paymentResponseFixture struct {
	expected controlplane.ResponseExpectation
	signer   ed25519.PrivateKey
	header   controlplane.ResponseHeader
	request  payment.QueryPayload
	settled  payment.Document
	body     payment.CatalogDocument
	trusted  attest.TrustedKeys
	client   controlplane.Client
	server   controlplane.Server
}

type paymentResponseIdentityCase struct {
	name            string
	authorityMarker byte
	deviceMarker    byte
}

func TestPaymentResponseVerificationLayerTriadClosesThePaymentRouteFamily(t *testing.T) {
	t.Parallel()

	fixture := newPaymentResponseFixture(t)

	t.Run("positive ten authentic payment responses expose their exact settled receipts", func(t *testing.T) {
		t.Parallel()

		for _, tc := range paymentResponseIdentityCases() {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				candidate := newPaymentResponseFixtureWithMarkers(t, tc.authorityMarker, tc.deviceMarker)
				document := issuePaymentResponseDocument(t, candidate)
				got, gotErr := VerifyResponse(ResponseVerification{
					Client: candidate.client, Document: document, Expected: candidate.expected,
				})
				if gotErr != nil {
					t.Fatalf("VerifyResponse(authentic payment family) error = %v, want nil", gotErr)
				}
				proveExactPaymentCatalog(t, candidate, got)
			})
		}
	})

	t.Run("negative every authentic sibling family is refused without proof", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name   string
			family controlwire.RouteFamily
		}{
			{name: "registration response cannot cross the payment verifier", family: controlwire.RouteFamilyRegistrations},
			{name: "check-in response cannot cross the payment verifier", family: controlwire.RouteFamilyCheckIns},
			{name: "submission response cannot cross the payment verifier", family: controlwire.RouteFamilySubmissions},
			{name: "submission completion cannot cross the payment verifier", family: controlwire.RouteFamilySubmissionCompletions},
			{name: "retrieval response cannot cross the payment verifier", family: controlwire.RouteFamilyRetrievals},
			{name: "chit response cannot cross the payment verifier", family: controlwire.RouteFamilyChits},
			{name: "release material response cannot cross the payment verifier", family: controlwire.RouteFamilyReleaseMaterials},
			{name: "release publication cannot cross the payment verifier", family: controlwire.RouteFamilyReleasePublications},
			{name: "release publication completion cannot cross the payment verifier", family: controlwire.RouteFamilyReleasePublicationCompletions},
			{name: "update check cannot cross the payment verifier", family: controlwire.RouteFamilyUpdateChecks},
			{name: "upgrade response cannot cross the payment verifier", family: controlwire.RouteFamilyUpgrades},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				document, expected := issueAuthenticPaymentCatalogForFamily(t, fixture, tc.family)
				got, gotErr := VerifyResponse(ResponseVerification{
					Client: fixture.client, Document: document, Expected: expected,
				})
				if !errors.Is(gotErr, core.ErrControlPlaneResponseBinding) {
					t.Fatalf("VerifyResponse(authentic %v family) error = %v, want errors.Is %v",
						tc.family, gotErr, core.ErrControlPlaneResponseBinding)
				}
				if gotValidationErr := got.Validate(); !errors.Is(gotValidationErr, core.ErrControlPlaneResponseDocument) {
					t.Fatalf("VerifyResponse(authentic %v family) proof validation error = %v, want errors.Is %v",
						tc.family, gotValidationErr, core.ErrControlPlaneResponseDocument)
				}
			})
		}
		document := issuePaymentResponseDocument(t, fixture)
		provePaymentResponseVerificationRejections(t, fixture, document)
	})

	t.Run("neutral zero verification exposes neither body nor proof", func(t *testing.T) {
		t.Parallel()

		got, gotErr := VerifyResponse(ResponseVerification{})
		if !errors.Is(gotErr, core.ErrControlPlaneResponseDocument) {
			t.Fatalf("VerifyResponse(zero) error = %v, want errors.Is %v",
				gotErr, core.ErrControlPlaneResponseDocument)
		}
		if gotValidationErr := got.Validate(); !errors.Is(gotValidationErr, core.ErrControlPlaneResponseDocument) {
			t.Fatalf("VerifyResponse(zero) proof validation error = %v, want errors.Is %v",
				gotValidationErr, core.ErrControlPlaneResponseDocument)
		}
		gotBody, gotBodyErr := got.Body()
		if !errors.Is(gotBodyErr, core.ErrControlPlaneResponseDocument) || gotBody.Validate() == nil {
			t.Fatalf("VerifyResponse(zero).Body() = (%v, %v), want invalid zero catalog and errors.Is %v",
				gotBody, gotBodyErr, core.ErrControlPlaneResponseDocument)
		}
		gotHeader, gotHeaderErr := got.Header()
		if !errors.Is(gotHeaderErr, core.ErrControlPlaneResponseDocument) || gotHeader != (controlplane.ResponseHeader{}) {
			t.Fatalf("VerifyResponse(zero).Header() = (%v, %v), want zero and errors.Is %v",
				gotHeader, gotHeaderErr, core.ErrControlPlaneResponseDocument)
		}
	})
}

func TestPaymentResponseIssuanceLayerTriadBindsOnlyThePaymentRouteFamily(t *testing.T) {
	t.Parallel()

	fixture := newPaymentResponseFixture(t)

	t.Run("positive ten authenticated catalog projections survive wire closure", func(t *testing.T) {
		t.Parallel()

		for _, tc := range paymentResponseIdentityCases() {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				candidate := newPaymentResponseFixtureWithMarkers(t, tc.authorityMarker, tc.deviceMarker)
				document := issuePaymentResponseDocument(t, candidate)
				got, gotErr := VerifyResponse(ResponseVerification{
					Client: candidate.client, Document: document, Expected: candidate.expected,
				})
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("issued payment response verification = (%v, %v), want valid proof and nil", got, gotErr)
				}
			})
		}
	})

	t.Run("negative sibling families and invalid authority inputs emit no projection", func(t *testing.T) {
		t.Parallel()
		provePaymentResponseIssuanceRejections(t, fixture)
	})

	t.Run("neutral zero issuance emits no authenticated document", func(t *testing.T) {
		t.Parallel()

		got, gotErr := IssueResponse(ResponseIssuance{})
		if !errors.Is(gotErr, core.ErrControlPlaneResponseDocument) ||
			!errors.Is(gotErr, core.ErrControlPlaneContract) {
			t.Fatalf("IssueResponse(zero) error = %v, want errors.Is %v/%v",
				gotErr, core.ErrControlPlaneResponseDocument, core.ErrControlPlaneContract)
		}
		if gotValidationErr := got.Validate(); !errors.Is(gotValidationErr, core.ErrControlPlaneResponseDocument) {
			t.Fatalf("IssueResponse(zero) projection validation error = %v, want errors.Is %v",
				gotValidationErr, core.ErrControlPlaneResponseDocument)
		}
		encoded, marshalErr := got.MarshalJSON()
		if encoded != nil || !errors.Is(marshalErr, core.ErrControlPlaneResponseDocument) {
			t.Fatalf("IssueResponse(zero).MarshalJSON() = (%d bytes, %v), want nil and errors.Is %v",
				len(encoded), marshalErr, core.ErrControlPlaneResponseDocument)
		}
	})
}

func paymentResponseIdentityCases() []paymentResponseIdentityCase {
	return []paymentResponseIdentityCase{
		{name: "minimum authority and one-above-minimum device markers", authorityMarker: 1, deviceMarker: 2},
		{name: "minimum authority and maximum device markers", authorityMarker: 1, deviceMarker: 255},
		{name: "maximum authority and minimum device markers", authorityMarker: 255, deviceMarker: 1},
		{name: "maximum authority and one-below-maximum device markers", authorityMarker: 255, deviceMarker: 254},
		{name: "one below authority midpoint", authorityMarker: 127, deviceMarker: 128},
		{name: "authority midpoint", authorityMarker: 128, deviceMarker: 127},
		{name: "one above authority midpoint", authorityMarker: 129, deviceMarker: 126},
		{name: "distinct low authority and device markers", authorityMarker: 2, deviceMarker: 3},
		{name: "distinct high authority and device markers", authorityMarker: 253, deviceMarker: 254},
		{name: "ordinary authority and device markers", authorityMarker: 81, deviceMarker: 82},
	}
}

func proveExactPaymentCatalog(
	t *testing.T,
	fixture paymentResponseFixture,
	verified controlplane.VerifiedResponse[payment.CatalogDocument, *payment.CatalogDocument],
) {
	t.Helper()

	gotBody, gotBodyErr := verified.Body()
	gotJSON, gotJSONErr := gotBody.MarshalJSON()
	wantJSON, wantJSONErr := fixture.body.MarshalJSON()
	if gotBodyErr != nil || gotJSONErr != nil || wantJSONErr != nil || !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("verified payment catalog = (%d bytes, %v, %v), want exact %d-byte signed body and nil",
			len(gotJSON), gotBodyErr, gotJSONErr, len(wantJSON))
	}
	catalog, catalogErr := payment.VerifyCatalog(payment.CatalogVerification{
		Document: gotBody, Request: fixture.request, TrustedKeys: fixture.trusted,
	})
	if catalogErr != nil || len(catalog.Entries) != 1 || catalog.Entries[0] != fixture.settled {
		t.Fatalf("payment.VerifyCatalog(authenticated response) = (%v, %v), want one exact settled receipt and nil",
			catalog, catalogErr)
	}
	receiptProof, receiptErr := payment.Verify(payment.Verification{
		Document: catalog.Entries[0],
		Expected: payment.Expectation{
			Identity: fixture.settled.Payload.Identity, Scope: fixture.settled.Payload.Scope,
		},
		TrustedKeys: fixture.trusted,
	})
	if receiptErr != nil {
		t.Fatalf("payment.Verify(catalog receipt) error = %v, want nil", receiptErr)
	}
	settled, settledErr := receiptProof.Document()
	if settledErr != nil || settled != fixture.settled {
		t.Fatalf("authenticated catalog receipt = (%v, %v), want exact settled receipt and nil", settled, settledErr)
	}
}

func provePaymentResponseIssuanceRejections(t *testing.T, fixture paymentResponseFixture) {
	t.Helper()

	valid := ResponseIssuance{
		Server: fixture.server, Signer: fixture.signer, Header: fixture.header,
		Body: fixture.body, Assessment: acceptedPaymentResponseAssessment(t, fixture.header),
	}
	zeroServer := valid
	zeroServer.Server = controlplane.Server{}
	nilSigner := valid
	nilSigner.Signer = nil
	zeroBody := valid
	zeroBody.Body = payment.CatalogDocument{}
	zeroAssessment := valid
	zeroAssessment.Assessment = controlwire.ProtocolAssessment{}
	families := []struct {
		name   string
		family controlwire.RouteFamily
	}{
		{name: "registration family", family: controlwire.RouteFamilyRegistrations},
		{name: "check-in family", family: controlwire.RouteFamilyCheckIns},
		{name: "submission family", family: controlwire.RouteFamilySubmissions},
		{name: "submission completion family", family: controlwire.RouteFamilySubmissionCompletions},
		{name: "retrieval family", family: controlwire.RouteFamilyRetrievals},
		{name: "chit family", family: controlwire.RouteFamilyChits},
		{name: "release material family", family: controlwire.RouteFamilyReleaseMaterials},
		{name: "release publication family", family: controlwire.RouteFamilyReleasePublications},
		{name: "release publication completion family", family: controlwire.RouteFamilyReleasePublicationCompletions},
		{name: "update check family", family: controlwire.RouteFamilyUpdateChecks},
		{name: "upgrade family", family: controlwire.RouteFamilyUpgrades},
	}
	for _, tc := range families {
		t.Run(tc.name+" cannot be issued by the payment wrapper", func(t *testing.T) {
			t.Parallel()

			value := valid
			value.Header.Family = tc.family
			value.Assessment = acceptedPaymentResponseAssessment(t, value.Header)
			got, gotErr := IssueResponse(value)
			if !errors.Is(gotErr, core.ErrControlPlaneResponseDocument) ||
				!errors.Is(gotErr, core.ErrControlPlaneResponseBinding) {
				t.Fatalf("IssueResponse(%v family) error = %v, want errors.Is %v/%v",
					tc.family, gotErr, core.ErrControlPlaneResponseDocument, core.ErrControlPlaneResponseBinding)
			}
			if gotValidationErr := got.Validate(); !errors.Is(gotValidationErr, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("IssueResponse(%v family) projection validation error = %v, want errors.Is %v",
					tc.family, gotValidationErr, core.ErrControlPlaneResponseDocument)
			}
		})
	}
	cases := []struct {
		name  string
		value ResponseIssuance
		want  error
	}{
		{name: "zero server capability cannot issue a response", value: zeroServer, want: core.ErrControlPlaneContract},
		{name: "nil authority signer cannot issue a response", value: nilSigner, want: core.ErrAttestContract},
		{name: "zero catalog body cannot issue a response", value: zeroBody, want: core.ErrPaymentContract},
		{name: "zero protocol assessment cannot issue a response", value: zeroAssessment, want: core.ErrControlWireContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := IssueResponse(tc.value)
			if !errors.Is(gotErr, core.ErrControlPlaneResponseDocument) || !errors.Is(gotErr, tc.want) {
				t.Fatalf("IssueResponse(%s) error = %v, want errors.Is %v/%v",
					tc.name, gotErr, core.ErrControlPlaneResponseDocument, tc.want)
			}
			if validationErr := got.Validate(); !errors.Is(validationErr, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("IssueResponse(%s) projection validation error = %v, want errors.Is %v",
					tc.name, validationErr, core.ErrControlPlaneResponseDocument)
			}
			if encoded, marshalErr := got.MarshalJSON(); encoded != nil ||
				!errors.Is(marshalErr, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("IssueResponse(%s).MarshalJSON() = (%d bytes, %v), want nil and errors.Is %v",
					tc.name, len(encoded), marshalErr, core.ErrControlPlaneResponseDocument)
			}
		})
	}
}

func provePaymentResponseVerificationRejections(
	t *testing.T,
	fixture paymentResponseFixture,
	document controlplane.ResponseDocument[payment.CatalogDocument, *payment.CatalogDocument],
) {
	t.Helper()

	foreign := newPaymentResponseFixtureWithMarkers(t, 0x91, 0x92)
	foreignDocument := issuePaymentResponseDocument(t, foreign)
	base := ResponseVerification{Client: fixture.client, Document: document, Expected: fixture.expected}
	cases := []struct {
		name      string
		value     ResponseVerification
		want      error
		wantField controlplane.ResponseHeaderField
	}{
		{name: "different request nonce names the bound fact", value: paymentResponseWithExpectation(base, func(value *controlplane.ResponseExpectation) {
			value.RequestNonce = paymentQueryNonce(t, 0x72)
		}), want: core.ErrControlPlaneResponseBinding, wantField: controlplane.ResponseHeaderFieldRequestNonce},
		{name: "different account names the bound fact", value: paymentResponseWithExpectation(base, func(value *controlplane.ResponseExpectation) {
			value.Account = paymentQueryAccount(t, 0x73)
		}), want: core.ErrControlPlaneResponseBinding, wantField: controlplane.ResponseHeaderFieldAccount},
		{name: "different installation names the bound fact", value: paymentResponseWithExpectation(base, func(value *controlplane.ResponseExpectation) {
			value.Installation = foreign.expected.Installation
		}), want: core.ErrControlPlaneResponseBinding, wantField: controlplane.ResponseHeaderFieldInstallation},
		{name: "different route family names the bound fact", value: paymentResponseWithExpectation(base, func(value *controlplane.ResponseExpectation) {
			value.Family = controlwire.RouteFamilyChits
		}), want: core.ErrControlPlaneResponseBinding, wantField: controlplane.ResponseHeaderFieldRouteFamily},
		{name: "different opaque offering names the bound fact", value: paymentResponseWithExpectation(base, func(value *controlplane.ResponseExpectation) {
			value.Offering = paymentAuthOffering(t, 1)
		}), want: core.ErrControlPlaneResponseBinding, wantField: controlplane.ResponseHeaderFieldOffering},
		{name: "provider time rollback preserves its typed identity", value: paymentResponseWithExpectation(base, func(value *controlplane.ResponseExpectation) {
			value.PriorProviderTime = temporal.InstantFromNanoseconds(math.MaxInt64)
		}), want: core.ErrControlPlaneProviderTimeRollback},
		{name: "zero client cannot authenticate an otherwise valid response", value: ResponseVerification{
			Document: document, Expected: fixture.expected,
		}, want: core.ErrControlPlaneContract},
		{name: "foreign client refuses a valid authority response", value: ResponseVerification{
			Client: foreign.client, Document: document, Expected: fixture.expected,
		}, want: core.ErrAttestVerification},
		{name: "zero document cannot acquire response proof", value: ResponseVerification{
			Client: fixture.client, Expected: fixture.expected,
		}, want: core.ErrControlPlaneResponseDocument},
		{name: "zero expectation cannot acquire response proof", value: ResponseVerification{
			Client: fixture.client, Document: document,
		}, want: core.ErrControlPlaneResponseHeader},
		{name: "foreign authority document is refused by the trusted client", value: ResponseVerification{
			Client: fixture.client, Document: foreignDocument, Expected: foreign.expected,
		}, want: core.ErrAttestVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := VerifyResponse(tc.value)
			if !errors.Is(gotErr, tc.want) {
				t.Fatalf("VerifyResponse(%s) error = %v, want errors.Is %v", tc.name, gotErr, tc.want)
			}
			if validationErr := got.Validate(); !errors.Is(validationErr, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("VerifyResponse(%s) proof validation error = %v, want errors.Is %v",
					tc.name, validationErr, core.ErrControlPlaneResponseDocument)
			}
			gotBody, bodyErr := got.Body()
			if !errors.Is(bodyErr, core.ErrControlPlaneResponseDocument) || gotBody.Validate() == nil {
				t.Fatalf("VerifyResponse(%s).Body() = (%v, %v), want invalid zero body and errors.Is %v",
					tc.name, gotBody, bodyErr, core.ErrControlPlaneResponseDocument)
			}
			gotHeader, headerErr := got.Header()
			if !errors.Is(headerErr, core.ErrControlPlaneResponseDocument) || gotHeader != (controlplane.ResponseHeader{}) {
				t.Fatalf("VerifyResponse(%s).Header() = (%v, %v), want zero header and errors.Is %v",
					tc.name, gotHeader, headerErr, core.ErrControlPlaneResponseDocument)
			}
			if tc.wantField == controlplane.ResponseHeaderFieldUnknown {
				return
			}
			var binding controlplane.ResponseBindingError
			if !errors.As(gotErr, &binding) || binding.Field() != tc.wantField {
				t.Fatalf("VerifyResponse(%s) binding field = %v, want %v", tc.name, binding.Field(), tc.wantField)
			}
		})
	}
}

func paymentResponseWithExpectation(
	base ResponseVerification,
	mutate func(*controlplane.ResponseExpectation),
) ResponseVerification {
	mutate(&base.Expected)
	return base
}

func newPaymentResponseFixture(t testing.TB) paymentResponseFixture {
	t.Helper()

	return newPaymentResponseFixtureWithMarkers(t, 0x82, 0x83)
}

func newPaymentResponseFixtureWithMarkers(
	t testing.TB,
	authorityMarker byte,
	deviceMarker byte,
) paymentResponseFixture {
	t.Helper()

	requestSpec := standardPaymentQueryFixtureRequest(t)
	requestSpec.authorityByte = authorityMarker
	requestSpec.deviceByte = deviceMarker
	request := newPaymentQueryFixture(t, requestSpec)
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
	settled := paymentResponseReceipt(t, paymentResponseReceiptRequest{
		Signer: signer, Scope: request.payload.Query.Scope,
	})
	commitment, err := payment.CommitQuery(request.payload)
	if err != nil {
		t.Fatalf("payment.CommitQuery(real query) error = %v, want nil", err)
	}
	body, err := payment.IssueCatalog(payment.CatalogIssuance{Signer: signer, Payload: payment.CatalogPayload{
		Entries: []payment.Document{settled}, Watermark: watermark,
		ObservedAt: request.document.Certificate.Body.IssuedAt,
		Scope:      request.payload.Query.Scope, Request: commitment, Continuation: payment.End(),
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
		body: body, header: header, expected: expected, trusted: request.trusted,
		signer: signer, request: request.payload, settled: settled,
		client: request.client, server: request.server,
	}
}

func issuePaymentResponseDocument(
	t testing.TB,
	fixture paymentResponseFixture,
) controlplane.ResponseDocument[payment.CatalogDocument, *payment.CatalogDocument] {
	t.Helper()

	projection, err := IssueResponse(ResponseIssuance{
		Server: fixture.server, Signer: fixture.signer, Header: fixture.header,
		Body: fixture.body, Assessment: acceptedPaymentResponseAssessment(t, fixture.header),
	})
	if err != nil {
		t.Fatalf("IssueResponse(fixture) error = %v, want nil", err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil || len(encoded) == 0 {
		t.Fatalf("ResponseProjection.MarshalJSON(fixture) = (%d bytes, %v), want non-empty and nil",
			len(encoded), err)
	}
	var document controlplane.ResponseDocument[payment.CatalogDocument, *payment.CatalogDocument]
	if err := document.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("ResponseDocument.UnmarshalJSON(fixture) error = %v, want nil", err)
	}
	return document
}

func issueAuthenticPaymentCatalogForFamily(
	t testing.TB,
	fixture paymentResponseFixture,
	family controlwire.RouteFamily,
) (
	controlplane.ResponseDocument[payment.CatalogDocument, *payment.CatalogDocument],
	controlplane.ResponseExpectation,
) {
	t.Helper()

	header := fixture.header
	header.Family = family
	projection, err := controlplane.IssueResponse(controlplane.ResponseIssuance[payment.CatalogDocument]{
		Server: fixture.server, Signer: fixture.signer, Header: header,
		Body: fixture.body, Assessment: acceptedPaymentResponseAssessment(t, header),
	})
	if err != nil {
		t.Fatalf("controlplane.IssueResponse(authentic %v family) error = %v, want nil", family, err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("ResponseProjection.MarshalJSON(authentic %v family) error = %v, want nil", family, err)
	}
	var document controlplane.ResponseDocument[payment.CatalogDocument, *payment.CatalogDocument]
	if err := document.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("ResponseDocument.UnmarshalJSON(authentic %v family) error = %v, want nil", family, err)
	}
	expected := controlplane.ResponseExpectation{
		RequestNonce: header.RequestNonce,
		Account:      header.Account,
		Installation: header.Installation,
		Revision:     header.Revision,
		Family:       header.Family,
		Offering:     header.Offering,
	}
	return document, expected
}

type paymentResponseReceiptRequest struct {
	Signer ed25519.PrivateKey
	Scope  receipt.Scope
}

func paymentResponseReceipt(t testing.TB, request paymentResponseReceiptRequest) payment.Document {
	t.Helper()

	selection := paymentQuerySpecificSelection(t)
	amount, err := currency.New(currency.CodeUSD, 1)
	if err != nil {
		t.Fatalf("currency.New(minimum positive USD amount) error = %v, want nil", err)
	}
	document, err := payment.Issue(payment.Issuance{
		Signer: request.Signer,
		Payload: payment.Payload{
			Identity: selection.Payment, Scope: request.Scope, Amount: amount,
			PaidAt: temporal.InstantFromNanoseconds(1),
			Service: payment.ServicePeriod{
				Start: temporal.InstantFromNanoseconds(1), End: temporal.InstantFromNanoseconds(2),
			},
		},
	})
	if err != nil {
		t.Fatalf("payment.Issue(exact settled receipt) error = %v, want nil", err)
	}
	return document
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
