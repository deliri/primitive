package retrievalauth

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzRequestDocumentExternalDecoderAndVerifier(f *testing.F) {
	fixture := newRetrievalAuthFixture(f, retrievalAuthFixtureRequest{})
	other := newRetrievalAuthFixture(f, retrievalAuthFixtureRequest{NonceByte: 0x42})
	canonical, err := fixture.document.MarshalJSON()
	if err != nil {
		f.Fatalf("RequestDocument.MarshalJSON(seed) error = %v, want nil", err)
	}
	mutation := fixture.document
	mutation.Request.Payload.Nonce = other.request.Payload.Nonce
	mutated, err := mutation.MarshalJSON()
	if err != nil {
		f.Fatalf("RequestDocument.MarshalJSON(mutation) error = %v, want nil", err)
	}
	for _, seed := range [][]byte{
		canonical, mutated, nil, {}, []byte("null"), []byte("{}"), []byte("[]"),
		[]byte(`{"unknown":true}`), bytes.Repeat([]byte{' '}, RequestDocumentJSONMaximumBytes+1),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		candidate := fixture.document
		decodeErr := candidate.UnmarshalJSON(data)
		if decodeErr != nil {
			if !errors.Is(decodeErr, core.ErrJSONContract) || !errors.Is(decodeErr, core.ErrRetrievalContract) ||
				candidate != fixture.document {
				t.Fatalf("RequestDocument.UnmarshalJSON() refusal = (%v, preserved %t), want %v/%v and exact receiver",
					decodeErr, candidate == fixture.document, core.ErrJSONContract, core.ErrRetrievalContract)
			}
			return
		}
		if err := candidate.Validate(); err != nil {
			t.Fatalf("accepted RequestDocument.Validate() error = %v, want nil", err)
		}
		encoded, marshalErr := candidate.MarshalJSON()
		var roundTrip RequestDocument
		roundTripErr := roundTrip.UnmarshalJSON(encoded)
		second, secondErr := roundTrip.MarshalJSON()
		if marshalErr != nil || roundTripErr != nil || secondErr != nil || roundTrip != candidate || !bytes.Equal(second, encoded) {
			t.Fatalf("accepted RequestDocument canonical closure = (%v, %x, %v, %v, %v), want exact fixed point",
				roundTrip, second, marshalErr, roundTripErr, secondErr)
		}
		verified, verifyErr := Verify(Verification{Document: candidate, Server: retrievalAuthServer(t, fixture.trusted)})
		authentic := bytes.Equal(encoded, canonical)
		if authentic {
			if verifyErr != nil || verified.Validate() != nil {
				t.Fatalf("authentic RequestDocument Verify() = (%v, %v), want valid proof and nil", verified, verifyErr)
			}
			return
		}
		if !errors.Is(verifyErr, core.ErrAttestVerification) || verified != (Verified{}) {
			t.Fatalf("altered RequestDocument Verify() = (%v, %v), want zero and errors.Is %v",
				verified, verifyErr, core.ErrAttestVerification)
		}
	})
}
