package chit

import (
	"bytes"
	"crypto"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

type catalogFixture struct {
	private  crypto.Signer
	payload  CatalogPayload
	document CatalogDocument
	trusted  attest.TrustedKeys
	scope    receipt.Scope
}

func TestCustodyStateExhaustsItsByteDomainAndCanonicalJSON(t *testing.T) {
	t.Parallel()

	admitted := 0
	for value := 0; value <= 255; value++ {
		state := CustodyState(value)
		encoded, marshalErr := state.MarshalJSON()
		if state.IsValid() {
			admitted++
			if marshalErr != nil || state.String() == "" {
				t.Fatalf("CustodyState(%d) = (%q, %v), want named canonical member", value, state, marshalErr)
			}
			receiver := CustodyStateUnknown
			if err := receiver.UnmarshalJSON(encoded); err != nil || receiver != state {
				t.Fatalf("CustodyState(%d) JSON round trip = (%v, %v), want exact %v and nil", value, receiver, err, state)
			}
			continue
		}
		if !errors.Is(marshalErr, core.ErrChitContract) || !errors.Is(marshalErr, core.ErrJSONContract) ||
			encoded != nil || state.String() != "" {
			t.Fatalf("CustodyState(%d) = (%q, %v, %v), want unnamed nil output and errors.Is %v and %v",
				value, state, encoded, marshalErr, core.ErrChitContract, core.ErrJSONContract)
		}
	}
	if admitted != int(custodyStateLimit-CustodyStateUnknown-1) {
		t.Fatalf("admitted custody states = %d, want %d", admitted, custodyStateLimit-CustodyStateUnknown-1)
	}

	receiver := CustodyStateStored
	if err := receiver.UnmarshalJSON([]byte{}); !errors.Is(err, core.ErrJSONContract) || receiver != CustodyStateStored {
		t.Fatalf("CustodyState.UnmarshalJSON(empty) = (%v, %v), want preserved and errors.Is %v",
			receiver, err, core.ErrJSONContract)
	}
	var nilReceiver *CustodyState
	if err := nilReceiver.UnmarshalJSON([]byte{}); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil CustodyState.UnmarshalJSON() error = %v, want errors.Is %v", err, core.ErrJSONContract)
	}
}

func TestCatalogLayerTriadAuthenticatesTenIndependentPages(t *testing.T) {
	t.Parallel()

	for index := range 10 {
		fixture := newCatalogFixture(t, byte(0x21+index), uint64(index+1))
		verified, err := VerifyCatalog(CatalogVerification{
			Document: fixture.document, Scope: fixture.scope, TrustedKeys: fixture.trusted,
		})
		if err != nil || !catalogPayloadsEqual(verified, fixture.payload) {
			t.Fatalf("VerifyCatalog(configuration %d) = (%v, %v), want exact signed payload and nil",
				index, verified, err)
		}
		payloadJSON, err := fixture.payload.MarshalJSON()
		if err != nil {
			t.Fatalf("CatalogPayload.MarshalJSON(configuration %d) error = %v, want nil", index, err)
		}
		var payloadRoundTrip CatalogPayload
		if err := payloadRoundTrip.UnmarshalJSON(payloadJSON); err != nil ||
			!catalogPayloadsEqual(payloadRoundTrip, fixture.payload) {
			t.Fatalf("CatalogPayload round trip(configuration %d) = (%v, %v), want exact payload and nil",
				index, payloadRoundTrip, err)
		}
	}
}

func TestVerifyCatalogRejectsEveryIndependentAgreementSubstitution(t *testing.T) {
	t.Parallel()

	fixture := newCatalogFixture(t, 0x41, 1)
	other := newCatalogFixture(t, 0x71, 2)
	alternatePayload := fixture.payload
	alternatePayload.Entries = append([]CatalogEntry(nil), fixture.payload.Entries...)
	alternatePayload.Entries[0].Chit.Payload.Version = mustVersion(t, 2)
	alternateChit, err := Issue(Issuance{Signer: fixture.private, Payload: alternatePayload.Entries[0].Chit.Payload})
	if err != nil {
		t.Fatalf("Issue(alternate catalog chit) error = %v, want nil", err)
	}
	alternateWatermark := catalogWatermarkFixture(t, fixture.scope, 0x62)
	more, err := More(catalogCursorFixture(t, 0x63))
	if err != nil {
		t.Fatalf("More() error = %v, want nil", err)
	}

	cases := []struct {
		mutate  func(*CatalogVerification)
		wantErr error
		name    string
	}{
		{name: "zero verification", wantErr: core.ErrChitContract, mutate: func(value *CatalogVerification) { *value = CatalogVerification{} }},
		{name: "foreign authority", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) { value.TrustedKeys = other.trusted }},
		{name: "foreign expected scope", wantErr: core.ErrChitConflict, mutate: func(value *CatalogVerification) { value.Scope = other.scope }},
		{name: "signed observation substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Payload.ObservedAt = temporal.InstantFromNanoseconds(10_000)
		}},
		{name: "signed watermark substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Payload.Watermark = alternateWatermark
		}},
		{name: "signed chit substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Payload.Entries[0].Chit = alternateChit
		}},
		{name: "signed custody state substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			if value.Document.Payload.Entries[0].State == CustodyStateStored {
				value.Document.Payload.Entries[0].State = CustodyStateDeleted
			} else {
				value.Document.Payload.Entries[0].State = CustodyStateStored
			}
		}},
		{name: "signed continuation substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Payload.Continuation = more
		}},
		{name: "signing domain substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Attestation.Domain = SigningDomainQueryV1
		}},
		{name: "signer substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Attestation.Signer = other.document.Attestation.Signer
		}},
		{name: "signature substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Attestation.Signature = other.document.Attestation.Signature
		}},
		{name: "body digest substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Attestation.BodySHA256 = other.document.Attestation.BodySHA256
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input := CatalogVerification{
				Document: cloneCatalogDocument(fixture.document), Scope: fixture.scope, TrustedKeys: fixture.trusted,
			}
			tc.mutate(&input)
			got, gotErr := VerifyCatalog(input)
			if !errors.Is(gotErr, tc.wantErr) || !catalogPayloadsEqual(got, CatalogPayload{}) {
				t.Fatalf("VerifyCatalog(%s) = (%v, %v), want zero and errors.Is %v", tc.name, got, gotErr, tc.wantErr)
			}
		})
	}
}

func TestCatalogJSONPressuresValidRejectedAndExactExtentBoundaries(t *testing.T) {
	t.Parallel()

	fixture := newCatalogFixture(t, 0x51, 1)
	canonical, err := fixture.document.MarshalJSON()
	if err != nil {
		t.Fatalf("CatalogDocument.MarshalJSON() error = %v, want nil", err)
	}
	for index := range 10 {
		candidate := newCatalogFixture(t, byte(0x52+index), uint64(index+1))
		encoded, err := candidate.document.MarshalJSON()
		if err != nil {
			t.Fatalf("CatalogDocument.MarshalJSON(valid %d) error = %v, want nil", index, err)
		}
		var got CatalogDocument
		if err := got.UnmarshalJSON(encoded); err != nil || !catalogDocumentsEqual(got, candidate.document) {
			t.Fatalf("CatalogDocument.UnmarshalJSON(valid %d) = (%v, %v), want exact document and nil", index, got, err)
		}
	}

	below := padCatalogJSON(canonical, core.JSONDocumentMaximumBytes-1)
	at := padCatalogJSON(canonical, core.JSONDocumentMaximumBytes)
	above := padCatalogJSON(canonical, core.JSONDocumentMaximumBytes+1)
	for _, accepted := range [][]byte{below, at} {
		var got CatalogDocument
		if err := got.UnmarshalJSON(accepted); err != nil || !catalogDocumentsEqual(got, fixture.document) {
			t.Fatalf("CatalogDocument.UnmarshalJSON(%d-byte boundary) = (%v, %v), want exact document and nil",
				len(accepted), got, err)
		}
	}

	rejected := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: []byte{}},
		{name: "whitespace", data: []byte{' '}},
		{name: "null", data: []byte("null")},
		{name: "empty object", data: []byte("{}")},
		{name: "array", data: []byte("[]")},
		{name: "truncated", data: canonical[:len(canonical)-1]},
		{name: "trailing zero", data: append(bytes.Clone(canonical), 0)},
		{name: "trailing document", data: append(bytes.Clone(canonical), canonical...)},
		{name: "leading invalid token", data: append([]byte{'x'}, canonical...)},
		{name: "one above maximum", data: above},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			receiver := cloneCatalogDocument(fixture.document)
			before := cloneCatalogDocument(receiver)
			gotErr := receiver.UnmarshalJSON(tc.data)
			if !errors.Is(gotErr, core.ErrJSONContract) || !catalogDocumentsEqual(receiver, before) {
				t.Fatalf("CatalogDocument.UnmarshalJSON(%s) = (%v, %v), want preserved and errors.Is %v",
					tc.name, receiver, gotErr, core.ErrJSONContract)
			}
		})
	}
	var nilReceiver *CatalogDocument
	if err := nilReceiver.UnmarshalJSON(canonical); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil CatalogDocument.UnmarshalJSON() error = %v, want errors.Is %v", err, core.ErrJSONContract)
	}
}

func newCatalogFixture(t testing.TB, marker byte, version uint64) catalogFixture {
	t.Helper()

	chit := newChitFixture(t, marker, version)
	state := CustodyStateStored
	if marker%3 == 1 {
		state = CustodyStateRetrievalUnavailable
	}
	if marker%3 == 2 {
		state = CustodyStateDeleted
	}
	continuation := End()
	if marker%2 == 1 {
		var err error
		continuation, err = More(catalogCursorFixture(t, marker+1))
		if err != nil {
			t.Fatalf("More() error = %v, want nil", err)
		}
	}
	payload := CatalogPayload{
		Entries:    []CatalogEntry{{Chit: chit.document, State: state}},
		Watermark:  catalogWatermarkFixture(t, chit.scope, marker+2),
		ObservedAt: chit.document.Payload.RetainUntil,
		Scope:      chit.scope, Continuation: continuation,
	}
	document, err := IssueCatalog(CatalogIssuance{Signer: chit.private, Payload: payload})
	if err != nil {
		t.Fatalf("IssueCatalog() error = %v, want nil", err)
	}
	return catalogFixture{
		private: chit.private, trusted: chit.trusted, document: document, payload: payload, scope: chit.scope,
	}
}

func catalogCursorFixture(t testing.TB, marker byte) Cursor {
	t.Helper()
	cursor, err := NewCursor(core.SHA256Of([]byte{marker}))
	if err != nil {
		t.Fatalf("NewCursor() error = %v, want nil", err)
	}
	return cursor
}

func catalogWatermarkFixture(t testing.TB, scope receipt.Scope, marker byte) receipt.Watermark {
	t.Helper()

	generation, err := receipt.NewGeneration(uint64(marker) + 1)
	if err != nil {
		t.Fatalf("receipt.NewGeneration() error = %v, want nil", err)
	}
	cursor, err := receipt.NewCursorDigest(core.SHA256Of([]byte{marker, 1}))
	if err != nil {
		t.Fatalf("receipt.NewCursorDigest() error = %v, want nil", err)
	}
	chain, err := receipt.NewChainHash(core.SHA256Of([]byte{marker, 2}))
	if err != nil {
		t.Fatalf("receipt.NewChainHash() error = %v, want nil", err)
	}
	watermark, err := receipt.NewWatermark(receipt.WatermarkRequest{
		Generation: generation, Scope: scope, CursorDigest: cursor, ChainHash: chain,
	})
	if err != nil {
		t.Fatalf("receipt.NewWatermark() error = %v, want nil", err)
	}
	return watermark
}

func cloneCatalogDocument(document CatalogDocument) CatalogDocument {
	clone := document
	clone.Payload.Entries = append([]CatalogEntry(nil), document.Payload.Entries...)
	return clone
}

func catalogDocumentsEqual(left, right CatalogDocument) bool {
	return left.Attestation == right.Attestation && catalogPayloadsEqual(left.Payload, right.Payload)
}

func catalogPayloadsEqual(left, right CatalogPayload) bool {
	if left.Watermark != right.Watermark || left.ObservedAt != right.ObservedAt ||
		left.Scope != right.Scope || left.Continuation != right.Continuation ||
		(left.Entries == nil) != (right.Entries == nil) || len(left.Entries) != len(right.Entries) {
		return false
	}
	for index := range left.Entries {
		if left.Entries[index] != right.Entries[index] {
			return false
		}
	}
	return true
}

func padCatalogJSON(canonical []byte, target int) []byte {
	padding := bytes.Repeat([]byte{' '}, target-len(canonical))
	return append(padding, canonical...)
}
