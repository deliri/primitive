package submission

import (
	"encoding/json"
	"errors"

	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
)

// UploadID is a caller-created UUIDv7 joining every object in one logical
// uploaded version without disclosing a repository or project identity.
type UploadID struct{ value id.UUIDv7 }

// NewUploadID closes a generated UUIDv7 into the submission namespace.
func NewUploadID(value id.UUIDv7) (UploadID, error) {
	upload := UploadID{value: value}
	if err := upload.Validate(); err != nil {
		return UploadID{}, err
	}
	return upload, nil
}

// ParseUploadID admits one canonical UUIDv7 spelling.
func ParseUploadID(value string) (UploadID, error) {
	parsed, err := id.ParseUUIDv7(value)
	if err != nil {
		return UploadID{}, contractError(err)
	}
	return NewUploadID(parsed)
}

func (i UploadID) Validate() error {
	if err := i.value.Validate(); err != nil {
		return contractError(errors.New("submission upload identity is invalid"), err)
	}
	return nil
}

func (i UploadID) String() string {
	if i.Validate() != nil {
		return ""
	}
	return i.value.String()
}

func (i UploadID) MarshalJSON() ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(i.value)
}

func (i *UploadID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil submission upload identity receiver"))
	}
	var value id.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewUploadID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

// ManifestIntent binds one declared object to its portable customer-visible
// name and exact position in one opaque collection version. It carries no
// local path, repository identity, or product-specific metadata.
type ManifestIntent struct {
	Name       chit.EntryName     `json:"name"`
	Sequence   chit.EntrySequence `json:"sequence"`
	Objects    chit.ObjectCount   `json:"objects"`
	Collection chit.CollectionID  `json:"collection_id"`
	Upload     UploadID           `json:"upload_id"`
}

func (i ManifestIntent) Validate() error {
	if err := errors.Join(
		i.Upload.Validate(), i.Collection.Validate(), i.Name.Validate(),
		i.Sequence.Validate(), i.Objects.Validate(),
	); err != nil {
		return contractError(err)
	}
	if i.Sequence.Uint64() > i.Objects.Uint64() {
		return contractError(errors.New("submission manifest sequence exceeds object count"))
	}
	return nil
}

var (
	_ core.Validatable            = UploadID{}
	_ core.Validatable            = ManifestIntent{}
	_ core.ValidatedJSONMarshaler = UploadID{}
)
