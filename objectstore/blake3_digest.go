package objectstore

import (
	"encoding/hex"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// BLAKE3DigestBytes is the exact binary width of a BLAKE3-256 digest.
	BLAKE3DigestBytes                 = 32
	blake3DigestNilReceiverDiagnostic = "nil BLAKE3 digest receiver"
	blake3DigestUnsetDiagnostic       = "BLAKE3 digest is unset"
)

// BLAKE3Digest is a set 256-bit BLAKE3 content identity. Its zero value is
// invalid; a constructed digest containing thirty-two zero bytes is valid.
type BLAKE3Digest struct {
	value [BLAKE3DigestBytes]byte
	set   bool
}

// NewBLAKE3Digest constructs a set digest from all thirty-two bytes.
func NewBLAKE3Digest(value [BLAKE3DigestBytes]byte) BLAKE3Digest {
	return BLAKE3Digest{value: value, set: true}
}

func parseBLAKE3Hex(value string) (BLAKE3Digest, error) {
	var digest [BLAKE3DigestBytes]byte
	if err := core.DecodeCanonicalHex(digest[:], value); err != nil {
		return BLAKE3Digest{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	return NewBLAKE3Digest(digest), nil
}

// Bytes returns the digest bytes after validating that the value is set.
func (d BLAKE3Digest) Bytes() ([BLAKE3DigestBytes]byte, error) {
	if err := d.Validate(); err != nil {
		return [BLAKE3DigestBytes]byte{}, err
	}
	return d.value, nil
}

// Hex returns canonical lowercase hexadecimal.
func (d BLAKE3Digest) Hex() (string, error) {
	if err := d.Validate(); err != nil {
		return "", err
	}
	return hex.EncodeToString(d.value[:]), nil
}

// Validate rejects an unset digest.
func (d BLAKE3Digest) Validate() error {
	if !d.set {
		return errors.Join(core.ErrObjectStoreContract, errors.New(blake3DigestUnsetDiagnostic))
	}
	return nil
}

// UnmarshalText accepts exactly one canonical lowercase hexadecimal digest.
// The receiver is unchanged on rejection.
func (d *BLAKE3Digest) UnmarshalText(text []byte) error {
	if d == nil {
		return errors.Join(core.ErrObjectStoreContract, errors.New(blake3DigestNilReceiverDiagnostic))
	}
	if len(text) != hex.EncodedLen(BLAKE3DigestBytes) {
		return errors.Join(core.ErrObjectStoreContract, core.ErrPrimitiveContract)
	}
	parsed, err := parseBLAKE3Hex(string(text))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// MarshalJSON emits canonical lowercase hexadecimal as a JSON string.
func (d BLAKE3Digest) MarshalJSON() ([]byte, error) {
	value, err := d.Hex()
	if err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(value)
}

// UnmarshalJSON accepts only canonical lowercase hexadecimal. The receiver is
// unchanged on rejection.
func (d *BLAKE3Digest) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrJSONContract, core.ErrObjectStoreContract, errors.New(blake3DigestNilReceiverDiagnostic))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	parsed, err := parseBLAKE3Hex(value)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*d = parsed
	return nil
}

var _ core.Validatable = BLAKE3Digest{}
var _ core.ValidatedJSONMarshaler = BLAKE3Digest{}
