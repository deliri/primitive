package attest

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// Signature is an owned, set Ed25519 signature.
type Signature struct {
	value [ed25519.SignatureSize]byte
	set   bool
}

func newSignature(value []byte) (Signature, error) {
	if len(value) != ed25519.SignatureSize {
		return Signature{}, contractError(errors.New(signatureLengthErrorText))
	}
	var signature Signature
	copy(signature.value[:], value)
	signature.set = true
	return signature, nil
}

// Validate rejects an unset signature.
func (s Signature) Validate() error {
	if !s.set {
		return contractError(errors.New(signatureUnsetErrorText))
	}
	return nil
}

// Bytes returns a fixed-size copy.
func (s Signature) Bytes() ([ed25519.SignatureSize]byte, error) {
	if err := s.Validate(); err != nil {
		return [ed25519.SignatureSize]byte{}, err
	}
	return s.value, nil
}

// Hex returns canonical lowercase hexadecimal.
func (s Signature) Hex() (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	return hex.EncodeToString(s.value[:]), nil
}

// MarshalJSON emits canonical lowercase hexadecimal.
func (s Signature) MarshalJSON() ([]byte, error) {
	value, err := s.Hex()
	if err != nil {
		return nil, envelopeJSONError(err)
	}
	return core.MarshalCanonicalJSONString(value)
}

// UnmarshalJSON accepts one exact lowercase hexadecimal signature.
func (s *Signature) UnmarshalJSON(data []byte) error {
	if s == nil {
		return envelopeJSONError(errors.New(signatureUnsetErrorText))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return envelopeJSONError(err)
	}
	if len(value) != hex.EncodedLen(ed25519.SignatureSize) {
		return envelopeJSONError(errors.New(signatureLengthErrorText))
	}
	decoded := make([]byte, len(value)/2)
	if err := core.DecodeCanonicalHex(decoded, value); err != nil {
		return envelopeJSONError(errors.New(signatureEncodingErrorText))
	}
	candidate, err := newSignature(decoded)
	if err != nil {
		return envelopeJSONError(err)
	}
	*s = candidate
	return nil
}
