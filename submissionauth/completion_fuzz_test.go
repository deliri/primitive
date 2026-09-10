package submissionauth

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzCredentialedCompletionProjectionValidateJSONProjectionOracle(f *testing.F) {
	fixture := newAuthCompletionFixture(f, authCompletionFixtureRequest{nonceByte: 0x71})
	projection := assembleAuthCompletionProjection(f, fixture)
	canonical, err := projection.MarshalJSON()
	if err != nil {
		f.Fatalf("CompletionProjection.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(append(bytes.Clone(canonical), 0))

	f.Fuzz(func(t *testing.T, data []byte) {
		gotErr := projection.ValidateJSONProjection(data, core.DefaultStrictJSONLimits())
		if gotErr != nil {
			if bytes.Equal(data, canonical) {
				t.Fatalf("valid seed refused: %v", gotErr)
			}
			if !errors.Is(gotErr, core.ErrJSONContract) {
				t.Fatalf("ValidateJSONProjection(rejected) error = %v, want errors.Is %v", gotErr, core.ErrJSONContract)
			}
			return
		}
		if !bytes.Equal(data, canonical) {
			t.Fatalf("ValidateJSONProjection authenticated bytes other than the compiler-owned issued projection")
		}
		encoded, err := core.EncodeValidatedJSON(projection, core.DefaultStrictJSONLimits())
		if err != nil || !bytes.Equal(encoded, canonical) {
			t.Fatalf("EncodeValidatedJSON(accepted projection) = (%d bytes, %v), want exact seed", len(encoded), err)
		}
	})
}

func FuzzCredentialedCompletionJSONSemanticAndAuthorityClosure(f *testing.F) {
	fixture := newAuthCompletionFixture(f, authCompletionFixtureRequest{})
	foreign := newAuthCompletionFixture(f, authCompletionFixtureRequest{authorityByte: 0x65, deviceByte: 0x66, nonceByte: 0x67, generation: 8})
	canonical, err := fixture.credentialed.MarshalJSON()
	if err != nil {
		f.Fatalf("CompletionDocument.MarshalJSON(seed) error = %v, want nil", err)
	}
	for selector := range uint8(7) {
		f.Add(canonical, selector)
	}
	f.Add([]byte{}, uint8(0))
	f.Add([]byte(`{}`), uint8(0))
	f.Add(append(bytes.Clone(canonical), 0), uint8(0))

	f.Fuzz(func(t *testing.T, data []byte, selector uint8) {
		if selector%7 != 0 {
			candidate := fixture.credentialed
			switch selector % 7 {
			case 1:
				candidate.Completion.Attestation.Signature = foreign.completionDocument.Attestation.Signature
			case 2:
				candidate.Certificate.Attestation.Signature = foreign.request.certificate.Attestation.Signature
			case 3:
				candidate.Completion.Payload.Nonce = foreign.completionNonce
			case 4:
				candidate.Completion.Payload.Authorization = authAuthorityNonce(t, 0x72)
			case 5:
				candidate.Completion.Payload.Evidence = foreign.completionDocument.Payload.Evidence
			case 6:
				candidate.Completion.Attestation.Signer = foreign.completionDocument.Attestation.Signer
			}
			if candidate == fixture.credentialed {
				t.Fatalf("mutation selector = %d preserved every fact, want a load-bearing difference", selector)
			}
			var err error
			data, err = core.MarshalCanonicalJSONDocument(completionDocumentWire(candidate))
			if err != nil {
				t.Fatal(err)
			}
		}
		got := fixture.credentialed
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if bytes.Equal(data, canonical) {
				t.Fatalf("valid seed refused: %v", gotErr)
			}
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrControlPlaneContract) || got != fixture.credentialed {
				t.Fatalf("CompletionDocument.UnmarshalJSON(rejected) = (%v, %v), want preserved and typed JSON/control-plane rejection",
					got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("CompletionDocument.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("canonical encoding failed: %v", err)
		}
		var roundTrip CompletionDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("CompletionDocument canonical round trip = (%v, %v), want exact %v and nil", roundTrip, err, got)
		}
		again, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(encoded, again) {
			t.Fatalf("second canonical output differs: %v", err)
		}
		assembled, err := AssembleCompletion(CompletionAssembly(roundTrip))
		if err != nil || assembled != roundTrip {
			t.Fatalf("assembly=%v error=%v, want exact received document", assembled, err)
		}
		verified, verifyErr := VerifyCompletion(CompletionVerification{
			Document: roundTrip, Request: fixture.verifiedRequest,
			Grant: fixture.grant, GrantKeys: fixture.request.trusted,
			Server: submissionAuthServer(t, fixture.request.trusted),
			Nonce:  fixture.completionNonce,
		})
		if verifyErr != nil {
			if roundTrip == fixture.credentialed {
				t.Fatalf("authentic credential refused: %v", verifyErr)
			}
			stableRejection := errors.Is(verifyErr, core.ErrControlPlaneResponseBinding) ||
				errors.Is(verifyErr, core.ErrAttestVerification)
			if !errors.Is(verifyErr, core.ErrControlPlaneContract) || !stableRejection ||
				verified != (VerifiedCompletion{}) {
				t.Fatalf("VerifyCompletion(fuzzed credential) = (%v, %v), want zero typed binding/attestation rejection",
					verified, verifyErr)
			}
			return
		}
		if roundTrip != fixture.credentialed {
			t.Fatalf("VerifyCompletion authenticated a credential other than the compiler-owned signed fixture")
		}
	})
}
