package reviewcontrol

import (
	json "encoding/json/v2"

	"github.com/deliri/primitive/v2026/core"
)

type packetWire Packet
type observationWire Observation
type decisionIntentWire DecisionIntent
type eventPayloadWire EventPayload

func marshalDocument[T core.Validatable, W any](value T, wire W, maximum int) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(wire)
	if err != nil || len(encoded) > maximum {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func validateEncodedDocument[W any](wire W, maximum int) error {
	encoded, err := core.MarshalCanonicalJSONDocument(wire)
	if err != nil || len(encoded) > maximum {
		return jsonError(err)
	}
	return nil
}

func (p Packet) MarshalJSON() ([]byte, error) {
	return marshalDocument(p, packetWire(p), PacketJSONMaximumBytes)
}
func (o Observation) MarshalJSON() ([]byte, error) {
	return marshalDocument(o, observationWire(o), ObservationJSONMaximumBytes)
}
func (d DecisionIntent) MarshalJSON() ([]byte, error) {
	return marshalDocument(d, decisionIntentWire(d), DecisionJSONMaximumBytes)
}
func (p EventPayload) MarshalJSON() ([]byte, error) {
	return marshalDocument(p, eventPayloadWire(p), EventPayloadJSONMaximumBytes)
}

func decodePacket(data []byte) (Packet, error) {
	w, e := core.DecodeStrictJSONStructure[packetWire](data, core.DefaultStrictJSONLimits())
	c := Packet(w)
	if e != nil {
		return Packet{}, jsonError(e)
	}
	if e = c.Validate(); e != nil {
		return Packet{}, jsonError(e)
	}
	return c, nil
}
func decodeObservation(data []byte) (Observation, error) {
	w, e := core.DecodeStrictJSONStructure[observationWire](data, core.DefaultStrictJSONLimits())
	c := Observation(w)
	if e != nil {
		return Observation{}, jsonError(e)
	}
	if e = c.Validate(); e != nil {
		return Observation{}, jsonError(e)
	}
	return c, nil
}
func decodeDecisionIntent(data []byte) (DecisionIntent, error) {
	w, e := core.DecodeStrictJSONStructure[decisionIntentWire](data, core.DefaultStrictJSONLimits())
	c := DecisionIntent(w)
	if e != nil {
		return DecisionIntent{}, jsonError(e)
	}
	if e = c.Validate(); e != nil {
		return DecisionIntent{}, jsonError(e)
	}
	return c, nil
}
func decodeEventPayload(data []byte) (EventPayload, error) {
	w, e := core.DecodeStrictJSONStructure[eventPayloadWire](data, core.DefaultStrictJSONLimits())
	c := EventPayload(w)
	if e != nil {
		return EventPayload{}, jsonError(e)
	}
	if e = c.Validate(); e != nil {
		return EventPayload{}, jsonError(e)
	}
	return c, nil
}

func (p *Packet) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError()
	}
	c, e := decodePacket(data)
	if e == nil {
		*p = c
	}
	return e
}
func (o *Observation) UnmarshalJSON(data []byte) error {
	if o == nil {
		return jsonError()
	}
	c, e := decodeObservation(data)
	if e == nil {
		*o = c
	}
	return e
}
func (d *DecisionIntent) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError()
	}
	c, e := decodeDecisionIntent(data)
	if e == nil {
		*d = c
	}
	return e
}
func (p *EventPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError()
	}
	c, e := decodeEventPayload(data)
	if e == nil {
		*p = c
	}
	return e
}

var _ json.Unmarshaler = (*Packet)(nil)
