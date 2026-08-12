package retrieval

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

const (
	retrievalFixtureChitA = "00000000-0001-7000-8000-000000000001"
	retrievalFixtureChitB = "00000000-0002-7000-8000-000000000002"
)

type retrievalRequestFixture struct {
	private ed25519.PrivateKey
	trusted attest.TrustedKeys
	payload RequestPayload
}

type retrievalRequestFixtureRequest struct {
	Selection    Selection
	VersionPatch uint32
	SignerByte   byte
	NonceByte    byte
}

func TestRetrievalRequestAuthenticatesAllAndSpecificSelections(t *testing.T) {
	t.Parallel()

	fixture := newRetrievalRequestFixture(t, retrievalRequestFixtureRequest{Selection: All()})
	allDocument := issueRetrievalRequestFixture(t, fixture)
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: allDocument.Payload, Envelope: allDocument.Attestation,
		TrustedKeys: fixture.trusted,
	})
	if err != nil || proof.Validate() != nil {
		t.Fatalf("attest.Verify(all retrieval request) = (%v, %v), want valid and nil", proof, err)
	}
	encoded, err := allDocument.MarshalJSON()
	if err != nil {
		t.Fatalf("RequestDocument.MarshalJSON(all) error = %v, want nil", err)
	}
	var decoded RequestDocument
	if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != allDocument {
		t.Fatalf("RequestDocument.UnmarshalJSON(all) = (%v, %v), want exact document and nil", decoded, err)
	}

	sequence, err := chit.NewEntrySequence(1)
	if err != nil {
		t.Fatalf("chit.NewEntrySequence(1) error = %v, want nil", err)
	}
	specific, err := Specific(sequence)
	if err != nil {
		t.Fatalf("retrieval.Specific() error = %v, want nil", err)
	}
	specificFixture := newRetrievalRequestFixture(t, retrievalRequestFixtureRequest{Selection: specific})
	specificDocument := issueRetrievalRequestFixture(t, specificFixture)
	if specificDocument.Payload.Selection != specific {
		t.Fatalf("specific request selection = %v, want %v", specificDocument.Payload.Selection, specific)
	}
	allCommitment, err := CommitRequest(allDocument.Payload)
	if err != nil {
		t.Fatalf("CommitRequest(all) error = %v, want nil", err)
	}
	specificCommitment, err := CommitRequest(specificDocument.Payload)
	if err != nil {
		t.Fatalf("CommitRequest(specific) error = %v, want nil", err)
	}
	if allCommitment == specificCommitment {
		t.Fatalf("all request commitment = %v, specific request commitment = %v, want distinct", allCommitment, specificCommitment)
	}
}

func TestRetrievalSelectionTaggedUnionRefusesEveryContradictoryArm(t *testing.T) {
	t.Parallel()

	sequence, err := chit.NewEntrySequence(1)
	if err != nil {
		t.Fatalf("chit.NewEntrySequence(1) error = %v, want nil", err)
	}
	cases := []struct {
		name      string
		selection Selection
	}{
		{name: "zero selection", selection: Selection{}},
		{
			name:      "all carries sequence",
			selection: Selection{Kind: core.CatalogSelectionAll, Sequence: sequence},
		},
		{
			name:      "specific omits sequence",
			selection: Selection{Kind: core.CatalogSelectionSpecific},
		},
		{
			name:      "selection enum above domain",
			selection: Selection{Kind: core.CatalogSelectionKind(255)},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if gotErr := testCase.selection.Validate(); !errors.Is(gotErr, core.ErrRetrievalContract) {
				t.Fatalf("Selection.Validate() error = %v, want errors.Is %v", gotErr, core.ErrRetrievalContract)
			}
			if encoded, gotErr := testCase.selection.MarshalJSON(); !errors.Is(gotErr, core.ErrJSONContract) || encoded != nil {
				t.Fatalf("Selection.MarshalJSON() = (%q, %v), want nil and errors.Is %v",
					encoded, gotErr, core.ErrJSONContract)
			}
		})
	}
}

func TestRetrievalRequestJSONBoundaryLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := newRetrievalRequestFixture(t, retrievalRequestFixtureRequest{Selection: All()})
	document := issueRetrievalRequestFixture(t, fixture)
	encoded, gotErr := document.MarshalJSON()
	if gotErr != nil {
		t.Fatalf("RequestDocument.MarshalJSON() error = %v, want nil", gotErr)
	}

	t.Run("positive canonical structure and exact extent boundaries preserve facts", func(t *testing.T) {
		t.Parallel()

		sequence, setupErr := chit.NewEntrySequence(1)
		if setupErr != nil {
			t.Fatalf("chit.NewEntrySequence(1) setup error = %v, want nil", setupErr)
		}
		specific, setupErr := Specific(sequence)
		if setupErr != nil {
			t.Fatalf("Specific(1) setup error = %v, want nil", setupErr)
		}
		specificDocument := issueRetrievalRequestFixture(t, newRetrievalRequestFixture(t, retrievalRequestFixtureRequest{Selection: specific}))
		specificEncoded, setupErr := specificDocument.MarshalJSON()
		if setupErr != nil {
			t.Fatalf("specific RequestDocument.MarshalJSON() setup error = %v, want nil", setupErr)
		}
		reordered := marshalReorderedRetrievalRequest(t, document)
		cases := []struct {
			name string
			data []byte
			want RequestDocument
		}{
			{name: "canonical all-selection document", data: encoded, want: document},
			{name: "canonical specific-selection document", data: specificEncoded, want: specificDocument},
			{name: "leading whitespace", data: append([]byte(" \n\t"), encoded...), want: document},
			{name: "trailing whitespace", data: append(append([]byte(nil), encoded...), ' ', '\n', '\t'), want: document},
			{name: "both-side whitespace", data: append(append([]byte(" \n"), encoded...), '\n', ' '), want: document},
			{name: "top-level members reordered", data: reordered, want: document},
			{name: "one below document ceiling", data: retrievalPadJSON(encoded, RequestDocumentJSONMaximumBytes-1), want: document},
			{name: "at document ceiling", data: retrievalPadJSON(encoded, RequestDocumentJSONMaximumBytes), want: document},
			{name: "one trailing carriage return", data: append(append([]byte(nil), encoded...), '\r'), want: document},
			{name: "multiple legal json whitespace bytes", data: append(append([]byte("\t\r\n "), encoded...), " \n\r\t"...), want: document},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				var got RequestDocument
				gotErr := got.UnmarshalJSON(tc.data)
				if gotErr != nil || got != tc.want {
					t.Fatalf("RequestDocument.UnmarshalJSON() = (%v, %v), want (%v, nil)", got, gotErr, tc.want)
				}
			})
		}
	})

	t.Run("negative malformed missing duplicate and type-wrong documents reject", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			data []byte
		}{
			{name: "empty document", data: nil},
			{name: "whitespace-only document", data: []byte(" \n\t")},
			{name: "null document", data: []byte("null")},
			{name: "array instead of structure", data: []byte("[]")},
			{name: "string instead of structure", data: []byte(`"request"`)},
			{name: "number instead of structure", data: []byte("1")},
			{name: "boolean instead of structure", data: []byte("true")},
			{name: "truncated opening brace", data: []byte("{")},
			{name: "truncated inside first member", data: encoded[:len(encoded)/2]},
			{name: "truncated before final brace", data: encoded[:len(encoded)-1]},
			{name: "trailing object", data: append(append([]byte(nil), encoded...), '{', '}')},
			{name: "two concatenated documents", data: append(append([]byte(nil), encoded...), encoded...)},
			{name: "unknown top-level member", data: bytes.Replace(encoded, []byte(`{"payload"`), []byte(`{"unknown":1,"payload"`), 1)},
			{name: "duplicate payload member", data: bytes.Replace(encoded, []byte(`{"payload":`), []byte(`{"payload":null,"payload":`), 1)},
			{name: "missing both required members", data: []byte("{}")},
			{name: "missing payload member", data: []byte(`{"attestation":null}`)},
			{name: "missing attestation member", data: []byte(`{"payload":null}`)},
			{name: "payload has wrong scalar type", data: []byte(`{"payload":1,"attestation":null}`)},
			{name: "attestation has wrong scalar type", data: []byte(`{"payload":null,"attestation":1}`)},
			{name: "one above document ceiling", data: retrievalPadJSON(encoded, RequestDocumentJSONMaximumBytes+1)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := document
				gotErr := got.UnmarshalJSON(tc.data)
				if !errors.Is(gotErr, core.ErrJSONContract) || got != document {
					t.Fatalf("RequestDocument.UnmarshalJSON() = (%v, %v), want preserved receiver and errors.Is %v", got, gotErr, core.ErrJSONContract)
				}
			})
		}
	})

	t.Run("neutral rejection never clears an authenticated receiver", func(t *testing.T) {
		t.Parallel()

		got := document
		gotErr := got.UnmarshalJSON(nil)
		if !errors.Is(gotErr, core.ErrJSONContract) || got != document {
			t.Fatalf("RequestDocument.UnmarshalJSON(nil) = (%v, %v), want preserved receiver and errors.Is %v", got, gotErr, core.ErrJSONContract)
		}
	})
}

func TestRetrievalRequestSignatureRefusesWrongKeyAndEveryMutableSignedPayloadFact(t *testing.T) {
	t.Parallel()

	fixture := newRetrievalRequestFixture(t, retrievalRequestFixtureRequest{Selection: All()})
	document := issueRetrievalRequestFixture(t, fixture)
	otherSigner := newRetrievalRequestFixture(t, retrievalRequestFixtureRequest{
		Selection: All(), SignerByte: 0x52,
	})
	if proof, gotErr := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: document.Payload, Envelope: document.Attestation, TrustedKeys: otherSigner.trusted,
	}); !errors.Is(gotErr, core.ErrAttestVerification) || proof.Validate() == nil {
		t.Fatalf("attest.Verify(wrong key) = (%v, %v), want invalid proof and errors.Is %v",
			proof, gotErr, core.ErrAttestVerification)
	}

	sequence, gotErr := chit.NewEntrySequence(1)
	if gotErr != nil {
		t.Fatalf("chit.NewEntrySequence(1) setup error = %v, want nil", gotErr)
	}
	specific, gotErr := Specific(sequence)
	if gotErr != nil {
		t.Fatalf("Specific(1) setup error = %v, want nil", gotErr)
	}
	otherBuild := newRetrievalRequestFixture(t, retrievalRequestFixtureRequest{Selection: All(), VersionPatch: 2})
	otherNonce := newRetrievalRequestFixture(t, retrievalRequestFixtureRequest{Selection: All(), NonceByte: 2})
	cases := []struct {
		mutate func(*RequestPayload)
		name   string
	}{
		{name: "chit identity", mutate: func(value *RequestPayload) { value.Chit = mustRetrievalChitID(t, retrievalFixtureChitB) }},
		{name: "all-or-specific selection", mutate: func(value *RequestPayload) { value.Selection = specific }},
		{name: "installed build identity", mutate: func(value *RequestPayload) { value.Build = otherBuild.payload.Build }},
		{name: "request nonce", mutate: func(value *RequestPayload) { value.Nonce = otherNonce.payload.Nonce }},
	}
	beforeCommitment, gotErr := CommitRequest(document.Payload)
	if gotErr != nil {
		t.Fatalf("CommitRequest(authentic setup) error = %v, want nil", gotErr)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tampered := document
			tc.mutate(&tampered.Payload)
			gotCommitment, commitmentErr := CommitRequest(tampered.Payload)
			if commitmentErr != nil || gotCommitment == beforeCommitment {
				t.Fatalf("CommitRequest(mutated %s) = (%v, %v), want distinct valid commitment", tc.name, gotCommitment, commitmentErr)
			}
			proof, gotErr := attest.Verify(attest.VerifyRequest[SigningDomain]{
				Body: tampered.Payload, Envelope: tampered.Attestation, TrustedKeys: fixture.trusted,
			})
			if !errors.Is(gotErr, core.ErrAttestVerification) || proof.Validate() == nil {
				t.Fatalf("attest.Verify(mutated %s) = (%v, %v), want invalid proof and errors.Is %v",
					tc.name, proof, gotErr, core.ErrAttestVerification)
			}
		})
	}

	invalidRevision := document.Payload
	invalidRevision.Revision = controlwire.RevisionUnknown
	if gotErr := invalidRevision.Validate(); !errors.Is(gotErr, core.ErrRetrievalContract) {
		t.Fatalf("RequestPayload.Validate(unpublished singleton revision) error = %v, want errors.Is %v", gotErr, core.ErrRetrievalContract)
	}
}

func marshalReorderedRetrievalRequest(t *testing.T, document RequestDocument) []byte {
	t.Helper()

	encoded, gotErr := core.MarshalCanonicalJSONDocument(struct {
		Attestation attest.Envelope[SigningDomain] `json:"attestation"`
		Payload     RequestPayload                 `json:"payload"`
	}{Attestation: document.Attestation, Payload: document.Payload})
	if gotErr != nil {
		t.Fatalf("core.MarshalCanonicalJSONDocument(reordered request) error = %v, want nil", gotErr)
	}
	return encoded
}

func retrievalPadJSON(document []byte, wantBytes int) []byte {
	if len(document) >= wantBytes {
		return append([]byte(nil), document...)
	}
	return append(append([]byte(nil), document...), bytes.Repeat([]byte{' '}, wantBytes-len(document))...)
}

func newRetrievalRequestFixture(
	t testing.TB,
	request retrievalRequestFixtureRequest,
) retrievalRequestFixture {
	t.Helper()

	if request.SignerByte == 0 {
		request.SignerByte = 0x51
	}
	if request.NonceByte == 0 {
		request.NonceByte = 1
	}
	if request.VersionPatch == 0 {
		request.VersionPatch = 1
	}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{request.SignerByte}, ed25519.SeedSize))
	public, err := core.NewEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("core.NewEd25519PublicKey() error = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{public}})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}
	commit, err := core.ParseBuildCommit(strings.Repeat("a", 40))
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v, want nil", err)
	}
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Version: core.NewReleaseVersion(2026, 0, request.VersionPatch), Commit: commit,
		Platform: core.Platform{
			OperatingSystem: core.OperatingSystemLinux,
			Architecture:    core.CPUArchitectureAMD64,
		},
		Offering: core.OfferingWitness,
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity() error = %v, want nil", err)
	}
	rawNonce := [controlwire.NonceBytes]byte{}
	rawNonce[0] = request.NonceByte
	nonce, err := controlwire.NewRequestNonce(rawNonce)
	if err != nil {
		t.Fatalf("controlwire.NewRequestNonce() error = %v, want nil", err)
	}
	return retrievalRequestFixture{
		private: private, trusted: trusted,
		payload: RequestPayload{
			Build: build, Chit: mustRetrievalChitID(t, retrievalFixtureChitA),
			Selection: request.Selection, Revision: controlwire.Revision2026V1, Nonce: nonce,
		},
	}
}

func issueRetrievalRequestFixture(t testing.TB, fixture retrievalRequestFixture) RequestDocument {
	t.Helper()
	document, err := IssueRequest(RequestIssuance{Signer: fixture.private, Payload: fixture.payload})
	if err != nil {
		t.Fatalf("IssueRequest() error = %v, want nil", err)
	}
	return document
}

func mustRetrievalChitID(t testing.TB, value string) chit.ChitID {
	t.Helper()
	identity, err := chit.ParseChitID(value)
	if err != nil {
		t.Fatalf("chit.ParseChitID(%q) error = %v, want nil", value, err)
	}
	return identity
}
