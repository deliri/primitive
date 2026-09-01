package retrievalauth

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"fmt"
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
	document    RequestDocument
	certificate controlplane.InstallationCertificateDocument
	request     retrieval.RequestDocument
	trusted     attest.TrustedKeys
}

type retrievalAuthJSONCase struct {
	name string
	data []byte
}

func TestRetrievalAuthAssemblyLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive every offering and distinct installation facts assemble unchanged", func(t *testing.T) {
		t.Parallel()

		cases := []retrievalAuthFixtureRequest{
			{Offering: retrievalAuthOffering(t, 1), AuthorityByte: 0x11, DeviceByte: 0x21, NonceByte: 0x31},
			{Offering: retrievalAuthOffering(t, 2), AuthorityByte: 0x12, DeviceByte: 0x22, NonceByte: 0x32},
			{Offering: retrievalAuthOffering(t, 3), AuthorityByte: 0x13, DeviceByte: 0x23, NonceByte: 0x33},
			{Offering: retrievalAuthOffering(t, 1), AuthorityByte: 0x14, DeviceByte: 0x24, NonceByte: 0x34},
			{Offering: retrievalAuthOffering(t, 2), AuthorityByte: 0x15, DeviceByte: 0x25, NonceByte: 0x35},
			{Offering: retrievalAuthOffering(t, 3), AuthorityByte: 0x16, DeviceByte: 0x26, NonceByte: 0x36},
			{Offering: retrievalAuthOffering(t, 1), AuthorityByte: 0x17, DeviceByte: 0x27, NonceByte: 0x37},
			{Offering: retrievalAuthOffering(t, 2), AuthorityByte: 0x18, DeviceByte: 0x28, NonceByte: 0x38},
			{Offering: retrievalAuthOffering(t, 3), AuthorityByte: 0x19, DeviceByte: 0x29, NonceByte: 0x39},
			{Offering: retrievalAuthOffering(t, 2), AuthorityByte: 0x1a, DeviceByte: 0x2a, NonceByte: 0x3a},
		}
		for _, tc := range cases {
			fixture := newRetrievalAuthFixture(t, tc)
			route, routeErr := fixture.document.ControlRoute()
			if routeErr != nil || route.Offering() != fixture.request.Payload.Build.Offering() ||
				route.Family() != controlwire.RouteFamilyRetrievals ||
				fixture.document.ControlNonce() != fixture.request.Payload.Nonce {
				t.Fatalf("retrieval control projection(%v) = (%v, %v, %v), want exact route and signed nonce",
					tc.Offering, route, fixture.document.ControlNonce(), routeErr)
			}
			got, gotErr := Assemble(RequestAssembly{Request: fixture.request, Certificate: fixture.certificate})
			if gotErr != nil || got != fixture.document {
				t.Fatalf("Assemble(%v) = (%v, %v), want exact document and nil", tc.Offering, got, gotErr)
			}
		}
	})

	t.Run("negative missing malformed and cross-build structures assemble nothing", func(t *testing.T) {
		t.Parallel()

		fixture := newRetrievalAuthFixture(t, retrievalAuthFixtureRequest{})
		other := newRetrievalAuthFixture(t, retrievalAuthFixtureRequest{
			Offering: retrievalAuthOffering(t, 1), AuthorityByte: 0x51, DeviceByte: 0x52, NonceByte: 0x53,
		})
		otherAccount := fixture.document.Request.Payload.Scope.Account
		if err := otherAccount.UnmarshalJSON([]byte(`"99999999999999999999999999999999"`)); err != nil {
			t.Fatalf("AccountIdentity.UnmarshalJSON() error = %v, want nil", err)
		}
		cases := []struct {
			wantErr error
			mutate  func(*RequestAssembly)
			name    string
		}{
			{name: "zero assembly", mutate: func(value *RequestAssembly) { *value = RequestAssembly{} }, wantErr: core.ErrRetrievalContract},
			{name: "request absent", mutate: func(value *RequestAssembly) { value.Request = retrieval.RequestDocument{} }, wantErr: core.ErrRetrievalContract},
			{name: "certificate absent", mutate: func(value *RequestAssembly) { value.Certificate = controlplane.InstallationCertificateDocument{} }, wantErr: core.ErrRetrievalContract},
			{name: "request payload absent", mutate: func(value *RequestAssembly) { value.Request.Payload = retrieval.RequestPayload{} }, wantErr: core.ErrRetrievalContract},
			{name: "request attestation absent", mutate: func(value *RequestAssembly) { value.Request.Attestation = attest.Envelope[retrieval.SigningDomain]{} }, wantErr: core.ErrRetrievalContract},
			{name: "certificate body absent", mutate: func(value *RequestAssembly) { value.Certificate.Body = controlplane.InstallationCertificateBody{} }, wantErr: core.ErrRetrievalContract},
			{name: "certificate attestation absent", mutate: func(value *RequestAssembly) {
				value.Certificate.Attestation = attest.Envelope[controlplane.SigningDomain]{}
			}, wantErr: core.ErrRetrievalContract},
			{name: "request build belongs to another offering", mutate: func(value *RequestAssembly) { value.Request = other.request }, wantErr: core.ErrRetrievalBinding},
			{name: "request account scope differs from certificate", mutate: func(value *RequestAssembly) {
				value.Request.Payload.Scope.Account = otherAccount
			}, wantErr: core.ErrRetrievalBinding},
			{name: "certificate build belongs to another offering", mutate: func(value *RequestAssembly) { value.Certificate = other.certificate }, wantErr: core.ErrRetrievalBinding},
			{name: "request build absent", mutate: func(value *RequestAssembly) { value.Request.Payload.Build = core.BuildIdentity{} }, wantErr: core.ErrRetrievalContract},
			{name: "certificate build absent", mutate: func(value *RequestAssembly) { value.Certificate.Body.Build = core.BuildIdentity{} }, wantErr: core.ErrRetrievalContract},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				input := RequestAssembly{Request: fixture.request, Certificate: fixture.certificate}
				tc.mutate(&input)
				got, gotErr := Assemble(input)
				if !errors.Is(gotErr, tc.wantErr) || got != (RequestDocument{}) {
					t.Fatalf("Assemble() = (%v, %v), want zero and errors.Is %v", got, gotErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral zero assembly emits no credentialed request", func(t *testing.T) {
		t.Parallel()

		got, gotErr := Assemble(RequestAssembly{})
		if !errors.Is(gotErr, core.ErrRetrievalContract) || got != (RequestDocument{}) {
			t.Fatalf("Assemble(zero) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrRetrievalContract)
		}
	})
}

func TestRetrievalAuthVerificationLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive independent authority and device pairs authenticate exact documents", func(t *testing.T) {
		t.Parallel()

		cases := []retrievalAuthFixtureRequest{
			{Offering: retrievalAuthOffering(t, 1), AuthorityByte: 0x41, DeviceByte: 0x51, NonceByte: 0x61},
			{Offering: retrievalAuthOffering(t, 2), AuthorityByte: 0x42, DeviceByte: 0x52, NonceByte: 0x62},
			{Offering: retrievalAuthOffering(t, 3), AuthorityByte: 0x43, DeviceByte: 0x53, NonceByte: 0x63},
			{Offering: retrievalAuthOffering(t, 1), AuthorityByte: 0x44, DeviceByte: 0x54, NonceByte: 0x64},
			{Offering: retrievalAuthOffering(t, 3), AuthorityByte: 0x46, DeviceByte: 0x56, NonceByte: 0x66},
			{Offering: retrievalAuthOffering(t, 1), AuthorityByte: 0x47, DeviceByte: 0x57, NonceByte: 0x67},
			{Offering: retrievalAuthOffering(t, 2), AuthorityByte: 0x48, DeviceByte: 0x58, NonceByte: 0x68},
			{Offering: retrievalAuthOffering(t, 3), AuthorityByte: 0x49, DeviceByte: 0x59, NonceByte: 0x69},
			{Offering: retrievalAuthOffering(t, 2), AuthorityByte: 0x4a, DeviceByte: 0x5a, NonceByte: 0x6a},
		}
		for _, tc := range cases {
			fixture := newRetrievalAuthFixture(t, tc)
			verified, gotErr := Verify(Verification{Document: fixture.document, Server: retrievalAuthServer(t, fixture.trusted)})
			got, documentErr := verified.Document()
			if gotErr != nil || documentErr != nil || got != fixture.document {
				t.Fatalf("Verify()/Verified.Document(%v) = (%v, %v, %v), want exact document and nil", tc.Offering, got, gotErr, documentErr)
			}
		}
	})

	t.Run("negative authority device envelope and signed fact substitutions authenticate nothing", func(t *testing.T) {
		t.Parallel()

		fixture := newRetrievalAuthFixture(t, retrievalAuthFixtureRequest{AuthorityByte: 0x71, DeviceByte: 0x72, NonceByte: 0x73})
		otherAuthority := newRetrievalAuthFixture(t, retrievalAuthFixtureRequest{AuthorityByte: 0x74, DeviceByte: 0x75, NonceByte: 0x76})
		otherNonce := newRetrievalAuthFixture(t, retrievalAuthFixtureRequest{AuthorityByte: 0x71, DeviceByte: 0x72, NonceByte: 0x77})
		otherDevice := newRetrievalAuthFixture(t, retrievalAuthFixtureRequest{AuthorityByte: 0x71, DeviceByte: 0x78, NonceByte: 0x73})
		cases := []struct {
			wantErr error
			mutate  func(*Verification)
			name    string
		}{
			{name: "zero verification", mutate: func(value *Verification) { *value = Verification{} }, wantErr: core.ErrRetrievalContract},
			{name: "document absent", mutate: func(value *Verification) { value.Document = RequestDocument{} }, wantErr: core.ErrRetrievalContract},
			{name: "trusted authority absent", mutate: func(value *Verification) { value.Server = controlplane.Server{} }, wantErr: core.ErrRetrievalContract},
			{name: "different authority trust set", mutate: func(value *Verification) { value.Server = retrievalAuthServer(t, otherAuthority.trusted) }, wantErr: core.ErrAttestVerification},
			{name: "authentic certificate names another device", mutate: func(value *Verification) { value.Document.Certificate = otherDevice.certificate }, wantErr: core.ErrAttestVerification},
			{name: "request nonce substituted after signing", mutate: func(value *Verification) { value.Document.Request.Payload.Nonce = otherNonce.request.Payload.Nonce }, wantErr: core.ErrAttestVerification},
			{name: "request signer substituted", mutate: func(value *Verification) {
				value.Document.Request.Attestation.Signer = otherDevice.request.Attestation.Signer
			}, wantErr: core.ErrAttestVerification},
			{name: "request signature substituted", mutate: func(value *Verification) {
				value.Document.Request.Attestation.Signature = otherDevice.request.Attestation.Signature
			}, wantErr: core.ErrAttestVerification},
			{name: "request body digest substituted", mutate: func(value *Verification) {
				value.Document.Request.Attestation.BodySHA256 = otherNonce.request.Attestation.BodySHA256
			}, wantErr: core.ErrAttestVerification},
			{name: "certificate device binding substituted after signing", mutate: func(value *Verification) {
				value.Document.Certificate.Body.DeviceKey = otherDevice.certificate.Body.DeviceKey
				value.Document.Certificate.Body.Subject.DeviceID = otherDevice.certificate.Body.Subject.DeviceID
			}, wantErr: core.ErrAttestVerification},
			{name: "certificate signer substituted", mutate: func(value *Verification) {
				value.Document.Certificate.Attestation.Signer = otherAuthority.certificate.Attestation.Signer
			}, wantErr: core.ErrAttestVerification},
			{name: "certificate signature substituted", mutate: func(value *Verification) {
				value.Document.Certificate.Attestation.Signature = otherAuthority.certificate.Attestation.Signature
			}, wantErr: core.ErrAttestVerification},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				input := Verification{Document: fixture.document, Server: retrievalAuthServer(t, fixture.trusted)}
				tc.mutate(&input)
				got, gotErr := Verify(input)
				if !errors.Is(gotErr, tc.wantErr) || got != (Verified{}) {
					t.Fatalf("Verify() = (%v, %v), want zero and errors.Is %v", got, gotErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral zero verified value discloses no request", func(t *testing.T) {
		t.Parallel()

		got, gotErr := (Verified{}).Document()
		if !errors.Is(gotErr, core.ErrRetrievalContract) || got != (RequestDocument{}) {
			t.Fatalf("zero Verified.Document() = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrRetrievalContract)
		}
	})
}

func TestRetrievalAuthDocumentJSONLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := newRetrievalAuthFixture(t, retrievalAuthFixtureRequest{})
	canonical, gotErr := fixture.document.MarshalJSON()
	if gotErr != nil {
		t.Fatalf("RequestDocument.MarshalJSON() error = %v, want nil", gotErr)
	}

	t.Run("positive canonical reordered and exact extent documents preserve every fact", func(t *testing.T) {
		t.Parallel()

		cases := []retrievalAuthJSONCase{
			{name: "canonical credentialed request", data: canonical},
			{name: "leading whitespace", data: append([]byte(" \n\t"), canonical...)},
			{name: "trailing whitespace", data: append(append([]byte(nil), canonical...), ' ', '\n', '\t')},
			{name: "both-side whitespace", data: append(append([]byte(" \n"), canonical...), '\n', ' ')},
			{name: "top-level members reordered", data: marshalReorderedRetrievalAuthDocument(t, fixture.document)},
			{name: "one below document ceiling", data: retrievalAuthPadJSON(canonical, RequestDocumentJSONMaximumBytes-1)},
			{name: "at document ceiling", data: retrievalAuthPadJSON(canonical, RequestDocumentJSONMaximumBytes)},
			{name: "one trailing carriage return", data: append(append([]byte(nil), canonical...), '\r')},
			{name: "four leading whitespace forms", data: append([]byte("\t\r\n "), canonical...)},
			{name: "four trailing whitespace forms", data: append(append([]byte(nil), canonical...), " \n\r\t"...)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				var got RequestDocument
				decodeErr := got.UnmarshalJSON(tc.data)
				if decodeErr != nil || got != fixture.document {
					t.Fatalf("RequestDocument.UnmarshalJSON() = (%v, %v), want exact document and nil", got, decodeErr)
				}
			})
		}
	})

	t.Run("negative malformed missing duplicate type-wrong and oversized documents reject", func(t *testing.T) {
		t.Parallel()

		cases := retrievalAuthHostileJSONCases(canonical)
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := fixture.document
				decodeErr := got.UnmarshalJSON(tc.data)
				if !errors.Is(decodeErr, core.ErrJSONContract) || got != fixture.document {
					t.Fatalf("RequestDocument.UnmarshalJSON() = (%v, %v), want preserved receiver and errors.Is %v", got, decodeErr, core.ErrJSONContract)
				}
			})
		}
	})

	t.Run("neutral rejected input discloses no credentialed request", func(t *testing.T) {
		t.Parallel()

		var got RequestDocument
		decodeErr := got.UnmarshalJSON(nil)
		if !errors.Is(decodeErr, core.ErrJSONContract) || got != (RequestDocument{}) {
			t.Fatalf("zero RequestDocument.UnmarshalJSON(nil) = (%v, %v), want zero and errors.Is %v", got, decodeErr, core.ErrJSONContract)
		}
	})
}

func marshalReorderedRetrievalAuthDocument(t *testing.T, document RequestDocument) []byte {
	t.Helper()

	encoded, gotErr := core.MarshalCanonicalJSONDocument(struct {
		Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
		Request     retrieval.RequestDocument                    `json:"request"`
	}{Certificate: document.Certificate, Request: document.Request})
	if gotErr != nil {
		t.Fatalf("core.MarshalCanonicalJSONDocument(reordered retrieval auth) error = %v, want nil", gotErr)
	}
	return encoded
}

func retrievalAuthPadJSON(document []byte, wantBytes int) []byte {
	if len(document) >= wantBytes {
		return append([]byte(nil), document...)
	}
	return append(append([]byte(nil), document...), bytes.Repeat([]byte{' '}, wantBytes-len(document))...)
}

func retrievalAuthHostileJSONCases(canonical []byte) []retrievalAuthJSONCase {
	return []retrievalAuthJSONCase{
		{name: "empty document", data: nil},
		{name: "whitespace-only document", data: []byte(" \n\t")},
		{name: "null document", data: []byte("null")},
		{name: "array instead of structure", data: []byte("[]")},
		{name: "string instead of structure", data: []byte(`"retrieval"`)},
		{name: "number instead of structure", data: []byte("1")},
		{name: "boolean instead of structure", data: []byte("true")},
		{name: "truncated opening brace", data: []byte("{")},
		{name: "truncated inside request", data: canonical[:len(canonical)/2]},
		{name: "truncated before final brace", data: canonical[:len(canonical)-1]},
		{name: "trailing object", data: append(append([]byte(nil), canonical...), '{', '}')},
		{name: "two concatenated documents", data: append(append([]byte(nil), canonical...), canonical...)},
		{name: "unknown top-level member", data: append([]byte(`{"unknown":1,`), canonical[1:]...)},
		{name: "duplicate request member", data: append(bytes.Clone(canonical[:len(canonical)-1]), []byte(`,"request":null}`)...)},
		{name: "duplicate certificate member", data: append(bytes.Clone(canonical[:len(canonical)-1]), []byte(`,"certificate":null}`)...)},
		{name: "missing every member", data: []byte("{}")},
		{name: "missing request", data: []byte(`{"certificate":null}`)},
		{name: "missing certificate", data: []byte(`{"request":null}`)},
		{name: "request has wrong scalar type", data: []byte(`{"request":1,"certificate":null}`)},
		{name: "certificate has wrong scalar type", data: []byte(`{"request":null,"certificate":1}`)},
		{name: "one above document ceiling", data: retrievalAuthPadJSON(canonical, RequestDocumentJSONMaximumBytes+1)},
	}
}

func newRetrievalAuthFixture(
	t testing.TB,
	request retrievalAuthFixtureRequest,
) retrievalAuthFixture {
	t.Helper()

	if !request.Offering.IsValid() {
		request.Offering = retrievalAuthOffering(t, 2)
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
	payload := retrievalAuthPayload(t, installation.Certificate.Body, request.NonceByte)
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

func retrievalAuthOffering(t testing.TB, marker byte) core.Offering {
	t.Helper()
	offering := core.Offering{Token: fmt.Sprintf("retrievalauth-fixture-%02x", marker)}
	if err := offering.Validate(); err != nil {
		t.Fatalf("Offering.Validate() error = %v, want nil", err)
	}
	return offering
}

func retrievalAuthPayload(
	t testing.TB,
	certificate controlplane.InstallationCertificateBody,
	nonceByte byte,
) retrieval.RequestPayload {
	t.Helper()
	scope, err := certificate.Scope()
	if err != nil {
		t.Fatalf("InstallationCertificateBody.Scope() error = %v, want nil", err)
	}

	rawNonce := [core.SHA256DigestBytes]byte{}
	rawNonce[0] = nonceByte
	nonce, err := controlwire.NewRequestNonce(rawNonce)
	if err != nil {
		t.Fatalf("controlwire.NewRequestNonce() error = %v, want nil", err)
	}
	payload := retrieval.RequestPayload{
		Build: certificate.Build, Scope: scope, Selection: retrieval.StartAll(),
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
