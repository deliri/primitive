package proofledger

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const (
	proofLedgerJSONDoorLedgerIdentity uint8 = iota + 1
	proofLedgerJSONDoorEventIdentity
	proofLedgerJSONDoorSequence
	proofLedgerJSONDoorPageLimit
	proofLedgerJSONDoorReceipt
	proofLedgerJSONDoorReceiptDocument
	proofLedgerJSONDoorSigningDomain
	proofLedgerJSONDoorPosition
)

func addProofLedgerJSONSeed[D core.ValidatedJSONMarshaler](f *testing.F, door uint8, document D) {
	f.Helper()
	encoded, err := document.MarshalJSON()
	if err != nil {
		f.Fatalf("proof ledger JSON door %d MarshalJSON(seed) error = %v, want nil", door, err)
	}
	f.Add(door, encoded)
}

func proveProofLedgerJSONDoor[D interface {
	comparable
	core.ValidatedJSONMarshaler
}, P interface {
	*D
	json.Unmarshaler
}](t *testing.T, data []byte, admitted D) {
	t.Helper()
	got := admitted
	gotErr := P(&got).UnmarshalJSON(data)
	if gotErr != nil {
		if !errors.Is(gotErr, core.ErrJSONContract) || got != admitted {
			t.Fatalf("proof ledger UnmarshalJSON(rejected) = (%+v, %v), want preserved %+v and %v", got, gotErr, admitted, core.ErrJSONContract)
		}
		return
	}
	encoded, err := got.MarshalJSON()
	if err != nil {
		t.Fatalf("proof ledger MarshalJSON(accepted) error = %v, want nil", err)
	}
	var roundTrip D
	if err := P(&roundTrip).UnmarshalJSON(encoded); err != nil {
		t.Fatalf("proof ledger UnmarshalJSON(round trip) error = %v, want nil", err)
	}
	canonical, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(canonical, encoded) {
		t.Fatalf("proof ledger canonical closure = (%q, %v), want (%q, nil)", canonical, err, encoded)
	}
}

func FuzzProofLedgerExternalJSONDoorsSemanticClosure(f *testing.F) {
	genesis, _ := NewGenesisHead(fixtureLedger(f))
	event := fixtureEvent(f, genesis, 0, 1)
	document := fixtureReceiptDocument(f, event)
	limit, _ := NewPageLimit(PageEventMaximum)
	addProofLedgerJSONSeed(f, proofLedgerJSONDoorLedgerIdentity, event.Ledger)
	addProofLedgerJSONSeed(f, proofLedgerJSONDoorEventIdentity, event.Event)
	addProofLedgerJSONSeed(f, proofLedgerJSONDoorSequence, event.Sequence)
	addProofLedgerJSONSeed(f, proofLedgerJSONDoorPageLimit, limit)
	addProofLedgerJSONSeed(f, proofLedgerJSONDoorReceipt, document.Receipt)
	addProofLedgerJSONSeed(f, proofLedgerJSONDoorReceiptDocument, document)
	addProofLedgerJSONSeed(f, proofLedgerJSONDoorSigningDomain, AppendReceiptSigningDomainV1)
	addProofLedgerJSONSeed(f, proofLedgerJSONDoorPosition, genesis.Sequence)
	f.Add(proofLedgerJSONDoorSequence, []byte{})
	f.Add(proofLedgerJSONDoorReceiptDocument, []byte(`{}`))

	f.Fuzz(func(t *testing.T, door uint8, data []byte) {
		switch door {
		case proofLedgerJSONDoorLedgerIdentity:
			proveProofLedgerJSONDoor[LedgerIdentity, *LedgerIdentity](t, data, event.Ledger)
		case proofLedgerJSONDoorEventIdentity:
			proveProofLedgerJSONDoor[EventIdentity, *EventIdentity](t, data, event.Event)
		case proofLedgerJSONDoorSequence:
			proveProofLedgerJSONDoor[Sequence, *Sequence](t, data, event.Sequence)
		case proofLedgerJSONDoorPageLimit:
			proveProofLedgerJSONDoor[PageLimit, *PageLimit](t, data, limit)
		case proofLedgerJSONDoorReceipt:
			proveProofLedgerJSONDoor[AppendReceipt, *AppendReceipt](t, data, document.Receipt)
		case proofLedgerJSONDoorReceiptDocument:
			proveProofLedgerJSONDoor[AppendReceiptDocument, *AppendReceiptDocument](t, data, document)
		case proofLedgerJSONDoorSigningDomain:
			proveProofLedgerJSONDoor[AppendReceiptSigningDomain, *AppendReceiptSigningDomain](t, data, AppendReceiptSigningDomainV1)
		case proofLedgerJSONDoorPosition:
			proveProofLedgerJSONDoor[Position, *Position](t, data, genesis.Sequence)
		}
	})
}
