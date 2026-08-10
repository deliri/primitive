package retrievalauth

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlplanetest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/retrieval"
)

const retrievalAuthFixtureChit = "00000000-0010-7000-8000-000000000010"

type retrievalAuthFixtureRequest struct {
	Offering      core.Offering
	AuthorityByte byte
	DeviceByte    byte
	NonceByte     byte
}

type retrievalAuthFixture struct {
	device      ed25519.PrivateKey
	request     retrieval.RequestDocument
	document    RequestDocument
	trusted     attest.TrustedKeys
	certificate controlplane.InstallationCertificateDocument
}

func TestCredentialedRetrievalAuthenticatesEveryOfferingThroughOneBlindPath(t *testing.T) {
	t.Parallel()

	admitted := 0
	for raw := 0; raw <= 255; raw++ {
		offering := core.Offering(raw)
		if !offering.IsValid() {
			continue
		}
		admitted++
		t.Run(offering.String(), func(t *testing.T) {
			t.Parallel()

			fixture := newRetrievalAuthFixture(t, retrievalAuthFixtureRequest{
				Offering: offering, AuthorityByte: byte(raw) + 0x20,
				DeviceByte: byte(raw) + 0x40, NonceByte: byte(raw) + 1,
			})
			verified, err := Verify(Verification{
				Document: fixture.document, TrustedKeys: fixture.trusted,
			})
			if err != nil {
				t.Fatalf("retrievalauth.Verify(%v) error = %v, want nil", offering, err)
			}
			got, err := verified.Document()
			if err != nil || got != fixture.document {
				t.Fatalf("Verified.Document(%v) = (%v, %v), want exact document and nil",
					offering, got, err)
			}
		})
	}
	if admitted < 3 {
		t.Fatalf("admitted offerings = %d, want at least the shipped set", admitted)
	}
}

func TestCredentialedRetrievalRefusesAuthorityDeviceAndBuildSubstitution(t *testing.T) {
	t.Parallel()

	fixture := newRetrievalAuthFixture(t, retrievalAuthFixtureRequest{})
	otherAuthority := newRetrievalAuthFixture(t, retrievalAuthFixtureRequest{
		AuthorityByte: 0x61, DeviceByte: 0x62, NonceByte: 0x63,
	})
	if got, gotErr := Verify(Verification{
		Document: fixture.document, TrustedKeys: otherAuthority.trusted,
	}); !errors.Is(gotErr, core.ErrAttestVerification) || got != (Verified{}) {
		t.Fatalf("Verify(other authority) = (%v, %v), want zero and errors.Is %v",
			got, gotErr, core.ErrAttestVerification)
	}

	otherDevice := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x71}, ed25519.SeedSize))
	otherRequest, err := retrieval.IssueRequest(retrieval.RequestIssuance{
		Payload: fixture.request.Payload, Signer: otherDevice,
	})
	if err != nil {
		t.Fatalf("retrieval.IssueRequest(other device) error = %v, want nil", err)
	}
	otherDeviceDocument, err := Assemble(RequestAssembly{
		Request: otherRequest, Certificate: fixture.certificate,
	})
	if err != nil {
		t.Fatalf("Assemble(other device) error = %v, want nil", err)
	}
	if got, gotErr := Verify(Verification{
		Document: otherDeviceDocument, TrustedKeys: fixture.trusted,
	}); !errors.Is(gotErr, core.ErrAttestVerification) || got != (Verified{}) {
		t.Fatalf("Verify(other device) = (%v, %v), want zero and errors.Is %v",
			got, gotErr, core.ErrAttestVerification)
	}

	bugInstallation, err := controlplanetest.IssueInstallation(controlplanetest.InstallationRequest{
		AuthoritySeed: retrievalAuthSeed(0x72), DeviceSeed: retrievalAuthSeed(0x73),
		Offering: core.OfferingBug,
	})
	if err != nil {
		t.Fatalf("controlplanetest.IssueInstallation(Bug) error = %v, want nil", err)
	}
	wrongBuildPayload := retrievalAuthPayload(t, bugInstallation.Build, 0x74)
	wrongBuildRequest, err := retrieval.IssueRequest(retrieval.RequestIssuance{
		Payload: wrongBuildPayload, Signer: fixture.device,
	})
	if err != nil {
		t.Fatalf("retrieval.IssueRequest(wrong build) error = %v, want nil", err)
	}
	if got, gotErr := Assemble(RequestAssembly{
		Request: wrongBuildRequest, Certificate: fixture.certificate,
	}); !errors.Is(gotErr, core.ErrRetrievalBinding) || got != (RequestDocument{}) {
		t.Fatalf("Assemble(wrong build) = (%v, %v), want zero and errors.Is %v",
			got, gotErr, core.ErrRetrievalBinding)
	}
}

func TestCredentialedRetrievalStrictJSONIsBoundedAndTransactional(t *testing.T) {
	t.Parallel()

	fixture := newRetrievalAuthFixture(t, retrievalAuthFixtureRequest{})
	encoded, err := fixture.document.MarshalJSON()
	if err != nil {
		t.Fatalf("RequestDocument.MarshalJSON() error = %v, want nil", err)
	}
	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty document", data: []byte{}},
		{name: "null document", data: []byte("null")},
		{name: "array instead of structure", data: []byte{'[', ']'}},
		{name: "trailing object", data: append(append([]byte(nil), encoded...), '{', '}')},
		{name: "over owned bound", data: bytes.Repeat([]byte{' '}, RequestDocumentJSONMaximumBytes+1)},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := fixture.document
			gotErr := got.UnmarshalJSON(testCase.data)
			if !errors.Is(gotErr, core.ErrJSONContract) || got != fixture.document {
				t.Fatalf("RequestDocument.UnmarshalJSON(%q) = (%v, %v), want preserved document and errors.Is %v",
					testCase.data, got, gotErr, core.ErrJSONContract)
			}
		})
	}

	missingCertificate := RequestDocument{Request: fixture.request}
	if gotErr := missingCertificate.Validate(); !errors.Is(gotErr, core.ErrRetrievalContract) {
		t.Fatalf("RequestDocument.Validate(missing certificate) error = %v, want errors.Is %v",
			gotErr, core.ErrRetrievalContract)
	}
}

func newRetrievalAuthFixture(
	t *testing.T,
	request retrievalAuthFixtureRequest,
) retrievalAuthFixture {
	t.Helper()

	if request.Offering == core.OfferingUnknown {
		request.Offering = core.OfferingWitness
	}
	if request.AuthorityByte == 0 {
		request.AuthorityByte = 0x21
	}
	if request.DeviceByte == 0 {
		request.DeviceByte = 0x31
	}
	if request.NonceByte == 0 {
		request.NonceByte = 0x41
	}
	installation, err := controlplanetest.IssueInstallation(controlplanetest.InstallationRequest{
		AuthoritySeed: retrievalAuthSeed(request.AuthorityByte),
		DeviceSeed:    retrievalAuthSeed(request.DeviceByte), Offering: request.Offering,
	})
	if err != nil {
		t.Fatalf("controlplanetest.IssueInstallation() error = %v, want nil", err)
	}
	payload := retrievalAuthPayload(t, installation.Build, request.NonceByte)
	signed, err := retrieval.IssueRequest(retrieval.RequestIssuance{
		Payload: payload, Signer: installation.DevicePrivate,
	})
	if err != nil {
		t.Fatalf("retrieval.IssueRequest() error = %v, want nil", err)
	}
	document, err := Assemble(RequestAssembly{
		Request: signed, Certificate: installation.Certificate,
	})
	if err != nil {
		t.Fatalf("retrievalauth.Assemble() error = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{installation.AuthorityPublic},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}
	return retrievalAuthFixture{
		device: installation.DevicePrivate, request: signed, document: document,
		trusted: trusted, certificate: installation.Certificate,
	}
}

func retrievalAuthPayload(
	t *testing.T,
	build core.BuildIdentity,
	nonceByte byte,
) retrieval.RequestPayload {
	t.Helper()

	rawNonce := [controlwire.NonceBytes]byte{}
	rawNonce[0] = nonceByte
	nonce, err := controlwire.NewRequestNonce(rawNonce)
	if err != nil {
		t.Fatalf("controlwire.NewRequestNonce() error = %v, want nil", err)
	}
	payload := retrieval.RequestPayload{
		Build: build, Selection: retrieval.All(),
		Revision: controlwire.Revision2026V1, Nonce: nonce,
	}
	encodedIdentity, err := core.MarshalCanonicalJSONString(retrievalAuthFixtureChit)
	if err != nil {
		t.Fatalf("core.MarshalCanonicalJSONString(chit) error = %v, want nil", err)
	}
	if err := payload.Chit.UnmarshalJSON(encodedIdentity); err != nil {
		t.Fatalf("ChitID.UnmarshalJSON() error = %v, want nil", err)
	}
	return payload
}

func retrievalAuthSeed(marker byte) [ed25519.SeedSize]byte {
	seed := [ed25519.SeedSize]byte{}
	for index := range seed {
		seed[index] = marker
	}
	return seed
}
