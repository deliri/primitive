package lease

import (
	"github.com/deliri/primitive/v2026/core"
)

// DeviceIDForPublicKey derives one registered-installation identity from the
// exact bytes of a validated Ed25519 public key.
//
// The identity is the first IdentifierBytes of SHA-256 over all
// ed25519.PublicKeySize key bytes, in key order, with no prefix, salt, or
// separator. Every key byte is covered. The derivation is pure, total over set
// keys, and stable: it is the durable identity OGS registers, and it appears
// inside signed decision subjects, so any change to these bytes retires every
// registered installation.
//
// Truncation to IdentifierBytes is the Lease identifier width, not a security
// budget increase: distinctness rests on 128 bits of SHA-256 output rather than
// 256. Callers that need the full digest keep the key and hash it themselves.
func DeviceIDForPublicKey(key core.Ed25519PublicKey) (DeviceID, error) {
	if err := key.Validate(); err != nil {
		return DeviceID{}, contractError(err)
	}
	encoded, err := key.Bytes()
	if err != nil {
		return DeviceID{}, contractError(err)
	}
	digest := core.SHA256BytesOf(encoded[:])
	var value [IdentifierBytes]byte
	copy(value[:], digest[:IdentifierBytes])
	device, err := NewDeviceID(value)
	if err != nil {
		return DeviceID{}, contractError(err)
	}
	return device, nil
}
