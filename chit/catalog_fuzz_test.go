package chit

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzCustodyStateJSONSemanticClosure(f *testing.F) {
	for state := CustodyStateUnknown + 1; state < custodyStateLimit; state++ {
		encoded, err := state.MarshalJSON()
		if err != nil {
			f.Fatalf("CustodyState(%d).MarshalJSON(seed) error = %v, want nil", state, err)
		}
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Add([]byte("{}"))

	f.Fuzz(func(t *testing.T, data []byte) {
		receiver := CustodyStateStored
		err := receiver.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) || receiver != CustodyStateStored {
				t.Fatalf("CustodyState.UnmarshalJSON(rejected) = (%v, %v), want preserved and typed JSON rejection",
					receiver, err)
			}
			return
		}
		if err := receiver.Validate(); err != nil {
			t.Fatalf("CustodyState.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		canonical, err := receiver.MarshalJSON()
		if err != nil {
			t.Fatalf("CustodyState.MarshalJSON(accepted) error = %v, want nil", err)
		}
		var roundTrip CustodyState
		if err := roundTrip.UnmarshalJSON(canonical); err != nil || roundTrip != receiver {
			t.Fatalf("CustodyState canonical round trip = (%v, %v), want %v and nil", roundTrip, err, receiver)
		}
	})
}

func FuzzCursorJSONSemanticClosure(f *testing.F) {
	seed := catalogCursorFixture(f, 0x31)
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("Cursor.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Add(append(bytes.Clone(canonical), 0))

	f.Fuzz(func(t *testing.T, data []byte) {
		receiver := seed
		err := receiver.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) || receiver != seed {
				t.Fatalf("Cursor.UnmarshalJSON(rejected) = (%v, %v), want preserved and typed JSON rejection", receiver, err)
			}
			return
		}
		if err := receiver.Validate(); err != nil {
			t.Fatalf("Cursor.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := receiver.MarshalJSON()
		if err != nil {
			t.Fatalf("Cursor.MarshalJSON(accepted) error = %v, want nil", err)
		}
		var roundTrip Cursor
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != receiver {
			t.Fatalf("Cursor canonical round trip = (%v, %v), want %v and nil", roundTrip, err, receiver)
		}
	})
}

func FuzzCatalogPayloadJSONSemanticClosure(f *testing.F) {
	fixture := newCatalogFixture(f, 0x32, 1)
	canonical, err := fixture.payload.MarshalJSON()
	if err != nil {
		f.Fatalf("CatalogPayload.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Add(append(bytes.Clone(canonical), 0))

	f.Fuzz(func(t *testing.T, data []byte) {
		receiver := fixture.payload
		err := receiver.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) || !catalogPayloadsEqual(receiver, fixture.payload) {
				t.Fatalf("CatalogPayload.UnmarshalJSON(rejected) = (%v, %v), want preserved and typed JSON rejection",
					receiver, err)
			}
			return
		}
		if err := receiver.Validate(); err != nil {
			t.Fatalf("CatalogPayload.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := receiver.MarshalJSON()
		if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
			t.Fatalf("CatalogPayload.MarshalJSON(accepted) = (%d bytes, %v), want bounded and nil", len(encoded), err)
		}
		var roundTrip CatalogPayload
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !catalogPayloadsEqual(roundTrip, receiver) {
			t.Fatalf("CatalogPayload canonical round trip = (%v, %v), want exact payload and nil", roundTrip, err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("CatalogPayload second canonical projection = (%q, %v), want byte-identical %q and nil",
				second, err, encoded)
		}
	})
}

func FuzzCatalogDocumentJSONSemanticAndAuthorityClosure(f *testing.F) {
	fixture := newCatalogFixture(f, 0x33, 1)
	canonical, err := fixture.document.MarshalJSON()
	if err != nil {
		f.Fatalf("CatalogDocument.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Add(append(bytes.Clone(canonical), 0))

	f.Fuzz(func(t *testing.T, data []byte) {
		receiver := cloneCatalogDocument(fixture.document)
		err := receiver.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) || !catalogDocumentsEqual(receiver, fixture.document) {
				t.Fatalf("CatalogDocument.UnmarshalJSON(rejected) = (%v, %v), want preserved and typed JSON rejection",
					receiver, err)
			}
			return
		}
		if err := receiver.Validate(); err != nil {
			t.Fatalf("CatalogDocument.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := receiver.MarshalJSON()
		if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
			t.Fatalf("CatalogDocument.MarshalJSON(accepted) = (%d bytes, %v), want bounded and nil", len(encoded), err)
		}
		var roundTrip CatalogDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !catalogDocumentsEqual(roundTrip, receiver) {
			t.Fatalf("CatalogDocument canonical round trip = (%v, %v), want exact document and nil", roundTrip, err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("CatalogDocument second canonical projection = (%q, %v), want byte-identical %q and nil",
				second, err, encoded)
		}
		verified, verifyErr := VerifyCatalog(CatalogVerification{
			Document: roundTrip, Request: fixture.request, TrustedKeys: fixture.trusted,
		})
		if verifyErr != nil {
			if !errors.Is(verifyErr, core.ErrChitVerification) || verified != (VerifiedCatalog{}) {
				t.Fatalf("VerifyCatalog(fuzzed document) = (%v, %v), want zero typed verification rejection",
					verified, verifyErr)
			}
			return
		}
		if !catalogDocumentsEqual(roundTrip, fixture.document) || !verifiedCatalogPayloadsEqual(verified, fixture.payload) {
			t.Fatalf("VerifyCatalog authenticated a document other than the compiler-produced signed seed")
		}
	})
}
