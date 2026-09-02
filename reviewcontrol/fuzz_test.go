package reviewcontrol

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzReviewPacketSemanticClosure(f *testing.F) {
	packet := reviewPacket(f)
	canonical, err := packet.MarshalJSON()
	if err != nil {
		f.Fatalf("Packet.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, data []byte) {
		preserved := packet
		gotErr := preserved.UnmarshalJSON(data)
		if gotErr != nil {
			after, afterErr := preserved.MarshalJSON()
			if !errors.Is(gotErr, core.ErrJSONContract) || afterErr != nil || !bytes.Equal(after, canonical) {
				t.Fatalf("Packet.UnmarshalJSON(rejected) = (after=%q, decode=%v, encode=%v), want preserved %q and %v", after, gotErr, afterErr, canonical, core.ErrJSONContract)
			}
			return
		}
		encoded, err := preserved.MarshalJSON()
		if err != nil {
			t.Fatalf("Packet.MarshalJSON(accepted) error = %v, want nil", err)
		}
		var roundTrip Packet
		if err := roundTrip.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("Packet.UnmarshalJSON(round trip) error = %v, want nil", err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("Packet second projection = (%q, %v), want (%q, nil)", second, err, encoded)
		}
	})
}

func FuzzReviewEventPayloadSemanticClosure(f *testing.F) {
	packet := reviewPacket(f)
	payload := EventPayload{Kind: EventReviewIssued, Review: &packet}
	canonical, err := payload.MarshalJSON()
	if err != nil {
		f.Fatalf("EventPayload.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{"kind":"human_accepted"}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, data []byte) {
		preserved := payload
		gotErr := preserved.UnmarshalJSON(data)
		if gotErr != nil {
			after, afterErr := preserved.MarshalJSON()
			if !errors.Is(gotErr, core.ErrJSONContract) || afterErr != nil || !bytes.Equal(after, canonical) {
				t.Fatalf("EventPayload.UnmarshalJSON(rejected) = (after=%q, decode=%v, encode=%v), want preserved %q and %v", after, gotErr, afterErr, canonical, core.ErrJSONContract)
			}
			return
		}
		encoded, err := preserved.MarshalJSON()
		if err != nil {
			t.Fatalf("EventPayload.MarshalJSON(accepted) error = %v, want nil", err)
		}
		var roundTrip EventPayload
		if err := roundTrip.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("EventPayload.UnmarshalJSON(round trip) error = %v, want nil", err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("EventPayload second projection = (%q, %v), want (%q, nil)", second, err, encoded)
		}
	})
}

func FuzzReviewObservationSemanticClosure(f *testing.F) {
	packet := reviewPacket(f)
	observation := reviewObservation(f, packet)
	canonical, err := observation.MarshalJSON()
	if err != nil {
		f.Fatalf("Observation.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := observation
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			after, afterErr := got.MarshalJSON()
			if !errors.Is(gotErr, core.ErrJSONContract) || afterErr != nil || !bytes.Equal(after, canonical) {
				t.Fatalf("Observation.UnmarshalJSON(rejected) = (after=%q, decode=%v, encode=%v), want preserved %q and %v", after, gotErr, afterErr, canonical, core.ErrJSONContract)
			}
			return
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("Observation.MarshalJSON(accepted) error = %v, want nil", err)
		}
		var roundTrip Observation
		if err := roundTrip.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("Observation.UnmarshalJSON(round trip) error = %v, want nil", err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("Observation second projection = (%q, %v), want (%q, nil)", second, err, encoded)
		}
	})
}
