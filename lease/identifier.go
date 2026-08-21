package lease

import (
	"encoding/hex"
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// IdentifierBytes is the exact extent of each opaque lease identifier.
	IdentifierBytes = 16
	// IdentifierHexBytes is the exact lowercase-hex extent.
	IdentifierHexBytes = 2 * IdentifierBytes
	// IdentifierCanonicalJSONMaximumBytes is the exact compact JSON extent.
	IdentifierCanonicalJSONMaximumBytes = IdentifierHexBytes + len(`""`)
	identifierJSONWhitespaceAllowance   = 256
	// IdentifierJSONMaximumBytes bounds accepted identifier JSON.
	IdentifierJSONMaximumBytes = IdentifierCanonicalJSONMaximumBytes +
		identifierJSONWhitespaceAllowance
)

type identifier struct {
	value [IdentifierBytes]byte
}

func newIdentifier(value [IdentifierBytes]byte) (identifier, error) {
	candidate := identifier{value: value}
	return candidate, candidate.Validate()
}

func parseIdentifier(text string) (identifier, error) {
	if len(text) != IdentifierHexBytes {
		return identifier{}, contractError(errors.New("lease identifier extent is invalid"))
	}
	var value [IdentifierBytes]byte
	if err := core.DecodeCanonicalHex(value[:], text); err != nil {
		return identifier{}, contractError(errors.New("lease identifier encoding is invalid"), err)
	}
	return newIdentifier(value)
}

func (i identifier) Validate() error {
	if i.value == ([IdentifierBytes]byte{}) {
		return contractError(errors.New("lease identifier is unset"))
	}
	return nil
}

func (i identifier) String() string {
	if i.Validate() != nil {
		return ""
	}
	return hex.EncodeToString(i.value[:])
}

func marshalIdentifier(i identifier) ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(i.String())
}

func unmarshalIdentifier(data []byte) (identifier, error) {
	if len(data) == 0 || len(data) > IdentifierJSONMaximumBytes {
		return identifier{}, jsonError(errors.New("lease identifier JSON extent is invalid"))
	}
	text, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return identifier{}, jsonError(err)
	}
	value, err := parseIdentifier(text)
	if err != nil {
		return identifier{}, jsonError(err)
	}
	return value, nil
}

// EntitlementID is one opaque OGS entitlement identity.
type EntitlementID struct {
	value identifier
}

// NewEntitlementID constructs one nonzero opaque entitlement identity.
func NewEntitlementID(value [IdentifierBytes]byte) (EntitlementID, error) {
	parsed, err := newIdentifier(value)
	return EntitlementID{value: parsed}, err
}

// ParseEntitlementID parses exact lowercase hexadecimal.
func ParseEntitlementID(text string) (EntitlementID, error) {
	parsed, err := parseIdentifier(text)
	return EntitlementID{value: parsed}, err
}

// Validate rejects the unset entitlement identity.
func (i EntitlementID) Validate() error { return i.value.Validate() }

// String returns exact lowercase hexadecimal or empty for an invalid value.
func (i EntitlementID) String() string { return i.value.String() }

// MarshalJSON emits exact lowercase hexadecimal.
func (i EntitlementID) MarshalJSON() ([]byte, error) {
	return marshalIdentifier(i.value)
}

// UnmarshalJSON accepts one exact identity without mutating on rejection.
func (i *EntitlementID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("entitlement identity receiver is nil"))
	}
	value, err := unmarshalIdentifier(data)
	if err != nil {
		return err
	}
	*i = EntitlementID{value: value}
	return nil
}

// DeviceID is one opaque registered-installation identity.
type DeviceID struct {
	value identifier
}

// NewDeviceID constructs one nonzero opaque device identity.
func NewDeviceID(value [IdentifierBytes]byte) (DeviceID, error) {
	parsed, err := newIdentifier(value)
	return DeviceID{value: parsed}, err
}

// ParseDeviceID parses exact lowercase hexadecimal.
func ParseDeviceID(text string) (DeviceID, error) {
	parsed, err := parseIdentifier(text)
	return DeviceID{value: parsed}, err
}

// Validate rejects the unset device identity.
func (i DeviceID) Validate() error { return i.value.Validate() }

// String returns exact lowercase hexadecimal or empty for an invalid value.
func (i DeviceID) String() string { return i.value.String() }

// MarshalJSON emits exact lowercase hexadecimal.
func (i DeviceID) MarshalJSON() ([]byte, error) {
	return marshalIdentifier(i.value)
}

// UnmarshalJSON accepts one exact identity without mutating on rejection.
func (i *DeviceID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("device identity receiver is nil"))
	}
	value, err := unmarshalIdentifier(data)
	if err != nil {
		return err
	}
	*i = DeviceID{value: value}
	return nil
}
