package chit

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzQueryPayloadJSONSemanticClosure(f *testing.F) {
	fixture := newSignedQueryFixture(f, signedQueryFixtureRequest{
		marker: 0x31, selection: signedQuerySpecific(f, 0x31),
		pageSize: core.CatalogPageMaximumEntries,
	})
	canonical, err := fixture.payload.MarshalJSON()
	if err != nil {
		f.Fatalf("QueryPayload.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(append(bytes.Clone(canonical), 0))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := fixture.payload
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrChitContract) || got != fixture.payload {
				t.Fatalf("QueryPayload.UnmarshalJSON(rejected) = (%v, %v), want preserved and typed JSON/Chit rejection",
					got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("QueryPayload.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > QueryPayloadJSONMaximumBytes {
			t.Fatalf("QueryPayload.MarshalJSON(accepted) = (%d bytes, %v), want <= %d and nil",
				len(encoded), err, QueryPayloadJSONMaximumBytes)
		}
		var roundTrip QueryPayload
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("QueryPayload canonical round trip = (%v, %v), want exact %v and nil", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("QueryPayload second canonical projection = (%q, %v), want %q and nil", second, err, encoded)
		}
	})
}

func FuzzQueryDocumentJSONSemanticAndSignatureClosure(f *testing.F) {
	fixture := newSignedQueryFixture(f, signedQueryFixtureRequest{
		marker: 0x32, selection: signedQuerySpecific(f, 0x32),
		pageSize: core.CatalogPageMaximumEntries,
	})
	canonical, err := fixture.document.MarshalJSON()
	if err != nil {
		f.Fatalf("QueryDocument.MarshalJSON(seed) error = %v, want nil", err)
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
				!errors.Is(gotErr, core.ErrChitContract) || got != fixture.document {
				t.Fatalf("QueryDocument.UnmarshalJSON(rejected) = (%v, %v), want preserved and typed JSON/Chit rejection",
					got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("QueryDocument.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > QueryDocumentJSONMaximumBytes {
			t.Fatalf("QueryDocument.MarshalJSON(accepted) = (%d bytes, %v), want <= %d and nil",
				len(encoded), err, QueryDocumentJSONMaximumBytes)
		}
		var roundTrip QueryDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("QueryDocument canonical round trip = (%v, %v), want exact %v and nil", roundTrip, err, got)
		}
		verified, verifyErr := VerifyQuery(QueryVerification{Document: roundTrip, TrustedKeys: fixture.trusted})
		if verifyErr != nil {
			if !errors.Is(verifyErr, core.ErrChitVerification) || verified != (VerifiedQuery{}) {
				t.Fatalf("VerifyQuery(fuzzed document) = (%v, %v), want zero typed verification rejection", verified, verifyErr)
			}
			return
		}
		if roundTrip != fixture.document {
			t.Fatalf("VerifyQuery authenticated a document other than the compiler-owned signed fixture")
		}
	})
}
