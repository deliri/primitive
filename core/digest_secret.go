package core

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
)

const (
	// RedactedValueText is the only text emitted when formatting secret material.
	RedactedValueText = "[REDACTED]"
	// SecretMaterialMinimumBytes is the minimum admitted material length.
	SecretMaterialMinimumBytes = 16
	// SecretMaterialMaximumBytes is the maximum admitted material length.
	SecretMaterialMaximumBytes = 64
	// crc32CBytes is the width of a CRC32C checksum.
	crc32CBytes = 4
	// crc32CBase64Bytes is the canonical padded Base64 width of CRC32C.
	crc32CBase64Bytes             = 8
	crc32CNilReceiverDiagnostic   = "nil CRC32C receiver"
	secretMaterialLengthErrorText = "secret material has invalid length"
	secretMaterialUnsetErrorText  = "secret material is unset"
	secretMaterialJSONErrorText   = "secret material JSON serialization is prohibited"
)

// SHA256Digest is a set SHA-256 digest. Its zero value is invalid.
type SHA256Digest struct {
	value [sha256.Size]byte
	set   bool
}

// NewSHA256Digest constructs a set digest from all 32 bytes.
func NewSHA256Digest(value [sha256.Size]byte) SHA256Digest {
	return SHA256Digest{value: value, set: true}
}

// parseSHA256Hex accepts exactly 64 canonical lowercase hexadecimal bytes.
func parseSHA256Hex(value string) (SHA256Digest, error) {
	decoded, err := decodeCanonicalHex(value, sha256.Size)
	if err != nil {
		return SHA256Digest{}, errors.Join(ErrPrimitiveContract, err)
	}
	var digest [sha256.Size]byte
	copy(digest[:], decoded)
	return NewSHA256Digest(digest), nil
}

// Bytes returns the digest bytes after validating that the value is set.
func (d SHA256Digest) Bytes() ([sha256.Size]byte, error) {
	if err := d.Validate(); err != nil {
		return [sha256.Size]byte{}, err
	}
	return d.value, nil
}

// Hex returns canonical lowercase hexadecimal.
func (d SHA256Digest) Hex() (string, error) {
	if err := d.Validate(); err != nil {
		return "", err
	}
	return hex.EncodeToString(d.value[:]), nil
}

// Validate rejects an unset digest.
func (d SHA256Digest) Validate() error {
	if !d.set {
		return errors.Join(ErrPrimitiveContract, errors.New("sha-256 digest is unset"))
	}
	return nil
}

// MarshalJSON emits canonical lowercase hexadecimal as a JSON string.
func (d SHA256Digest) MarshalJSON() ([]byte, error) {
	value, err := d.Hex()
	if err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value)
}

// UnmarshalJSON accepts only canonical lowercase hexadecimal.
func (d *SHA256Digest) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(ErrJSONContract, errors.New("nil SHA-256 digest receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	decoded, err := parseSHA256Hex(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*d = decoded
	return nil
}

// CRC32C is a set CRC32C checksum. Its zero value is invalid.
type CRC32C struct {
	value uint32
	set   bool
}

// NewCRC32C constructs a set checksum, including the numeric value zero.
func NewCRC32C(value uint32) CRC32C {
	return CRC32C{value: value, set: true}
}

func parseCRC32CBase64(value string) (CRC32C, error) {
	if len(value) != crc32CBase64Bytes {
		return CRC32C{}, errors.Join(ErrPrimitiveContract, errors.New("crc32c encoding has invalid length"))
	}
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(raw) != crc32CBytes || base64.StdEncoding.EncodeToString(raw) != value {
		return CRC32C{}, errors.Join(ErrPrimitiveContract, errors.New("crc32c encoding is not canonical"))
	}
	return NewCRC32C(binary.BigEndian.Uint32(raw)), nil
}

// UnmarshalText accepts canonical padded standard Base64 without mutating the
// receiver on failure.
func (c *CRC32C) UnmarshalText(data []byte) error {
	if c == nil {
		return errors.Join(ErrPrimitiveContract, errors.New(crc32CNilReceiverDiagnostic))
	}
	decoded, err := parseCRC32CBase64(string(data))
	if err != nil {
		return err
	}
	*c = decoded
	return nil
}

// Uint32 returns the checksum after validating that it is set.
func (c CRC32C) Uint32() (uint32, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}
	return c.value, nil
}

// Base64 returns canonical padded standard Base64 in big-endian byte order.
func (c CRC32C) Base64() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	var raw [crc32CBytes]byte
	binary.BigEndian.PutUint32(raw[:], c.value)
	return base64.StdEncoding.EncodeToString(raw[:]), nil
}

// Validate rejects an unset checksum.
func (c CRC32C) Validate() error {
	if !c.set {
		return errors.Join(ErrPrimitiveContract, errors.New("crc32c checksum is unset"))
	}
	return nil
}

// MarshalJSON emits canonical padded Base64 as a JSON string.
func (c CRC32C) MarshalJSON() ([]byte, error) {
	value, err := c.Base64()
	if err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value)
}

// UnmarshalJSON accepts only canonical padded standard Base64.
func (c *CRC32C) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(ErrJSONContract, errors.New(crc32CNilReceiverDiagnostic))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	decoded, err := parseCRC32CBase64(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*c = decoded
	return nil
}

// Ed25519PublicKey is an owned, set Ed25519 public key.
type Ed25519PublicKey struct {
	value [ed25519.PublicKeySize]byte
	set   bool
}

// NewEd25519PublicKey copies a standard-library key of the exact required size.
func NewEd25519PublicKey(value ed25519.PublicKey) (Ed25519PublicKey, error) {
	if len(value) != ed25519.PublicKeySize {
		return Ed25519PublicKey{}, errors.Join(ErrPrimitiveContract, errors.New("ed25519 public key has invalid length"))
	}
	var key [ed25519.PublicKeySize]byte
	copy(key[:], value)
	return Ed25519PublicKey{value: key, set: true}, nil
}

// parseEd25519PublicKeyHex accepts canonical lowercase hexadecimal.
func parseEd25519PublicKeyHex(value string) (Ed25519PublicKey, error) {
	decoded, err := decodeCanonicalHex(value, ed25519.PublicKeySize)
	if err != nil {
		return Ed25519PublicKey{}, errors.Join(ErrPrimitiveContract, err)
	}
	return NewEd25519PublicKey(ed25519.PublicKey(decoded))
}

// Bytes returns an independent standard-library public-key copy.
func (k Ed25519PublicKey) Bytes() (ed25519.PublicKey, error) {
	if err := k.Validate(); err != nil {
		return nil, err
	}
	value := make(ed25519.PublicKey, ed25519.PublicKeySize)
	copy(value, k.value[:])
	return value, nil
}

// Hex returns canonical lowercase hexadecimal.
func (k Ed25519PublicKey) Hex() (string, error) {
	if err := k.Validate(); err != nil {
		return "", err
	}
	return hex.EncodeToString(k.value[:]), nil
}

// Validate rejects an unset public key.
func (k Ed25519PublicKey) Validate() error {
	if !k.set {
		return errors.Join(ErrPrimitiveContract, errors.New("ed25519 public key is unset"))
	}
	return nil
}

// MarshalJSON emits canonical lowercase hexadecimal as a JSON string.
func (k Ed25519PublicKey) MarshalJSON() ([]byte, error) {
	value, err := k.Hex()
	if err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value)
}

// UnmarshalJSON accepts only canonical lowercase hexadecimal.
func (k *Ed25519PublicKey) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(ErrJSONContract, errors.New("nil Ed25519 public key receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	decoded, err := parseEd25519PublicKeyHex(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*k = decoded
	return nil
}

// SecretMaterial is a shared handle to an owned 16-to-64-byte nonzero value
// whose formatter is always redacted. Copying the handle does not copy the
// bytes: every copy observes destruction of the same Core-owned storage. Its
// zero value is invalid.
type SecretMaterial struct {
	state *secretMaterialState
}

type secretMaterialState struct {
	mutex     sync.RWMutex
	value     [SecretMaterialMaximumBytes]byte
	size      uint32
	destroyed bool
}

// NewSecretMaterial validates and copies value into fixed-capacity storage.
func NewSecretMaterial(value []byte) (SecretMaterial, error) {
	if len(value) < SecretMaterialMinimumBytes || len(value) > SecretMaterialMaximumBytes {
		return SecretMaterial{}, errors.Join(ErrPrimitiveContract, errors.New(secretMaterialLengthErrorText))
	}
	if allZeroBytes(value) {
		return SecretMaterial{}, ErrSecretMaterialAllZero
	}
	size, err := CheckedUint32FromInt(len(value))
	if err != nil {
		return SecretMaterial{}, errors.Join(ErrPrimitiveContract, err)
	}
	state := &secretMaterialState{size: size}
	copy(state.value[:], value)
	return SecretMaterial{state: state}, nil
}

// ByteCount returns the validated material length.
func (m SecretMaterial) ByteCount() (ByteCount, error) {
	if m.state == nil {
		return ByteCount{}, secretMaterialStateError(secretMaterialUnsetErrorText)
	}
	m.state.mutex.RLock()
	defer m.state.mutex.RUnlock()
	if err := validateSecretMaterialState(m.state); err != nil {
		return ByteCount{}, err
	}
	return NewByteCount(uint64(m.state.size))
}

// CopyBytes returns an independent caller-owned copy after validating the
// material. Destroy cannot zero copies already returned to a caller.
func (m SecretMaterial) CopyBytes() ([]byte, error) {
	if m.state == nil {
		return nil, secretMaterialStateError(secretMaterialUnsetErrorText)
	}
	m.state.mutex.RLock()
	defer m.state.mutex.RUnlock()
	if err := validateSecretMaterialState(m.state); err != nil {
		return nil, err
	}
	value := make([]byte, m.state.size)
	copy(value, m.state.value[:m.state.size])
	return value, nil
}

// Validate enforces active state, length, nonzero content, and zero padding.
func (m SecretMaterial) Validate() error {
	if m.state == nil {
		return secretMaterialStateError(secretMaterialUnsetErrorText)
	}
	m.state.mutex.RLock()
	defer m.state.mutex.RUnlock()
	return validateSecretMaterialState(m.state)
}

// Destroy zeros Core-owned storage and invalidates every copied handle that
// shares it. Repeated destruction is an idempotent success.
func (m SecretMaterial) Destroy() error {
	if m.state == nil {
		return secretMaterialStateError(secretMaterialUnsetErrorText)
	}
	m.state.mutex.Lock()
	defer m.state.mutex.Unlock()
	if m.state.destroyed {
		return nil
	}
	for index := range m.state.value {
		m.state.value[index] = 0
	}
	m.state.size = 0
	m.state.destroyed = true
	return nil
}

// MarshalJSON refuses to serialize secret material through the JSON boundary.
func (m SecretMaterial) MarshalJSON() ([]byte, error) {
	return nil, errors.Join(ErrJSONContract, errors.New(secretMaterialJSONErrorText))
}

func validateSecretMaterialState(state *secretMaterialState) error {
	if state.destroyed {
		return secretMaterialStateError("secret material is destroyed")
	}
	if state.size < SecretMaterialMinimumBytes || state.size > SecretMaterialMaximumBytes {
		return secretMaterialStateError(secretMaterialLengthErrorText)
	}
	if allZeroBytes(state.value[:state.size]) {
		return ErrSecretMaterialAllZero
	}
	if !allZeroBytes(state.value[state.size:]) {
		return secretMaterialStateError("secret material has nonzero padding")
	}
	return nil
}

func secretMaterialStateError(message string) error {
	return errors.Join(ErrPrimitiveContract, errors.New(message))
}

// Format writes RedactedValueText for every formatting verb.
func (m SecretMaterial) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, RedactedValueText)
}

func decodeCanonicalHex(value string, size int) ([]byte, error) {
	if len(value) != hex.EncodedLen(size) {
		return nil, errors.New("hex value has invalid length")
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || hex.EncodeToString(decoded) != value {
		return nil, errors.New("hex value is not canonical lowercase")
	}
	return decoded, nil
}

func allZeroBytes(value []byte) bool {
	var aggregate byte
	for _, part := range value {
		aggregate |= part
	}
	return aggregate == 0
}
