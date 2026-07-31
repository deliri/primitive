package receipt

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzEvidenceDocumentJSON(f *testing.F) {
	fixture := newReceiptFixture(f, 150)
	document := issueFixture(f, fixture)
	seed, err := json.Marshal(document)
	if err != nil {
		f.Fatalf("json.Marshal(document) error = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte("null"))
	f.Add([]byte(`{"payload":{}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var got EvidenceDocument
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				got != (EvidenceDocument{}) {
				t.Fatalf("rejected document = (%v, %v), want zero and %v", got, gotErr, core.ErrJSONContract)
			}
			return
		}
		if got.Validate() != nil {
			t.Fatalf("accepted document Validate() error = %v, want nil", got.Validate())
		}
		canonical, err := json.Marshal(got)
		if err != nil || len(canonical) > EvidenceDocumentCanonicalJSONMaximumBytes {
			t.Fatalf("canonical document = (%d bytes, %v), want at most %d and nil",
				len(canonical), err, EvidenceDocumentCanonicalJSONMaximumBytes)
		}
		var roundTrip EvidenceDocument
		if err := json.Unmarshal(canonical, &roundTrip); err != nil || roundTrip != got {
			t.Fatalf("canonical round trip = (%v, %v), want exact document and nil", roundTrip, err)
		}
		second, err := json.Marshal(roundTrip)
		if err != nil || !bytes.Equal(second, canonical) {
			t.Fatalf("second canonical encoding = (%q, %v), want %q and nil", second, err, canonical)
		}
		verified, verifyErr := VerifyEvidence(VerifyEvidenceRequest{
			Document: roundTrip, TrustedKeys: fixture.trusted,
			Expected: EvidenceExpectation{
				Account:  roundTrip.Payload.Header.Account,
				Offering: roundTrip.Payload.Header.Offering,
				Body:     roundTrip.Payload.Body,
			},
		})
		if verifyErr != nil {
			if !errors.Is(verifyErr, core.ErrReceiptVerification) ||
				verified != (VerifiedEvidence{}) {
				t.Fatalf("VerifyEvidence(mutated) = (%v, %v), want zero typed verification rejection", verified, verifyErr)
			}
			return
		}
		if roundTrip != document {
			t.Fatalf("VerifyEvidence authenticated a document other than the signed fixture")
		}
	})
}
