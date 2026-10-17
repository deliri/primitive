// Package aesgcm executes AES-256-GCM through Go's native cipher implementation.
// It is a whole-value operation: callers own input and returned byte extents.
// The caller owns key selection and persistence policy, including Go's limit of
// at most 2^32 messages encrypted under one key across all processes and users.
package aesgcm

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"sync"

	"github.com/deliri/primitive/v2026/core"
)

// Cipher is an opaque shared capability. Copies share destruction and may be
// used concurrently. Input slices must not be modified during an operation.
type Cipher struct{ state *cipherState }

type cipherState struct {
	mutex  sync.RWMutex
	native cipher.AEAD
}

// SealRequest borrows plaintext and authenticated, unencrypted additional data.
type SealRequest struct{ Plaintext, AdditionalData []byte }

// SealResult owns ciphertext with a native random nonce prepended and full tag.
type SealResult struct{ Ciphertext []byte }

// OpenRequest borrows a nonce-prefixed ciphertext and its exact additional data.
type OpenRequest struct{ Ciphertext, AdditionalData []byte }

// OpenResult owns plaintext released only after successful authentication.
type OpenResult struct{ Plaintext []byte }

// NewAES256GCM copies the native key schedule from exactly 32 caller-owned bytes.
// All byte patterns, including the all-zero native-valid AES key, are admitted.
func NewAES256GCM(raw []byte) (Cipher, error) {
	if len(raw) != core.AES256KeyBytes {
		return Cipher{}, core.ErrAESGCMContract
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return Cipher{}, errors.Join(core.ErrAESGCMContract, err)
	}
	native, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return Cipher{}, errors.Join(core.ErrAESGCMContract, err)
	}
	return Cipher{state: &cipherState{native: native}}, nil
}

// Validate refuses zero and destroyed capabilities.
func (c Cipher) Validate() error {
	if c.state == nil {
		return core.ErrAESGCMContract
	}
	c.state.mutex.RLock()
	defer c.state.mutex.RUnlock()
	if c.state.native == nil {
		return core.ErrAESGCMContract
	}
	return nil
}

// Seal executes native encryption with a fresh Go-generated random nonce.
// It allocates a distinct result; callers cannot choose or reuse a nonce.
func (c Cipher) Seal(in SealRequest) (SealResult, error) {
	if c.state == nil {
		return SealResult{}, core.ErrAESGCMContract
	}
	if err := validatePlaintextExtent(uint64(len(in.Plaintext)), uint64(int(^uint(0)>>1))); err != nil {
		return SealResult{}, err
	}
	c.state.mutex.RLock()
	defer c.state.mutex.RUnlock()
	if c.state.native == nil {
		return SealResult{}, core.ErrAESGCMContract
	}
	return SealResult{Ciphertext: c.state.native.Seal(nil, nil, in.Plaintext, in.AdditionalData)}, nil
}

func validatePlaintextExtent(size, nativeMaximumInt uint64) error {
	if size > core.AESGCMMaximumPlaintextBytes || nativeMaximumInt < core.AESGCMOverheadBytes || size > nativeMaximumInt-core.AESGCMOverheadBytes {
		return core.ErrAESGCMContract
	}
	return nil
}

// Open authenticates before releasing any plaintext. A refused operation returns
// a zero result and retains the native authentication error as its cause.
func (c Cipher) Open(in OpenRequest) (OpenResult, error) {
	if c.state == nil {
		return OpenResult{}, core.ErrAESGCMContract
	}
	c.state.mutex.RLock()
	defer c.state.mutex.RUnlock()
	if c.state.native == nil {
		return OpenResult{}, core.ErrAESGCMContract
	}
	plaintext, err := c.state.native.Open(nil, nil, in.Ciphertext, in.AdditionalData)
	if err != nil {
		clear(plaintext)
		return OpenResult{}, errors.Join(core.ErrAESGCMAuthentication, err)
	}
	return OpenResult{Plaintext: plaintext}, nil
}

// Destroy waits for admitted operations, invalidates all copies and drops the
// native cipher reference. Go does not expose expanded-key erasure; this method
// cannot promise physical key erasure or clear previously returned plaintext.
// Repeated destruction of a constructed capability succeeds.
func (c Cipher) Destroy() error {
	if c.state == nil {
		return core.ErrAESGCMContract
	}
	c.state.mutex.Lock()
	defer c.state.mutex.Unlock()
	c.state.native = nil
	return nil
}

// Format prevents ordinary formatting from disclosing capability internals.
func (Cipher) Format(state fmt.State, _ rune) { _, _ = state.Write([]byte(core.RedactedValueText)) }

// MarshalJSON refuses serialization of the secret capability.
func (Cipher) MarshalJSON() ([]byte, error) {
	return nil, errors.Join(core.ErrAESGCMContract, core.ErrJSONContract)
}

var _ core.ValidatedJSONMarshaler = Cipher{}
