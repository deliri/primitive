package submissionauth

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzCredentialedRequestJSONSemanticAndAuthorityClosure(f *testing.F) {
	fixture := newAuthFixture(f, authFixtureRequest{})
	foreign := newAuthFixture(f, authFixtureRequest{authorityByte: 0x65, deviceByte: 0x66, nonceByte: 0x67})
	server := submissionAuthServer(f, fixture.trusted)
	canonical, err := fixture.document.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	for _, selector := range []uint8{0, 1, 2, 3, 4, 5, 6} {
		f.Add(canonical, selector)
	}
	f.Add([]byte{}, uint8(0))
	f.Add([]byte("{}"), uint8(0))
	f.Fuzz(func(t *testing.T, data []byte, selector uint8) {
		if selector%7 != 0 {
			candidate := fixture.document
			switch selector % 7 {
			case 1:
				candidate.Request.Attestation.Signer = foreign.document.Request.Attestation.Signer
			case 2:
				candidate.Request.Attestation.Signature = foreign.document.Request.Attestation.Signature
			case 3:
				candidate.Certificate.Attestation.Signature = foreign.certificate.Attestation.Signature
			case 4:
				candidate.Request.Payload.Nonce = foreign.request.Payload.Nonce
			case 5:
				candidate.Certificate = foreign.certificate
			case 6:
				candidate = foreign.document
			}
			if candidate == fixture.document {
				t.Fatalf("mutation selector = %d preserved every fact, want a load-bearing difference", selector)
			}
			var err error
			data, err = core.MarshalCanonicalJSONDocument(requestDocumentWire(candidate))
			if err != nil {
				t.Fatal(err)
			}
		}
		got := fixture.document
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrControlPlaneContract) || got != fixture.document || bytes.Equal(data, canonical) {
				t.Fatalf("decode=%v error=%v, want preserved receiver and typed refusal", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatal(err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var roundTrip RequestDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("round trip=%v error=%v, want exact %v", roundTrip, err, got)
		}
		again, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(encoded, again) {
			t.Fatalf("second canonical output differs: %v", err)
		}
		assembled, err := Assemble(RequestAssembly(got))
		if err != nil || assembled != got {
			t.Fatalf("assembly=%v error=%v, want exact received request", assembled, err)
		}
		verified, err := Verify(Verification{Document: got, Server: server})
		if got == fixture.document {
			if err != nil {
				t.Fatalf("authentic request refused: %v", err)
			}
			authenticated, err := verified.Document()
			if err != nil || authenticated != fixture.document {
				t.Fatalf("authenticated=%v error=%v, want exact seed", authenticated, err)
			}
		} else if !errors.Is(err, core.ErrControlPlaneContract) || !errors.Is(err, core.ErrAttestVerification) || verified != (Verified{}) {
			t.Fatalf("mutated request proof=%v error=%v, want zero attestation refusal", verified, err)
		}
	})
}
