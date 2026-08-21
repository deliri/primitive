package lease

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// SubjectCanonicalJSONMaximumBytes is the exact maximum compact subject extent.
	SubjectCanonicalJSONMaximumBytes = len(`{"offering":,"entitlement_id":,"device_id":}`) +
		core.OfferingCanonicalJSONMaximumBytes + 2*IdentifierCanonicalJSONMaximumBytes
	subjectJSONWhitespaceAllowance = 1 << 10
	// SubjectJSONMaximumBytes bounds accepted subject JSON.
	SubjectJSONMaximumBytes = SubjectCanonicalJSONMaximumBytes +
		subjectJSONWhitespaceAllowance
)

// Subject binds one decision to an exact offering, entitlement, and registered
// installation.
type Subject struct {
	Offering      core.Offering `json:"offering"`
	EntitlementID EntitlementID `json:"entitlement_id"`
	DeviceID      DeviceID      `json:"device_id"`
}

// Validate closes every subject component.
func (s Subject) Validate() error {
	if err := s.Offering.Validate(); err != nil {
		return contractError(err)
	}
	if err := s.EntitlementID.Validate(); err != nil {
		return contractError(err)
	}
	if err := s.DeviceID.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// MarshalJSON emits the exact subject field order.
func (s Subject) MarshalJSON() ([]byte, error) {
	type wire Subject
	if err := s.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(wire(s))
	if err != nil || len(encoded) > SubjectCanonicalJSONMaximumBytes {
		return nil, jsonError(errors.New("subject JSON encoding exceeded its contract"), err)
	}
	return encoded, nil
}

// UnmarshalJSON accepts one bounded strict subject without mutation on
// rejection.
func (s *Subject) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(errors.New("subject receiver is nil"))
	}
	type wire Subject
	limits, err := (jsonStructureContract{
		maximumBytes: SubjectJSONMaximumBytes,
		depth:        1,
		fields:       3,
	}).limits()
	if err != nil {
		return err
	}
	decoded, err := core.DecodeStrictJSONStructure[wire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	candidate := Subject(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*s = candidate
	return nil
}
