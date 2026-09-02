package reviewcontrol

import (
	json "encoding/json/v2"

	primitiveid "github.com/deliri/primitive/v2026/id"
)

type uuidIdentity struct{ value primitiveid.UUIDv7 }

type ReviewIdentity struct{ uuidIdentity }
type ContractIdentity struct{ uuidIdentity }
type ObservationIdentity struct{ uuidIdentity }
type FindingIdentity struct{ uuidIdentity }
type ReviewerIdentity struct{ uuidIdentity }
type PrincipalIdentity struct{ uuidIdentity }
type AuthorityIdentity struct{ uuidIdentity }

func (i ReviewIdentity) Validate() error      { return i.uuidIdentity.Validate() }
func (i ContractIdentity) Validate() error    { return i.uuidIdentity.Validate() }
func (i ObservationIdentity) Validate() error { return i.uuidIdentity.Validate() }
func (i FindingIdentity) Validate() error     { return i.uuidIdentity.Validate() }
func (i ReviewerIdentity) Validate() error    { return i.uuidIdentity.Validate() }
func (i PrincipalIdentity) Validate() error   { return i.uuidIdentity.Validate() }
func (i AuthorityIdentity) Validate() error   { return i.uuidIdentity.Validate() }

func newUUIDIdentity(value primitiveid.UUIDv7) (uuidIdentity, error) {
	candidate := uuidIdentity{value: value}
	if err := candidate.Validate(); err != nil {
		return uuidIdentity{}, err
	}
	return candidate, nil
}

func (i uuidIdentity) Validate() error {
	if err := i.value.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func (i uuidIdentity) String() string { return i.value.String() }

func makeReviewIdentity(value primitiveid.UUIDv7) (ReviewIdentity, error) {
	i, err := newUUIDIdentity(value)
	return ReviewIdentity{i}, err
}

func NewReviewIdentity(value primitiveid.UUIDv7) (ReviewIdentity, error) {
	return makeReviewIdentity(value)
}
func NewContractIdentity(value primitiveid.UUIDv7) (ContractIdentity, error) {
	i, err := newUUIDIdentity(value)
	return ContractIdentity{i}, err
}
func NewObservationIdentity(value primitiveid.UUIDv7) (ObservationIdentity, error) {
	i, err := newUUIDIdentity(value)
	return ObservationIdentity{i}, err
}
func NewFindingIdentity(value primitiveid.UUIDv7) (FindingIdentity, error) {
	i, err := newUUIDIdentity(value)
	return FindingIdentity{i}, err
}
func NewReviewerIdentity(value primitiveid.UUIDv7) (ReviewerIdentity, error) {
	i, err := newUUIDIdentity(value)
	return ReviewerIdentity{i}, err
}
func NewPrincipalIdentity(value primitiveid.UUIDv7) (PrincipalIdentity, error) {
	i, err := newUUIDIdentity(value)
	return PrincipalIdentity{i}, err
}
func NewAuthorityIdentity(value primitiveid.UUIDv7) (AuthorityIdentity, error) {
	i, err := newUUIDIdentity(value)
	return AuthorityIdentity{i}, err
}

func marshalUUIDIdentity(i uuidIdentity) ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return i.value.MarshalJSON()
}

func decodeUUIDIdentity(data []byte) (uuidIdentity, error) {
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return uuidIdentity{}, jsonError(err)
	}
	i, err := newUUIDIdentity(value)
	if err != nil {
		return uuidIdentity{}, jsonError(err)
	}
	return i, nil
}

func (i ReviewIdentity) MarshalJSON() ([]byte, error)   { return marshalUUIDIdentity(i.uuidIdentity) }
func (i ContractIdentity) MarshalJSON() ([]byte, error) { return marshalUUIDIdentity(i.uuidIdentity) }
func (i ObservationIdentity) MarshalJSON() ([]byte, error) {
	return marshalUUIDIdentity(i.uuidIdentity)
}
func (i FindingIdentity) MarshalJSON() ([]byte, error)   { return marshalUUIDIdentity(i.uuidIdentity) }
func (i ReviewerIdentity) MarshalJSON() ([]byte, error)  { return marshalUUIDIdentity(i.uuidIdentity) }
func (i PrincipalIdentity) MarshalJSON() ([]byte, error) { return marshalUUIDIdentity(i.uuidIdentity) }
func (i AuthorityIdentity) MarshalJSON() ([]byte, error) { return marshalUUIDIdentity(i.uuidIdentity) }

func unmarshalUUID(data []byte, destination *uuidIdentity) error {
	if destination == nil {
		return jsonError()
	}
	candidate, err := decodeUUIDIdentity(data)
	if err != nil {
		return err
	}
	*destination = candidate
	return nil
}

func (i *ReviewIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError()
	}
	return unmarshalUUID(data, &i.uuidIdentity)
}
func (i *ContractIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError()
	}
	return unmarshalUUID(data, &i.uuidIdentity)
}
func (i *ObservationIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError()
	}
	return unmarshalUUID(data, &i.uuidIdentity)
}
func (i *FindingIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError()
	}
	return unmarshalUUID(data, &i.uuidIdentity)
}
func (i *ReviewerIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError()
	}
	return unmarshalUUID(data, &i.uuidIdentity)
}
func (i *PrincipalIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError()
	}
	return unmarshalUUID(data, &i.uuidIdentity)
}
func (i *AuthorityIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError()
	}
	return unmarshalUUID(data, &i.uuidIdentity)
}
