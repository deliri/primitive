package controlwire

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// RegistrationTokenBytes is the exact width of the one-time token an
	// operator copies into a product's register command.
	RegistrationTokenBytes = 32
	// registrationTokenHexSize is that token's canonical text width.
	registrationTokenHexSize = RegistrationTokenBytes * 2
)

// RegistrationToken is the one-time secret an operator copies from an
// authenticated account into a product's register command.
//
// Its bounds, redaction, and destruction are SecretMaterial's and are not
// restated here. SecretMaterial refuses to serialize through JSON on purpose,
// so the token cannot escape through a careless marshal of some enclosing
// request; the encoding below is the single deliberate place it crosses.
type RegistrationToken struct {
	material core.SecretMaterial
}

// NewRegistrationToken copies one exact-width token.
func NewRegistrationToken(value [RegistrationTokenBytes]byte) (RegistrationToken, error) {
	material, err := core.NewSecretMaterial(value[:])
	if err != nil {
		return RegistrationToken{}, tokenError(err)
	}
	token := RegistrationToken{material: material}
	if err := token.Validate(); err != nil {
		return RegistrationToken{}, err
	}
	return token, nil
}

// ParseRegistrationToken accepts exact canonical lowercase hexadecimal and does
// not retain the caller's slice.
//
// Unlike the nonce and the verifier this does not route through Core's digest
// text grammar. SHA256Digest is a public carrier: it renders itself, and
// nothing about it is redacted. An unspent enrolment secret must not enter that
// type on its way to being parsed. The bytes below live in one array this
// function wipes on every path, and canonicality is decided on the encoded text
// so no second copy of the secret is built to answer it.
func ParseRegistrationToken(value []byte) (RegistrationToken, error) {
	var decoded [RegistrationTokenBytes]byte
	defer clear(decoded[:])
	if len(value) != registrationTokenHexSize || !isCanonicalLowercaseHex(value) {
		return RegistrationToken{}, tokenError()
	}
	written, err := hex.Decode(decoded[:], value)
	if err != nil || written != RegistrationTokenBytes {
		return RegistrationToken{}, tokenError(err)
	}
	return NewRegistrationToken(decoded)
}

// isCanonicalLowercaseHex reports whether every character is a lowercase
// hexadecimal digit. Decoding stays with encoding/hex; this decides only the
// case rule that hex.Decode does not, and it decides it on the encoded text so
// the secret is never re-encoded into a string nothing can wipe.
func isCanonicalLowercaseHex(value []byte) bool {
	for _, character := range value {
		digit := character >= '0' && character <= '9'
		lower := character >= 'a' && character <= 'f'
		if !digit && !lower {
			return false
		}
	}
	return true
}

// Validate rejects an unset token and any token that is not exactly wide.
func (t RegistrationToken) Validate() error {
	if err := t.material.Validate(); err != nil {
		return tokenError(err)
	}
	count, err := t.material.ByteCount()
	if err != nil {
		return tokenError(err)
	}
	extent, err := count.Uint64()
	if err != nil || extent != RegistrationTokenBytes {
		return tokenError(err)
	}
	return nil
}

// Destroy releases the secret. A registration keeps the token only long enough
// to exchange it for a durable device credential.
func (t RegistrationToken) Destroy() error { return t.material.Destroy() }

// Verifier derives the one-way value a control plane persists. The token cannot
// be recovered from it.
func (t RegistrationToken) Verifier() (RegistrationTokenVerifier, error) {
	raw, err := t.copyBytes()
	if err != nil {
		return RegistrationTokenVerifier{}, err
	}
	defer clear(raw)
	verifier := RegistrationTokenVerifier{value: core.NewSHA256Digest(sha256.Sum256(raw))}
	if err := verifier.Validate(); err != nil {
		return RegistrationTokenVerifier{}, err
	}
	return verifier, nil
}

// MarshalJSON emits canonical lowercase hexadecimal and clears its working copy
// on every path.
func (t RegistrationToken) MarshalJSON() ([]byte, error) {
	raw, err := t.copyBytes()
	if err != nil {
		return nil, jsonError(err)
	}
	defer clear(raw)
	encoded, err := core.MarshalCanonicalJSONString(hex.EncodeToString(raw))
	if err != nil {
		return nil, jsonError(tokenError(err))
	}
	return encoded, nil
}

// UnmarshalJSON accepts only canonical lowercase hexadecimal and leaves t
// unchanged on every rejection.
func (t *RegistrationToken) UnmarshalJSON(data []byte) error {
	if t == nil {
		return jsonError(tokenError())
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(tokenError(err))
	}
	parsed, err := ParseRegistrationToken([]byte(token))
	if err != nil {
		return jsonError(err)
	}
	*t = parsed
	return nil
}

// Format redacts every verb so no log line, wrapped error, or panic can print a
// registration token.
func (RegistrationToken) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte(core.RedactedValueText))
}

func (t RegistrationToken) copyBytes() ([]byte, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	raw, err := t.material.CopyBytes()
	if err != nil {
		return nil, tokenError(err)
	}
	return raw, nil
}

// RegistrationTokenVerifier is the one-way value a control plane stores so it
// can recognise a presented token without ever holding one.
type RegistrationTokenVerifier struct {
	value core.SHA256Digest
}

// Validate rejects an unset verifier and the all-zero digest.
//
// SHA-256 has no known preimage for the all-zero digest, so that value cannot
// be the verifier of any token. It is what a blank, truncated, or
// default-initialised persisted record decodes to, and admitting it would let
// two such records recognise each other as the same enrolment. RequestNonce
// refuses its own impossible value for the same reason.
func (v RegistrationTokenVerifier) Validate() error {
	if err := v.value.Validate(); err != nil {
		return tokenError(err)
	}
	raw, err := v.value.Bytes()
	if err != nil {
		return tokenError(err)
	}
	if raw == ([sha256.Size]byte{}) {
		return tokenError()
	}
	return nil
}

// ParseRegistrationTokenVerifier accepts the exact canonical lowercase
// hexadecimal a control plane persisted. The verifier is public and one-way, so
// unlike the token it has a text ingress.
func ParseRegistrationTokenVerifier(value string) (RegistrationTokenVerifier, error) {
	digest, err := parseCanonicalDigestText(value)
	if err != nil {
		return RegistrationTokenVerifier{}, tokenError(err)
	}
	verifier := RegistrationTokenVerifier{value: digest}
	if err := verifier.Validate(); err != nil {
		return RegistrationTokenVerifier{}, err
	}
	return verifier, nil
}

// String returns canonical lowercase hexadecimal, or empty text when unset.
func (v RegistrationTokenVerifier) String() string {
	value, _ := v.value.Hex()
	return value
}

// Equal reports whether v recognises the same token as other. Both sides are
// public one-way digests, so this is an identity comparison and deliberately
// not a constant-time secret comparison.
func (v RegistrationTokenVerifier) Equal(other RegistrationTokenVerifier) bool {
	if v.Validate() != nil || other.Validate() != nil {
		return false
	}
	return v.value == other.value
}

// MarshalJSON emits the canonical verifier text.
func (v RegistrationTokenVerifier) MarshalJSON() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(v.String())
	if err != nil {
		return nil, jsonError(tokenError(err))
	}
	return encoded, nil
}

// UnmarshalJSON accepts only canonical lowercase hexadecimal and leaves v
// unchanged on every rejection.
func (v *RegistrationTokenVerifier) UnmarshalJSON(data []byte) error {
	if v == nil {
		return jsonError(tokenError())
	}
	var digest core.SHA256Digest
	if err := digest.UnmarshalJSON(data); err != nil {
		return jsonError(tokenError(err))
	}
	candidate := RegistrationTokenVerifier{value: digest}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*v = candidate
	return nil
}
