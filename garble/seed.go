package garble

import (
	"encoding/base64"
	"encoding/json"
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
	seedJSONLimitErrorText = "garble seed JSON limits are unavailable"
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
	document, err := json.Marshal(encoded)
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
	limits, err := seedJSONLimits()
	if err != nil {
		return jsonError(err)
	}
	wire, err := core.DecodeStrictJSON[seedJSONWire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	*s = wire.value
	return nil
}

type seedJSONWire struct {
	value Seed
}

func (w seedJSONWire) Validate() error {
	return w.value.Validate()
}

func (w *seedJSONWire) UnmarshalJSON(data []byte) error {
	if w == nil {
		return contractError(errors.New(seedUnsetErrorText))
	}
	var encoded string
	if err := json.Unmarshal(data, &encoded); err != nil {
		return contractError(err)
	}
	seed, err := ParseSeed(encoded)
	if err != nil {
		return err
	}
	*w = seedJSONWire{value: seed}
	return nil
}

func seedJSONLimits() (core.StrictJSONLimits, error) {
	maximum, err := core.NewByteCount(uint64(SeedJSONMaximumBytes))
	if err != nil {
		return core.StrictJSONLimits{}, errors.Join(errors.New(seedJSONLimitErrorText), err)
	}
	return core.StrictJSONLimits{
		DocumentMaximumBytes: maximum,
		NestingDepthMaximum:  1,
		ObjectFieldMaximum:   1,
		ArrayItemMaximum:     1,
	}, nil
}
