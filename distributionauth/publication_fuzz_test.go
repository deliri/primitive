package distributionauth

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzCredentialedPublicationRequestJSONSemanticAndAuthorityClosure(f *testing.F) {
	fixture := newPublicationAuthFixture(f, publicationAuthFixtureRequest{})
	canonical, err := fixture.document.MarshalJSON()
	if err != nil {
		f.Fatalf("PublicationRequestDocument.MarshalJSON(seed) error = %v, want nil", err)
	}
	foreign := newPublicationAuthFixture(f, publicationAuthFixtureRequest{
		authorityByte: 0xc1, deviceByte: 0xc2, releaseByte: 0xc3, nonceByte: 0xc4,
	})
	foreignWire, err := foreign.document.MarshalJSON()
	if err != nil {
		f.Fatalf("PublicationRequestDocument.MarshalJSON(foreign seed) error = %v, want nil", err)
	}
	tampered := fixture.document
	tampered.Request.Payload.Nonce = distributionAuthNonce(f, 0xc5)
	tamperedWire, err := tampered.MarshalJSON()
	if err != nil {
		f.Fatalf("PublicationRequestDocument.MarshalJSON(tampered seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add(foreignWire)
	f.Add(tamperedWire)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(append(bytes.Clone(canonical), 0))
	f.Add(distributionAuthPadJSON(canonical, RequestDocumentJSONMaximumBytes+1))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := fixture.document
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrControlPlaneContract) || got != fixture.document {
				t.Fatalf("PublicationRequestDocument.UnmarshalJSON(rejected) = (%+v, %v), want preserved and typed JSON/control-plane rejection",
					got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("PublicationRequestDocument.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > RequestDocumentJSONMaximumBytes {
			t.Fatalf("PublicationRequestDocument.MarshalJSON(accepted) = (%d bytes, %v), want <= %d and nil",
				len(encoded), err, RequestDocumentJSONMaximumBytes)
		}
		var roundTrip PublicationRequestDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("PublicationRequestDocument canonical round trip = (%+v, %v), want exact and nil",
				roundTrip, err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("PublicationRequestDocument second canonical projection = (%q, %v), want (%q, nil)",
				second, err, encoded)
		}
		verified, verifyErr := VerifyPublication(PublicationVerification{
			Document: roundTrip, TrustedKeys: fixture.authority, ManifestKeys: fixture.release.keys,
		})
		if verifyErr != nil {
			if !errors.Is(verifyErr, core.ErrAttestVerification) || verified != (VerifiedPublication{}) {
				t.Fatalf("VerifyPublication(fuzzed credential) = (%+v, %v), want zero typed attestation rejection",
					verified, verifyErr)
			}
			return
		}
		if roundTrip != fixture.document {
			t.Fatalf("VerifyPublication authenticated document = %+v, want compiler-owned signed fixture %+v",
				roundTrip, fixture.document)
		}
	})
}

func FuzzCredentialedPublicationCompletionJSONSemanticAndAuthorityClosure(f *testing.F) {
	fixture := newPublicationAuthFixture(f, publicationAuthFixtureRequest{})
	canonical, err := fixture.completion.MarshalJSON()
	if err != nil {
		f.Fatalf("PublicationCompletionDocument.MarshalJSON(seed) error = %v, want nil", err)
	}
	foreign := newPublicationAuthFixture(f, publicationAuthFixtureRequest{
		authorityByte: 0xd1, deviceByte: 0xd2, releaseByte: 0xd3, nonceByte: 0xd4,
	})
	foreignWire, err := foreign.completion.MarshalJSON()
	if err != nil {
		f.Fatalf("PublicationCompletionDocument.MarshalJSON(foreign seed) error = %v, want nil", err)
	}
	tampered := fixture.completion
	tampered.Completion.Payload.Authorization = publicationAuthAuthorityNonce(f, 0xd5)
	tamperedWire, err := tampered.MarshalJSON()
	if err != nil {
		f.Fatalf("PublicationCompletionDocument.MarshalJSON(tampered seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add(foreignWire)
	f.Add(tamperedWire)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(append(bytes.Clone(canonical), 0))
	f.Add(distributionAuthPadJSON(canonical, PublicationCompletionDocumentJSONMaximumBytes+1))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := fixture.completion
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrControlPlaneContract) || got != fixture.completion {
				t.Fatalf("PublicationCompletionDocument.UnmarshalJSON(rejected) = (%+v, %v), want preserved and typed JSON/control-plane rejection",
					got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("PublicationCompletionDocument.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > PublicationCompletionDocumentJSONMaximumBytes {
			t.Fatalf("PublicationCompletionDocument.MarshalJSON(accepted) = (%d bytes, %v), want <= %d and nil",
				len(encoded), err, PublicationCompletionDocumentJSONMaximumBytes)
		}
		if bytes.Contains(encoded, []byte(core.GoogleCloudStorageHost)) {
			t.Fatalf("PublicationCompletionDocument disclosed provider target material: %q", encoded)
		}
		var roundTrip PublicationCompletionDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("PublicationCompletionDocument canonical round trip = (%+v, %v), want exact and nil",
				roundTrip, err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("PublicationCompletionDocument second canonical projection = (%q, %v), want (%q, nil)",
				second, err, encoded)
		}
		verified, verifyErr := VerifyPublicationCompletion(PublicationCompletionVerification{
			Grant: fixture.grant, Document: roundTrip,
			Request: fixture.verified, TrustedKeys: fixture.authority,
		})
		if verifyErr != nil {
			if (!errors.Is(verifyErr, core.ErrAttestVerification) &&
				!errors.Is(verifyErr, core.ErrControlPlaneResponseBinding)) ||
				verified != (VerifiedPublicationCompletion{}) {
				t.Fatalf("VerifyPublicationCompletion(fuzzed credential) = (%+v, %v), want zero typed attestation/binding rejection",
					verified, verifyErr)
			}
			return
		}
		if roundTrip != fixture.completion {
			t.Fatalf("VerifyPublicationCompletion authenticated document = %+v, want compiler-owned signed fixture %+v",
				roundTrip, fixture.completion)
		}
	})
}
