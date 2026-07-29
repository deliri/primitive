package timeproof

import (
	"crypto/subtle"
	"encoding/hex"
	"math/big"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

// nonceJSONMaximumBytes bounds the quoted canonical nonce token.
const nonceJSONMaximumBytes = 2*NonceBytes + 2

// Nonce is an exact nonzero 128-bit RFC 3161 nonce.
type Nonce struct {
	value [NonceBytes]byte
}

func generateNonce() (Nonce, error) {
	size, err := core.NewByteCount(NonceBytes)
	if err != nil {
		return Nonce{}, contractError(err)
	}
	material, err := keygen.GenerateSecret(keygen.SecretRequest{Size: size})
	if err != nil {
		return Nonce{}, contractError(err)
	}
	defer func() { _ = material.Destroy() }()
	raw, err := material.CopyBytes()
	if err != nil {
		return Nonce{}, contractError(err)
	}
	defer clear(raw)
	var nonce Nonce
	copy(nonce.value[:], raw)
	if err := nonce.Validate(); err != nil {
		return Nonce{}, err
	}
	return nonce, nil
}

func parseNonce(value string) (Nonce, error) {
	if len(value) != 2*NonceBytes {
		return Nonce{}, contractError(nil)
	}
	raw, err := hex.DecodeString(value)
	if err != nil || hex.EncodeToString(raw) != value {
		return Nonce{}, contractError(err)
	}
	defer clear(raw)
	var nonce Nonce
	copy(nonce.value[:], raw)
	if err := nonce.Validate(); err != nil {
		return Nonce{}, err
	}
	return nonce, nil
}

// Validate rejects the unavoidable all-zero value.
func (n Nonce) Validate() error {
	var combined byte
	for _, value := range n.value {
		combined |= value
	}
	if combined == 0 {
		return contractError(nil)
	}
	return nil
}

// String returns canonical lowercase hexadecimal.
func (n Nonce) String() string {
	if err := n.Validate(); err != nil {
		return ""
	}
	return hex.EncodeToString(n.value[:])
}

func (n Nonce) integer() *big.Int {
	return new(big.Int).SetBytes(n.value[:])
}

func (n Nonce) matches(value *big.Int) bool {
	if value == nil || value.Sign() <= 0 || value.BitLen() > NonceBytes*8 {
		return false
	}
	want := n.integer().FillBytes(make([]byte, NonceBytes))
	got := value.FillBytes(make([]byte, NonceBytes))
	return subtle.ConstantTimeCompare(want, got) == 1
}

// MarshalJSON emits canonical lowercase hexadecimal.
func (n Nonce) MarshalJSON() ([]byte, error) {
	if err := n.Validate(); err != nil {
		return nil, err
	}
	return []byte(`"` + n.String() + `"`), nil
}

// UnmarshalJSON accepts only exact canonical lowercase hexadecimal.
func (n *Nonce) UnmarshalJSON(data []byte) error {
	if n == nil {
		return errorsJSON()
	}
	token, err := decodeJSONToken(data, nonceJSONMaximumBytes)
	if err != nil {
		return err
	}
	parsed, err := parseNonce(token)
	if err != nil {
		return errorsJSON()
	}
	*n = parsed
	return nil
}
