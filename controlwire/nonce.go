package controlwire

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/keygen"
)

// RequestNonceBytes is the exact width of a control wire request nonce.
const RequestNonceBytes = 32

// RequestNonce is one public, unpredictable request identity.
//
// It is deliberately a distinct type from an installation identity even though
// both are SHA-256 wide, so the compiler refuses to substitute one for the
// other. The value is public: it names a request, it does not authorize one.
type RequestNonce struct {
	value core.SHA256Digest
}

// NewRequestNonce owns one exact nonzero nonce.
func NewRequestNonce(value [RequestNonceBytes]byte) (RequestNonce, error) {
	nonce := RequestNonce{value: core.NewSHA256Digest(value)}
	if err := nonce.Validate(); err != nil {
		return RequestNonce{}, err
	}
	return nonce, nil
}

// GenerateRequestNonce draws one nonce from Keygen's entropy substrate.
//
// Callers never assemble a nonce from their own randomness. Keeping the draw
// here means the working secret is destroyed and the copied bytes cleared on
// every path, and it keeps the width agreed with the parser instead of restated
// at each call site.
func GenerateRequestNonce() (RequestNonce, error) {
	size, err := core.NewByteCount(RequestNonceBytes)
	if err != nil {
		return RequestNonce{}, nonceError(err)
	}
	material, err := keygen.GenerateSecret(keygen.SecretRequest{Size: size})
	if err != nil {
		return RequestNonce{}, nonceError(err)
	}
	defer func() { _ = material.Destroy() }()
	raw, err := material.CopyBytes()
	if err != nil {
		return RequestNonce{}, nonceError(err)
	}
	defer clear(raw)
	var value [RequestNonceBytes]byte
	copy(value[:], raw)
	return NewRequestNonce(value)
}

// ParseRequestNonce accepts exact canonical lowercase hexadecimal.
func ParseRequestNonce(value string) (RequestNonce, error) {
	digest, err := parseCanonicalDigestText(value)
	if err != nil {
		return RequestNonce{}, nonceError(err)
	}
	nonce := RequestNonce{value: digest}
	if err := nonce.Validate(); err != nil {
		return RequestNonce{}, err
	}
	return nonce, nil
}

// Validate rejects the unset nonce and the all-zero nonce.
//
// An all-zero request identity is not unpredictable. Admitting it would let a
// replayed request read as a fresh one, so the zero value is refused at the
// type rather than at each caller.
func (n RequestNonce) Validate() error {
	if err := n.value.Validate(); err != nil {
		return nonceError(err)
	}
	raw, err := n.value.Bytes()
	if err != nil {
		return nonceError(err)
	}
	if raw == ([RequestNonceBytes]byte{}) {
		return nonceError()
	}
	return nil
}

// String returns canonical lowercase hexadecimal, or empty text when unset.
func (n RequestNonce) String() string {
	value, _ := n.value.Hex()
	return value
}

// IdempotencyKey projects the exact nonce into Exchange's bounded HTTP request
// identity, so both ends derive the header value the same way instead of each
// restating the convention.
func (n RequestNonce) IdempotencyKey() (exchange.IdempotencyKey, error) {
	if err := n.Validate(); err != nil {
		return exchange.IdempotencyKey{}, err
	}
	key, err := exchange.ParseIdempotencyKey(n.String())
	if err != nil {
		return exchange.IdempotencyKey{}, nonceError(err)
	}
	return key, nil
}

// MarshalJSON emits the canonical nonce text.
func (n RequestNonce) MarshalJSON() ([]byte, error) {
	if err := n.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(n.String())
	if err != nil {
		return nil, jsonError(nonceError(err))
	}
	return encoded, nil
}

// UnmarshalJSON accepts only canonical lowercase hexadecimal and leaves n
// unchanged on every rejection.
func (n *RequestNonce) UnmarshalJSON(data []byte) error {
	if n == nil {
		return jsonError(nonceError())
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(nonceError(err))
	}
	parsed, err := ParseRequestNonce(token)
	if err != nil {
		return jsonError(err)
	}
	*n = parsed
	return nil
}
