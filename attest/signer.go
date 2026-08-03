package attest

import (
	"crypto"
	"crypto/ed25519"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

type signingCapability struct {
	signer     crypto.Signer
	publicKey  core.Ed25519PublicKey
	privateKey [ed25519.PrivateKeySize]byte
}

func newSigningCapability(input crypto.Signer) (*signingCapability, error) {
	if input == nil {
		return nil, contractError(errors.New(signerMissingErrorText))
	}
	switch signer := input.(type) {
	case ed25519.PrivateKey:
		return newLocalSigningCapability(signer)
	case *ed25519.PrivateKey:
		if signer == nil {
			return nil, contractError(errors.New(signerMissingErrorText))
		}
		return newLocalSigningCapability(*signer)
	default:
		return newExternalSigningCapability(input)
	}
}

func newLocalSigningCapability(input ed25519.PrivateKey) (*signingCapability, error) {
	privateKey, publicKey, err := copyAndValidatePrivateKey(input)
	if err != nil {
		return nil, err
	}
	capability := &signingCapability{publicKey: publicKey, privateKey: privateKey}
	clear(privateKey[:])
	capability.signer = ed25519.PrivateKey(capability.privateKey[:])
	return capability, nil
}

func newExternalSigningCapability(input crypto.Signer) (*signingCapability, error) {
	publicValue, err := guardedCall(func() (crypto.PublicKey, error) {
		return input.Public(), nil
	})
	if err != nil {
		return nil, contractError(err)
	}
	publicBytes, ok := publicValue.(ed25519.PublicKey)
	if !ok {
		return nil, contractError(errors.New(signerPublicKeyTypeErrorText))
	}
	publicKey, err := core.NewEd25519PublicKey(publicBytes)
	if err != nil {
		return nil, contractError(err)
	}
	return &signingCapability{signer: input, publicKey: publicKey}, nil
}

func (c *signingCapability) close() {
	clear(c.privateKey[:])
	c.signer = nil
}
