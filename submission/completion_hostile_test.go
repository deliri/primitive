package submission

import (
	"bytes"
	"context"
	"crypto"
	json "encoding/json/v2"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"encoding/json/jsontext"
	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

type completionFixture struct {
	deviceSigner  crypto.Signer
	transfer      objectstore.Transfer
	request       RequestPayload
	grantDocument GrantDocument
	grant         VerifiedGrant
	grantKeys     attest.TrustedKeys
	deviceKeys    attest.TrustedKeys
	nonce         controlwire.RequestNonce
}

func TestCompletionAuthenticationLayerTriadBindsRealTransferToRepresentativeOpaqueOfferings(t *testing.T) {
	t.Parallel()

	for index, offering := range []core.Offering{
		submissionOffering(t, 1),
		submissionOffering(t, 127),
		submissionOffering(t, 255),
	} {
		t.Run(offering.String(), func(t *testing.T) {
			t.Parallel()

			fixture := newCompletionFixture(t, offering, []byte(`{"proof":"source-free"}`), byte(index)+0x10)
			projection, err := IssueCompletion(CompletionIssuance{
				Signer: fixture.deviceSigner, Request: fixture.request,
				Grant: fixture.grant, Transfer: fixture.transfer, Nonce: fixture.nonce,
			})
			if err != nil {
				t.Fatalf("IssueCompletion(%v) error = %v, want nil", offering, err)
			}
			document := receiveCompletionProjection(t, projection)
			verified, err := VerifyCompletion(CompletionExpectation{
				Document: document, Request: fixture.request,
				Grant:     fixture.grantDocument,
				GrantKeys: fixture.grantKeys, CompletionKeys: fixture.deviceKeys, Nonce: fixture.nonce,
			})
			if err != nil {
				t.Fatalf("VerifyCompletion(%v) error = %v, want nil", offering, err)
			}
			got, err := verified.Payload()
			if err != nil {
				t.Fatalf("VerifiedCompletion.Payload(%v) error = %v, want nil", offering, err)
			}
			if got.Build != fixture.request.Build || got.Request != fixture.grantDocument.Payload.Request ||
				got.Nonce != fixture.nonce ||
				got.Capability != fixture.grantDocument.Payload.Capability ||
				got.Authorization != fixture.grantDocument.Payload.Authorization ||
				got.Evidence.Provider() != objectstore.ProviderGoogleCloudStorage ||
				got.Evidence.Direction() != objectstore.DirectionUpload ||
				got.Evidence.Bytes() != fixture.request.Declaration.Extent ||
				got.Evidence.SHA256() != fixture.request.Declaration.SHA256 ||
				got.Evidence.CRC32C() != fixture.request.Declaration.CRC32C {
				t.Fatalf("completion payload = %+v, want exact request, grant, and transfer closure", got)
			}
		})
	}
}

func TestCompletionProjectionCarriesEveryAllowedFactAndNoUnownedMaterial(t *testing.T) {
	t.Parallel()

	content := []byte("completion source bytes must never cross the evidence boundary")
	fixture := newCompletionFixture(t, submissionOffering(t, 2), content, 0x10)
	projection, err := IssueCompletion(CompletionIssuance{
		Signer: fixture.deviceSigner, Request: fixture.request,
		Grant: fixture.grant, Transfer: fixture.transfer, Nonce: fixture.nonce,
	})
	if err != nil {
		t.Fatalf("IssueCompletion() error = %v, want nil", err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("CompletionProjection.MarshalJSON() error = %v, want nil", err)
	}
	forbidden := []struct {
		name  string
		value []byte
	}{
		{name: "source bytes", value: content},
		{name: "bearer object address", value: []byte(testCapabilityObjectPrefix)},
		{name: "bearer query and signed headers", value: []byte(testCapabilityQuery)},
		{name: "manifest object name", value: []byte(fixture.request.Manifest.Name.String())},
	}
	for _, tc := range forbidden {
		if len(tc.value) == 0 {
			t.Fatalf("%s exclusion fixture extent = 0, want a load-bearing marker", tc.name)
		}
		if bytes.Contains(encoded, tc.value) {
			t.Fatalf("CompletionProjection.MarshalJSON() disclosed %s", tc.name)
		}
	}
	encoded, err = core.EncodeValidatedJSON(projection, core.DefaultStrictJSONLimits())
	if err != nil {
		t.Fatalf("EncodeValidatedJSON(CompletionProjection) error = %v, want nil", err)
	}
	if err := projection.ValidateJSONProjection(encoded, core.DefaultStrictJSONLimits()); err != nil {
		t.Fatalf("ValidateJSONProjection(exact encoded bytes) error = %v, want nil", err)
	}
	if got, gotErr := core.EncodeValidatedJSON(CompletionProjection{}, core.DefaultStrictJSONLimits()); got != nil ||
		!errors.Is(gotErr, core.ErrJSONContract) {
		t.Fatalf("EncodeValidatedJSON(zero CompletionProjection) = (%d bytes, %v), want nil and %v",
			len(got), gotErr, core.ErrJSONContract)
	}
	document := receiveCompletionProjection(t, projection)
	payload := document.Payload
	if payload.Build != fixture.request.Build || payload.Nonce != fixture.nonce ||
		payload.Request != fixture.grantDocument.Payload.Request ||
		payload.Capability != fixture.grantDocument.Payload.Capability ||
		payload.Authorization != fixture.grantDocument.Payload.Authorization ||
		payload.Evidence.Provider() != fixture.transfer.Provider() ||
		payload.Evidence.Direction() != objectstore.DirectionUpload ||
		payload.Evidence.Bytes() != fixture.request.Declaration.Extent ||
		payload.Evidence.SHA256() != fixture.request.Declaration.SHA256 ||
		payload.Evidence.CRC32C() != fixture.request.Declaration.CRC32C {
		t.Fatalf("CompletionProjection allowed facts = %+v, want exact request, grant, and transfer projection", payload)
	}
}

func TestCompletionIssuanceLayerTriadRefusesEveryCrossAgreementSubstitution(t *testing.T) {
	t.Parallel()

	base := newCompletionFixture(t, submissionOffering(t, 2), []byte("original proof"), 0x10)
	otherContent := newCompletionFixture(t, submissionOffering(t, 2), []byte("different proof"), 0x10)
	otherObject := newCompletionFixture(t, submissionOffering(t, 2), []byte("original proof"), 0x30)
	otherOffering := newCompletionFixture(t, submissionOffering(t, 1), []byte("original proof"), 0x20)
	cases := []struct {
		signer   crypto.Signer
		name     string
		transfer objectstore.Transfer
		request  RequestPayload
		grant    VerifiedGrant
		nonce    controlwire.RequestNonce
	}{
		{name: "zero request", grant: base.grant, transfer: base.transfer, signer: base.deviceSigner},
		{name: "zero grant", request: base.request, transfer: base.transfer, signer: base.deviceSigner},
		{name: "zero transfer", request: base.request, grant: base.grant, signer: base.deviceSigner},
		{name: "nil signer", request: base.request, grant: base.grant, transfer: base.transfer},
		{name: "different content transfer", request: base.request, grant: base.grant, transfer: otherContent.transfer, signer: base.deviceSigner},
		{name: "same bytes uploaded through another capability", request: base.request, grant: base.grant, transfer: otherObject.transfer, signer: base.deviceSigner},
		{name: "different content request", request: otherContent.request, grant: base.grant, transfer: base.transfer, signer: base.deviceSigner},
		{name: "different offering request", request: otherOffering.request, grant: base.grant, transfer: base.transfer, signer: base.deviceSigner},
		{name: "different grant", request: base.request, grant: otherContent.grant, transfer: base.transfer, signer: base.deviceSigner},
		{name: "different offering grant", request: base.request, grant: otherOffering.grant, transfer: base.transfer, signer: base.deviceSigner},
		{name: "grant request with other transfer", request: otherContent.request, grant: otherContent.grant, transfer: base.transfer, signer: base.deviceSigner},
		{name: "submission request nonce reused by completion", request: base.request, grant: base.grant, transfer: base.transfer, signer: base.deviceSigner, nonce: base.request.Nonce},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			nonce := tc.nonce
			if nonce == (controlwire.RequestNonce{}) {
				nonce = base.nonce
			}
			got, err := IssueCompletion(CompletionIssuance{
				Signer: tc.signer, Request: tc.request, Grant: tc.grant, Transfer: tc.transfer, Nonce: nonce,
			})
			if !errors.Is(err, core.ErrControlPlaneContract) || got != (CompletionProjection{}) {
				t.Fatalf("IssueCompletion() = (%v, %v), want zero and errors.Is %v",
					got, err, core.ErrControlPlaneContract)
			}
		})
	}
}

// TestCompletionVerificationRefusesEveryAuthenticCrossAgreementSubstitution
// proves independently valid requests, grants, transfers, and signatures
// cannot be recombined after crossing the wire.
func TestCompletionVerificationLayerTriadRefusesEveryAuthenticCrossAgreementSubstitution(t *testing.T) {
	t.Parallel()

	base := newCompletionFixture(t, submissionOffering(t, 2), []byte("original proof"), 0x10)
	otherContent := newCompletionFixture(t, submissionOffering(t, 2), []byte("different proof"), 0x10)
	otherOffering := newCompletionFixture(t, submissionOffering(t, 1), []byte("original proof"), 0x20)
	baseDocument := receiveIssuedCompletion(t, base)
	otherContentDocument := receiveIssuedCompletion(t, otherContent)
	otherOfferingDocument := receiveIssuedCompletion(t, otherOffering)
	cases := []struct {
		want   error
		mutate func(*CompletionExpectation)
		name   string
	}{
		{name: "completion document absent", mutate: func(value *CompletionExpectation) {
			value.Document = CompletionDocument{}
		}, want: core.ErrControlPlaneContract},
		{name: "request absent", mutate: func(value *CompletionExpectation) {
			value.Request = RequestPayload{}
		}, want: core.ErrControlPlaneContract},
		{name: "grant absent", mutate: func(value *CompletionExpectation) {
			value.Grant = GrantDocument{}
		}, want: core.ErrControlPlaneContract},
		{name: "grant keys absent", mutate: func(value *CompletionExpectation) {
			value.GrantKeys = attest.TrustedKeys{}
		}, want: core.ErrControlPlaneContract},
		{name: "completion keys absent", mutate: func(value *CompletionExpectation) {
			value.CompletionKeys = attest.TrustedKeys{}
		}, want: core.ErrControlPlaneContract},
		{name: "other content request", mutate: func(value *CompletionExpectation) {
			value.Request = otherContent.request
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "other offering request", mutate: func(value *CompletionExpectation) {
			value.Request = otherOffering.request
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "other content grant", mutate: func(value *CompletionExpectation) {
			value.Grant = otherContent.grantDocument
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "other offering grant", mutate: func(value *CompletionExpectation) {
			value.Grant = otherOffering.grantDocument
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "other content completion", mutate: func(value *CompletionExpectation) {
			value.Document = otherContentDocument
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "other offering completion", mutate: func(value *CompletionExpectation) {
			value.Document = otherOfferingDocument
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "other authority grant keys", mutate: func(value *CompletionExpectation) {
			value.GrantKeys = otherOffering.grantKeys
		}, want: core.ErrAttestVerification},
		{name: "other device completion keys", mutate: func(value *CompletionExpectation) {
			value.CompletionKeys = otherOffering.deviceKeys
		}, want: core.ErrAttestVerification},
		{name: "other completion nonce", mutate: func(value *CompletionExpectation) {
			value.Nonce = otherOffering.nonce
		}, want: core.ErrControlPlaneResponseBinding},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			expectation := CompletionExpectation{
				Document: baseDocument, Request: base.request, Grant: base.grantDocument,
				GrantKeys: base.grantKeys, CompletionKeys: base.deviceKeys, Nonce: base.nonce,
			}
			tc.mutate(&expectation)
			got, err := VerifyCompletion(expectation)
			if !errors.Is(err, tc.want) || got != (VerifiedCompletion{}) {
				t.Fatalf("VerifyCompletion(%s) = (%v, %v), want zero and errors.Is %v",
					tc.name, got, err, tc.want)
			}
		})
	}
}

func TestCompletionAuthenticationLayerTriadZeroValuesNeverProjectEvidence(t *testing.T) {
	t.Parallel()

	projection, err := IssueCompletion(CompletionIssuance{})
	if !errors.Is(err, core.ErrControlPlaneContract) || projection != (CompletionProjection{}) {
		t.Fatalf("IssueCompletion(zero) = (%v, %v), want zero and errors.Is %v",
			projection, err, core.ErrControlPlaneContract)
	}
	verified, err := VerifyCompletion(CompletionExpectation{})
	if !errors.Is(err, core.ErrControlPlaneContract) || verified != (VerifiedCompletion{}) {
		t.Fatalf("VerifyCompletion(zero) = (%v, %v), want zero and errors.Is %v",
			verified, err, core.ErrControlPlaneContract)
	}
	payload, err := (VerifiedCompletion{}).Payload()
	if !errors.Is(err, core.ErrControlPlaneContract) || payload != (CompletionPayload{}) {
		t.Fatalf("VerifiedCompletion{}.Payload() = (%v, %v), want zero and errors.Is %v",
			payload, err, core.ErrControlPlaneContract)
	}
}

func TestCompletionDocumentJSONLayerTriadClosesFramingShapeAndExactByteBoundaries(t *testing.T) {
	t.Parallel()

	fixture := newCompletionFixture(t, submissionOffering(t, 2), []byte("completion document JSON proof"), 0x10)
	document := receiveIssuedCompletion(t, fixture)
	encoded, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("CompletionDocument.MarshalJSON() error = %v, want nil", err)
	}
	reordered, err := json.Marshal(struct {
		Payload     CompletionPayload              `json:"payload"`
		Attestation attest.Envelope[SigningDomain] `json:"attestation"`
	}{Attestation: document.Attestation, Payload: document.Payload})
	if err != nil {
		t.Fatalf("json.Marshal(reordered completion document) error = %v, want nil", err)
	}
	indented := jsontext.Value(bytes.Clone(encoded))
	if err := indented.Indent(jsontext.WithIndent("  ")); err != nil {
		t.Fatalf("json.Indent(completion document) error = %v, want nil", err)
	}
	valid := []struct {
		name string
		data []byte
	}{
		{name: "canonical document", data: encoded},
		{name: "reordered document members", data: reordered},
		{name: "indented document", data: []byte(indented)},
		{name: "leading space", data: append([]byte(" "), encoded...)},
		{name: "trailing newline", data: append(bytes.Clone(encoded), '\n')},
		{name: "carriage return framing", data: append(append([]byte("\r"), encoded...), '\r')},
		{name: "mixed outer whitespace", data: append(append([]byte("\t\r\n"), encoded...), ' ', '\t')},
		{name: "half document ceiling", data: leftPadJSON(encoded, CompletionDocumentJSONMaximumBytes/2)},
		{name: "one below document ceiling", data: leftPadJSON(encoded, CompletionDocumentJSONMaximumBytes-1)},
		{name: "exact document ceiling", data: leftPadJSON(encoded, CompletionDocumentJSONMaximumBytes)},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got CompletionDocument
			gotErr := got.UnmarshalJSON(tc.data)
			if gotErr != nil || got != document {
				t.Fatalf("CompletionDocument.UnmarshalJSON(%s) = (%v, %v), want exact %v and nil",
					tc.name, got, gotErr, document)
			}
		})
	}
	unknown := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"future":true}`)...)
	duplicatePayload := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"payload":null}`)...)
	duplicateAttestation := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"attestation":null}`)...)
	invalid := []struct {
		name string
		data []byte
	}{
		{name: "empty input"},
		{name: "whitespace only", data: []byte(" \t\r\n")},
		{name: "null root", data: []byte(`null`)},
		{name: "boolean root", data: []byte(`true`)},
		{name: "number root", data: []byte(`1`)},
		{name: "string root", data: []byte(`"completion"`)},
		{name: "array root", data: []byte(`[]`)},
		{name: "empty object", data: []byte(`{}`)},
		{name: "unknown member", data: unknown},
		{name: "duplicate payload", data: duplicatePayload},
		{name: "duplicate attestation", data: duplicateAttestation},
		{name: "missing payload", data: []byte(`{"attestation":null}`)},
		{name: "missing attestation", data: []byte(`{"payload":null}`)},
		{name: "payload wrong type", data: []byte(`{"payload":true,"attestation":null}`)},
		{name: "attestation wrong type", data: []byte(`{"payload":null,"attestation":true}`)},
		{name: "open object", data: []byte(`{`)},
		{name: "open array", data: []byte(`[`)},
		{name: "truncated document", data: encoded[:len(encoded)-1]},
		{name: "half truncated document", data: encoded[:len(encoded)/2]},
		{name: "two documents", data: append(bytes.Clone(encoded), encoded...)},
		{name: "trailing scalar", data: append(bytes.Clone(encoded), []byte(` 0`)...)},
		{name: "one above document ceiling", data: leftPadJSON(encoded, CompletionDocumentJSONMaximumBytes+1)},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := document
			gotErr := got.UnmarshalJSON(tc.data)
			if !errors.Is(gotErr, core.ErrJSONContract) || got != document {
				t.Fatalf("CompletionDocument.UnmarshalJSON(%s) = (%v, %v), want preserved and errors.Is %v",
					tc.name, got, gotErr, core.ErrJSONContract)
			}
		})
	}
	var receiver *CompletionDocument
	if err := receiver.UnmarshalJSON(encoded); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil CompletionDocument.UnmarshalJSON() error = %v, want errors.Is %v", err, core.ErrJSONContract)
	}
}

func TestCompletionPayloadJSONLayerTriadClosesFramingShapeAndExactByteBoundaries(t *testing.T) {
	t.Parallel()

	fixture := newCompletionFixture(t, submissionOffering(t, 2), []byte("completion payload JSON proof"), 0x10)
	payload := receiveIssuedCompletion(t, fixture).Payload
	encoded, err := payload.MarshalJSON()
	if err != nil {
		t.Fatalf("CompletionPayload.MarshalJSON() error = %v, want nil", err)
	}
	reordered, err := json.Marshal(struct {
		Build         core.BuildIdentity                     `json:"build"`
		Evidence      objectstore.TransferEvidence           `json:"evidence"`
		Authorization controlwire.AuthorityNonce             `json:"authorization_nonce"`
		Capability    objectstore.UploadCapabilityCommitment `json:"capability_commitment"`
		Nonce         controlwire.RequestNonce               `json:"request_nonce"`
		Request       RequestCommitment                      `json:"request_commitment"`
	}{
		Evidence: payload.Evidence, Authorization: payload.Authorization,
		Capability: payload.Capability, Nonce: payload.Nonce,
		Request: payload.Request, Build: payload.Build,
	})
	if err != nil {
		t.Fatalf("json.Marshal(reordered completion payload) error = %v, want nil", err)
	}
	indented := jsontext.Value(bytes.Clone(encoded))
	if err := indented.Indent(jsontext.WithIndent("  ")); err != nil {
		t.Fatalf("json.Indent(completion payload) error = %v, want nil", err)
	}
	valid := []struct {
		name string
		data []byte
	}{
		{name: "canonical payload", data: encoded},
		{name: "reordered payload members", data: reordered},
		{name: "indented payload", data: []byte(indented)},
		{name: "leading space", data: append([]byte(" "), encoded...)},
		{name: "trailing newline", data: append(bytes.Clone(encoded), '\n')},
		{name: "carriage return framing", data: append(append([]byte("\r"), encoded...), '\r')},
		{name: "mixed outer whitespace", data: append(append([]byte("\t\r\n"), encoded...), ' ', '\t')},
		{name: "half payload ceiling", data: leftPadJSON(encoded, CompletionPayloadJSONMaximumBytes/2)},
		{name: "one below payload ceiling", data: leftPadJSON(encoded, CompletionPayloadJSONMaximumBytes-1)},
		{name: "exact payload ceiling", data: leftPadJSON(encoded, CompletionPayloadJSONMaximumBytes)},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got CompletionPayload
			gotErr := got.UnmarshalJSON(tc.data)
			if gotErr != nil || got != payload {
				t.Fatalf("CompletionPayload.UnmarshalJSON(%s) = (%v, %v), want exact %v and nil",
					tc.name, got, gotErr, payload)
			}
		})
	}
	unknown := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"future":true}`)...)
	duplicateBuild := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"build":null}`)...)
	duplicateEvidence := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"evidence":null}`)...)
	duplicateNonce := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"request_nonce":null}`)...)
	invalid := []struct {
		name string
		data []byte
	}{
		{name: "empty input"},
		{name: "whitespace only", data: []byte(" \t\r\n")},
		{name: "null root", data: []byte(`null`)},
		{name: "boolean root", data: []byte(`true`)},
		{name: "number root", data: []byte(`1`)},
		{name: "string root", data: []byte(`"completion"`)},
		{name: "array root", data: []byte(`[]`)},
		{name: "empty object", data: []byte(`{}`)},
		{name: "unknown member", data: unknown},
		{name: "duplicate build", data: duplicateBuild},
		{name: "duplicate evidence", data: duplicateEvidence},
		{name: "duplicate nonce", data: duplicateNonce},
		{name: "missing build", data: []byte(`{"request_commitment":null}`)},
		{name: "missing nonce", data: []byte(`{"build":null,"request_commitment":null}`)},
		{name: "null nonce", data: []byte(`{"request_nonce":null}`)},
		{name: "wrong-type nonce", data: []byte(`{"request_nonce":true}`)},
		{name: "missing request commitment", data: []byte(`{"build":null}`)},
		{name: "build wrong type", data: []byte(`{"build":true}`)},
		{name: "evidence wrong type", data: []byte(`{"evidence":true}`)},
		{name: "open object", data: []byte(`{`)},
		{name: "open array", data: []byte(`[`)},
		{name: "truncated payload", data: encoded[:len(encoded)-1]},
		{name: "half truncated payload", data: encoded[:len(encoded)/2]},
		{name: "two payloads", data: append(bytes.Clone(encoded), encoded...)},
		{name: "trailing scalar", data: append(bytes.Clone(encoded), []byte(` 0`)...)},
		{name: "one above payload ceiling", data: leftPadJSON(encoded, CompletionPayloadJSONMaximumBytes+1)},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := payload
			gotErr := got.UnmarshalJSON(tc.data)
			if !errors.Is(gotErr, core.ErrJSONContract) || got != payload {
				t.Fatalf("CompletionPayload.UnmarshalJSON(%s) = (%v, %v), want preserved and errors.Is %v",
					tc.name, got, gotErr, core.ErrJSONContract)
			}
		})
	}
	var receiver *CompletionPayload
	if err := receiver.UnmarshalJSON(encoded); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil CompletionPayload.UnmarshalJSON() error = %v, want errors.Is %v", err, core.ErrJSONContract)
	}
}

func receiveIssuedCompletion(t testing.TB, fixture completionFixture) CompletionDocument {
	t.Helper()

	projection, err := IssueCompletion(CompletionIssuance{
		Signer: fixture.deviceSigner, Request: fixture.request,
		Grant: fixture.grant, Transfer: fixture.transfer, Nonce: fixture.nonce,
	})
	if err != nil {
		t.Fatalf("IssueCompletion() error = %v, want nil", err)
	}
	return receiveCompletionProjection(t, projection)
}

func receiveCompletionProjection(t testing.TB, projection CompletionProjection) CompletionDocument {
	t.Helper()

	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("CompletionProjection.MarshalJSON() error = %v, want nil", err)
	}
	var document CompletionDocument
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("json.Unmarshal(CompletionDocument) error = %v, want nil", err)
	}
	return document
}

func newCompletionFixture(t testing.TB, offering core.Offering, content []byte, seed byte) completionFixture {
	t.Helper()

	grantFixture := newGrantFixture(t, grantFixtureRequest{
		content: content, offering: offering,
		objectName:        "proof-" + strconv.Itoa(int(seed)) + ".json",
		requestNonceByte:  seed + 0x20,
		authorityByte:     seed + 0x40,
		authorizationByte: seed + 0x60,
	})
	verifiedGrant, err := VerifyGrant(GrantExpectation{
		Document: grantFixture.document, Request: grantFixture.request,
		ObservedAt:  temporal.InstantFromNanoseconds(testGrantIssuedAt),
		TrustedKeys: grantFixture.trusted,
	})
	if err != nil {
		t.Fatalf("VerifyGrant() error = %v, want nil", err)
	}
	devicePublic, deviceSigner := testSigningKey(t, seed+0x70)
	deviceKeys, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{devicePublic},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys(device) error = %v, want nil", err)
	}
	nonceBytes := [core.SHA256DigestBytes]byte{seed + 0x21}
	nonce, err := controlwire.NewRequestNonce(nonceBytes)
	if err != nil {
		t.Fatalf("controlwire.NewRequestNonce(completion) error = %v, want nil", err)
	}
	return completionFixture{
		request: grantFixture.request, grant: verifiedGrant,
		grantDocument: grantFixture.document, grantKeys: grantFixture.trusted,
		deviceSigner: deviceSigner, deviceKeys: deviceKeys,
		nonce:    nonce,
		transfer: completionUpload(t, verifiedGrant, grantFixture.request, content),
	}
}

func completionUpload(
	t testing.TB,
	grant VerifiedGrant,
	request RequestPayload,
	content []byte,
) objectstore.Transfer {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, incoming *http.Request) {
		got, err := io.ReadAll(io.LimitReader(incoming.Body, int64(len(content))+1))
		if err != nil {
			t.Errorf("provider body read error = %v, want nil", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !bytes.Equal(got, content) {
			t.Errorf("provider body = %q, want %q", got, content)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writer.Header().Set("x-goog-generation", "7")
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	serverAddress := server.Listener.Addr().String()
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network string, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, serverAddress)
	}
	t.Cleanup(transport.CloseIdleConnections)
	exchangeClient, err := exchange.NewClient(&http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	client, err := objectstore.NewClient(exchangeClient)
	if err != nil {
		t.Fatalf("objectstore.NewClient() error = %v, want nil", err)
	}
	capability, err := grant.Capability()
	if err != nil {
		t.Fatalf("VerifiedGrant.Capability() error = %v, want nil", err)
	}
	transfer, err := objectstore.Upload(context.Background(), client, objectstore.UploadCapabilityRequest{
		Source: bytes.NewReader(content), ContentType: request.Declaration.ContentType,
		Capability: capability, Integrity: request.Declaration.Integrity(),
		Policy: completionObjectstorePolicy(t),
	})
	if err != nil {
		t.Fatalf("objectstore.Upload() error = %v, want nil", err)
	}
	return transfer
}

func completionObjectstorePolicy(t testing.TB) objectstore.Policy {
	t.Helper()

	operation, err := temporal.DurationFromSeconds(10)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(operation) error = %v, want nil", err)
	}
	attempt, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(attempt) error = %v, want nil", err)
	}
	errorLimit, err := core.NewByteCount(4 << 10)
	if err != nil {
		t.Fatalf("core.NewByteCount(error limit) error = %v, want nil", err)
	}
	policy := objectstore.Policy{
		OperationTimeout: operation, AttemptTimeout: attempt, ErrorBodyLimit: errorLimit,
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("objectstore.Policy.Validate() error = %v, want nil", err)
	}
	return policy
}
