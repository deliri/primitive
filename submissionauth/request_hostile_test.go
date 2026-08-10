package submissionauth

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"hash/crc32"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlplanetest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/submission"
)

type authFixtureRequest struct {
	offering      core.Offering
	authorityByte byte
	deviceByte    byte
	nonceByte     byte
}

type authFixture struct {
	device      ed25519.PrivateKey
	request     submission.RequestDocument
	document    RequestDocument
	trusted     attest.TrustedKeys
	certificate controlplane.InstallationCertificateDocument
}

func newAuthFixture(t *testing.T, request authFixtureRequest) authFixture {
	t.Helper()

	if request.offering == core.OfferingUnknown {
		request.offering = core.OfferingWitness
	}
	if request.authorityByte == 0 {
		request.authorityByte = 0x21
	}
	if request.deviceByte == 0 {
		request.deviceByte = 0x31
	}
	if request.nonceByte == 0 {
		request.nonceByte = 0x41
	}
	installation, err := controlplanetest.IssueInstallation(
		controlplanetest.InstallationRequest{
			AuthoritySeed: authSeed(request.authorityByte),
			DeviceSeed:    authSeed(request.deviceByte),
			Offering:      request.offering,
		},
	)
	if err != nil {
		t.Fatalf("controlplanetest.IssueInstallation() error = %v, want nil", err)
	}
	payload := authRequestPayload(t, installation.Build, request.nonceByte)
	signedRequest, err := submission.IssueRequest(submission.RequestIssuance{
		Payload: payload, Signer: installation.DevicePrivate,
	})
	if err != nil {
		t.Fatalf("submission.IssueRequest() error = %v, want nil", err)
	}
	document, err := Assemble(RequestAssembly{
		Request: signedRequest, Certificate: installation.Certificate,
	})
	if err != nil {
		t.Fatalf("submissionauth.Assemble() error = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{installation.AuthorityPublic},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}
	return authFixture{
		document: document, request: signedRequest, certificate: installation.Certificate,
		device: installation.DevicePrivate, trusted: trusted,
	}
}

func authSeed(value byte) [ed25519.SeedSize]byte {
	seed := [ed25519.SeedSize]byte{}
	for index := range seed {
		seed[index] = value
	}
	return seed
}

func authSigningKey(t *testing.T, value byte) (core.Ed25519PublicKey, ed25519.PrivateKey) {
	t.Helper()

	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = value
	}
	private := ed25519.NewKeyFromSeed(seed)
	public, err := core.NewEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("core.NewEd25519PublicKey() error = %v, want nil", err)
	}
	return public, private
}

func authRequestPayload(
	t *testing.T,
	build core.BuildIdentity,
	nonceByte byte,
) submission.RequestPayload {
	t.Helper()

	content := []byte(`{"proof":"source-free"}`)
	contentType, err := core.ParseHTTPMediaType("application/json")
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType() error = %v, want nil", err)
	}
	extent, err := core.NewByteLength(uint64(len(content)))
	if err != nil {
		t.Fatalf("core.NewByteLength() error = %v, want nil", err)
	}
	rawNonce := [controlwire.NonceBytes]byte{}
	for index := range rawNonce {
		rawNonce[index] = nonceByte
	}
	nonce, err := controlwire.NewRequestNonce(rawNonce)
	if err != nil {
		t.Fatalf("controlwire.NewRequestNonce() error = %v, want nil", err)
	}
	return submission.RequestPayload{
		Declaration: submission.Declaration{
			ContentType: contentType, Extent: extent, SHA256: core.SHA256Of(content),
			CRC32C: core.NewCRC32C(crc32.Checksum(content, crc32.MakeTable(crc32.Castagnoli))),
		},
		Manifest: submissionManifestIntent(t),
		Build:    build, Revision: controlwire.Revision2026V1, Nonce: nonce,
	}
}

func submissionManifestIntent(t *testing.T) submission.ManifestIntent {
	t.Helper()
	upload, err := submission.ParseUploadID("00000000-0006-7000-8000-000000000006")
	if err != nil {
		t.Fatalf("submission.ParseUploadID() error = %v, want nil", err)
	}
	collection, err := chit.ParseCollectionID("00000000-0007-7000-8000-000000000007")
	if err != nil {
		t.Fatalf("chit.ParseCollectionID() error = %v, want nil", err)
	}
	name, err := chit.ParseEntryName("proof.json")
	if err != nil {
		t.Fatalf("chit.ParseEntryName() error = %v, want nil", err)
	}
	sequence, err := chit.NewEntrySequence(1)
	if err != nil {
		t.Fatalf("chit.NewEntrySequence() error = %v, want nil", err)
	}
	objects, err := chit.NewObjectCount(1)
	if err != nil {
		t.Fatalf("chit.NewObjectCount() error = %v, want nil", err)
	}
	return submission.ManifestIntent{
		Upload: upload, Collection: collection, Name: name,
		Sequence: sequence, Objects: objects,
	}
}

// TestCredentialedRequestLayerTriadAuthenticatesEveryOfferingThroughOneBlindPath proves
// every product reaches the same certificate-first authentication sequence.
func TestCredentialedRequestLayerTriadAuthenticatesEveryOfferingThroughOneBlindPath(t *testing.T) {
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

			fixture := newAuthFixture(t, authFixtureRequest{
				offering: offering, authorityByte: byte(value) + 0x20,
				deviceByte: byte(value) + 0x40, nonceByte: byte(value) + 1,
			})
			verified, err := Verify(Verification{
				Document: fixture.document, TrustedKeys: fixture.trusted,
			})
			if err != nil {
				t.Fatalf("submissionauth.Verify(%v) error = %v, want nil", offering, err)
			}
			got, err := verified.Document()
			if err != nil || got != fixture.document {
				t.Fatalf("Verified.Document(%v) = (%+v, %v), want exact document and nil",
					offering, got, err)
			}
		})
	}
	if admitted < 3 {
		t.Fatalf("admitted offerings = %d, want at least the shipped set", admitted)
	}
}

// TestCredentialedRequestLayerTriadRefusesEveryAuthorityAndDeviceSubstitution proves a
// real signature is insufficient when it belongs to the wrong authority or a
// device the authenticated certificate did not nominate.
func TestCredentialedRequestLayerTriadRefusesEveryAuthorityAndDeviceSubstitution(t *testing.T) {
	t.Parallel()

	fixture := newAuthFixture(t, authFixtureRequest{})
	otherAuthorityPublic, _ := authSigningKey(t, 0x61)
	otherAuthorityTrust, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{otherAuthorityPublic},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys(other authority) error = %v, want nil", err)
	}
	if verified, err := Verify(Verification{
		Document: fixture.document, TrustedKeys: otherAuthorityTrust,
	}); !errors.Is(err, core.ErrAttestVerification) {
		t.Fatalf("Verify(other authority) = (%v, %v), want zero and errors.Is %v",
			verified, err, core.ErrAttestVerification)
	}

	_, otherDevice := authSigningKey(t, 0x62)
	otherRequest, err := submission.IssueRequest(submission.RequestIssuance{
		Payload: fixture.request.Payload, Signer: otherDevice,
	})
	if err != nil {
		t.Fatalf("submission.IssueRequest(other device) error = %v, want nil", err)
	}
	otherDeviceDocument, err := Assemble(RequestAssembly{
		Request: otherRequest, Certificate: fixture.certificate,
	})
	if err != nil {
		t.Fatalf("Assemble(other device signature) error = %v, want nil", err)
	}
	if verified, err := Verify(Verification{
		Document: otherDeviceDocument, TrustedKeys: fixture.trusted,
	}); !errors.Is(err, core.ErrAttestVerification) {
		t.Fatalf("Verify(other device) = (%v, %v), want zero and errors.Is %v",
			verified, err, core.ErrAttestVerification)
	}
}

// TestAssemblyRefusesAuthenticDocumentsForDifferentBuilds proves the binding
// owner, not either signature verifier, rejects cross-product assembly.
func TestAssemblyRefusesAuthenticDocumentsForDifferentBuilds(t *testing.T) {
	t.Parallel()

	fixture := newAuthFixture(t, authFixtureRequest{})
	bugInstallation, err := controlplanetest.IssueInstallation(
		controlplanetest.InstallationRequest{
			AuthoritySeed: authSeed(0x51), DeviceSeed: authSeed(0x52),
			Offering: core.OfferingBug,
		},
	)
	if err != nil {
		t.Fatalf("controlplanetest.IssueInstallation(Bug) error = %v, want nil", err)
	}
	bugPayload := authRequestPayload(t, bugInstallation.Build, 0x41)
	bugRequest, err := submission.IssueRequest(submission.RequestIssuance{
		Payload: bugPayload, Signer: fixture.device,
	})
	if err != nil {
		t.Fatalf("submission.IssueRequest(Bug) error = %v, want nil", err)
	}
	document, err := Assemble(RequestAssembly{
		Request: bugRequest, Certificate: fixture.certificate,
	})
	if !errors.Is(err, core.ErrControlPlaneResponseBinding) || document != (RequestDocument{}) {
		t.Fatalf("Assemble(different builds) = (%+v, %v), want zero and errors.Is %v",
			document, err, core.ErrControlPlaneResponseBinding)
	}
}

// TestCredentialedRequestJSONIsStrictBoundedAndPreserving proves the outer
// wire rejects framing attacks without weakening either nested document.
func TestCredentialedRequestJSONIsStrictBoundedAndPreserving(t *testing.T) {
	t.Parallel()

	fixture := newAuthFixture(t, authFixtureRequest{})
	encoded, err := fixture.document.MarshalJSON()
	if err != nil {
		t.Fatalf("RequestDocument.MarshalJSON() error = %v, want nil", err)
	}
	missingCertificate, err := json.Marshal(struct {
		Request submission.RequestDocument `json:"request"`
	}{Request: fixture.request})
	if err != nil {
		t.Fatalf("json.Marshal(missing certificate fixture) error = %v, want nil", err)
	}
	missingRequest, err := json.Marshal(struct {
		Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
	}{Certificate: fixture.certificate})
	if err != nil {
		t.Fatalf("json.Marshal(missing request fixture) error = %v, want nil", err)
	}
	reordered, err := json.Marshal(struct {
		Request     submission.RequestDocument                   `json:"request"`
		Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
	}{Certificate: fixture.certificate, Request: fixture.request})
	if err != nil {
		t.Fatalf("json.Marshal(reordered credentialed request fixture) error = %v, want nil", err)
	}
	nullRequest, err := json.Marshal(struct {
		Request     *submission.RequestDocument                  `json:"request"`
		Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
	}{Certificate: fixture.certificate})
	if err != nil {
		t.Fatalf("json.Marshal(null request fixture) error = %v, want nil", err)
	}
	nullCertificate, err := json.Marshal(struct {
		Certificate *controlplane.InstallationCertificateDocument `json:"certificate"`
		Request     submission.RequestDocument                    `json:"request"`
	}{Request: fixture.request})
	if err != nil {
		t.Fatalf("json.Marshal(null certificate fixture) error = %v, want nil", err)
	}
	wrongRequestType, err := json.Marshal(struct {
		Request     int                                          `json:"request"`
		Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
	}{Request: 1, Certificate: fixture.certificate})
	if err != nil {
		t.Fatalf("json.Marshal(wrong request type fixture) error = %v, want nil", err)
	}
	wrongCertificateType, err := json.Marshal(struct {
		Request     submission.RequestDocument `json:"request"`
		Certificate int                        `json:"certificate"`
	}{Request: fixture.request, Certificate: 1})
	if err != nil {
		t.Fatalf("json.Marshal(wrong certificate type fixture) error = %v, want nil", err)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, encoded, "", "  "); err != nil {
		t.Fatalf("json.Indent(credentialed request) error = %v, want nil", err)
	}
	unknown := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"future":true}`)...)
	duplicateRequest := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"request":null}`)...)
	duplicateCertificate := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"certificate":null}`)...)
	validCases := []struct {
		name string
		data []byte
	}{
		{name: "canonical credentialed request", data: encoded},
		{name: "one leading space", data: append([]byte(" "), encoded...)},
		{name: "one trailing space", data: append(bytes.Clone(encoded), ' ')},
		{name: "leading and trailing newlines", data: append(append([]byte("\n"), encoded...), '\n')},
		{name: "mixed legal outer whitespace", data: append(append([]byte("\t\r\n"), encoded...), ' ', '\t')},
		{name: "members in reverse order", data: reordered},
		{name: "indented credentialed request", data: indented.Bytes()},
		{name: "one byte below document ceiling", data: authLeftPadJSON(encoded, RequestDocumentJSONMaximumBytes-1)},
		{name: "exactly at document ceiling", data: authLeftPadJSON(encoded, RequestDocumentJSONMaximumBytes)},
		{name: "canonical second decode", data: bytes.Clone(encoded)},
	}
	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var receiver RequestDocument
			if err := receiver.UnmarshalJSON(tc.data); err != nil {
				t.Fatalf("RequestDocument.UnmarshalJSON(%s) error = %v, want nil", tc.name, err)
			}
			if receiver != fixture.document {
				t.Fatalf("RequestDocument.UnmarshalJSON(%s) = %+v, want %+v",
					tc.name, receiver, fixture.document)
			}
		})
	}
	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty input", data: nil},
		{name: "whitespace without a value", data: []byte(" \t\r\n")},
		{name: "null root", data: []byte(`null`)},
		{name: "string root", data: []byte(`"credentialed request"`)},
		{name: "number root", data: []byte(`1`)},
		{name: "boolean root", data: []byte(`true`)},
		{name: "array root", data: []byte(`[]`)},
		{name: "empty object", data: []byte(`{}`)},
		{name: "unknown member", data: unknown},
		{name: "duplicate request", data: duplicateRequest},
		{name: "duplicate certificate", data: duplicateCertificate},
		{name: "missing request", data: missingRequest},
		{name: "missing certificate", data: missingCertificate},
		{name: "null request", data: nullRequest},
		{name: "null certificate", data: nullCertificate},
		{name: "request has scalar type", data: wrongRequestType},
		{name: "certificate has scalar type", data: wrongCertificateType},
		{name: "truncated after opening brace", data: []byte(`{`)},
		{name: "truncated after request name", data: []byte(`{"request":`)},
		{name: "truncated canonical credentialed request", data: encoded[:len(encoded)-1]},
		{name: "second document trails canonical value", data: append(bytes.Clone(encoded), encoded...)},
		{name: "one byte above document ceiling", data: authLeftPadJSON(encoded, RequestDocumentJSONMaximumBytes+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			receiver := fixture.document
			if err := receiver.UnmarshalJSON(tc.data); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("json.Unmarshal(%s) error = %v, want errors.Is %v",
					tc.name, err, core.ErrJSONContract)
			}
			if receiver != fixture.document {
				t.Fatalf("json.Unmarshal(%s) mutated receiver = %+v, want preserved %+v",
					tc.name, receiver, fixture.document)
			}
		})
	}
}

func authLeftPadJSON(encoded []byte, length int) []byte {
	if length < len(encoded) {
		return nil
	}
	padded := make([]byte, length)
	for index := 0; index < length-len(encoded); index++ {
		padded[index] = ' '
	}
	copy(padded[length-len(encoded):], encoded)
	return padded
}

// TestCredentialedRequestLayerTriadZeroValuesNeverAcquireProof is the neutral
// boundary proof for assembly, verification input, and authenticated output.
func TestCredentialedRequestLayerTriadZeroValuesNeverAcquireProof(t *testing.T) {
	t.Parallel()

	cases := []struct {
		run  func() error
		name string
	}{
		{name: "document", run: func() error { return (RequestDocument{}).Validate() }},
		{name: "assembly", run: func() error { return (RequestAssembly{}).Validate() }},
		{name: "verification", run: func() error { return (Verification{}).Validate() }},
		{name: "verified", run: func() error { return (Verified{}).Validate() }},
		{name: "nil document JSON receiver", run: func() error {
			var receiver *RequestDocument
			return receiver.UnmarshalJSON([]byte(`{}`))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.run(); !errors.Is(err, core.ErrControlPlaneContract) {
				t.Fatalf("zero %s error = %v, want errors.Is %v",
					tc.name, err, core.ErrControlPlaneContract)
			}
		})
	}
}
