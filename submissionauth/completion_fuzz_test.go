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
	canonical, err := fixture.credentialed.MarshalJSON()
	if err != nil {
		f.Fatalf("CompletionDocument.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(append(bytes.Clone(canonical), 0))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := fixture.credentialed
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
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
		if err != nil || len(encoded) > CompletionDocumentJSONMaximumBytes {
			t.Fatalf("CompletionDocument.MarshalJSON(accepted) = (%d bytes, %v), want <= %d and nil",
				len(encoded), err, CompletionDocumentJSONMaximumBytes)
		}
		var roundTrip CompletionDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("CompletionDocument canonical round trip = (%v, %v), want exact %v and nil", roundTrip, err, got)
		}
		verified, verifyErr := VerifyCompletion(CompletionVerification{
			Document: roundTrip, Request: fixture.verifiedRequest,
			Grant: fixture.grant, TrustedKeys: fixture.request.trusted,
		})
		if verifyErr != nil {
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
