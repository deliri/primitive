package proofledger

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzEnvelopeSemanticClosure(f *testing.F) {
	genesis, _ := NewGenesisHead(fixtureLedger(f))
	event := fixtureEvent(f, genesis, 0, 1)
	canonical, err := event.MarshalJSON()
	if err != nil {
		f.Fatalf("Envelope.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, gotErr := DecodeEnvelope[ledgerTestPayload, *ledgerTestPayload](data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || got != (Envelope[ledgerTestPayload]{}) {
				t.Fatalf("DecodeEnvelope(rejected) = (%+v, %v), want zero and %v", got, gotErr, core.ErrJSONContract)
			}
			return
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("Envelope.MarshalJSON(accepted) error = %v, want nil", err)
		}
		roundTrip, err := DecodeEnvelope[ledgerTestPayload, *ledgerTestPayload](encoded)
		if err != nil || roundTrip != got || !bytes.Equal(encoded, data) {
			t.Fatalf("canonical closure = (%+v, %v, canonical=%t), want original and nil", roundTrip, err, bytes.Equal(encoded, data))
		}
	})
}
