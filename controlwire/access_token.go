package controlwire

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"

	"github.com/deliri/primitive/v2026/core"
)

const (
	AccessTokenBytes            = 32
	AccessTokenPrefix           = "pat1_"
	AccessTokenTextBytes        = len(AccessTokenPrefix) + 2*AccessTokenBytes
	AccessTokenJSONMaximumBytes = AccessTokenTextBytes + 2
	accessTokenVerifierDomain   = "primitive/access-token/v1\x00"
)

// AccessToken is a reusable bearer secret, not a one-use RegistrationToken.
// The product decides scope, expiry, revocation and whether a purchase permits
// its use. Primitive owns only bytes, encoding, redaction and verification.
// Construct with keygen-generated material at the product's issuance boundary.
// Copies share ownership; Destroy invalidates every copied handle.
type AccessToken struct{ material core.SecretMaterial }

func NewAccessToken(value [AccessTokenBytes]byte) (AccessToken, error) {
	defer clear(value[:])
	material, err := core.NewSecretMaterial(value[:])
	if err != nil {
		return AccessToken{}, tokenError(err)
	}
	return AccessToken{material: material}, nil
}

// ParseAccessToken admits exactly one canonical token. The distinct prefix
// prevents accidental use of registration tokens at this external door.
func ParseAccessToken(text []byte) (AccessToken, error) {
	if len(text) != AccessTokenTextBytes || string(text[:len(AccessTokenPrefix)]) != AccessTokenPrefix {
		return AccessToken{}, tokenError()
	}
	var raw [AccessTokenBytes]byte
	defer clear(raw[:])
	if !isCanonicalLowercaseHex(text[len(AccessTokenPrefix):]) {
		return AccessToken{}, tokenError()
	}
	if _, err := hex.Decode(raw[:], text[len(AccessTokenPrefix):]); err != nil {
		return AccessToken{}, tokenError(err)
	}
	return NewAccessToken(raw)
}

func (t AccessToken) Validate() error {
	count, err := t.material.ByteCount()
	if err != nil {
		return tokenError(err)
	}
	size, err := count.Uint64()
	if err != nil || size != AccessTokenBytes {
		return tokenError(err)
	}
	return nil
}

func (t AccessToken) Destroy() error {
	if err := t.material.Destroy(); err != nil {
		return tokenError(err)
	}
	return nil
}

// Reveal returns an independent, caller-owned secret buffer. The caller must
// clear it after the deliberate account-to-customer or app-to-API crossing.
func (t AccessToken) Reveal() ([]byte, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	raw, err := t.material.CopyBytes()
	if err != nil {
		return nil, tokenError(err)
	}
	defer clear(raw)
	text := make([]byte, AccessTokenTextBytes)
	copy(text, AccessTokenPrefix)
	hex.Encode(text[len(AccessTokenPrefix):], raw)
	return text, nil
}

func (AccessToken) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte(core.RedactedValueText))
}

func (t AccessToken) MarshalJSON() ([]byte, error) {
	text, err := t.Reveal()
	if err != nil {
		return nil, jsonError(err)
	}
	defer clear(text)
	encoded := make([]byte, AccessTokenJSONMaximumBytes)
	encoded[0], encoded[len(encoded)-1] = '"', '"'
	copy(encoded[1:], text)
	return encoded, nil
}

// UnmarshalJSON accepts only the bounded canonical representation emitted by
// MarshalJSON. Refusal leaves the receiver unchanged. On success the caller
// still owns any prior handle: decoding does not invalidate another owner's copy.
func (t *AccessToken) UnmarshalJSON(data []byte) error {
	if t == nil || len(data) != AccessTokenJSONMaximumBytes || data[0] != '"' || data[len(data)-1] != '"' {
		return jsonError(tokenError())
	}
	parsed, err := ParseAccessToken(data[1 : len(data)-1])
	if err != nil {
		return jsonError(err)
	}
	*t = parsed
	return nil
}

// AccessTokenVerifier is safe to persist instead of a recoverable bearer.
// Its domain-separated digest cannot be confused with a registration verifier.
type AccessTokenVerifier struct{ digest core.SHA256Digest }

func (t AccessToken) Verifier() (AccessTokenVerifier, error) {
	if err := t.Validate(); err != nil {
		return AccessTokenVerifier{}, err
	}
	raw, err := t.material.CopyBytes()
	if err != nil {
		return AccessTokenVerifier{}, tokenError(err)
	}
	defer clear(raw)
	var input [len(accessTokenVerifierDomain) + AccessTokenBytes]byte
	defer clear(input[:])
	copy(input[:], accessTokenVerifierDomain)
	copy(input[len(accessTokenVerifierDomain):], raw)
	return AccessTokenVerifier{digest: core.NewSHA256Digest(sha256.Sum256(input[:]))}, nil
}

func (v AccessTokenVerifier) Validate() error {
	raw, err := v.digest.Bytes()
	if err != nil || raw == ([sha256.Size]byte{}) {
		return tokenError(err)
	}
	return nil
}

// Matches checks only token identity. It never implies account authority,
// purchase, expiry or revocation; the product must check those separately.
func (v AccessTokenVerifier) Matches(token AccessToken) (bool, error) {
	if err := v.Validate(); err != nil {
		return false, err
	}
	other, err := token.Verifier()
	if err != nil {
		return false, err
	}
	want, err := v.digest.Bytes()
	if err != nil {
		return false, tokenError(err)
	}
	got, err := other.digest.Bytes()
	if err != nil {
		return false, tokenError(err)
	}
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1, nil
}

func (v AccessTokenVerifier) MarshalJSON() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return v.digest.MarshalJSON()
}

func (v *AccessTokenVerifier) UnmarshalJSON(data []byte) error {
	if v == nil || len(data) != 2*sha256.Size+2 {
		return jsonError(tokenError())
	}
	var candidate AccessTokenVerifier
	if err := candidate.digest.UnmarshalJSON(data); err != nil {
		return jsonError(tokenError(err))
	}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*v = candidate
	return nil
}
