package garble

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// SeedBytes is the exact Garble seed width.
	SeedBytes = 8
	// SeedEncodedBytes is the exact unpadded standard-Base64 seed width.
	SeedEncodedBytes = 11
	// SeedCanonicalJSONBytes is the exact canonical seed JSON extent.
	SeedCanonicalJSONBytes = SeedEncodedBytes + len(`""`)
	// SeedJSONWhitespaceAllowanceBytes bounds insignificant JSON whitespace.
	SeedJSONWhitespaceAllowanceBytes = 256
	// SeedJSONMaximumBytes bounds one accepted seed JSON document.
	SeedJSONMaximumBytes = SeedCanonicalJSONBytes + SeedJSONWhitespaceAllowanceBytes
)

const (
	seedEncodingErrorText  = "garble seed is not canonical unpadded standard base64"
	seedUnsetErrorText     = "garble seed is unset"
	seedJSONLimitErrorText = "garble seed JSON exceeds its byte limit"
)

// Seed is one set, exact eight-byte Garble seed. The all-zero value produced
// by NewSeed is valid; the unset Go zero value is not.
type Seed struct {
	value [SeedBytes]byte
	set   bool
}

// NewSeed constructs a set seed from all eight bytes.
func NewSeed(value [SeedBytes]byte) Seed {
	return Seed{value: value, set: true}
}

// ParseSeed accepts only exact unpadded standard Base64. It deliberately
// rejects Garble CLI forms that the upstream parser normalizes or truncates.
func ParseSeed(value string) (Seed, error) {
	if len(value) != SeedEncodedBytes {
		return Seed{}, contractError(errors.New(seedEncodingErrorText))
	}
	decoded, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil ||
		len(decoded) != SeedBytes ||
		base64.RawStdEncoding.EncodeToString(decoded) != value {
		return Seed{}, contractError(errors.New(seedEncodingErrorText), err)
	}
	var fixed [SeedBytes]byte
	copy(fixed[:], decoded)
	clear(decoded)
	return NewSeed(fixed), nil
}

// Validate rejects an unset seed while admitting every set eight-byte value.
func (s Seed) Validate() error {
	if !s.set {
		return contractError(errors.New(seedUnsetErrorText))
	}
	return nil
}

// Bytes returns the exact seed bytes after validating that the value is set.
func (s Seed) Bytes() ([SeedBytes]byte, error) {
	if err := s.Validate(); err != nil {
		return [SeedBytes]byte{}, err
	}
	return s.value, nil
}

// Encoded returns exact unpadded standard Base64.
func (s Seed) Encoded() (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(s.value[:]), nil
}

// Format prevents accidental seed disclosure through formatting.
func (s Seed) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// MarshalJSON emits the exact canonical unpadded standard-Base64 string.
func (s Seed) MarshalJSON() ([]byte, error) {
	encoded, err := s.Encoded()
	if err != nil {
		return nil, jsonError(err)
	}
	document, err := core.MarshalCanonicalJSONString(encoded)
	if err != nil {
		return nil, jsonError(err)
	}
	return document, nil
}

// UnmarshalJSON accepts bounded strict JSON and preserves the receiver on
// failure.
func (s *Seed) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(errors.New(seedUnsetErrorText))
	}
	if len(data) > SeedJSONMaximumBytes {
		return jsonError(errors.New(seedJSONLimitErrorText))
	}
	encoded, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	seed, err := ParseSeed(encoded)
	if err != nil {
		return jsonError(err)
	}
	*s = seed
	return nil
}
