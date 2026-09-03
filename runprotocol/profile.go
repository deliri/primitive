package runprotocol

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// ProfileIdentity binds evidence to one named, versioned execution profile.
// Changing the profile version necessarily creates a different time series.
type ProfileIdentity struct {
	Name    Identifier `json:"name"`
	Version uint32     `json:"version"`
}

// NewProfileIdentity constructs one exact profile identity.
func NewProfileIdentity(name Identifier, version uint32) (ProfileIdentity, error) {
	candidate := ProfileIdentity{Name: name, Version: version}
	if err := candidate.Validate(); err != nil {
		return ProfileIdentity{}, err
	}
	return candidate, nil
}

func (p ProfileIdentity) Validate() error {
	if p.Version == 0 {
		return contractError(errors.New("run protocol profile version is zero"))
	}
	return p.Name.Validate()
}

type profileIdentityWire ProfileIdentity

func (p ProfileIdentity) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(profileIdentityWire(p))
	if err != nil {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p *ProfileIdentity) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil run protocol profile identity receiver"))
	}
	wire, err := core.DecodeStrictJSONStructure[profileIdentityWire](data, aboutJSONLimits())
	if err != nil {
		return jsonError(err)
	}
	candidate := ProfileIdentity(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}
