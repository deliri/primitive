package distributionauth

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/release"
)

func TestCredentialedPublicationVerificationAuthenticatesEveryOfferingAndExactCompletion(t *testing.T) {
	t.Parallel()

	admitted := 0
	for value := 0; value <= 255; value++ {
		offering := core.Offering(value)
		if !offering.IsValid() {
			continue
		}
		admitted++
		t.Run(offering.String(), func(t *testing.T) {
			t.Parallel()
			fixture := newPublicationAuthFixture(t, publicationAuthFixtureRequest{
				offering: offering, authorityByte: byte(value) + 0x20,
				deviceByte: byte(value) + 0x40, releaseByte: byte(value) + 0x60,
				nonceByte: byte(value) + 1,
			})
			requestRoute, requestRouteErr := fixture.document.ControlRoute()
			completionRoute, completionRouteErr := fixture.completion.ControlRoute()
			if requestRouteErr != nil || completionRouteErr != nil ||
				requestRoute.Offering() != offering || completionRoute.Offering() != offering ||
				requestRoute.Family() != controlwire.RouteFamilyReleasePublications ||
				completionRoute.Family() != controlwire.RouteFamilyReleasePublicationCompletions ||
				fixture.document.ControlNonce() != fixture.document.Request.Payload.Nonce ||
				fixture.completion.ControlNonce() != fixture.completion.Completion.Payload.Nonce {
				t.Fatalf("publication control projections(%v) = (%v, %v, %v, %v), want exact request/completion routes and nonces",
					offering, requestRoute, requestRouteErr, completionRoute, completionRouteErr)
			}
			verified, err := VerifyPublication(PublicationVerification{
				Document: fixture.document, TrustedKeys: fixture.authority,
				ManifestKeys: fixture.release.keys,
			})
			if err != nil {
				t.Fatalf("VerifyPublication(%v) error = %v, want nil", offering, err)
			}
			payload, err := verified.Payload()
			if err != nil || payload != fixture.document.Request.Payload || payload.Build.Offering() != offering {
				t.Fatalf("VerifiedPublication.Payload(%v) = (%+v, %v), want exact offering payload and nil",
					offering, payload, err)
			}
			manifest, err := verified.Manifest()
			if err != nil || manifest.Document() != fixture.release.document {
				t.Fatalf("VerifiedPublication.Manifest(%v) = (%+v, %v), want exact signed manifest and nil",
					offering, manifest, err)
			}
			completion, err := VerifyPublicationCompletion(PublicationCompletionVerification{
				Grant: fixture.grant, Document: fixture.completion,
				Request: fixture.verified, TrustedKeys: fixture.authority,
			})
			if err != nil {
				t.Fatalf("VerifyPublicationCompletion(%v) error = %v, want nil", offering, err)
			}
			completionPayload, err := completion.Payload()
			if err != nil || completionPayload.Build != fixture.installation.Build ||
				completionPayload.Manifest != fixture.release.verified.Identity() ||
				completionPayload.ManifestDocument != fixture.release.verified.DocumentDigest() {
				t.Fatalf("VerifiedPublicationCompletion.Payload(%v) = (%+v, %v), want exact installed build and manifest",
					offering, completionPayload, err)
			}
		})
	}
	if admitted < 3 {
		t.Fatalf("admitted offerings = %d, want at least the shipped set", admitted)
	}
}

func TestCredentialedPublicationRequestRefusesEveryAuthorityDeviceBuildAndManifestSubstitution(t *testing.T) {
	t.Parallel()

	base := newPublicationAuthFixture(t, publicationAuthFixtureRequest{})
	otherDevice := newPublicationAuthFixture(t, publicationAuthFixtureRequest{
		authorityByte: 0x71, deviceByte: 0x72, releaseByte: 0x73, nonceByte: 0x74,
	})
	otherBuild := newPublicationAuthFixture(t, publicationAuthFixtureRequest{
		offering: core.OfferingBug, authorityByte: 0x81, deviceByte: 0x82,
		releaseByte: 0x83, nonceByte: 0x84,
	})
	wrongDevice, err := AssemblePublication(PublicationRequestAssembly{
		Request: otherDevice.document.Request, Certificate: base.document.Certificate,
	})
	if err != nil {
		t.Fatalf("AssemblePublication(same-build other device) error = %v, want nil", err)
	}
	if document, gotErr := AssemblePublication(PublicationRequestAssembly{
		Request: otherBuild.document.Request, Certificate: base.document.Certificate,
	}); !errors.Is(gotErr, core.ErrControlPlaneResponseBinding) || document != (PublicationRequestDocument{}) {
		t.Fatalf("AssemblePublication(other build) = (%+v, %v), want zero and errors.Is %v",
			document, gotErr, core.ErrControlPlaneResponseBinding)
	}
	tamperedNonce := base.document
	tamperedNonce.Request.Payload.Nonce = distributionAuthNonce(t, 0x91)
	tamperedManifest := base.document
	tamperedManifest.Request.Payload.Manifest = otherDevice.release.document
	tamperedSigner := base.document
	tamperedSigner.Request.Attestation.Signer = otherDevice.document.Request.Attestation.Signer
	tamperedCertificate := base.document
	tamperedCertificate.Certificate.Body.IssuedAt = base.grant.Payload.ExpiresAt
	if tamperedCertificate.Certificate.Body.IssuedAt == base.document.Certificate.Body.IssuedAt {
		t.Fatalf("certificate issuance mutation = %v, want a value distinct from %v",
			tamperedCertificate.Certificate.Body.IssuedAt, base.document.Certificate.Body.IssuedAt)
	}
	cases := []struct {
		wantErr      error
		name         string
		document     PublicationRequestDocument
		authority    attest.TrustedKeys
		manifestKeys attest.TrustedKeys
	}{
		{name: "zero document", authority: base.authority, manifestKeys: base.release.keys, wantErr: core.ErrControlPlaneContract},
		{name: "zero authority trust", document: base.document, manifestKeys: base.release.keys, wantErr: core.ErrControlPlaneContract},
		{name: "zero manifest trust", document: base.document, authority: base.authority, wantErr: core.ErrControlPlaneContract},
		{name: "same-build other device", document: wrongDevice, authority: base.authority, manifestKeys: otherDevice.release.keys, wantErr: core.ErrAttestVerification},
		{name: "other certificate authority", document: base.document, authority: otherDevice.authority, manifestKeys: base.release.keys, wantErr: core.ErrAttestVerification},
		{name: "other release authority", document: base.document, authority: base.authority, manifestKeys: otherDevice.release.keys, wantErr: core.ErrAttestVerification},
		{name: "nonce changed after signing", document: tamperedNonce, authority: base.authority, manifestKeys: base.release.keys, wantErr: core.ErrAttestVerification},
		{name: "manifest changed after signing", document: tamperedManifest, authority: base.authority, manifestKeys: otherDevice.release.keys, wantErr: core.ErrAttestVerification},
		{name: "request signer changed after signing", document: tamperedSigner, authority: base.authority, manifestKeys: base.release.keys, wantErr: core.ErrAttestVerification},
		{name: "certificate fact changed after signing", document: tamperedCertificate, authority: base.authority, manifestKeys: base.release.keys, wantErr: core.ErrAttestVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := VerifyPublication(PublicationVerification{
				Document: tc.document, TrustedKeys: tc.authority, ManifestKeys: tc.manifestKeys,
			})
			if !errors.Is(gotErr, tc.wantErr) || got != (VerifiedPublication{}) {
				t.Fatalf("VerifyPublication(%s) = (%+v, %v), want zero and errors.Is %v",
					tc.name, got, gotErr, tc.wantErr)
			}
		})
	}
}

func TestCredentialedPublicationCompletionRefusesEveryCrossRequestAndCrossAuthorityRecombination(t *testing.T) {
	t.Parallel()

	base := newPublicationAuthFixture(t, publicationAuthFixtureRequest{})
	other := newPublicationAuthFixture(t, publicationAuthFixtureRequest{
		authorityByte: 0xa1, deviceByte: 0xa2, releaseByte: 0xa3, nonceByte: 0xa4,
	})
	otherBuild := newPublicationAuthFixture(t, publicationAuthFixtureRequest{
		offering: core.OfferingPeachfuzz, authorityByte: 0xb1, deviceByte: 0xb2,
		releaseByte: 0xb3, nonceByte: 0xb4,
	})
	otherDeviceCompletion, err := AssemblePublicationCompletion(PublicationCompletionAssembly{
		Completion: other.completion.Completion, Certificate: base.completion.Certificate,
	})
	if err != nil {
		t.Fatalf("AssemblePublicationCompletion(other device same build) error = %v, want nil", err)
	}
	wrongCertificate := base.completion
	wrongCertificate.Certificate = other.completion.Certificate
	tamperedEvidence := base.completion
	tamperedEvidence.Completion.Payload.Evidence[0], tamperedEvidence.Completion.Payload.Evidence[1] =
		tamperedEvidence.Completion.Payload.Evidence[1], tamperedEvidence.Completion.Payload.Evidence[0]
	if tamperedEvidence.Completion.Payload.Evidence == base.completion.Completion.Payload.Evidence {
		t.Fatalf("evidence mutation = %+v, want a set distinct from %+v",
			tamperedEvidence.Completion.Payload.Evidence, base.completion.Completion.Payload.Evidence)
	}
	tamperedRequest := base.completion
	tamperedRequest.Completion.Payload.Request = other.completion.Completion.Payload.Request
	otherNonceCompletion := base.completion
	otherNonceCompletion.Completion.Payload.Nonce = other.document.Request.Payload.Nonce
	otherNonceAttestation, err := attest.Sign(attest.SignRequest[distribution.SigningDomain]{
		Body: otherNonceCompletion.Completion.Payload, Signer: base.installation.DevicePrivate,
	})
	if err != nil {
		t.Fatalf("attest.Sign(publication completion with other nonce) error = %v, want nil", err)
	}
	otherNonceCompletion.Completion.Attestation = otherNonceAttestation
	tamperedAuthorization := base.completion
	tamperedAuthorization.Completion.Payload.Authorization = publicationAuthAuthorityNonce(t, 0x62)
	if tamperedAuthorization.Completion.Payload.Authorization == base.completion.Completion.Payload.Authorization {
		t.Fatalf("authorization mutation = %v, want a nonce distinct from %v",
			tamperedAuthorization.Completion.Payload.Authorization,
			base.completion.Completion.Payload.Authorization)
	}
	tamperedManifest := base.completion
	tamperedManifest.Completion.Payload.Manifest = otherBuild.completion.Completion.Payload.Manifest
	if tamperedManifest.Completion.Payload.Manifest == base.completion.Completion.Payload.Manifest {
		t.Fatalf("manifest mutation = %v, want an identity distinct from %v",
			tamperedManifest.Completion.Payload.Manifest,
			base.completion.Completion.Payload.Manifest)
	}
	tamperedBuild := base.completion
	tamperedBuild.Completion.Payload.Build = otherBuild.installation.Build
	if err := tamperedBuild.Validate(); !errors.Is(err, core.ErrControlPlaneResponseBinding) {
		t.Fatalf("PublicationCompletionDocument.Validate(other build) error = %v, want errors.Is %v",
			err, core.ErrControlPlaneResponseBinding)
	}
	cases := []struct {
		wantError error
		name      string
		grant     distribution.PublicationGrantDocument
		document  PublicationCompletionDocument
		request   VerifiedPublication
		trusted   attest.TrustedKeys
	}{
		{name: "zero verification", wantError: core.ErrControlPlaneContract},
		{name: "zero completion", grant: base.grant, request: base.verified, trusted: base.authority, wantError: core.ErrControlPlaneContract},
		{name: "zero grant", document: base.completion, request: base.verified, trusted: base.authority, wantError: core.ErrControlPlaneContract},
		{name: "zero request proof", grant: base.grant, document: base.completion, trusted: base.authority, wantError: core.ErrControlPlaneContract},
		{name: "zero authority trust", grant: base.grant, document: base.completion, request: base.verified, wantError: core.ErrControlPlaneContract},
		{name: "certificate differs from authenticated request", grant: base.grant, document: wrongCertificate, request: base.verified, trusted: other.authority, wantError: core.ErrControlPlaneResponseBinding},
		{name: "completion signed by other device", grant: base.grant, document: otherDeviceCompletion, request: base.verified, trusted: base.authority, wantError: core.ErrAttestVerification},
		{name: "provider evidence reordered after signing", grant: base.grant, document: tamperedEvidence, request: base.verified, trusted: base.authority, wantError: core.ErrAttestVerification},
		{name: "request commitment changed after signing", grant: base.grant, document: tamperedRequest, request: base.verified, trusted: base.authority, wantError: core.ErrAttestVerification},
		{name: "validly signed completion names another request nonce", grant: base.grant, document: otherNonceCompletion, request: base.verified, trusted: base.authority, wantError: core.ErrDistributionBinding},
		{name: "authorization changed after signing", grant: base.grant, document: tamperedAuthorization, request: base.verified, trusted: base.authority, wantError: core.ErrAttestVerification},
		{name: "manifest changed after signing", grant: base.grant, document: tamperedManifest, request: base.verified, trusted: base.authority, wantError: core.ErrAttestVerification},
		{name: "grant from other authority and request", grant: other.grant, document: base.completion, request: base.verified, trusted: base.authority, wantError: core.ErrAttestVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := VerifyPublicationCompletion(PublicationCompletionVerification{
				Grant: tc.grant, Document: tc.document, Request: tc.request, TrustedKeys: tc.trusted,
			})
			if !errors.Is(gotErr, tc.wantError) || got != (VerifiedPublicationCompletion{}) {
				t.Fatalf("VerifyPublicationCompletion(%s) = (%+v, %v), want zero and errors.Is %v",
					tc.name, got, gotErr, tc.wantError)
			}
		})
	}
}

func TestCredentialedPublicationJSONBoundariesAreStrictBoundedCanonicalAndPreserving(t *testing.T) {
	t.Parallel()

	fixture := newPublicationAuthFixture(t, publicationAuthFixtureRequest{})
	t.Run("request", func(t *testing.T) {
		t.Parallel()
		encoded, err := fixture.document.MarshalJSON()
		if err != nil {
			t.Fatalf("PublicationRequestDocument.MarshalJSON() error = %v, want nil", err)
		}
		reordered, err := json.Marshal(struct {
			Request     distribution.PublicationRequestDocument      `json:"request"`
			Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
		}{Certificate: fixture.document.Certificate, Request: fixture.document.Request})
		if err != nil {
			t.Fatalf("json.Marshal(reordered publication request) error = %v, want nil", err)
		}
		publicationAuthPressureRequestJSON(t, fixture.document, encoded, reordered)
	})
	t.Run("completion", func(t *testing.T) {
		t.Parallel()
		encoded, err := fixture.completion.MarshalJSON()
		if err != nil {
			t.Fatalf("PublicationCompletionDocument.MarshalJSON() error = %v, want nil", err)
		}
		reordered, err := json.Marshal(struct {
			Completion  distribution.PublicationCompletionDocument   `json:"completion"`
			Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
		}{Certificate: fixture.completion.Certificate, Completion: fixture.completion.Completion})
		if err != nil {
			t.Fatalf("json.Marshal(reordered publication completion) error = %v, want nil", err)
		}
		publicationAuthPressureCompletionJSON(t, fixture.completion, encoded, reordered)
	})
}

func TestCredentialedPublicationZeroValuesNeverAcquireProofOrPayload(t *testing.T) {
	t.Parallel()

	request, requestErr := VerifyPublication(PublicationVerification{})
	if !errors.Is(requestErr, core.ErrControlPlaneContract) || request != (VerifiedPublication{}) {
		t.Fatalf("VerifyPublication(zero) = (%+v, %v), want zero and errors.Is %v",
			request, requestErr, core.ErrControlPlaneContract)
	}
	requestPayload, requestErr := (VerifiedPublication{}).Payload()
	if !errors.Is(requestErr, core.ErrControlPlaneContract) || requestPayload != (distribution.PublicationRequestPayload{}) {
		t.Fatalf("VerifiedPublication{}.Payload() = (%+v, %v), want zero and errors.Is %v",
			requestPayload, requestErr, core.ErrControlPlaneContract)
	}
	manifest, manifestErr := (VerifiedPublication{}).Manifest()
	if !errors.Is(manifestErr, core.ErrControlPlaneContract) || manifest != (release.VerifiedManifest{}) {
		t.Fatalf("VerifiedPublication{}.Manifest() = (%+v, %v), want zero and errors.Is %v",
			manifest, manifestErr, core.ErrControlPlaneContract)
	}
	completion, completionErr := VerifyPublicationCompletion(PublicationCompletionVerification{})
	if !errors.Is(completionErr, core.ErrControlPlaneContract) || completion != (VerifiedPublicationCompletion{}) {
		t.Fatalf("VerifyPublicationCompletion(zero) = (%+v, %v), want zero and errors.Is %v",
			completion, completionErr, core.ErrControlPlaneContract)
	}
	completionPayload, completionErr := (VerifiedPublicationCompletion{}).Payload()
	if !errors.Is(completionErr, core.ErrControlPlaneContract) || completionPayload != (distribution.PublicationCompletionPayload{}) {
		t.Fatalf("VerifiedPublicationCompletion{}.Payload() = (%+v, %v), want zero and errors.Is %v",
			completionPayload, completionErr, core.ErrControlPlaneContract)
	}
}

func publicationAuthPressureRequestJSON(
	t *testing.T,
	document PublicationRequestDocument,
	canonical []byte,
	reordered []byte,
) {
	t.Helper()
	for _, tc := range publicationAuthValidJSONCases(t, canonical, reordered, RequestDocumentJSONMaximumBytes) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var receiver PublicationRequestDocument
			if err := receiver.UnmarshalJSON(tc.data); err != nil || receiver != document {
				t.Fatalf("PublicationRequestDocument.UnmarshalJSON(%s) = (%+v, %v), want exact and nil",
					tc.name, receiver, err)
			}
			reencoded, err := receiver.MarshalJSON()
			if err != nil || !bytes.Equal(reencoded, canonical) {
				t.Fatalf("PublicationRequestDocument.MarshalJSON(%s) = (%q, %v), want canonical %q and nil",
					tc.name, reencoded, err, canonical)
			}
		})
	}
	for _, tc := range publicationAuthInvalidJSONCases(canonical, RequestDocumentJSONMaximumBytes, "request") {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			receiver := document
			if err := receiver.UnmarshalJSON(tc.data); !errors.Is(err, core.ErrJSONContract) || receiver != document {
				t.Fatalf("PublicationRequestDocument.UnmarshalJSON(%s) = (%+v, %v), want preserved and errors.Is %v",
					tc.name, receiver, err, core.ErrJSONContract)
			}
		})
	}
	var receiver *PublicationRequestDocument
	if err := receiver.UnmarshalJSON(canonical); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil PublicationRequestDocument.UnmarshalJSON() error = %v, want errors.Is %v",
			err, core.ErrJSONContract)
	}
}

func publicationAuthPressureCompletionJSON(
	t *testing.T,
	document PublicationCompletionDocument,
	canonical []byte,
	reordered []byte,
) {
	t.Helper()
	for _, tc := range publicationAuthValidJSONCases(
		t, canonical, reordered, PublicationCompletionDocumentJSONMaximumBytes,
	) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var receiver PublicationCompletionDocument
			if err := receiver.UnmarshalJSON(tc.data); err != nil || receiver != document {
				t.Fatalf("PublicationCompletionDocument.UnmarshalJSON(%s) = (%+v, %v), want exact and nil",
					tc.name, receiver, err)
			}
			reencoded, err := receiver.MarshalJSON()
			if err != nil || !bytes.Equal(reencoded, canonical) {
				t.Fatalf("PublicationCompletionDocument.MarshalJSON(%s) = (%q, %v), want canonical %q and nil",
					tc.name, reencoded, err, canonical)
			}
		})
	}
	for _, tc := range publicationAuthInvalidJSONCases(
		canonical, PublicationCompletionDocumentJSONMaximumBytes, "completion",
	) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			receiver := document
			if err := receiver.UnmarshalJSON(tc.data); !errors.Is(err, core.ErrJSONContract) || receiver != document {
				t.Fatalf("PublicationCompletionDocument.UnmarshalJSON(%s) = (%+v, %v), want preserved and errors.Is %v",
					tc.name, receiver, err, core.ErrJSONContract)
			}
		})
	}
	var receiver *PublicationCompletionDocument
	if err := receiver.UnmarshalJSON(canonical); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil PublicationCompletionDocument.UnmarshalJSON() error = %v, want errors.Is %v",
			err, core.ErrJSONContract)
	}
}

func publicationAuthValidJSONCases(
	t *testing.T,
	canonical []byte,
	reordered []byte,
	maximum int,
) []distributionAuthJSONCase {
	t.Helper()
	var indented bytes.Buffer
	if err := json.Indent(&indented, canonical, "", "  "); err != nil {
		t.Fatalf("json.Indent(credentialed publication) error = %v, want nil", err)
	}
	return []distributionAuthJSONCase{
		{name: "canonical", data: canonical},
		{name: "reordered", data: reordered},
		{name: "indented", data: indented.Bytes()},
		{name: "leading space", data: append([]byte(" "), canonical...)},
		{name: "trailing newline", data: append(bytes.Clone(canonical), '\n')},
		{name: "carriage return framing", data: append(append([]byte("\r"), canonical...), '\r')},
		{name: "mixed whitespace", data: append(append([]byte("\t\r\n"), canonical...), ' ', '\t')},
		{name: "half ceiling", data: distributionAuthPadJSON(canonical, maximum/2)},
		{name: "one below ceiling", data: distributionAuthPadJSON(canonical, maximum-1)},
		{name: "exact ceiling", data: distributionAuthPadJSON(canonical, maximum)},
	}
}

func publicationAuthInvalidJSONCases(
	canonical []byte,
	maximum int,
	member string,
) []distributionAuthJSONCase {
	unknown := append(bytes.Clone(canonical[:len(canonical)-1]), []byte(`,"future":true}`)...)
	duplicate := append(bytes.Clone(canonical[:len(canonical)-1]), []byte(`,"`+member+`":null}`)...)
	return []distributionAuthJSONCase{
		{name: "empty"},
		{name: "whitespace only", data: []byte(" \t\r\n")},
		{name: "null", data: []byte(`null`)},
		{name: "boolean", data: []byte(`true`)},
		{name: "scalar", data: []byte(`1`)},
		{name: "string", data: []byte(`"publication"`)},
		{name: "array", data: []byte(`[]`)},
		{name: "empty object", data: []byte(`{}`)},
		{name: "unknown member", data: unknown},
		{name: "duplicate primary member", data: duplicate},
		{name: "missing certificate", data: []byte(`{"` + member + `":null}`)},
		{name: "missing primary member", data: []byte(`{"certificate":null}`)},
		{name: "primary member wrong type", data: []byte(`{"` + member + `":true,"certificate":null}`)},
		{name: "certificate wrong type", data: []byte(`{"` + member + `":null,"certificate":true}`)},
		{name: "open object", data: []byte(`{`)},
		{name: "open array", data: []byte(`[`)},
		{name: "truncated", data: canonical[:len(canonical)-1]},
		{name: "half truncated", data: canonical[:len(canonical)/2]},
		{name: "two documents", data: append(bytes.Clone(canonical), canonical...)},
		{name: "trailing scalar", data: append(bytes.Clone(canonical), []byte(` 0`)...)},
		{name: "one above ceiling", data: distributionAuthPadJSON(canonical, maximum+1)},
	}
}
