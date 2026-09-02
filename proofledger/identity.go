package proofledger

import (
	json "encoding/json/v2"

	primitiveid "github.com/deliri/primitive/v2026/id"
)

type LedgerIdentity struct{ value primitiveid.UUIDv7 }
type EventIdentity struct{ value primitiveid.UUIDv7 }

func NewLedgerIdentity(value primitiveid.UUIDv7) (LedgerIdentity, error) {
	candidate := LedgerIdentity{value: value}
	return candidate, candidate.Validate()
}

func NewEventIdentity(value primitiveid.UUIDv7) (EventIdentity, error) {
	candidate := EventIdentity{value: value}
	return candidate, candidate.Validate()
}

func (i LedgerIdentity) Validate() error { return identityError(i.value) }
func (i EventIdentity) Validate() error  { return identityError(i.value) }
func (i LedgerIdentity) String() string  { return i.value.String() }
func (i EventIdentity) String() string   { return i.value.String() }

func identityError(value primitiveid.UUIDv7) error {
	if err := value.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func (i LedgerIdentity) MarshalJSON() ([]byte, error) { return marshalIdentity(i.value, i.Validate) }
func (i EventIdentity) MarshalJSON() ([]byte, error)  { return marshalIdentity(i.value, i.Validate) }

func marshalIdentity(value primitiveid.UUIDv7, validate func() error) ([]byte, error) {
	if err := validate(); err != nil {
		return nil, jsonError(err)
	}
	return value.MarshalJSON()
}

func (i *LedgerIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError()
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewLedgerIdentity(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func (i *EventIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError()
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewEventIdentity(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}
