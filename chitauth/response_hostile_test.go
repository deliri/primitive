package chitauth

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

type chitResponseFixture struct {
	expected controlplane.ResponseExpectation
	signer   ed25519.PrivateKey
	header   controlplane.ResponseHeader
	body     chit.CatalogDocument
	client   controlplane.Client
	server   controlplane.Authority
}

type chitResponseIdentityCase struct {
	name            string
	authorityMarker byte
	deviceMarker    byte
}

func TestChitResponseVerificationLayerTriadClosesTheChitRouteFamily(t *testing.T) {
	t.Parallel()

	fixture := newChitResponseFixture(t)

	t.Run("positive ten authentic chit responses expose their exact catalogs", func(t *testing.T) {
		t.Parallel()

		for _, tc := range chitResponseIdentityCases() {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				candidate := newChitResponseFixtureWithMarkers(t, tc.authorityMarker, tc.deviceMarker)
				document := issueChitResponseDocument(t, candidate)
				verification := ResponseVerification{
					Client: candidate.client, Document: document, Expected: candidate.expected,
				}
				if validationErr := verification.Validate(); validationErr != nil {
					t.Fatalf("ResponseVerification.Validate(authentic chit family) error = %v, want nil", validationErr)
				}
				got, gotErr := VerifyResponse(verification)
				if gotErr != nil {
					t.Fatalf("VerifyResponse(authentic chit family) error = %v, want nil", gotErr)
				}
				gotBody, gotBodyErr := got.Body()
				gotJSON, gotJSONErr := gotBody.MarshalJSON()
				wantJSON, wantJSONErr := candidate.body.MarshalJSON()
				if gotBodyErr != nil || gotJSONErr != nil || wantJSONErr != nil || !bytes.Equal(gotJSON, wantJSON) {
					t.Fatalf("VerifyResponse(authentic chit family).Body() = (%d bytes, %v, %v), want exact %d-byte catalog and nil",
						len(gotJSON), gotBodyErr, gotJSONErr, len(wantJSON))
				}
			})
		}
	})

	t.Run("negative every authentic sibling family is refused without proof", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name   string
			family controlwire.RouteFamily
		}{
			{name: "registration response cannot cross the chit verifier", family: controlwire.RouteFamilyRegistrations},
			{name: "check-in response cannot cross the chit verifier", family: controlwire.RouteFamilyCheckIns},
			{name: "submission response cannot cross the chit verifier", family: controlwire.RouteFamilySubmissions},
			{name: "submission completion cannot cross the chit verifier", family: controlwire.RouteFamilySubmissionCompletions},
			{name: "retrieval response cannot cross the chit verifier", family: controlwire.RouteFamilyRetrievals},
			{name: "payment response cannot cross the chit verifier", family: controlwire.RouteFamilyPayments},
			{name: "release material response cannot cross the chit verifier", family: controlwire.RouteFamilyReleaseMaterials},
			{name: "release publication cannot cross the chit verifier", family: controlwire.RouteFamilyReleasePublications},
			{name: "release publication completion cannot cross the chit verifier", family: controlwire.RouteFamilyReleasePublicationCompletions},
			{name: "update check cannot cross the chit verifier", family: controlwire.RouteFamilyUpdateChecks},
			{name: "upgrade response cannot cross the chit verifier", family: controlwire.RouteFamilyUpgrades},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				document, expected := issueAuthenticChitCatalogForFamily(t, fixture, tc.family)
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
		document := issueChitResponseDocument(t, fixture)
		proveChitResponseVerificationRejections(t, fixture, document)
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

func TestChitResponseIssuanceLayerTriadBindsOnlyTheChitRouteFamily(t *testing.T) {
	t.Parallel()

	fixture := newChitResponseFixture(t)

	t.Run("positive ten authenticated catalog projections survive wire closure", func(t *testing.T) {
		t.Parallel()

		for _, tc := range chitResponseIdentityCases() {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				candidate := newChitResponseFixtureWithMarkers(t, tc.authorityMarker, tc.deviceMarker)
				document := issueChitResponseDocument(t, candidate)
				got, gotErr := VerifyResponse(ResponseVerification{
					Client: candidate.client, Document: document, Expected: candidate.expected,
				})
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("issued chit response verification = (%v, %v), want valid proof and nil", got, gotErr)
				}
			})
		}
	})

	t.Run("negative sibling families and invalid authority inputs emit no projection", func(t *testing.T) {
		t.Parallel()
		proveChitResponseIssuanceRejections(t, fixture)
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

func chitResponseIdentityCases() []chitResponseIdentityCase {
	return []chitResponseIdentityCase{
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

func proveChitResponseIssuanceRejections(t *testing.T, fixture chitResponseFixture) {
	t.Helper()

	valid := ResponseIssuance{
		Server: fixture.server, Signer: fixture.signer, Header: fixture.header,
		Body: fixture.body, Assessment: acceptedChitResponseAssessment(t, fixture.header),
	}
	zeroServer := valid
	zeroServer.Server = controlplane.Authority{}
	nilSigner := valid
	nilSigner.Signer = nil
	zeroBody := valid
	zeroBody.Body = chit.CatalogDocument{}
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
		{name: "payment family", family: controlwire.RouteFamilyPayments},
		{name: "release material family", family: controlwire.RouteFamilyReleaseMaterials},
		{name: "release publication family", family: controlwire.RouteFamilyReleasePublications},
		{name: "release publication completion family", family: controlwire.RouteFamilyReleasePublicationCompletions},
		{name: "update check family", family: controlwire.RouteFamilyUpdateChecks},
		{name: "upgrade family", family: controlwire.RouteFamilyUpgrades},
	}
	for _, tc := range families {
		t.Run(tc.name+" cannot be issued by the chit wrapper", func(t *testing.T) {
			t.Parallel()

			value := valid
			value.Header.Family = tc.family
			value.Assessment = acceptedChitResponseAssessment(t, value.Header)
			if validationErr := value.Validate(); !errors.Is(validationErr, core.ErrControlPlaneResponseBinding) {
				t.Fatalf("ResponseIssuance.Validate(%v family) error = %v, want errors.Is %v",
					tc.family, validationErr, core.ErrControlPlaneResponseBinding)
			}
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
		want  error
		name  string
		value ResponseIssuance
	}{
		{name: "zero server capability cannot issue a response", value: zeroServer, want: core.ErrControlPlaneContract},
		{name: "nil authority signer cannot issue a response", value: nilSigner, want: core.ErrAttestContract},
		{name: "zero catalog body cannot issue a response", value: zeroBody, want: core.ErrChitContract},
		{name: "zero protocol assessment cannot issue a response", value: zeroAssessment, want: core.ErrControlWireContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if validationErr := tc.value.Validate(); !errors.Is(validationErr, tc.want) {
				t.Fatalf("ResponseIssuance.Validate(%s) error = %v, want errors.Is %v", tc.name, validationErr, tc.want)
			}
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

func proveChitResponseVerificationRejections(
	t *testing.T,
	fixture chitResponseFixture,
	document controlplane.ResponseDocument[chit.CatalogDocument, *chit.CatalogDocument],
) {
	t.Helper()

	foreign := newChitResponseFixtureWithMarkers(t, 0x91, 0x92)
	foreignDocument := issueChitResponseDocument(t, foreign)
	base := ResponseVerification{Client: fixture.client, Document: document, Expected: fixture.expected}
	cases := []struct {
		want              error
		wantValidationErr error
		name              string
		value             ResponseVerification
		wantField         controlplane.ResponseHeaderField
	}{
		{name: "different request nonce names the bound fact", value: chitResponseWithExpectation(base, func(value *controlplane.ResponseExpectation) {
			value.RequestNonce = queryNonce(t, 0x72)
		}), want: core.ErrControlPlaneResponseBinding, wantField: controlplane.ResponseHeaderFieldRequestNonce},
		{name: "different account names the bound fact", value: chitResponseWithExpectation(base, func(value *controlplane.ResponseExpectation) {
			value.Account = queryAccount(t, 0x73)
		}), want: core.ErrControlPlaneResponseBinding, wantField: controlplane.ResponseHeaderFieldAccount},
		{name: "different installation names the bound fact", value: chitResponseWithExpectation(base, func(value *controlplane.ResponseExpectation) {
			value.Installation = foreign.expected.Installation
		}), want: core.ErrControlPlaneResponseBinding, wantField: controlplane.ResponseHeaderFieldInstallation},
		{name: "different route family names the bound fact", value: chitResponseWithExpectation(base, func(value *controlplane.ResponseExpectation) {
			value.Family = controlwire.RouteFamilyPayments
		}), want: core.ErrControlPlaneResponseBinding, wantField: controlplane.ResponseHeaderFieldRouteFamily},
		{name: "different opaque offering names the bound fact", value: chitResponseWithExpectation(base, func(value *controlplane.ResponseExpectation) {
			value.Offering = chitAuthOffering(t, 1)
		}), want: core.ErrControlPlaneResponseBinding, wantField: controlplane.ResponseHeaderFieldOffering},
		{name: "provider time rollback preserves its typed identity", value: chitResponseWithExpectation(base, func(value *controlplane.ResponseExpectation) {
			value.PriorProviderTime = temporal.InstantFromNanoseconds(math.MaxInt64)
		}), want: core.ErrControlPlaneProviderTimeRollback},
		{name: "zero client cannot authenticate an otherwise valid response", value: ResponseVerification{
			Document: document, Expected: fixture.expected,
		}, want: core.ErrControlPlaneContract, wantValidationErr: core.ErrControlPlaneContract},
		{name: "foreign client refuses a valid authority response", value: ResponseVerification{
			Client: foreign.client, Document: document, Expected: fixture.expected,
		}, want: core.ErrAttestVerification},
		{name: "zero document cannot acquire response proof", value: ResponseVerification{
			Client: fixture.client, Expected: fixture.expected,
		}, want: core.ErrControlPlaneResponseDocument, wantValidationErr: core.ErrControlPlaneResponseDocument},
		{name: "zero expectation cannot acquire response proof", value: ResponseVerification{
			Client: fixture.client, Document: document,
		}, want: core.ErrControlPlaneResponseHeader, wantValidationErr: core.ErrControlPlaneResponseHeader},
		{name: "foreign authority document is refused by the trusted client", value: ResponseVerification{
			Client: fixture.client, Document: foreignDocument, Expected: foreign.expected,
		}, want: core.ErrAttestVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			validationErr := tc.value.Validate()
			if tc.wantValidationErr == nil && validationErr != nil {
				t.Fatalf("ResponseVerification.Validate(%s) error = %v, want nil before authentication", tc.name, validationErr)
			}
			if tc.wantValidationErr != nil && !errors.Is(validationErr, tc.wantValidationErr) {
				t.Fatalf("ResponseVerification.Validate(%s) error = %v, want errors.Is %v", tc.name, validationErr, tc.wantValidationErr)
			}
			got, gotErr := VerifyResponse(tc.value)
			if !errors.Is(gotErr, tc.want) {
				t.Fatalf("VerifyResponse(%s) error = %v, want errors.Is %v", tc.name, gotErr, tc.want)
			}
			if validationErr := got.Validate(); !errors.Is(validationErr, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("VerifyResponse(%s) proof validation error = %v, want errors.Is %v",
					tc.name, validationErr, core.ErrControlPlaneResponseDocument)
			}
			if gotBody, bodyErr := got.Body(); bodyErr == nil || gotBody.Validate() == nil {
				t.Fatalf("VerifyResponse(%s).Body() = (%v, %v), want invalid zero body and error", tc.name, gotBody, bodyErr)
			}
			if gotHeader, headerErr := got.Header(); headerErr == nil || gotHeader != (controlplane.ResponseHeader{}) {
				t.Fatalf("VerifyResponse(%s).Header() = (%v, %v), want zero header and error", tc.name, gotHeader, headerErr)
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

func chitResponseWithExpectation(
	base ResponseVerification,
	mutate func(*controlplane.ResponseExpectation),
) ResponseVerification {
	mutate(&base.Expected)
	return base
}

func newChitResponseFixture(t testing.TB) chitResponseFixture {
	t.Helper()

	return newChitResponseFixtureWithMarkers(t, 0x81, 0x82)
}

func newChitResponseFixtureWithMarkers(
	t testing.TB,
	authorityMarker byte,
	deviceMarker byte,
) chitResponseFixture {
	t.Helper()

	requestSpec := standardQueryFixtureRequest(t)
	requestSpec.authorityByte = authorityMarker
	requestSpec.deviceByte = deviceMarker
	request := newQueryFixture(t, requestSpec)
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
		t.Fatalf("receipt.NewWatermark(real query scope) error = %v, want nil", err)
	}
	commitment, err := chit.CommitQuery(request.payload)
	if err != nil {
		t.Fatalf("chit.CommitQuery(real request) error = %v, want nil", err)
	}
	seed := querySeed(authorityMarker)
	signer := ed25519.NewKeyFromSeed(seed[:])
	body, err := chit.IssueCatalog(chit.CatalogIssuance{Signer: signer, Payload: chit.CatalogPayload{
		Entries: []chit.CatalogEntry{}, Watermark: watermark,
		ObservedAt: request.document.Certificate.Body.IssuedAt,
		Scope:      request.payload.Query.Scope, Request: commitment, Continuation: chit.End(),
	}})
	if err != nil {
		t.Fatalf("chit.IssueCatalog(real empty page) error = %v, want nil", err)
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
		Family:       controlwire.RouteFamilyChits,
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
	return chitResponseFixture{
		body: body, header: header, expected: expected, signer: signer,
		client: request.client, server: request.server,
	}
}

func issueChitResponseDocument(
	t testing.TB,
	fixture chitResponseFixture,
) controlplane.ResponseDocument[chit.CatalogDocument, *chit.CatalogDocument] {
	t.Helper()

	issuance := ResponseIssuance{
		Server: fixture.server, Signer: fixture.signer, Header: fixture.header,
		Body: fixture.body, Assessment: acceptedChitResponseAssessment(t, fixture.header),
	}
	if validationErr := issuance.Validate(); validationErr != nil {
		t.Fatalf("ResponseIssuance.Validate(fixture) error = %v, want nil", validationErr)
	}
	projection, err := IssueResponse(issuance)
	if err != nil {
		t.Fatalf("IssueResponse(fixture) error = %v, want nil", err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("ResponseProjection.MarshalJSON(fixture) error = %v, want nil", err)
	}
	var document controlplane.ResponseDocument[chit.CatalogDocument, *chit.CatalogDocument]
	if err := document.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("ResponseDocument.UnmarshalJSON(fixture) error = %v, want nil", err)
	}
	return document
}

func issueAuthenticChitCatalogForFamily(
	t testing.TB,
	fixture chitResponseFixture,
	family controlwire.RouteFamily,
) (
	controlplane.ResponseDocument[chit.CatalogDocument, *chit.CatalogDocument],
	controlplane.ResponseExpectation,
) {
	t.Helper()

	header := fixture.header
	header.Family = family
	projection, err := controlplane.IssueResponse(controlplane.ResponseIssuance[chit.CatalogDocument]{
		Server: fixture.server, Signer: fixture.signer, Header: header,
		Body: fixture.body, Assessment: acceptedChitResponseAssessment(t, header),
	})
	if err != nil {
		t.Fatalf("controlplane.IssueResponse(authentic %v family) error = %v, want nil", family, err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("ResponseProjection.MarshalJSON(authentic %v family) error = %v, want nil", family, err)
	}
	var document controlplane.ResponseDocument[chit.CatalogDocument, *chit.CatalogDocument]
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

func acceptedChitResponseAssessment(t testing.TB, header controlplane.ResponseHeader) controlwire.ProtocolAssessment {
	t.Helper()
	support, err := controlwire.PublishedProtocolSupport()
	if err != nil {
		t.Fatalf("controlwire.PublishedProtocolSupport() error = %v, want nil", err)
	}
	assessment, err := controlwire.AssessProtocol(controlwire.ProtocolAssessmentRequest{
		Support: support, Capability: controlwire.ProtocolCapability{Revision: header.Revision, Family: header.Family},
	})
	if err != nil {
		t.Fatalf("controlwire.AssessProtocol(published chit response pair) error = %v, want nil", err)
	}
	return assessment
}
