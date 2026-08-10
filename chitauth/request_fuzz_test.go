package chitauth

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzCredentialedChitQueryJSONSemanticAndAuthorityClosure(f *testing.F) {
	fixture := newQueryFixture(f, queryFixtureRequest{selection: querySpecificSelection(f)})
	canonical, err := fixture.document.MarshalJSON()
	if err != nil {
		f.Fatalf("RequestDocument.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(append(bytes.Clone(canonical), 0))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := fixture.document
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrControlPlaneContract) || got != fixture.document {
				t.Fatalf("RequestDocument.UnmarshalJSON(rejected) = (%v, %v), want preserved and typed JSON/control-plane rejection",
					got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("RequestDocument.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > RequestDocumentJSONMaximumBytes {
			t.Fatalf("RequestDocument.MarshalJSON(accepted) = (%d bytes, %v), want <= %d and nil",
				len(encoded), err, RequestDocumentJSONMaximumBytes)
		}
		var roundTrip RequestDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("RequestDocument canonical round trip = (%v, %v), want exact %v and nil", roundTrip, err, got)
		}
		verified, verifyErr := Verify(Verification{Document: roundTrip, TrustedKeys: fixture.trusted})
		if verifyErr != nil {
			if !errors.Is(verifyErr, core.ErrAttestVerification) || verified != (Verified{}) {
				t.Fatalf("Verify(fuzzed credential) = (%v, %v), want zero typed attestation rejection", verified, verifyErr)
			}
			return
		}
		if roundTrip != fixture.document {
			t.Fatalf("Verify authenticated a credentialed query other than the compiler-owned signed fixture")
		}
	})
}
