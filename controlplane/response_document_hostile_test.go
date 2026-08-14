package controlplane_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

type authenticatedResponseFixture struct {
	signer    ed25519.PrivateKey
	canonical []byte
	body      controlplane.RegistrationDocument
	trusted   attest.TrustedKeys
	header    controlplane.ResponseHeader
	expected  controlplane.ResponseExpectation
}

type responseValidCase struct {
	mutate func(testing.TB, *controlplane.ResponseHeader)
	name   string
}

func TestAuthenticatedUpgradeRequiredResponseCannotExposeAProductBody(t *testing.T) {
	t.Parallel()

	fixture := authenticatedResponseForTest(t, 30)
	header := fixture.header
	header.Status = controlplane.ProductStatusUpgradeRequired
	projection, err := controlplane.IssueUpgradeRequiredResponse[controlplane.RegistrationDocument](controlplane.UpgradeRequiredIssuance{
		Signer:     fixture.signer,
		Header:     header,
		Assessment: upgradeRequiredProtocolAssessment(t, header),
	})
	if err != nil {
		t.Fatalf("IssueUpgradeRequiredResponse() error = %v, want nil signed refusal", err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("ResponseProjection.MarshalJSON(upgrade-required status) error = %v, want nil", err)
	}
	document := decodeAuthenticatedResponse(t, encoded)
	expected := fixture.expected
	verified, err := controlplane.VerifyResponse(controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{
		Document: document, Expected: expected, TrustedKeys: fixture.trusted,
	})
	if err != nil {
		t.Fatalf("VerifyResponse(authentic upgrade-required response) error = %v, want nil", err)
	}
	got, gotErr := verified.Body()
	if !errors.Is(gotErr, core.ErrControlPlaneUpgradeRequired) {
		t.Fatalf("VerifiedResponse.Body(upgrade-required response) error = %v, want %v", gotErr, core.ErrControlPlaneUpgradeRequired)
	}
	if got != (controlplane.RegistrationDocument{}) {
		t.Fatalf("VerifiedResponse.Body(upgrade-required response) = %+v, want zero product body", got)
	}
}

func TestAuthenticatedResponseProducerExhaustsValidDecisionDomains(t *testing.T) {
	t.Parallel()

	base := authenticatedResponseForTest(t, 31)
	cases := []responseValidCase{
		{name: "active status survives signed round trip", mutate: responseStatusMutation(controlplane.ProductStatusActive)},
		{name: "payment retry status survives signed round trip", mutate: responseStatusMutation(controlplane.ProductStatusPaymentRetry)},
		{name: "read only status survives signed round trip", mutate: responseStatusMutation(controlplane.ProductStatusReadOnly)},
		{name: "stopped status survives signed round trip", mutate: responseStatusMutation(controlplane.ProductStatusStopped)},
		{name: "revoked status survives signed round trip", mutate: responseStatusMutation(controlplane.ProductStatusRevoked)},
		{name: "bug offering survives signed round trip", mutate: responseOfferingMutation(core.OfferingBug)},
		{name: "witness offering survives signed round trip", mutate: responseOfferingMutation(core.OfferingWitness)},
		{name: "peachfuzz offering survives signed round trip", mutate: responseOfferingMutation(core.OfferingPeachfuzz)},
		{name: "minimum signed Unix instant survives signed round trip", mutate: responseProviderTimeMutation(math.MinInt64)},
		{name: "maximum signed Unix instant survives signed round trip", mutate: responseProviderTimeMutation(math.MaxInt64)},
		{name: "maximum policy activation survives signed round trip", mutate: responsePolicyActivationMutation(math.MaxUint64)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			header := base.header
			tc.mutate(t, &header)
			fixture := authenticatedResponseWithHeader(t, base, header)
			document := decodeAuthenticatedResponse(t, fixture.canonical)
			verified, err := controlplane.VerifyResponse(controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{
				Document: document, Expected: fixture.expected, TrustedKeys: fixture.trusted,
			})
			if err != nil {
				t.Fatalf("VerifyResponse(compiler-produced response) error = %v, want nil", err)
			}
			gotHeader, err := verified.Header()
			if err != nil || gotHeader != header {
				t.Fatalf("VerifiedResponse.Header() = (%+v, %v), want (%+v, nil)", gotHeader, err, header)
			}
			gotBody, err := verified.Body()
			if err != nil {
				t.Fatalf("VerifiedResponse.Body() error = %v, want nil", err)
			}
			proveRegistrationBodyEqual(t, gotBody, base.body)
			proveAuthenticatedResponseCanonicalClosure(t, fixture, document)
		})
	}
}

func TestAuthenticatedResponseProducerRejectsIndependentInvalidInputs(t *testing.T) {
	t.Parallel()

	fixture := authenticatedResponseForTest(t, 41)
	cases := []struct {
		want   error
		mutate func(*controlplane.ResponseIssuance[controlplane.RegistrationDocument])
		name   string
	}{
		{name: "nil authority signer is rejected", mutate: func(value *controlplane.ResponseIssuance[controlplane.RegistrationDocument]) { value.Signer = nil }, want: core.ErrAttestContract},
		{name: "zero response header is rejected", mutate: func(value *controlplane.ResponseIssuance[controlplane.RegistrationDocument]) {
			value.Header = controlplane.ResponseHeader{}
		}, want: core.ErrControlPlaneResponseHeader},
		{name: "zero provider time is rejected", mutate: func(value *controlplane.ResponseIssuance[controlplane.RegistrationDocument]) {
			value.Header.ProviderTime = temporal.Instant{}
		}, want: core.ErrControlPlaneResponseHeader},
		{name: "zero request nonce is rejected", mutate: func(value *controlplane.ResponseIssuance[controlplane.RegistrationDocument]) {
			value.Header.RequestNonce = controlwire.RequestNonce{}
		}, want: core.ErrControlPlaneResponseHeader},
		{name: "zero account is rejected", mutate: func(value *controlplane.ResponseIssuance[controlplane.RegistrationDocument]) {
			value.Header.Account = receipt.AccountIdentity{}
		}, want: core.ErrControlPlaneResponseHeader},
		{name: "zero installation is rejected", mutate: func(value *controlplane.ResponseIssuance[controlplane.RegistrationDocument]) {
			value.Header.Installation = lease.DeviceID{}
		}, want: core.ErrControlPlaneResponseHeader},
		{name: "zero revision is rejected", mutate: func(value *controlplane.ResponseIssuance[controlplane.RegistrationDocument]) {
			value.Header.Revision = 0
		}, want: core.ErrControlPlaneResponseHeader},
		{name: "zero route family is rejected", mutate: func(value *controlplane.ResponseIssuance[controlplane.RegistrationDocument]) {
			value.Header.Family = controlwire.RouteFamilyUnknown
		}, want: core.ErrControlPlaneResponseHeader},
		{name: "unset product status is rejected", mutate: func(value *controlplane.ResponseIssuance[controlplane.RegistrationDocument]) {
			value.Header.Status = controlplane.ProductStatusInvalid
		}, want: core.ErrControlPlaneProductStatus},
		{name: "future product status is rejected", mutate: func(value *controlplane.ResponseIssuance[controlplane.RegistrationDocument]) {
			value.Header.Status = controlplane.ProductStatus(math.MaxUint8)
		}, want: core.ErrControlPlaneProductStatus},
		{name: "future offering is rejected", mutate: func(value *controlplane.ResponseIssuance[controlplane.RegistrationDocument]) {
			value.Header.Offering = core.Offering(math.MaxUint8)
		}, want: core.ErrControlPlaneResponseHeader},
		{name: "zero policy cursor is rejected", mutate: func(value *controlplane.ResponseIssuance[controlplane.RegistrationDocument]) {
			value.Header.Policy = controlwire.PolicyCursor{}
		}, want: core.ErrControlPlaneResponseHeader},
		{name: "zero product body is rejected", mutate: func(value *controlplane.ResponseIssuance[controlplane.RegistrationDocument]) {
			value.Body = controlplane.RegistrationDocument{}
		}, want: core.ErrControlPlaneRegistration},
		{name: "zero protocol assessment is rejected", mutate: func(value *controlplane.ResponseIssuance[controlplane.RegistrationDocument]) {
			value.Assessment = controlwire.ProtocolAssessment{}
		}, want: core.ErrControlWireContract},
		{name: "upgrade-required assessment cannot issue a product body", mutate: func(value *controlplane.ResponseIssuance[controlplane.RegistrationDocument]) {
			value.Assessment = upgradeRequiredProtocolAssessment(t, value.Header)
		}, want: core.ErrControlPlaneResponseBinding},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			issuance := controlplane.ResponseIssuance[controlplane.RegistrationDocument]{
				Signer: fixture.signer, Header: fixture.header, Body: fixture.body,
				Assessment: acceptedProtocolAssessment(t, fixture.header),
			}
			tc.mutate(&issuance)
			got, gotErr := controlplane.IssueResponse(issuance)
			if !errors.Is(gotErr, core.ErrControlPlaneResponseDocument) || !errors.Is(gotErr, tc.want) {
				t.Fatalf("IssueResponse() error = %v, want %v/%v", gotErr, core.ErrControlPlaneResponseDocument, tc.want)
			}
			if encoded, err := got.MarshalJSON(); encoded != nil || !errors.Is(err, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("rejected projection MarshalJSON() = (%s, %v), want nil and %v", encoded, err, core.ErrControlPlaneResponseDocument)
			}
		})
	}
}

func TestAuthenticatedResponseProjectionStrictlyEncodesWithoutBecomingAnIngressType(t *testing.T) {
	t.Parallel()

	fixture := authenticatedResponseForTest(t, 46)
	issuance := controlplane.ResponseIssuance[controlplane.RegistrationDocument]{
		Signer: fixture.signer, Header: fixture.header, Body: fixture.body,
		Assessment: acceptedProtocolAssessment(t, fixture.header),
	}
	if err := issuance.Validate(); err != nil {
		t.Fatalf("ResponseIssuance.Validate() error = %v, want nil", err)
	}
	projection, err := controlplane.IssueResponse(issuance)
	if err != nil {
		t.Fatalf("IssueResponse() error = %v, want nil", err)
	}
	encoded, err := core.EncodeValidatedJSON(projection, core.DefaultStrictJSONLimits())
	if err != nil || !bytes.Equal(encoded, fixture.canonical) {
		t.Fatalf("EncodeValidatedJSON(issue-only projection) = (%d bytes, %v), want exact %d-byte authenticated response", len(encoded), err, len(fixture.canonical))
	}

	foreign := authenticatedResponseForTest(t, 47)
	if bytes.Equal(foreign.canonical, fixture.canonical) {
		t.Fatalf("foreign authenticated response bytes = %d identical bytes, want a load-bearing difference", len(foreign.canonical))
	}
	if err := projection.ValidateJSONProjection(foreign.canonical, core.DefaultStrictJSONLimits()); !errors.Is(err, core.ErrControlPlaneResponseDocument) || !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("ValidateJSONProjection(foreign exact bytes) error = %v, want %v/%v", err, core.ErrControlPlaneResponseDocument, core.ErrJSONContract)
	}

	maximum, err := core.NewByteCount(uint64(len(fixture.canonical) - 1))
	if err != nil {
		t.Fatalf("NewByteCount(one below response length) error = %v, want nil", err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	if got, err := core.EncodeValidatedJSON(projection, limits); got != nil || !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("EncodeValidatedJSON(one-byte-short limit) = (%d bytes, %v), want nil and %v", len(got), err, core.ErrJSONContract)
	}

	var zero controlplane.ResponseProjection[controlplane.RegistrationDocument]
	if got, err := core.EncodeValidatedJSON(zero, core.DefaultStrictJSONLimits()); got != nil ||
		!errors.Is(err, core.ErrControlPlaneResponseDocument) {
		t.Fatalf("EncodeValidatedJSON(zero projection) = (%d bytes, %v), want nil and %v", len(got), err, core.ErrControlPlaneResponseDocument)
	}
}

func TestProductResponseFamilyGateRejectsAnOtherwiseValidSiblingRoute(t *testing.T) {
	t.Parallel()

	fixture := authenticatedResponseForTest(t, 45)
	issuance := controlplane.ResponseIssuance[controlplane.RegistrationDocument]{
		Signer: fixture.signer, Header: fixture.header, Body: fixture.body,
		Assessment: acceptedProtocolAssessment(t, fixture.header),
	}
	if err := issuance.Validate(); err != nil {
		t.Fatalf("ResponseIssuance.Validate(generic valid response) error = %v, want nil", err)
	}
	if err := issuance.ValidateForFamily(controlwire.RouteFamilyCheckIns); !errors.Is(err, core.ErrControlPlaneResponseDocument) ||
		!errors.Is(err, core.ErrControlPlaneResponseBinding) {
		t.Fatalf("ResponseIssuance.ValidateForFamily(sibling route) error = %v, want %v/%v", err, core.ErrControlPlaneResponseDocument, core.ErrControlPlaneResponseBinding)
	}
	projection, err := controlplane.IssueResponseForFamily(issuance, controlwire.RouteFamilyCheckIns)
	if !errors.Is(err, core.ErrControlPlaneResponseDocument) || !errors.Is(err, core.ErrControlPlaneResponseBinding) || projection.Validate() == nil {
		t.Fatalf("IssueResponseForFamily(sibling route) = (%v, %v), want invalid zero and %v/%v", projection, err, core.ErrControlPlaneResponseDocument, core.ErrControlPlaneResponseBinding)
	}
}

func TestAuthenticatedResponseDecoderRejectsEveryCrossResponseFactSubstitution(t *testing.T) {
	t.Parallel()

	fixture := authenticatedResponseForTest(t, 48)
	baseParts := responsePartsForTest(t, fixture)
	cases := []responseValidCase{
		{name: "authority time from another signed response", mutate: responseProviderTimeMutation(1_700_000_000_000_000_001)},
		{name: "request nonce from another signed response", mutate: func(t testing.TB, header *controlplane.ResponseHeader) { header.RequestNonce = otherRequestNonce(t) }},
		{name: "account from another signed response", mutate: func(t testing.TB, header *controlplane.ResponseHeader) {
			header.Account = responseAccountIdentity(t, 0xa3)
		}},
		{name: "installation from another signed response", mutate: func(t testing.TB, header *controlplane.ResponseHeader) {
			header.Installation = responseDeviceID(t, 0xb4)
		}},
		{name: "route family from another signed response", mutate: func(_ testing.TB, header *controlplane.ResponseHeader) {
			header.Family = controlwire.RouteFamilyCheckIns
		}},
		{name: "product status from another signed response", mutate: responseStatusMutation(alternateStatus(fixture.header.Status))},
		{name: "offering from another signed response", mutate: responseOfferingMutation(alternateOffering(fixture.header.Offering))},
		{name: "policy cursor from another signed response", mutate: nextPolicyActivationMutation(t, fixture.header.Policy.Activation)},
	}

	preserved := decodeAuthenticatedResponse(t, fixture.canonical)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			header := fixture.header
			tc.mutate(t, &header)
			variant := authenticatedResponseWithHeader(t, fixture, header)
			variantParts := responsePartsForTest(t, variant)
			if bytes.Equal(variantParts.header, baseParts.header) {
				t.Fatalf("compiler-produced signed header mutation = %d identical bytes, want a load-bearing difference", len(variantParts.header))
			}
			recombined := baseParts.object(variantParts.header, baseParts.body, baseParts.attestation)
			candidate := preserved
			gotErr := candidate.UnmarshalJSON(recombined)
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrControlPlaneResponseDocument) ||
				!errors.Is(gotErr, core.ErrAttestVerification) {
				t.Fatalf("ResponseDocument.UnmarshalJSON(cross-response header) error = %v, want %v/%v/%v", gotErr, core.ErrJSONContract, core.ErrControlPlaneResponseDocument, core.ErrAttestVerification)
			}
			if err := candidate.Validate(); err != nil {
				t.Fatalf("cross-response rejection changed populated receiver: Validate() error = %v, want nil", err)
			}
		})
	}

	foreign := authenticatedResponseForTest(t, 49)
	foreignParts := responsePartsForTest(t, foreign)
	if bytes.Equal(foreignParts.body, baseParts.body) {
		t.Fatalf("compiler-produced foreign product body = %d identical bytes, want a load-bearing difference", len(foreignParts.body))
	}
	recombined := baseParts.object(baseParts.header, foreignParts.body, baseParts.attestation)
	candidate := preserved
	gotErr := candidate.UnmarshalJSON(recombined)
	if !errors.Is(gotErr, core.ErrJSONContract) ||
		!errors.Is(gotErr, core.ErrControlPlaneResponseDocument) ||
		!errors.Is(gotErr, core.ErrAttestVerification) {
		t.Fatalf("ResponseDocument.UnmarshalJSON(cross-response body) error = %v, want %v/%v/%v", gotErr, core.ErrJSONContract, core.ErrControlPlaneResponseDocument, core.ErrAttestVerification)
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("cross-response body rejection changed populated receiver: Validate() error = %v, want nil", err)
	}
}

func TestAuthenticatedResponseVerifierNamesEveryBoundFactAndTrustFailure(t *testing.T) {
	t.Parallel()

	fixture := authenticatedResponseForTest(t, 51)
	document := decodeAuthenticatedResponse(t, fixture.canonical)
	otherAccount := responseAccountIdentity(t, 0xa1)
	otherInstallation := responseDeviceID(t, 0xb2)
	_, untrustedSigner := testSigningKey(t, 52)
	untrustedProjection, err := controlplane.IssueResponse(controlplane.ResponseIssuance[controlplane.RegistrationDocument]{
		Signer: untrustedSigner, Header: fixture.header, Body: fixture.body,
		Assessment: acceptedProtocolAssessment(t, fixture.header),
	})
	if err != nil {
		t.Fatalf("IssueResponse(untrusted signer) error = %v, want nil", err)
	}
	untrustedJSON, err := untrustedProjection.MarshalJSON()
	if err != nil {
		t.Fatalf("untrusted projection MarshalJSON() error = %v, want nil", err)
	}
	untrustedDocument := decodeAuthenticatedResponse(t, untrustedJSON)

	cases := []struct {
		want         error
		name         string
		verification controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]
		wantField    controlplane.ResponseHeaderField
	}{
		{name: "matching request and trusted authority expose the exact body", verification: controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{Document: document, Expected: fixture.expected, TrustedKeys: fixture.trusted}},
		{name: "different request nonce is named", verification: responseVerificationWithExpectation(document, fixture, func(value *controlplane.ResponseExpectation) { value.RequestNonce = otherRequestNonce(t) }), want: core.ErrControlPlaneResponseBinding, wantField: controlplane.ResponseHeaderFieldRequestNonce},
		{name: "different account is named", verification: responseVerificationWithExpectation(document, fixture, func(value *controlplane.ResponseExpectation) { value.Account = otherAccount }), want: core.ErrControlPlaneResponseBinding, wantField: controlplane.ResponseHeaderFieldAccount},
		{name: "different installation is named", verification: responseVerificationWithExpectation(document, fixture, func(value *controlplane.ResponseExpectation) { value.Installation = otherInstallation }), want: core.ErrControlPlaneResponseBinding, wantField: controlplane.ResponseHeaderFieldInstallation},
		{name: "different route family is named", verification: responseVerificationWithExpectation(document, fixture, func(value *controlplane.ResponseExpectation) {
			value.Family = controlwire.RouteFamilyCheckIns
		}), want: core.ErrControlPlaneResponseBinding, wantField: controlplane.ResponseHeaderFieldRouteFamily},
		{name: "different offering is named", verification: responseVerificationWithExpectation(document, fixture, func(value *controlplane.ResponseExpectation) {
			value.Offering = alternateOffering(fixture.header.Offering)
		}), want: core.ErrControlPlaneResponseBinding, wantField: controlplane.ResponseHeaderFieldOffering},
		{name: "provider time one nanosecond behind prior is rollback", verification: responseVerificationWithExpectation(document, fixture, func(value *controlplane.ResponseExpectation) {
			value.PriorProviderTime = instantAfter(t, fixture.header.ProviderTime)
		}), want: core.ErrControlPlaneProviderTimeRollback},
		{name: "provider time exactly equal to prior is accepted", verification: responseVerificationWithExpectation(document, fixture, func(value *controlplane.ResponseExpectation) { value.PriorProviderTime = fixture.header.ProviderTime })},
		{name: "unset prior provider time is first contact and accepted", verification: controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{Document: document, Expected: fixture.expected, TrustedKeys: fixture.trusted}},
		{name: "foreign valid signature is rejected by caller trust", verification: controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{Document: untrustedDocument, Expected: fixture.expected, TrustedKeys: fixture.trusted}, want: core.ErrAttestVerification},
		{name: "zero document is rejected", verification: controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{Expected: fixture.expected, TrustedKeys: fixture.trusted}, want: core.ErrControlPlaneResponseDocument},
		{name: "zero expectation is rejected", verification: controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{Document: document, TrustedKeys: fixture.trusted}, want: core.ErrControlPlaneResponseHeader},
		{name: "zero trust set is rejected", verification: controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{Document: document, Expected: fixture.expected}, want: core.ErrAttestContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			verified, gotErr := controlplane.VerifyResponse(tc.verification)
			if tc.want == nil {
				if gotErr != nil {
					t.Fatalf("VerifyResponse() error = %v, want nil", gotErr)
				}
				if err := verified.Validate(); err != nil {
					t.Fatalf("VerifiedResponse.Validate() error = %v, want nil", err)
				}
				return
			}
			if !errors.Is(gotErr, tc.want) {
				t.Fatalf("VerifyResponse() error = %v, want %v", gotErr, tc.want)
			}
			if err := verified.Validate(); !errors.Is(err, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("rejected VerifiedResponse.Validate() error = %v, want %v", err, core.ErrControlPlaneResponseDocument)
			}
			if tc.wantField != controlplane.ResponseHeaderFieldUnknown {
				var binding controlplane.ResponseBindingError
				if !errors.As(gotErr, &binding) || binding.Field() != tc.wantField {
					t.Fatalf("VerifyResponse() binding = (%v, %+v), want field %v", gotErr, binding, tc.wantField)
				}
			}
		})
	}
}

type responseRepresentationCase struct {
	want       error
	wantVerify error
	name       string
	document   []byte
	wantValid  bool
}

func TestAuthenticatedResponseDecoderPressuresFiftySixRepresentations(t *testing.T) {
	t.Parallel()

	fixture := authenticatedResponseForTest(t, 61)
	foreign := authenticatedResponseForTest(t, 62)
	cases := authenticatedResponseRepresentationCases(t, fixture, foreign)
	validCount := 0
	rejectCount := 0
	for _, tc := range cases {
		if tc.wantValid {
			validCount++
		} else {
			rejectCount++
		}
	}
	if len(cases) != 56 || validCount != 11 || rejectCount != 45 {
		t.Fatalf("response pressure inventory = %d total/%d valid/%d reject, want exactly 56/11/45", len(cases), validCount, rejectCount)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			preserved := decodeAuthenticatedResponse(t, fixture.canonical)
			got := preserved
			gotErr := got.UnmarshalJSON(tc.document)
			if tc.wantValid {
				if gotErr != nil {
					t.Fatalf("ResponseDocument.UnmarshalJSON() error = %v, want nil", gotErr)
				}
				if err := got.Validate(); err != nil {
					t.Fatalf("accepted ResponseDocument.Validate() error = %v, want nil", err)
				}
				verified, err := controlplane.VerifyResponse(controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{
					Document: got, Expected: fixture.expected, TrustedKeys: fixture.trusted,
				})
				if tc.wantVerify != nil {
					if !errors.Is(err, core.ErrControlPlaneResponseDocument) || !errors.Is(err, tc.wantVerify) || verified.Validate() == nil {
						t.Fatalf("VerifyResponse(structurally valid foreign response) = (%v, %v), want zero and %v/%v", verified, err, core.ErrControlPlaneResponseDocument, tc.wantVerify)
					}
					return
				}
				if err != nil {
					t.Fatalf("VerifyResponse(accepted representation) error = %v, want nil", err)
				}
				body, err := verified.Body()
				if err != nil {
					t.Fatalf("VerifiedResponse.Body() error = %v, want nil", err)
				}
				proveRegistrationBodyEqual(t, body, fixture.body)
				return
			}
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrControlPlaneResponseDocument) ||
				(tc.want != nil && !errors.Is(gotErr, tc.want)) {
				t.Fatalf("ResponseDocument.UnmarshalJSON(rejected) error = %v, want %v/%v/%v", gotErr, core.ErrJSONContract, core.ErrControlPlaneResponseDocument, tc.want)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("rejected decode changed populated receiver: Validate() error = %v, want nil", err)
			}
			var zero controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]
			zeroErr := zero.UnmarshalJSON(tc.document)
			if !errors.Is(zeroErr, core.ErrJSONContract) ||
				!errors.Is(zeroErr, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("fresh receiver UnmarshalJSON(%d bytes) error = %v, want %v/%v", len(tc.document), zeroErr, core.ErrJSONContract, core.ErrControlPlaneResponseDocument)
			}
			if err := zero.Validate(); !errors.Is(err, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("rejected fresh receiver Validate() error = %v, want %v", err, core.ErrControlPlaneResponseDocument)
			}
		})
	}
}

func FuzzAuthenticatedResponseExternalSemanticClosure(f *testing.F) {
	fixture := authenticatedResponseForTest(f, 71)
	foreign := authenticatedResponseForTest(f, 72)
	for _, tc := range authenticatedResponseRepresentationCases(f, fixture, foreign) {
		f.Add(tc.document)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		preserved := decodeAuthenticatedResponse(t, fixture.canonical)
		got := preserved
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("ResponseDocument.UnmarshalJSON(rejected) error = %v, want %v/%v", gotErr, core.ErrJSONContract, core.ErrControlPlaneResponseDocument)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("rejected decode changed populated receiver: Validate() error = %v, want nil", err)
			}
			var zero controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]
			zeroErr := zero.UnmarshalJSON(data)
			if !errors.Is(zeroErr, core.ErrJSONContract) ||
				!errors.Is(zeroErr, core.ErrControlPlaneResponseDocument) ||
				!errors.Is(zero.Validate(), core.ErrControlPlaneResponseDocument) {
				t.Fatalf("fresh rejected receiver = (decode %v, validate %v), want %v/%v and invalid zero", zeroErr, zero.Validate(), core.ErrJSONContract, core.ErrControlPlaneResponseDocument)
			}
			return
		}

		if err := got.Validate(); err != nil {
			t.Fatalf("ResponseDocument.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		verified, verifyErr := controlplane.VerifyResponse(controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{
			Document: got, Expected: fixture.expected, TrustedKeys: fixture.trusted,
		})
		if verifyErr != nil {
			if !errors.Is(verifyErr, core.ErrControlPlaneResponseDocument) ||
				!errors.Is(verifyErr, core.ErrAttestVerification) || verified.Validate() == nil {
				t.Fatalf("VerifyResponse(untrusted accepted document) = (%v, %v), want zero and %v/%v", verified, verifyErr, core.ErrControlPlaneResponseDocument, core.ErrAttestVerification)
			}
			return
		}

		header, headerErr := verified.Header()
		body, bodyErr := verified.Body()
		if headerErr != nil || bodyErr != nil || header != fixture.header {
			t.Fatalf("verified accepted facts = (header=%+v, errors=%v/%v), want exact trusted header", header, headerErr, bodyErr)
		}
		proveRegistrationBodyEqual(t, body, fixture.body)
		projection, issueErr := controlplane.IssueResponse(controlplane.ResponseIssuance[controlplane.RegistrationDocument]{
			Signer: fixture.signer, Header: header, Body: body,
			Assessment: acceptedProtocolAssessment(t, header),
		})
		canonical, canonicalErr := projection.MarshalJSON()
		if issueErr != nil || canonicalErr != nil || len(canonical) > core.JSONDocumentMaximumBytes || !bytes.Equal(canonical, fixture.canonical) {
			t.Fatalf("accepted response canonical closure = (%d bytes, %v, %v), want exact %d-byte production projection", len(canonical), issueErr, canonicalErr, len(fixture.canonical))
		}
		roundTrip := decodeAuthenticatedResponse(t, canonical)
		second, secondErr := controlplane.VerifyResponse(controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{
			Document: roundTrip, Expected: fixture.expected, TrustedKeys: fixture.trusted,
		})
		if secondErr != nil || second.Validate() != nil {
			t.Fatalf("canonical second verification = (%v, %v), want valid and nil", second, secondErr)
		}
		secondHeader, secondHeaderErr := second.Header()
		secondBody, secondBodyErr := second.Body()
		secondProjection, secondIssueErr := controlplane.IssueResponse(controlplane.ResponseIssuance[controlplane.RegistrationDocument]{
			Signer: fixture.signer, Header: secondHeader, Body: secondBody,
			Assessment: acceptedProtocolAssessment(t, secondHeader),
		})
		secondCanonical, secondCanonicalErr := secondProjection.MarshalJSON()
		if secondHeaderErr != nil || secondBodyErr != nil || secondIssueErr != nil || secondCanonicalErr != nil ||
			!bytes.Equal(secondCanonical, canonical) {
			t.Fatalf("second canonical write = (%d bytes, header %v, body %v, issue %v, marshal %v), want exact first canonical %d bytes", len(secondCanonical), secondHeaderErr, secondBodyErr, secondIssueErr, secondCanonicalErr, len(canonical))
		}
	})
}

func authenticatedResponseRepresentationCases(
	t testing.TB,
	fixture authenticatedResponseFixture,
	foreign authenticatedResponseFixture,
) []responseRepresentationCase {
	t.Helper()

	parts := responsePartsForTest(t, fixture)
	foreignBody, err := foreign.body.MarshalJSON()
	if err != nil {
		t.Fatalf("foreign body MarshalJSON() error = %v, want nil", err)
	}
	headerVariant := fixture.header
	headerVariant.Status = alternateStatus(fixture.header.Status)
	headerVariantParts := responsePartsForTest(t, authenticatedResponseWithHeader(t, fixture, headerVariant))
	fixtureBody, err := fixture.body.MarshalJSON()
	if err != nil {
		t.Fatalf("fixture body MarshalJSON() error = %v, want nil", err)
	}
	if bytes.Equal(foreignBody, fixtureBody) || bytes.Equal(headerVariantParts.header, parts.header) {
		t.Fatalf("signed hostile substitutions = body changed %t/header changed %t, want true/true", !bytes.Equal(foreignBody, fixtureBody), !bytes.Equal(headerVariantParts.header, parts.header))
	}
	below := padResponseToSize(t, fixture.canonical, core.JSONDocumentMaximumBytes-1)
	at := padResponseToSize(t, fixture.canonical, core.JSONDocumentMaximumBytes)
	above := padResponseToSize(t, fixture.canonical, core.JSONDocumentMaximumBytes+1)

	return []responseRepresentationCase{
		{name: "canonical production document is accepted", document: fixture.canonical, wantValid: true},
		{name: "one leading space is accepted", document: append([]byte{' '}, fixture.canonical...), wantValid: true},
		{name: "one trailing space is accepted", document: append(bytes.Clone(fixture.canonical), ' '), wantValid: true},
		{name: "leading and trailing JSON whitespace are accepted", document: append(append([]byte{'\n', '\t'}, fixture.canonical...), '\r', ' '), wantValid: true},
		{name: "body then header then attestation order is accepted", document: parts.object(parts.body, parts.header, parts.attestation), wantValid: true},
		{name: "attestation then body then header order is accepted", document: parts.object(parts.attestation, parts.body, parts.header), wantValid: true},
		{name: "header then attestation then body order is accepted", document: parts.object(parts.header, parts.attestation, parts.body), wantValid: true},
		{name: "body then attestation then header order is accepted", document: parts.object(parts.body, parts.attestation, parts.header), wantValid: true},
		{name: "one byte below document ceiling is accepted", document: below, wantValid: true},
		{name: "exact document ceiling is accepted", document: at, wantValid: true},
		{name: "empty input is rejected", document: nil},
		{name: "zero length input is rejected", document: []byte{}},
		{name: "whitespace only input is rejected", document: []byte{' ', '\n', '\t'}},
		{name: "null input is rejected", document: []byte("null")},
		{name: "invalid UTF-8 input is rejected", document: []byte{0xff}},
		{name: "NUL input is rejected", document: []byte{0}},
		{name: "unpaired high surrogate is rejected", document: []byte(`"\ud800"`)},
		{name: "unpaired low surrogate is rejected", document: []byte(`"\udc00"`)},
		{name: "empty object is rejected", document: []byte("{}")},
		{name: "array input is rejected", document: []byte("[]")},
		{name: "boolean input is rejected", document: []byte("true")},
		{name: "numeric input is rejected", document: []byte("0")},
		{name: "string input is rejected", document: []byte("\"response\"")},
		{name: "opening object only is rejected", document: []byte{'{'}},
		{name: "first byte truncation is rejected", document: bytes.Clone(fixture.canonical[:1])},
		{name: "midpoint truncation is rejected", document: bytes.Clone(fixture.canonical[:len(fixture.canonical)/2])},
		{name: "one byte truncation is rejected", document: bytes.Clone(fixture.canonical[:len(fixture.canonical)-1])},
		{name: "trailing scalar is rejected", document: append(bytes.Clone(fixture.canonical), '0')},
		{name: "trailing object is rejected", document: append(bytes.Clone(fixture.canonical), '{', '}')},
		{name: "one byte above document ceiling is rejected", document: above},
		{name: "unknown outer member is rejected", document: parts.object(parts.header, parts.body, parts.attestation, []byte("\"future\":true"))},
		{name: "duplicate header member is rejected", document: parts.object(parts.header, parts.header, parts.body, parts.attestation)},
		{name: "duplicate body member is rejected", document: parts.object(parts.header, parts.body, parts.body, parts.attestation)},
		{name: "duplicate attestation member is rejected", document: parts.object(parts.header, parts.body, parts.attestation, parts.attestation)},
		{name: "case-folded duplicate header member is rejected", document: parts.object(parts.header, parts.body, parts.attestation, []byte(`"Header":null`))},
		{name: "case-folded duplicate body member is rejected", document: parts.object(parts.header, parts.body, parts.attestation, []byte(`"Body":null`))},
		{name: "case-folded duplicate attestation member is rejected", document: parts.object(parts.header, parts.body, parts.attestation, []byte(`"Attestation":null`))},
		{name: "missing header member is rejected", document: parts.object(parts.body, parts.attestation)},
		{name: "missing body member is rejected", document: parts.object(parts.header, parts.attestation)},
		{name: "missing attestation member is rejected", document: parts.object(parts.header, parts.body)},
		{name: "null header is rejected", document: parts.replaceHeader([]byte("null"))},
		{name: "string header is rejected", document: parts.replaceHeader([]byte("\"header\""))},
		{name: "array body is rejected", document: parts.replaceBody([]byte("[]"))},
		{name: "null body is rejected", document: parts.replaceBody([]byte("null"))},
		{name: "boolean body is rejected", document: parts.replaceBody([]byte("true"))},
		{name: "numeric body is rejected", document: parts.replaceBody([]byte("1"))},
		{name: "string body is rejected", document: parts.replaceBody([]byte(`"body"`))},
		{name: "empty object body is rejected", document: parts.replaceBody([]byte("{}"))},
		{name: "body beyond nesting ceiling is rejected", document: parts.replaceBody(overNestedResponseBody())},
		{name: "body beyond array item ceiling is rejected", document: parts.replaceBody(overItemResponseBody())},
		{name: "null attestation is rejected", document: parts.replaceAttestation([]byte("null"))},
		{name: "truncated attestation is rejected", document: parts.replaceAttestation([]byte{'{'})},
		{name: "independently valid foreign body breaks exact digest", document: parts.replaceBody(foreignBody), want: core.ErrAttestVerification},
		{name: "independently valid alternate header breaks exact signature", document: parts.object(headerVariantParts.header, parts.body, parts.attestation), want: core.ErrAttestVerification},
		{name: "independently valid foreign attestation breaks exact signature", document: parts.object(parts.header, parts.body, responsePartsForTest(t, foreign).attestation), want: core.ErrAttestVerification},
		{name: "independently valid foreign signed document fails caller trust", document: foreign.canonical, wantValid: true, wantVerify: core.ErrAttestVerification},
	}
}

func overNestedResponseBody() []byte {
	const depth = core.JSONNestingDepthMaximum + 1
	value := make([]byte, 0, depth*2)
	value = append(value, bytes.Repeat([]byte{'['}, depth)...)
	return append(value, bytes.Repeat([]byte{']'}, depth)...)
}

func overItemResponseBody() []byte {
	items := int(core.DefaultStrictJSONLimits().ArrayItemMaximum) + 1
	value := make([]byte, 0, items*2+1)
	value = append(value, '[')
	for index := range items {
		if index != 0 {
			value = append(value, ',')
		}
		value = append(value, '0')
	}
	return append(value, ']')
}

type responseWireParts struct {
	header      []byte
	body        []byte
	attestation []byte
}

func responsePartsForTest(t testing.TB, fixture authenticatedResponseFixture) responseWireParts {
	t.Helper()

	headerJSON, err := fixture.header.MarshalJSON()
	if err != nil {
		t.Fatalf("ResponseHeader.MarshalJSON() error = %v, want nil", err)
	}
	bodyJSON, err := fixture.body.MarshalJSON()
	if err != nil {
		t.Fatalf("RegistrationDocument.MarshalJSON() error = %v, want nil", err)
	}
	members := responseCanonicalMembers(t, fixture.canonical)
	header, headerIndex := responseMemberWithValue(t, members, headerJSON)
	body, bodyIndex := responseMemberWithValue(t, members, bodyJSON)
	if len(members) != 3 || headerIndex == bodyIndex {
		t.Fatalf("canonical response members = %d with header/body indexes %d/%d, want three distinct members",
			len(members), headerIndex, bodyIndex)
	}
	attestationIndex := 3 - headerIndex - bodyIndex
	return responseWireParts{
		header: header, body: body, attestation: bytes.Clone(members[attestationIndex]),
	}
}

func responseCanonicalMembers(t testing.TB, document []byte) [][]byte {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(document))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		t.Fatalf("canonical response opening token = (%v, %v), want object", opening, err)
	}
	var members [][]byte
	for decoder.More() {
		key, keyErr := decoder.Token()
		var value json.RawMessage
		valueErr := decoder.Decode(&value)
		keyText, keyOK := key.(string)
		encodedKey, encodeErr := json.Marshal(keyText)
		if keyErr != nil || valueErr != nil || !keyOK || encodeErr != nil {
			t.Fatalf("canonical response member decode = (key %v/%v, value %v, key encode %v), want exact member",
				key, keyErr, valueErr, encodeErr)
		}
		member := make([]byte, 0, len(encodedKey)+1+len(value))
		member = append(member, encodedKey...)
		member = append(member, ':')
		member = append(member, value...)
		members = append(members, member)
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		t.Fatalf("canonical response closing token = (%v, %v), want object", closing, err)
	}
	return members
}

func responseMemberWithValue(t testing.TB, members [][]byte, value []byte) ([]byte, int) {
	t.Helper()
	for index, member := range members {
		if bytes.HasSuffix(member, value) {
			return bytes.Clone(member), index
		}
	}
	t.Fatalf("canonical response has no member with exact compiler-produced value")
	return nil, -1
}

func (p responseWireParts) object(members ...[]byte) []byte {
	total := 2
	for _, member := range members {
		total += len(member)
	}
	if len(members) > 1 {
		total += len(members) - 1
	}
	encoded := make([]byte, 0, total)
	encoded = append(encoded, '{')
	for index, member := range members {
		if index != 0 {
			encoded = append(encoded, ',')
		}
		encoded = append(encoded, member...)
	}
	return append(encoded, '}')
}

func (p responseWireParts) replaceHeader(value []byte) []byte {
	separator := bytes.IndexByte(p.header, ':')
	member := append(bytes.Clone(p.header[:separator+1]), value...)
	return p.object(member, p.body, p.attestation)
}

func (p responseWireParts) replaceBody(value []byte) []byte {
	separator := bytes.IndexByte(p.body, ':')
	member := append(bytes.Clone(p.body[:separator+1]), value...)
	return p.object(p.header, member, p.attestation)
}

func (p responseWireParts) replaceAttestation(value []byte) []byte {
	separator := bytes.IndexByte(p.attestation, ':')
	member := append(bytes.Clone(p.attestation[:separator+1]), value...)
	return p.object(p.header, p.body, member)
}

func padResponseToSize(t testing.TB, canonical []byte, size int) []byte {
	t.Helper()

	if size < len(canonical) {
		t.Fatalf("requested response size %d is below canonical size %d", size, len(canonical))
	}
	padded := make([]byte, size)
	copy(padded[size-len(canonical):], canonical)
	for index := range size - len(canonical) {
		padded[index] = ' '
	}
	return padded
}

func authenticatedResponseForTest(t testing.TB, seed byte) authenticatedResponseFixture {
	t.Helper()

	registration := issueTestRegistration(t)
	public, signer := testSigningKey(t, seed)
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{public}})
	if err != nil {
		t.Fatalf("NewTrustedKeys() error = %v, want nil", err)
	}
	payload := registration.document.Payload
	payload.Lease = resignLease(t, payload.Lease, signer)
	payload.Certificate = resignCertificate(t, payload.Certificate, signer)
	body, err := controlplane.IssueRegistration(payload, signer)
	if err != nil {
		t.Fatalf("IssueRegistration() error = %v, want nil", err)
	}
	base := authenticatedResponseFixture{body: body, header: body.Payload.Header, trusted: trusted, signer: signer}
	return authenticatedResponseWithHeader(t, base, base.header)
}

func authenticatedResponseWithHeader(
	t testing.TB,
	base authenticatedResponseFixture,
	header controlplane.ResponseHeader,
) authenticatedResponseFixture {
	t.Helper()

	projection, err := controlplane.IssueResponse(controlplane.ResponseIssuance[controlplane.RegistrationDocument]{
		Signer: base.signer, Header: header, Body: base.body,
		Assessment: acceptedProtocolAssessment(t, header),
	})
	if err != nil {
		t.Fatalf("IssueResponse() error = %v, want nil", err)
	}
	canonical, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("ResponseProjection.MarshalJSON() error = %v, want nil", err)
	}
	base.header = header
	base.expected = expectationFor(header)
	base.canonical = canonical
	return base
}

func decodeAuthenticatedResponse(t testing.TB, encoded []byte) controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument] {
	t.Helper()

	var document controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]
	if err := document.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("ResponseDocument.UnmarshalJSON(compiler-produced response) error = %v, want nil", err)
	}
	return document
}

func proveAuthenticatedResponseCanonicalClosure(
	t testing.TB,
	fixture authenticatedResponseFixture,
	document controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument],
) {
	t.Helper()

	verified, err := controlplane.VerifyResponse(controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{
		Document: document, Expected: fixture.expected, TrustedKeys: fixture.trusted,
	})
	if err != nil {
		t.Fatalf("VerifyResponse() error = %v, want nil", err)
	}
	body, err := verified.Body()
	if err != nil {
		t.Fatalf("VerifiedResponse.Body() error = %v, want nil", err)
	}
	header, err := verified.Header()
	if err != nil {
		t.Fatalf("VerifiedResponse.Header() error = %v, want nil", err)
	}
	projection, err := controlplane.IssueResponse(controlplane.ResponseIssuance[controlplane.RegistrationDocument]{
		Signer: fixture.signer, Header: header, Body: body,
		Assessment: acceptedProtocolAssessment(t, header),
	})
	if err != nil {
		t.Fatalf("IssueResponse(round trip) error = %v, want nil", err)
	}
	canonical, err := projection.MarshalJSON()
	if err != nil || !bytes.Equal(canonical, fixture.canonical) {
		t.Fatalf("canonical response round trip = (%d bytes, %v), want exact %d bytes", len(canonical), err, len(fixture.canonical))
	}
}

func proveRegistrationBodyEqual(
	t testing.TB,
	got controlplane.RegistrationDocument,
	want controlplane.RegistrationDocument,
) {
	t.Helper()

	gotJSON, gotErr := got.MarshalJSON()
	wantJSON, wantErr := want.MarshalJSON()
	if gotErr != nil || wantErr != nil || !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("registration body bytes = (%d, %v), want exact (%d, %v)", len(gotJSON), gotErr, len(wantJSON), wantErr)
	}
}

func responseStatusMutation(status controlplane.ProductStatus) func(testing.TB, *controlplane.ResponseHeader) {
	return func(_ testing.TB, header *controlplane.ResponseHeader) { header.Status = status }
}

func responseOfferingMutation(offering core.Offering) func(testing.TB, *controlplane.ResponseHeader) {
	return func(_ testing.TB, header *controlplane.ResponseHeader) { header.Offering = offering }
}

func responseProviderTimeMutation(nanoseconds int64) func(testing.TB, *controlplane.ResponseHeader) {
	return func(_ testing.TB, header *controlplane.ResponseHeader) {
		header.ProviderTime = temporal.InstantFromNanoseconds(nanoseconds)
	}
}

func responsePolicyActivationMutation(value uint64) func(testing.TB, *controlplane.ResponseHeader) {
	return func(t testing.TB, header *controlplane.ResponseHeader) {
		t.Helper()
		activation, err := controlwire.NewPolicyActivation(value)
		if err != nil {
			t.Fatalf("NewPolicyActivation(%d) error = %v, want nil", value, err)
		}
		header.Policy.Activation = activation
	}
}

func nextPolicyActivationMutation(
	t testing.TB,
	current controlwire.PolicyActivation,
) func(testing.TB, *controlplane.ResponseHeader) {
	t.Helper()

	value := current.Uint64()
	if value == math.MaxUint64 {
		t.Fatalf("fixture policy activation = %d, want below %d for a distinct valid successor", value, uint64(math.MaxUint64))
	}
	return responsePolicyActivationMutation(value + 1)
}

func responseVerificationWithExpectation(
	document controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument],
	fixture authenticatedResponseFixture,
	mutate func(*controlplane.ResponseExpectation),
) controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument] {
	expected := fixture.expected
	mutate(&expected)
	return controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{Document: document, Expected: expected, TrustedKeys: fixture.trusted}
}

func responseAccountIdentity(t testing.TB, fill byte) receipt.AccountIdentity {
	t.Helper()

	var raw [receipt.LifecycleIdentityBytes]byte
	for index := range raw {
		raw[index] = fill
	}
	identity, err := receipt.NewAccountIdentity(raw)
	if err != nil {
		t.Fatalf("NewAccountIdentity() error = %v, want nil", err)
	}
	return identity
}

func responseDeviceID(t testing.TB, fill byte) lease.DeviceID {
	t.Helper()

	var raw [lease.IdentifierBytes]byte
	for index := range raw {
		raw[index] = fill
	}
	identity, err := lease.NewDeviceID(raw)
	if err != nil {
		t.Fatalf("NewDeviceID() error = %v, want nil", err)
	}
	return identity
}

func alternateOffering(value core.Offering) core.Offering {
	if value == core.OfferingBug {
		return core.OfferingWitness
	}
	return core.OfferingBug
}

func alternateStatus(value controlplane.ProductStatus) controlplane.ProductStatus {
	if value == controlplane.ProductStatusActive {
		return controlplane.ProductStatusPaymentRetry
	}
	return controlplane.ProductStatusActive
}

func instantAfter(t testing.TB, value temporal.Instant) temporal.Instant {
	t.Helper()

	nanoseconds, err := value.Nanoseconds()
	if err != nil || nanoseconds == math.MaxInt64 {
		t.Fatalf("fixture provider time = (%d, %v), want room for one later nanosecond", nanoseconds, err)
	}
	return temporal.InstantFromNanoseconds(nanoseconds + 1)
}
