package attest

import (
	"crypto/ed25519"
	"crypto/subtle"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func copyAndValidatePrivateKey(
	input ed25519.PrivateKey,
) ([ed25519.PrivateKeySize]byte, core.Ed25519PublicKey, error) {
	var copied [ed25519.PrivateKeySize]byte
	if len(input) != ed25519.PrivateKeySize {
		return copied, core.Ed25519PublicKey{}, contractError(errors.New(privateKeyLengthErrorText))
	}
	copy(copied[:], input)
	derived := ed25519.NewKeyFromSeed(copied[:ed25519.SeedSize])
	defer clear(derived)
	if subtle.ConstantTimeCompare(copied[:], derived) != 1 {
		clear(copied[:])
		return copied, core.Ed25519PublicKey{}, contractError(errors.New(privateKeyPublicHalfErrorText))
	}
	publicKey, err := core.NewEd25519PublicKey(ed25519.PublicKey(copied[ed25519.SeedSize:]))
	if err != nil {
		clear(copied[:])
		return copied, core.Ed25519PublicKey{}, contractError(err)
	}
	return copied, publicKey, nil
}
