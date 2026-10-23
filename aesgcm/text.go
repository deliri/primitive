package aesgcm

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

// SealTextRequest selects the caller's actual transport capacity. Text and
// plaintext are whole values; this is not an unbounded-stream encryption API.
type SealTextRequest struct {
	Plaintext        []byte
	AdditionalData   []byte
	MaximumTextBytes core.ByteCount
}

// OpenTextRequest admits encrypted representation data at this public decoder.
// The caller's capacity is checked before decoding or native authentication.
type OpenTextRequest struct {
	Ciphertext       string
	AdditionalData   []byte
	MaximumTextBytes core.ByteCount
}

// SealedText is canonical unpadded URL-safe ciphertext, including the native
// nonce and tag. String explicitly discloses it for caller-owned transport.
type SealedText struct{ value string }

func (s SealedText) String() string { return s.value }

func (SealedText) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func (s SealedText) Validate() error {
	if len(s.value) < base64.RawURLEncoding.EncodedLen(core.AESGCMOverheadBytes) || strings.ContainsAny(s.value, "\r\n") {
		return core.ErrAESGCMContract
	}
	// Go owns the grammar. Streaming validation avoids a second retained copy
	// when a caller selected a transport larger than an HTTP cookie.
	_, err := io.Copy(io.Discard, base64.NewDecoder(base64.RawURLEncoding.Strict(), strings.NewReader(s.value)))
	if err != nil {
		return errors.Join(core.ErrAESGCMContract, err)
	}
	return nil
}

func validateSealedTextExtent(size uint64, maximum core.ByteCount, nativeMaximumInt uint64) error {
	limit, err := maximum.Uint64()
	if err != nil {
		return errors.Join(core.ErrAESGCMContract, err)
	}
	if err := validatePlaintextExtent(size, nativeMaximumInt); err != nil {
		return err
	}
	bytes := size + core.AESGCMOverheadBytes
	// Native GCM's already-checked extent makes these wide additions safe.
	encoded := bytes/3*4 + (bytes%3*8+5)/6
	if encoded > limit || encoded > nativeMaximumInt {
		return core.ErrAESGCMTextExtent
	}
	return nil
}

// SealText refuses an oversized transport value before random nonce generation
// or encryption. Go's RawURL encoder supplies the one canonical representation.
func (c Cipher) SealText(in SealTextRequest) (SealedText, error) {
	if err := validateSealedTextExtent(uint64(len(in.Plaintext)), in.MaximumTextBytes, uint64(int(^uint(0)>>1))); err != nil {
		return SealedText{}, err
	}
	sealed, err := c.Seal(SealRequest{Plaintext: in.Plaintext, AdditionalData: in.AdditionalData})
	if err != nil {
		return SealedText{}, err
	}
	defer clear(sealed.Ciphertext)
	return SealedText{value: base64.RawURLEncoding.EncodeToString(sealed.Ciphertext)}, nil
}

// OpenText rejects noncanonical spellings, including padding, line breaks and
// nonzero spare bits. Only successful native GCM authentication releases bytes.
func (c Cipher) OpenText(in OpenTextRequest) (OpenResult, error) {
	maximum, err := in.MaximumTextBytes.Uint64()
	if err != nil {
		return OpenResult{}, errors.Join(core.ErrAESGCMContract, err)
	}
	if uint64(len(in.Ciphertext)) > maximum {
		return OpenResult{}, core.ErrAESGCMTextExtent
	}
	if in.Ciphertext == "" || strings.ContainsAny(in.Ciphertext, "\r\n") {
		return OpenResult{}, core.ErrAESGCMContract
	}
	if err := c.Validate(); err != nil {
		return OpenResult{}, err
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(in.Ciphertext)
	if err != nil {
		clear(decoded)
		return OpenResult{}, errors.Join(core.ErrAESGCMContract, err)
	}
	defer clear(decoded)
	return c.Open(OpenRequest{Ciphertext: decoded, AdditionalData: in.AdditionalData})
}
