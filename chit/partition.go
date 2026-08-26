package chit

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// Partition is an opaque nonzero commitment supplied by product policy to
// bound one custody catalog without disclosing what the partition means.
type Partition struct {
	digest core.SHA256Digest
}

// NewPartition closes one product-owned commitment into Chit's blind catalog
// namespace.
func NewPartition(digest core.SHA256Digest) (Partition, error) {
	candidate := Partition{digest: digest}
	if err := candidate.Validate(); err != nil {
		return Partition{}, err
	}
	return candidate, nil
}

// Validate requires a set, nonzero SHA-256 commitment.
func (p Partition) Validate() error {
	raw, err := p.digest.Bytes()
	if err != nil {
		return contractError(errors.New("chit partition commitment is unset"), err)
	}
	for _, value := range raw {
		if value != 0 {
			return nil
		}
	}
	return contractError(errors.New("chit partition commitment is all zero"))
}

// MarshalJSON emits the commitment as canonical lowercase hexadecimal.
func (p Partition) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := p.digest.MarshalJSON()
	if err != nil {
		return nil, jsonError(err)
	}
	return encoded, nil
}

// UnmarshalJSON admits one canonical nonzero commitment and preserves the
// receiver on every refusal.
func (p *Partition) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil chit partition receiver"))
	}
	var digest core.SHA256Digest
	if err := json.Unmarshal(data, &digest); err != nil {
		return jsonError(err)
	}
	candidate, err := NewPartition(digest)
	if err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

var (
	_ core.Validatable            = Partition{}
	_ core.ValidatedJSONMarshaler = Partition{}
	_ json.Unmarshaler            = (*Partition)(nil)
)
