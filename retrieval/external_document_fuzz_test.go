package retrieval

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type retrievalExternalDoor[T any] struct {
	Seed         T
	Mutations    []T
	MaximumBytes uint64
	Marshal      func(T) ([]byte, error)
	Unmarshal    func(*T, []byte) error
	Validate     func(T) error
	Authenticate func(T, bool) error
}

func FuzzRequestPayloadExternalDecoderAndVerifier(f *testing.F) {
	fixture := newRetrievalRequestFixture(f, retrievalRequestFixtureRequest{Selection: All()})
	document := issueRetrievalRequestFixture(f, fixture)
	other := newRetrievalRequestFixture(f, retrievalRequestFixtureRequest{Selection: All(), NonceByte: 2})
	mutation := fixture.payload
	mutation.Nonce = other.payload.Nonce
	fuzzRetrievalExternalDoor(f, retrievalExternalDoor[RequestPayload]{
		Seed: fixture.payload, Mutations: []RequestPayload{mutation}, MaximumBytes: RequestPayloadJSONMaximumBytes,
		Marshal:   func(value RequestPayload) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *RequestPayload, data []byte) error { return value.UnmarshalJSON(data) },
		Validate:  func(value RequestPayload) error { return value.Validate() },
		Authenticate: func(value RequestPayload, authentic bool) error {
			proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
				Body: value, Envelope: document.Attestation, TrustedKeys: fixture.trusted,
			})
			return requestProofOracle(proof, err, authentic)
		},
	})
}

func FuzzRequestDocumentExternalDecoderAndVerifier(f *testing.F) {
	fixture := newRetrievalRequestFixture(f, retrievalRequestFixtureRequest{Selection: All()})
	document := issueRetrievalRequestFixture(f, fixture)
	other := newRetrievalRequestFixture(f, retrievalRequestFixtureRequest{Selection: All(), NonceByte: 2})
	mutation := document
	mutation.Payload.Nonce = other.payload.Nonce
	fuzzRetrievalExternalDoor(f, retrievalExternalDoor[RequestDocument]{
		Seed: document, Mutations: []RequestDocument{mutation}, MaximumBytes: RequestDocumentJSONMaximumBytes,
		Marshal:   func(value RequestDocument) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *RequestDocument, data []byte) error { return value.UnmarshalJSON(data) },
		Validate:  func(value RequestDocument) error { return value.Validate() },
		Authenticate: func(value RequestDocument, authentic bool) error {
			proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
				Body: value.Payload, Envelope: value.Attestation, TrustedKeys: fixture.trusted,
			})
			return requestProofOracle(proof, err, authentic)
		},
	})
}

func FuzzRequestCommitmentExternalDecoder(f *testing.F) {
	fixture := newRetrievalRequestFixture(f, retrievalRequestFixtureRequest{Selection: All()})
	seed, err := CommitRequest(fixture.payload)
	if err != nil {
		f.Fatalf("CommitRequest(seed) error = %v, want nil", err)
	}
	other := newRetrievalRequestFixture(f, retrievalRequestFixtureRequest{Selection: All(), NonceByte: 2})
	mutation, err := CommitRequest(other.payload)
	if err != nil {
		f.Fatalf("CommitRequest(mutation) error = %v, want nil", err)
	}
	fuzzRetrievalExternalDoor(f, retrievalExternalDoor[RequestCommitment]{
		Seed: seed, Mutations: []RequestCommitment{mutation}, MaximumBytes: RequestPayloadJSONMaximumBytes,
		Marshal:   func(value RequestCommitment) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *RequestCommitment, data []byte) error { return value.UnmarshalJSON(data) },
		Validate:  func(value RequestCommitment) error { return value.Validate() },
	})
}

func FuzzGrantPayloadExternalDecoderAndVerifier(f *testing.F) {
	fixture := newDownloadCallFixture(f, []byte{0x11, 0x12})
	mutation := fixture.grantPayload
	mutation.Authorization = retrievalFuzzAuthorityNonce(f, 0x72)
	fuzzRetrievalExternalDoor(f, retrievalExternalDoor[GrantPayload]{
		Seed: fixture.grantPayload, Mutations: []GrantPayload{mutation}, MaximumBytes: GrantPayloadJSONMaximumBytes,
		Marshal:   func(value GrantPayload) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *GrantPayload, data []byte) error { return value.UnmarshalJSON(data) },
		Validate:  func(value GrantPayload) error { return value.Validate() },
		Authenticate: func(value GrantPayload, authentic bool) error {
			document := fixture.document
			document.Payload = value
			grant, err := VerifyGrant(GrantExpectation{
				Document: document, Request: fixture.request, Chit: fixture.chit,
				ObservedAt: retrievalObservedInstant(), TrustedKeys: fixture.trusted,
			})
			return grantProofOracle(grant, err, authentic)
		},
	})
}

func FuzzGrantDocumentExternalDecoderAndVerifier(f *testing.F) {
	fixture := newDownloadCallFixture(f, []byte{0x21, 0x22})
	projection, err := IssueGrant(GrantIssuance{
		Signer: fixture.private, Capability: fixture.capability, Payload: fixture.grantPayload,
		Entry: fixture.addition, Chit: fixture.chit,
	})
	if err != nil {
		f.Fatalf("IssueGrant(seed) error = %v, want nil", err)
	}
	canonical, err := projection.MarshalJSON()
	if err != nil {
		f.Fatalf("GrantProjection.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	for _, data := range retrievalHostileSeeds(GrantDocumentJSONMaximumBytes) {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		candidate := fixture.document
		decodeErr := candidate.UnmarshalJSON(data)
		if decodeErr != nil {
			if !errors.Is(decodeErr, core.ErrJSONContract) || !errors.Is(decodeErr, core.ErrRetrievalContract) ||
				!sameGrantDocument(candidate, fixture.document) {
				t.Fatalf("GrantDocument.UnmarshalJSON() refusal = (%v, preserved %t), want %v/%v and exact receiver",
					decodeErr, sameGrantDocument(candidate, fixture.document), core.ErrJSONContract, core.ErrRetrievalContract)
			}
			return
		}
		if err := candidate.Validate(); err != nil {
			t.Fatalf("accepted GrantDocument.Validate() error = %v, want nil", err)
		}
		grant, verifyErr := VerifyGrant(GrantExpectation{
			Document: candidate, Request: fixture.request, Chit: fixture.chit,
			ObservedAt: retrievalObservedInstant(), TrustedKeys: fixture.trusted,
		})
		if err := grantProofOracle(grant, verifyErr, sameGrantDocument(candidate, fixture.document)); err != nil {
			t.Fatalf("accepted GrantDocument verification oracle error = %v, want nil", err)
		}
	})
}

func fuzzRetrievalExternalDoor[T any](f *testing.F, door retrievalExternalDoor[T]) {
	f.Helper()
	canonical := mustRetrievalProjection(f, door, door.Seed)
	f.Add(canonical)
	for _, mutation := range door.Mutations {
		f.Add(mustRetrievalProjection(f, door, mutation))
	}
	for _, data := range retrievalHostileSeeds(door.MaximumBytes) {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		candidate := door.Seed
		decodeErr := door.Unmarshal(&candidate, data)
		if decodeErr != nil {
			requireRetrievalDecodeRefusal(t, retrievalDecodeRefusal[T]{
				Door: door, Candidate: candidate, Before: canonical, Err: decodeErr,
			})
			return
		}
		encoded := mustRetrievalProjection(t, door, candidate)
		requireRetrievalCanonicalClosure(t, retrievalCanonicalClosure[T]{
			Door: door, Candidate: candidate, Encoded: encoded,
		})
		if door.Authenticate != nil {
			if err := door.Authenticate(candidate, bytes.Equal(encoded, canonical)); err != nil {
				t.Fatalf("accepted retrieval authentication oracle error = %v, want nil", err)
			}
		}
	})
}

func mustRetrievalProjection[T any](t testing.TB, door retrievalExternalDoor[T], value T) []byte {
	t.Helper()
	encoded, err := door.Marshal(value)
	if err != nil {
		t.Fatalf("retrieval external MarshalJSON() error = %v, want nil", err)
	}
	return encoded
}

type retrievalDecodeRefusal[T any] struct {
	Door      retrievalExternalDoor[T]
	Candidate T
	Before    []byte
	Err       error
}

func requireRetrievalDecodeRefusal[T any](t *testing.T, refusal retrievalDecodeRefusal[T]) {
	t.Helper()
	if !errors.Is(refusal.Err, core.ErrJSONContract) || !errors.Is(refusal.Err, core.ErrRetrievalContract) {
		t.Fatalf("retrieval external UnmarshalJSON() error = %v, want %v/%v", refusal.Err, core.ErrJSONContract, core.ErrRetrievalContract)
	}
	if after := mustRetrievalProjection(t, refusal.Door, refusal.Candidate); !bytes.Equal(after, refusal.Before) {
		t.Fatalf("rejected retrieval receiver projection = %x, want preserved %x", after, refusal.Before)
	}
}

type retrievalCanonicalClosure[T any] struct {
	Door      retrievalExternalDoor[T]
	Candidate T
	Encoded   []byte
}

func requireRetrievalCanonicalClosure[T any](t *testing.T, closure retrievalCanonicalClosure[T]) {
	t.Helper()
	if err := closure.Door.Validate(closure.Candidate); err != nil {
		t.Fatalf("accepted retrieval Validate() error = %v, want nil", err)
	}
	var roundTrip T
	decodeErr := closure.Door.Unmarshal(&roundTrip, closure.Encoded)
	second := mustRetrievalProjection(t, closure.Door, roundTrip)
	if decodeErr != nil || !bytes.Equal(second, closure.Encoded) {
		t.Fatalf("accepted retrieval canonical closure = (%x, %v), want (%x, nil)", second, decodeErr, closure.Encoded)
	}
}

func requestProofOracle(proof attest.Verified[SigningDomain], err error, authentic bool) error {
	if authentic {
		return errors.Join(err, proof.Validate())
	}
	if !errors.Is(err, core.ErrAttestVerification) || proof != (attest.Verified[SigningDomain]{}) {
		return errors.Join(core.ErrAttestVerification, err)
	}
	return nil
}

func grantProofOracle(grant VerifiedGrant, err error, authentic bool) error {
	if authentic {
		return errors.Join(err, grant.Validate())
	}
	if !errors.Is(err, core.ErrRetrievalBinding) || !verifiedGrantIsZero(grant) {
		return errors.Join(core.ErrRetrievalBinding, err)
	}
	return nil
}

func retrievalHostileSeeds(maximum uint64) [][]byte {
	return [][]byte{
		nil, {}, []byte("null"), []byte("{}"), []byte("[]"), []byte(`{"unknown":true}`),
		[]byte(`{"payload":null}`), bytes.Repeat([]byte{' '}, int(maximum)+1),
	}
}

func retrievalFuzzAuthorityNonce(t testing.TB, marker byte) controlwire.AuthorityNonce {
	t.Helper()
	raw := [controlwire.NonceBytes]byte{marker}
	nonce, err := controlwire.NewAuthorityNonce(raw)
	if err != nil {
		t.Fatalf("controlwire.NewAuthorityNonce() error = %v, want nil", err)
	}
	return nonce
}

func retrievalObservedInstant() temporal.Instant {
	return temporal.InstantFromNanoseconds(retrievalGrantObserved)
}
