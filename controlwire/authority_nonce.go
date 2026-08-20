package controlwire

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

// AuthorityNonce is one public unpredictable identity for an authority's
// grant decision. It is nominally distinct from the request nonce it answers.
type AuthorityNonce struct {
	value core.SHA256Digest
}

func NewAuthorityNonce(value [core.SHA256DigestBytes]byte) (AuthorityNonce, error) {
	nonce := AuthorityNonce{value: core.NewSHA256Digest(value)}
	if err := nonce.Validate(); err != nil {
		return AuthorityNonce{}, err
	}
	return nonce, nil
}

// GenerateAuthorityNonce owns the entropy draw for an issuing authority.
func GenerateAuthorityNonce() (AuthorityNonce, error) {
	size, err := core.NewByteCount(core.SHA256DigestBytes)
	if err != nil {
		return AuthorityNonce{}, nonceError(err)
	}
	material, err := keygen.GenerateSecret(keygen.SecretRequest{Size: size})
	if err != nil {
		return AuthorityNonce{}, nonceError(err)
	}
	defer func() { _ = material.Destroy() }()
	raw, err := material.CopyBytes()
	if err != nil {
		return AuthorityNonce{}, nonceError(err)
	}
	defer clear(raw)
	var value [core.SHA256DigestBytes]byte
	copy(value[:], raw)
	return NewAuthorityNonce(value)
}

func ParseAuthorityNonce(value string) (AuthorityNonce, error) {
	digest, err := parseCanonicalDigestText(value)
	if err != nil {
		return AuthorityNonce{}, nonceError(err)
	}
	nonce := AuthorityNonce{value: digest}
	if err := nonce.Validate(); err != nil {
		return AuthorityNonce{}, err
	}
	return nonce, nil
}

func (n AuthorityNonce) Validate() error {
	if err := n.value.Validate(); err != nil {
		return nonceError(err)
	}
	raw, err := n.value.Bytes()
	if err != nil {
		return nonceError(err)
	}
	if raw == ([core.SHA256DigestBytes]byte{}) {
		return nonceError()
	}
	return nil
}

func (n AuthorityNonce) String() string {
	value, _ := n.value.Hex()
	return value
}

func (n AuthorityNonce) MarshalJSON() ([]byte, error) {
	if err := n.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(n.String())
	if err != nil {
		return nil, jsonError(nonceError(err))
	}
	return encoded, nil
}

func (n *AuthorityNonce) UnmarshalJSON(data []byte) error {
	if n == nil {
		return jsonError(nonceError())
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(nonceError(err))
	}
	parsed, err := ParseAuthorityNonce(token)
	if err != nil {
		return jsonError(err)
	}
	*n = parsed
	return nil
}

var (
	_ core.Validatable            = AuthorityNonce{}
	_ core.ValidatedJSONMarshaler = AuthorityNonce{}
)
