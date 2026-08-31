package distributionauth

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzCredentialedUpdateRequestJSONSemanticAndAuthorityClosure(f *testing.F) {
	fixture := newDistributionAuthFixture(f, distributionAuthFixtureRequest{})
	canonical, err := fixture.update.MarshalJSON()
	if err != nil {
		f.Fatalf("UpdateRequestDocument.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(append(bytes.Clone(canonical), 0))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := fixture.update
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrControlPlaneContract) || got != fixture.update {
				t.Fatalf("UpdateRequestDocument.UnmarshalJSON(rejected) = (%v, %v), want preserved and typed JSON/control-plane rejection",
					got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("UpdateRequestDocument.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > RequestDocumentJSONMaximumBytes {
			t.Fatalf("UpdateRequestDocument.MarshalJSON(accepted) = (%d bytes, %v), want <= %d and nil",
				len(encoded), err, RequestDocumentJSONMaximumBytes)
		}
		var roundTrip UpdateRequestDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("UpdateRequestDocument canonical round trip = (%v, %v), want exact %v and nil", roundTrip, err, got)
		}
		verified, verifyErr := VerifyUpdate(UpdateVerification{Document: roundTrip, Server: distributionAuthServer(t, fixture.trusted)})
		if verifyErr != nil {
			if !errors.Is(verifyErr, core.ErrAttestVerification) || verified != (VerifiedUpdate{}) {
				t.Fatalf("VerifyUpdate(fuzzed credential) = (%v, %v), want zero typed attestation rejection", verified, verifyErr)
			}
			return
		}
		if roundTrip != fixture.update {
			t.Fatalf("VerifyUpdate authenticated a credential other than the compiler-owned signed fixture")
		}
	})
}

func FuzzCredentialedUpgradeRequestJSONSemanticAndAuthorityClosure(f *testing.F) {
	fixture := newDistributionAuthFixture(f, distributionAuthFixtureRequest{})
	canonical, err := fixture.upgrade.MarshalJSON()
	if err != nil {
		f.Fatalf("UpgradeRequestDocument.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(append(bytes.Clone(canonical), 0))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := fixture.upgrade
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrControlPlaneContract) || got != fixture.upgrade {
				t.Fatalf("UpgradeRequestDocument.UnmarshalJSON(rejected) = (%v, %v), want preserved and typed JSON/control-plane rejection",
					got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("UpgradeRequestDocument.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > RequestDocumentJSONMaximumBytes {
			t.Fatalf("UpgradeRequestDocument.MarshalJSON(accepted) = (%d bytes, %v), want <= %d and nil",
				len(encoded), err, RequestDocumentJSONMaximumBytes)
		}
		var roundTrip UpgradeRequestDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("UpgradeRequestDocument canonical round trip = (%v, %v), want exact %v and nil", roundTrip, err, got)
		}
		verified, verifyErr := VerifyUpgrade(UpgradeVerification{Document: roundTrip, Server: distributionAuthServer(t, fixture.trusted)})
		if verifyErr != nil {
			if !errors.Is(verifyErr, core.ErrAttestVerification) || verified != (VerifiedUpgrade{}) {
				t.Fatalf("VerifyUpgrade(fuzzed credential) = (%v, %v), want zero typed attestation rejection", verified, verifyErr)
			}
			return
		}
		if roundTrip != fixture.upgrade {
			t.Fatalf("VerifyUpgrade authenticated a credential other than the compiler-owned signed fixture")
		}
	})
}
