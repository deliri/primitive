package chit

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
)

type chitIdentityDomain uint8
type collectionIdentityDomain uint8

// ChitID is the authority-issued UUIDv7 for one immutable uploaded version.
type ChitID struct {
	value id.UUIDv7
	_     chitIdentityDomain
}

// CollectionID is the stable UUIDv7 joining versions of one product-owned
// evidence collection without disclosing a repository or project name.
type CollectionID struct {
	value id.UUIDv7
	_     collectionIdentityDomain
}

func NewChitID(value id.UUIDv7) (ChitID, error) {
	candidate := ChitID{value: value}
	if err := candidate.Validate(); err != nil {
		return ChitID{}, err
	}
	return candidate, nil
}

func ParseChitID(value string) (ChitID, error) {
	parsed, err := id.ParseUUIDv7(value)
	if err != nil {
		return ChitID{}, contractError(err)
	}
	return NewChitID(parsed)
}

func (i ChitID) Validate() error {
	if err := i.value.Validate(); err != nil {
		return contractError(errors.New("chit identity is invalid"), err)
	}
	return nil
}

func (i ChitID) String() string {
	if i.Validate() != nil {
		return ""
	}
	return i.value.String()
}

func (i ChitID) MarshalJSON() ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(i.value)
}

func (i *ChitID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil chit identity receiver"))
	}
	var value id.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewChitID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func NewCollectionID(value id.UUIDv7) (CollectionID, error) {
	candidate := CollectionID{value: value}
	if err := candidate.Validate(); err != nil {
		return CollectionID{}, err
	}
	return candidate, nil
}

func ParseCollectionID(value string) (CollectionID, error) {
	parsed, err := id.ParseUUIDv7(value)
	if err != nil {
		return CollectionID{}, contractError(err)
	}
	return NewCollectionID(parsed)
}

func (i CollectionID) Validate() error {
	if err := i.value.Validate(); err != nil {
		return contractError(errors.New("collection identity is invalid"), err)
	}
	return nil
}

func (i CollectionID) String() string {
	if i.Validate() != nil {
		return ""
	}
	return i.value.String()
}

func (i CollectionID) MarshalJSON() ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(i.value)
}

func (i *CollectionID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil collection identity receiver"))
	}
	var value id.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewCollectionID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

var (
	_ core.Validatable            = ChitID{}
	_ core.Validatable            = CollectionID{}
	_ core.ValidatedJSONMarshaler = ChitID{}
	_ core.ValidatedJSONMarshaler = CollectionID{}
)
