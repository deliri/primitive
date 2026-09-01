package submission

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzCompletionPayloadJSONSemanticClosure(f *testing.F) {
	fixture := newCompletionFixture(f, submissionOffering(f, 2), []byte("fuzz completion payload"), 0x10)
	payload := receiveIssuedCompletion(f, fixture).Payload
	canonical, err := payload.MarshalJSON()
	if err != nil {
		f.Fatalf("CompletionPayload.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(append(bytes.Clone(canonical), 0))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := payload
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrControlPlaneContract) || got != payload {
				t.Fatalf("CompletionPayload.UnmarshalJSON(rejected) = (%v, %v), want preserved and typed JSON/control-plane rejection",
					got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("CompletionPayload.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > CompletionPayloadJSONMaximumBytes {
			t.Fatalf("CompletionPayload.MarshalJSON(accepted) = (%d bytes, %v), want <= %d and nil",
				len(encoded), err, CompletionPayloadJSONMaximumBytes)
		}
		var roundTrip CompletionPayload
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("CompletionPayload canonical round trip = (%v, %v), want exact %v and nil", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("CompletionPayload second canonical projection = (%q, %v), want %q and nil", second, err, encoded)
		}
	})
}

func FuzzCompletionDocumentJSONSemanticAndSignatureClosure(f *testing.F) {
	fixture := newCompletionFixture(f, submissionOffering(f, 2), []byte("fuzz completion document"), 0x10)
	document := receiveIssuedCompletion(f, fixture)
	canonical, err := document.MarshalJSON()
	if err != nil {
		f.Fatalf("CompletionDocument.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(append(bytes.Clone(canonical), 0))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := document
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrControlPlaneContract) || got != document {
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
		verified, verifyErr := VerifyCompletion(CompletionExpectation{
			Document: roundTrip, Request: fixture.request, Grant: fixture.grantDocument,
			GrantKeys: fixture.grantKeys, CompletionKeys: fixture.deviceKeys, Nonce: fixture.nonce,
		})
		if verifyErr != nil {
			stableRejection := errors.Is(verifyErr, core.ErrControlPlaneResponseBinding) ||
				errors.Is(verifyErr, core.ErrAttestVerification)
			if !errors.Is(verifyErr, core.ErrControlPlaneContract) || !stableRejection ||
				verified != (VerifiedCompletion{}) {
				t.Fatalf("VerifyCompletion(fuzzed document) = (%v, %v), want zero typed binding/attestation rejection",
					verified, verifyErr)
			}
			return
		}
		if roundTrip != document {
			t.Fatalf("VerifyCompletion authenticated a document other than the compiler-owned signed fixture")
		}
	})
}
