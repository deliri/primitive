package attest

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// TrustedKeys is an immutable bounded caller-selected authority set.
type TrustedKeys struct {
	keys  [TrustedKeyMaximumCount]core.Ed25519PublicKey
	count int
}

// NewTrustedKeys validates and copies one caller-selected authority set.
func NewTrustedKeys(request TrustedKeysRequest) (TrustedKeys, error) {
	if err := request.Validate(); err != nil {
		return TrustedKeys{}, err
	}
	var trusted TrustedKeys
	copy(trusted.keys[:], request.Keys)
	trusted.count = len(request.Keys)
	return trusted, nil
}

func validateTrustedKeyInput(keys []core.Ed25519PublicKey) error {
	if len(keys) == 0 || len(keys) > TrustedKeyMaximumCount {
		return contractError(errors.New(trustedKeyCountErrorText))
	}
	for index, key := range keys {
		if err := key.Validate(); err != nil {
			return contractError(err)
		}
		for _, prior := range keys[:index] {
			if prior == key {
				return contractError(errors.New(trustedKeyDuplicateErrorText))
			}
		}
	}
	return nil
}

// Validate proves private fixed storage is populated, distinct, and closed.
func (t TrustedKeys) Validate() error {
	count := t.count
	if count <= 0 || count > len(t.keys) {
		return contractError(errors.New(trustedKeyCountErrorText))
	}
	if err := validateTrustedKeyInput(t.keys[:count]); err != nil {
		return err
	}
	for _, key := range t.keys[count:] {
		if key != (core.Ed25519PublicKey{}) {
			return contractError(errors.New(trustedKeyStorageErrorText))
		}
	}
	return nil
}

func (t TrustedKeys) contains(key core.Ed25519PublicKey) bool {
	for _, candidate := range t.keys[:t.count] {
		if candidate == key {
			return true
		}
	}
	return false
}
