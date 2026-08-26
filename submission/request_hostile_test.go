package submission

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"io"
	"strings"
	"testing"

	"encoding/json/jsontext"
	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/core"
)

type canonicalWriterResponse struct {
	written func(int) int
	err     error
}

type canonicalResponseWriter struct {
	response canonicalWriterResponse
	bytes    bytes.Buffer
}

func (w *canonicalResponseWriter) Write(payload []byte) (int, error) {
	written := w.response.written(len(payload))
	if written > 0 && written <= len(payload) {
		_, _ = w.bytes.Write(payload[:written])
	}
	return written, w.response.err
}

func TestSignedPayloadCanonicalOutputLayerTriad(t *testing.T) {
	t.Parallel()

	grant := newGrantFixture(t, grantFixtureRequest{})
	completion := newCompletionFixture(t, submissionOffering(t, 2), []byte("canonical output proof"), 0x10)
	projection, err := IssueCompletion(CompletionIssuance{
		Signer: completion.deviceSigner, Transfer: completion.transfer,
		Request: completion.request, Grant: completion.grant,
	})
	if err != nil {
		t.Fatalf("IssueCompletion() setup error = %v, want nil", err)
	}
	projectionPayload, err := completionProjection(CompletionIssuance{
		Signer: completion.deviceSigner, Transfer: completion.transfer,
		Request: completion.request, Grant: completion.grant,
	})
	if err != nil {
		t.Fatalf("completionProjection() setup error = %v, want nil", err)
	}
	document := receiveCompletionProjection(t, projection)
	payloads := []struct {
		marshal func() ([]byte, error)
		write   func(io.Writer) error
		name    string
	}{
		{name: "device request", marshal: grant.request.MarshalJSON, write: grant.request.WriteCanonical},
		{name: "authority grant", marshal: grant.payload.MarshalJSON, write: grant.payload.WriteCanonical},
		{name: "received completion", marshal: document.Payload.MarshalJSON, write: document.Payload.WriteCanonical},
		{name: "issue-only completion", marshal: projectionPayload.MarshalJSON, write: projectionPayload.WriteCanonical},
	}
	responses := []struct {
		result  canonicalWriterResponse
		wantErr error
		name    string
	}{
		{name: "exact count succeeds", result: canonicalWriterResponse{written: func(length int) int { return length }}},
		{name: "one byte short with nil error", result: canonicalWriterResponse{written: func(length int) int { return length - 1 }}, wantErr: io.ErrShortWrite},
		{name: "zero bytes with nil error", result: canonicalWriterResponse{written: func(int) int { return 0 }}, wantErr: io.ErrShortWrite},
		{name: "negative count with nil error", result: canonicalWriterResponse{written: func(int) int { return -1 }}, wantErr: io.ErrShortWrite},
		{name: "one byte overreported with nil error", result: canonicalWriterResponse{written: func(length int) int { return length + 1 }}, wantErr: io.ErrShortWrite},
		{name: "half written with native error", result: canonicalWriterResponse{written: func(length int) int { return length / 2 }, err: io.ErrClosedPipe}, wantErr: io.ErrClosedPipe},
		{name: "exact count with native error", result: canonicalWriterResponse{written: func(length int) int { return length }, err: io.ErrClosedPipe}, wantErr: io.ErrClosedPipe},
	}
	for _, payload := range payloads {
		t.Run(payload.name, func(t *testing.T) {
			t.Parallel()
			canonical, gotErr := payload.marshal()
			if gotErr != nil {
				t.Fatalf("MarshalJSON() setup error = %v, want nil", gotErr)
			}
			for _, response := range responses {
				t.Run(response.name, func(t *testing.T) {
					t.Parallel()
					writer := &canonicalResponseWriter{response: response.result}
					gotErr := payload.write(writer)
					if response.wantErr == nil {
						if gotErr != nil || !bytes.Equal(writer.bytes.Bytes(), canonical) {
							t.Fatalf("WriteCanonical(exact) = (%x, %v), want (%x, nil)", writer.bytes.Bytes(), gotErr, canonical)
						}
						return
					}
					if !errors.Is(gotErr, core.ErrControlPlaneContract) || !errors.Is(gotErr, response.wantErr) {
						t.Fatalf("WriteCanonical(%s) error = %v, want errors.Is %v and %v",
							response.name, gotErr, core.ErrControlPlaneContract, response.wantErr)
					}
				})
			}
			if gotErr := payload.write(nil); !errors.Is(gotErr, core.ErrControlPlaneContract) {
				t.Fatalf("WriteCanonical(nil) error = %v, want errors.Is %v", gotErr, core.ErrControlPlaneContract)
			}
		})
	}
}

// TestRequestAuthenticationLayerTriadAuthenticatesRepresentativeOpaqueOfferingsThroughOneShape
// proves the blind request protocol has no product arm.
func TestRequestAuthenticationLayerTriadAuthenticatesRepresentativeOpaqueOfferingsThroughOneShape(t *testing.T) {
	t.Parallel()

	for value, offering := range []core.Offering{
		submissionOffering(t, 1),
		submissionOffering(t, 127),
		submissionOffering(t, 255),
	} {
		t.Run(offering.String(), func(t *testing.T) {
			t.Parallel()

			public, signer := testSigningKey(t, byte(value)+0x60)
			payload := testRequestPayload(t, grantFixtureRequest{
				offering: offering, requestNonceByte: byte(value) + 1,
			})
			document, err := IssueRequest(RequestIssuance{Payload: payload, Signer: signer})
			if err != nil {
				t.Fatalf("IssueRequest(%v) error = %v, want nil", offering, err)
			}
			trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
				Keys: []core.Ed25519PublicKey{public},
			})
			if err != nil {
				t.Fatalf("attest.NewTrustedKeys(%v) error = %v, want nil", offering, err)
			}
			verified, err := VerifyRequest(RequestVerification{
				Document: document, TrustedKeys: trusted,
			})
			if err != nil {
				t.Fatalf("VerifyRequest(%v) error = %v, want nil", offering, err)
			}
			got, err := verified.Document()
			if err != nil || got != document {
				t.Fatalf("VerifiedRequest.Document(%v) = (%+v, %v), want exact document and nil",
					offering, got, err)
			}
		})
	}
}

// TestRequestAuthenticationLayerTriadRefusesEveryValidFactSubstitution changes one valid
// signed fact at a time while preserving an otherwise valid document. The
// structural gate stays green and signature authentication must go red.
func TestRequestAuthenticationLayerTriadRefusesEveryValidFactSubstitution(t *testing.T) {
	t.Parallel()

	public, signer := testSigningKey(t, 0x71)
	original := testRequestPayload(t, grantFixtureRequest{})
	document, err := IssueRequest(RequestIssuance{Payload: original, Signer: signer})
	if err != nil {
		t.Fatalf("IssueRequest() error = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{public},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}

	differentMediaType, err := core.ParseHTTPMediaType("application/cbor")
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType() error = %v, want nil", err)
	}
	mediaTypeMutation := original
	mediaTypeMutation.Declaration.ContentType = differentMediaType
	partitionMutation := original
	partitionMutation.Manifest.Partition = submissionPartition(t, 0x52)
	cases := []struct {
		name    string
		payload RequestPayload
		wantErr error
	}{
		{
			name: "different object integrity and extent",
			payload: testRequestPayload(t, grantFixtureRequest{
				content: []byte("different proof"), offering: submissionOffering(t, 2),
				requestNonceByte: 0x31,
			}),
			wantErr: core.ErrAttestVerification,
		},
		{name: "different media type", payload: mediaTypeMutation, wantErr: core.ErrAttestVerification},
		{name: "different custody partition", payload: partitionMutation, wantErr: core.ErrAttestVerification},
		{
			name: "different build offering",
			payload: testRequestPayload(t, grantFixtureRequest{
				offering: submissionOffering(t, 1), requestNonceByte: 0x31,
			}),
			wantErr: core.ErrAttestVerification,
		},
		{
			name: "different request nonce",
			payload: testRequestPayload(t, grantFixtureRequest{
				offering: submissionOffering(t, 2), requestNonceByte: 0x32,
			}),
			wantErr: core.ErrAttestVerification,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mutated := document
			mutated.Payload = tc.payload
			if err := mutated.Validate(); err != nil {
				t.Fatalf("mutated RequestDocument.Validate() error = %v, want nil", err)
			}
			verified, err := VerifyRequest(RequestVerification{
				Document: mutated, TrustedKeys: trusted,
			})
			if !errors.Is(err, tc.wantErr) || verified != (VerifiedRequest{}) {
				t.Fatalf("VerifyRequest(valid substitution) = (%v, %v), want zero and errors.Is %v",
					verified, err, tc.wantErr)
			}
		})
	}
}

// TestRequestRefusesAnAuthenticSignatureFromEveryOtherKey proves caller trust,
// not signature validity alone, decides which device may request custody.
func TestRequestRefusesAnAuthenticSignatureFromEveryOtherKey(t *testing.T) {
	t.Parallel()

	_, signer := testSigningKey(t, 0x72)
	otherPublic, _ := testSigningKey(t, 0x73)
	document, err := IssueRequest(RequestIssuance{
		Payload: testRequestPayload(t, grantFixtureRequest{}), Signer: signer,
	})
	if err != nil {
		t.Fatalf("IssueRequest() error = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{otherPublic},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys(other) error = %v, want nil", err)
	}
	verified, err := VerifyRequest(RequestVerification{Document: document, TrustedKeys: trusted})
	if !errors.Is(err, core.ErrAttestVerification) {
		t.Fatalf("VerifyRequest(other key) = (%v, %v), want zero and errors.Is %v",
			verified, err, core.ErrAttestVerification)
	}
}

// TestRequestCommitmentChangesForEveryAuthorizationFact proves the grant's one
// digest closes every field the device signed, rather than a hand-selected
// subset that could drift from RequestPayload.
func TestRequestCommitmentChangesForEveryAuthorizationFact(t *testing.T) {
	t.Parallel()

	original := testRequestPayload(t, grantFixtureRequest{})
	want, err := CommitRequest(original)
	if err != nil {
		t.Fatalf("CommitRequest(original) error = %v, want nil", err)
	}
	differentMediaType, err := core.ParseHTTPMediaType("application/cbor")
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType() error = %v, want nil", err)
	}
	mediaTypeMutation := original
	mediaTypeMutation.Declaration.ContentType = differentMediaType
	differentUpload := original
	differentUpload.Manifest.Upload, err = ParseUploadID("00000000-0008-7000-8000-000000000008")
	if err != nil {
		t.Fatalf("ParseUploadID(different) error = %v, want nil", err)
	}
	differentName := original
	differentName.Manifest.Name, err = chit.ParseEntryName("different.json")
	if err != nil {
		t.Fatalf("chit.ParseEntryName(different) error = %v, want nil", err)
	}
	differentPosition := original
	differentPosition.Manifest.Sequence = manifestSequence(t, 2)
	differentPosition.Manifest.Objects = manifestObjects(t, 2)
	differentCount := original
	differentCount.Manifest.Objects = manifestObjects(t, 2)
	differentPartition := original
	differentPartition.Manifest.Partition = submissionPartition(t, 0x52)
	cases := []struct {
		name    string
		payload RequestPayload
	}{
		{name: "object integrity extent and offering", payload: testRequestPayload(t, grantFixtureRequest{
			content: []byte("different proof"), offering: submissionOffering(t, 2),
			requestNonceByte: 0x31,
		})},
		{name: "content type", payload: mediaTypeMutation},
		{name: "upload identity", payload: differentUpload},
		{name: "portable entry name", payload: differentName},
		{name: "manifest position", payload: differentPosition},
		{name: "manifest object count", payload: differentCount},
		{name: "custody partition", payload: differentPartition},
		{name: "build offering", payload: testRequestPayload(t, grantFixtureRequest{
			offering: submissionOffering(t, 1), requestNonceByte: 0x31,
		})},
		{name: "request nonce", payload: testRequestPayload(t, grantFixtureRequest{
			offering: submissionOffering(t, 2), requestNonceByte: 0x32,
		})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := CommitRequest(tc.payload)
			if gotErr != nil || got == want {
				t.Fatalf("CommitRequest(%s) = (%v, %v), want distinct commitment and nil", tc.name, got, gotErr)
			}
		})
	}
}

// TestRequestCommitmentJSONRejectsEveryNonCanonicalDigestForm proves the
// authority-facing commitment cannot be zero, widened, shortened, recased, or
// partially decoded, and every refusal preserves the prior exact request.
func TestRequestCommitmentJSONRejectsEveryNonCanonicalDigestForm(t *testing.T) {
	t.Parallel()

	preserved, err := CommitRequest(testRequestPayload(t, grantFixtureRequest{}))
	if err != nil {
		t.Fatalf("CommitRequest() error = %v, want nil", err)
	}
	encoded, err := preserved.MarshalJSON()
	if err != nil {
		t.Fatalf("RequestCommitment.MarshalJSON() error = %v, want nil", err)
	}
	var roundTrip RequestCommitment
	if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != preserved {
		t.Fatalf("RequestCommitment.UnmarshalJSON(canonical) = (%v, %v), want (%v, nil)",
			roundTrip, err, preserved)
	}
	canonical := string(encoded[1 : len(encoded)-1])
	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty input", data: nil},
		{name: "null value", data: []byte(`null`)},
		{name: "numeric value", data: []byte(`1`)},
		{name: "boolean value", data: []byte(`true`)},
		{name: "array value", data: []byte(`[]`)},
		{name: "object value", data: []byte(`{}`)},
		{name: "empty text", data: []byte(`""`)},
		{name: "one hex digit", data: []byte(`"0"`)},
		{name: "one byte below exact extent", data: []byte(`"` + canonical[:len(canonical)-1] + `"`)},
		{name: "one byte above exact extent", data: []byte(`"` + canonical + `0"`)},
		{name: "all-zero digest", data: []byte(`"` + strings.Repeat("0", 64) + `"`)},
		{name: "uppercase hex", data: []byte(`"` + strings.ToUpper(canonical) + `"`)},
		{name: "non-hex character", data: []byte(`"` + canonical[:63] + `g"`)},
		{name: "hex prefix", data: []byte(`"0x` + canonical[:62] + `"`)},
		{name: "leading space inside digest", data: []byte(`" ` + canonical[:63] + `"`)},
		{name: "trailing second JSON value", data: append(bytes.Clone(encoded), []byte(` null`)...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			receiver := preserved
			if err := receiver.UnmarshalJSON(tc.data); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("RequestCommitment.UnmarshalJSON(%s) error = %v, want errors.Is %v",
					tc.name, err, core.ErrJSONContract)
			}
			if receiver != preserved {
				t.Fatalf("RequestCommitment.UnmarshalJSON(%s) receiver = %v, want preserved %v",
					tc.name, receiver, preserved)
			}
		})
	}
	if encoded, err := (RequestCommitment{}).MarshalJSON(); !errors.Is(err, core.ErrJSONContract) || encoded != nil {
		t.Fatalf("zero RequestCommitment.MarshalJSON() = (%v, %v), want nil and errors.Is %v",
			encoded, err, core.ErrJSONContract)
	}
}

// TestRequestJSONBoundaryIsStrictBoundedAndPreserving attacks document framing
// rather than implementation fields: unknown, duplicate, missing, oversized,
// and malformed inputs all refuse without changing an already-valid receiver.
func TestRequestJSONBoundaryIsStrictBoundedAndPreserving(t *testing.T) {
	t.Parallel()

	_, signer := testSigningKey(t, 0x74)
	document, err := IssueRequest(RequestIssuance{
		Payload: testRequestPayload(t, grantFixtureRequest{}), Signer: signer,
	})
	if err != nil {
		t.Fatalf("IssueRequest() error = %v, want nil", err)
	}
	encoded, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("RequestDocument.MarshalJSON() error = %v, want nil", err)
	}
	withoutAttestation, err := json.Marshal(struct {
		Payload RequestPayload `json:"payload"`
	}{Payload: document.Payload})
	if err != nil {
		t.Fatalf("json.Marshal(missing attestation fixture) error = %v, want nil", err)
	}
	withoutPayload, err := json.Marshal(struct {
		Attestation attest.Envelope[SigningDomain] `json:"attestation"`
	}{Attestation: document.Attestation})
	if err != nil {
		t.Fatalf("json.Marshal(missing payload fixture) error = %v, want nil", err)
	}
	reordered, err := json.Marshal(struct {
		Payload     RequestPayload                 `json:"payload"`
		Attestation attest.Envelope[SigningDomain] `json:"attestation"`
	}{Attestation: document.Attestation, Payload: document.Payload})
	if err != nil {
		t.Fatalf("json.Marshal(reordered fixture) error = %v, want nil", err)
	}
	nullPayload, err := json.Marshal(struct {
		Payload     *RequestPayload                `json:"payload"`
		Attestation attest.Envelope[SigningDomain] `json:"attestation"`
	}{Attestation: document.Attestation})
	if err != nil {
		t.Fatalf("json.Marshal(null payload fixture) error = %v, want nil", err)
	}
	wrongPayloadType, err := json.Marshal(struct {
		Payload     int                            `json:"payload"`
		Attestation attest.Envelope[SigningDomain] `json:"attestation"`
	}{Payload: 1, Attestation: document.Attestation})
	if err != nil {
		t.Fatalf("json.Marshal(wrong payload type fixture) error = %v, want nil", err)
	}
	wrongAttestationType, err := json.Marshal(struct {
		Payload     RequestPayload `json:"payload"`
		Attestation int            `json:"attestation"`
	}{Payload: document.Payload, Attestation: 1})
	if err != nil {
		t.Fatalf("json.Marshal(wrong attestation type fixture) error = %v, want nil", err)
	}
	indented := jsontext.Value(bytes.Clone(encoded))
	if err := indented.Indent(jsontext.WithIndent("  ")); err != nil {
		t.Fatalf("json.Indent() error = %v, want nil", err)
	}
	validCases := []struct {
		name string
		data []byte
	}{
		{name: "canonical document", data: encoded},
		{name: "one leading space", data: append([]byte(" "), encoded...)},
		{name: "one trailing space", data: append(bytes.Clone(encoded), ' ')},
		{name: "leading and trailing newlines", data: append(append([]byte("\n"), encoded...), '\n')},
		{name: "mixed legal outer whitespace", data: append(append([]byte("\t\r\n"), encoded...), ' ', '\t')},
		{name: "members in reverse order", data: reordered},
		{name: "indented object", data: []byte(indented)},
		{name: "one byte below document ceiling", data: leftPadJSON(encoded, RequestDocumentJSONMaximumBytes-1)},
		{name: "exactly at document ceiling", data: leftPadJSON(encoded, RequestDocumentJSONMaximumBytes)},
		{name: "canonical second decode", data: bytes.Clone(encoded)},
	}
	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			receiver := RequestDocument{}
			if err := receiver.UnmarshalJSON(tc.data); err != nil {
				t.Fatalf("RequestDocument.UnmarshalJSON(%s) error = %v, want nil", tc.name, err)
			}
			if receiver != document {
				t.Fatalf("RequestDocument.UnmarshalJSON(%s) = %+v, want %+v", tc.name, receiver, document)
			}
		})
	}
	unknown := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"future":true}`)...)
	duplicatePayload := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"payload":null}`)...)
	duplicateAttestation := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"attestation":null}`)...)
	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty input", data: nil},
		{name: "whitespace without a value", data: []byte(" \t\r\n")},
		{name: "null root", data: []byte(`null`)},
		{name: "string root", data: []byte(`"document"`)},
		{name: "number root", data: []byte(`1`)},
		{name: "boolean root", data: []byte(`true`)},
		{name: "array root", data: []byte(`[]`)},
		{name: "empty object", data: []byte(`{}`)},
		{name: "unknown member", data: unknown},
		{name: "duplicate payload", data: duplicatePayload},
		{name: "duplicate attestation", data: duplicateAttestation},
		{name: "missing payload", data: withoutPayload},
		{name: "missing attestation", data: withoutAttestation},
		{name: "null payload", data: nullPayload},
		{name: "null attestation", data: append(bytes.Clone(withoutAttestation[:len(withoutAttestation)-1]), []byte(`,"attestation":null}`)...)},
		{name: "payload has scalar type", data: wrongPayloadType},
		{name: "attestation has scalar type", data: wrongAttestationType},
		{name: "truncated after opening brace", data: []byte(`{`)},
		{name: "truncated after payload name", data: []byte(`{"payload":`)},
		{name: "truncated canonical document", data: encoded[:len(encoded)-1]},
		{name: "second document trails canonical value", data: append(bytes.Clone(encoded), encoded...)},
		{name: "one byte above document ceiling", data: leftPadJSON(encoded, RequestDocumentJSONMaximumBytes+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			receiver := document
			if err := receiver.UnmarshalJSON(tc.data); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("json.Unmarshal(%s) error = %v, want errors.Is %v",
					tc.name, err, core.ErrJSONContract)
			}
			if receiver != document {
				t.Fatalf("json.Unmarshal(%s) mutated receiver = %+v, want preserved %+v",
					tc.name, receiver, document)
			}
		})
	}
}

func leftPadJSON(encoded []byte, length int) []byte {
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

// TestSigningDomainClosesItsEntireByteDomain proves only the request and grant
// namespaces can enter an attestation envelope and that their texts are
// distinct fixed points of their own parser.
func TestSigningDomainClosesItsEntireByteDomain(t *testing.T) {
	t.Parallel()

	admitted := 0
	seen := make(map[string]SigningDomain)
	for value := 0; value <= 255; value++ {
		domain := SigningDomain(value)
		if !domain.IsValid() {
			if err := domain.Validate(); !errors.Is(err, core.ErrControlPlaneSigningDomain) {
				t.Fatalf("SigningDomain(%d).Validate() error = %v, want errors.Is %v",
					value, err, core.ErrControlPlaneSigningDomain)
			}
			if domain.String() != "" {
				t.Fatalf("SigningDomain(%d).String() = %q, want empty", value, domain.String())
			}
			continue
		}
		admitted++
		text, err := domain.MarshalText()
		if err != nil {
			t.Fatalf("SigningDomain(%d).MarshalText() error = %v, want nil", value, err)
		}
		parsed, err := (SigningDomainUnknown).ParseCanonicalText(text)
		if err != nil || parsed != domain {
			t.Fatalf("ParseCanonicalText(SigningDomain(%d)) = (%v, %v), want (%v, nil)",
				value, parsed, err, domain)
		}
		encoded, err := domain.MarshalJSON()
		if err != nil {
			t.Fatalf("SigningDomain(%d).MarshalJSON() error = %v, want nil", value, err)
		}
		var decoded SigningDomain
		if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != domain {
			t.Fatalf("SigningDomain.UnmarshalJSON(%d) = (%v, %v), want (%v, nil)",
				value, decoded, err, domain)
		}
		if prior, duplicate := seen[string(text)]; duplicate {
			t.Fatalf("SigningDomain(%d) and %d share %q", value, prior, text)
		}
		seen[string(text)] = domain
	}
	if admitted != 3 {
		t.Fatalf("admitted signing domains = %d, want request, grant, and completion", admitted)
	}

	preserved := SigningDomainGrantV1
	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty input", data: nil},
		{name: "empty namespace", data: []byte(`""`)},
		{name: "unknown namespace", data: []byte(`"primitive-submission-future"`)},
		{name: "request namespace with wrong case", data: []byte(`"Primitive-submission-request-2026-1"`)},
		{name: "request namespace with leading space", data: []byte(`" primitive-submission-request-2026-1"`)},
		{name: "grant namespace with trailing space", data: []byte(`"primitive-submission-grant-2026-1 "`)},
		{name: "null value", data: []byte(`null`)},
		{name: "numeric value", data: []byte(`1`)},
		{name: "boolean value", data: []byte(`true`)},
		{name: "array value", data: []byte(`[]`)},
		{name: "object value", data: []byte(`{}`)},
		{name: "second value after namespace", data: []byte(`"primitive-submission-request-2026-1" null`)},
		{name: "namespace exceeds attestation ceiling", data: []byte(`"` + strings.Repeat("x", attest.SigningDomainMaximumBytes+1) + `"`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			receiver := preserved
			if err := receiver.UnmarshalJSON(tc.data); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("SigningDomain.UnmarshalJSON(%s) error = %v, want errors.Is %v",
					tc.name, err, core.ErrJSONContract)
			}
			if receiver != preserved {
				t.Fatalf("SigningDomain.UnmarshalJSON(%s) receiver = %v, want preserved %v",
					tc.name, receiver, preserved)
			}
		})
	}
	var nilReceiver *SigningDomain
	if err := nilReceiver.UnmarshalJSON([]byte(`"primitive-submission-request-2026-1"`)); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil SigningDomain.UnmarshalJSON() error = %v, want errors.Is %v",
			err, core.ErrJSONContract)
	}
}

// TestIssueRequestReturnsNeutralOnEveryInvalidIngress proves no signer or
// invalid payload can leave behind a partially authoritative document.
func TestIssueRequestReturnsNeutralOnEveryInvalidIngress(t *testing.T) {
	t.Parallel()

	_, signer := testSigningKey(t, 0x75)
	valid := testRequestPayload(t, grantFixtureRequest{})
	cases := []struct {
		name     string
		issuance RequestIssuance
	}{
		{name: "zero payload", issuance: RequestIssuance{Signer: signer}},
		{name: "nil signer", issuance: RequestIssuance{Payload: valid}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			document, err := IssueRequest(tc.issuance)
			if !errors.Is(err, core.ErrControlPlaneContract) || document != (RequestDocument{}) {
				t.Fatalf("IssueRequest(%s) = (%+v, %v), want zero and errors.Is %v",
					tc.name, document, err, core.ErrControlPlaneContract)
			}
		})
	}
}

func TestRequestNilJSONReceiversRefuseWithoutAuthority(t *testing.T) {
	t.Parallel()

	cases := []struct {
		run  func() error
		name string
	}{
		{name: "request payload", run: func() error {
			var receiver *RequestPayload
			return receiver.UnmarshalJSON([]byte(`{}`))
		}},
		{name: "request document", run: func() error {
			var receiver *RequestDocument
			return receiver.UnmarshalJSON([]byte(`{}`))
		}},
		{name: "request commitment", run: func() error {
			var receiver *RequestCommitment
			return receiver.UnmarshalJSON([]byte(`"` + strings.Repeat("1", 64) + `"`))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.run(); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("nil %s JSON receiver error = %v, want errors.Is %v",
					tc.name, err, core.ErrJSONContract)
			}
		})
	}
}
