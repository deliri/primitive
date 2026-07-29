package keygen

import (
	"crypto/ed25519"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// SigningKey is one active Ed25519 seed and its derived public key. Copies
// share the Core-owned seed lifecycle, so destroying any copy invalidates all
// copies. The zero value is invalid.
type SigningKey struct {
	seed   core.SecretMaterial
	public core.Ed25519PublicKey
}

// GenerateSigningKey delegates entropy acquisition and Ed25519 derivation to
// crypto/ed25519.GenerateKey with its Go 1.26 protected nil-source path.
func GenerateSigningKey() (SigningKey, error) {
	public, private, err := ed25519.GenerateKey(nil)
	return adoptGeneratedSigningKey(public, private, err)
}

func adoptGeneratedSigningKey(
	public ed25519.PublicKey,
	private ed25519.PrivateKey,
	sourceErr error,
) (SigningKey, error) {
	defer clear(public)
	defer clear(private)
	if sourceErr != nil {
		return SigningKey{}, entropyError(sourceErr)
	}
	if len(public) != ed25519.PublicKeySize ||
		len(private) != ed25519.PrivateKeySize {
		return SigningKey{}, contractError(errors.New("keygen standard Ed25519 result has invalid extent"))
	}
	seed := private.Seed()
	defer clear(seed)
	material, err := core.NewSecretMaterial(seed)
	if err != nil {
		if errors.Is(err, core.ErrSecretMaterialAllZero) {
			return SigningKey{}, entropyError(err)
		}
		return SigningKey{}, contractError(err)
	}
	ownedPublic, err := core.NewEd25519PublicKey(public)
	if err != nil {
		_ = material.Destroy()
		return SigningKey{}, contractError(err)
	}
	key := SigningKey{seed: material, public: ownedPublic}
	if err := key.Validate(); err != nil {
		_ = material.Destroy()
		return SigningKey{}, err
	}
	return key, nil
}

// Validate checks active seed custody, exact RFC 8032 seed width, and the
// deterministic public-key relationship.
func (k SigningKey) Validate() error {
	seed, err := k.validatedSeed()
	clear(seed[:])
	return err
}

// PublicKey returns the immutable Core-owned public-key value.
func (k SigningKey) PublicKey() (core.Ed25519PublicKey, error) {
	if err := k.Validate(); err != nil {
		return core.Ed25519PublicKey{}, err
	}
	return k.public, nil
}

// PrivateKey explicitly projects an independent standard-library private-key
// copy. The caller owns and should clear the returned mutable slice.
func (k SigningKey) PrivateKey() (ed25519.PrivateKey, error) {
	seed, err := k.validatedSeed()
	if err != nil {
		clear(seed[:])
		return nil, err
	}
	private := ed25519.NewKeyFromSeed(seed[:])
	clear(seed[:])
	return private, nil
}

// Destroy clears the Core-owned seed and invalidates every copied handle.
func (k SigningKey) Destroy() error {
	if k.seed == (core.SecretMaterial{}) {
		return contractError(errors.New("keygen signing key is unset"))
	}
	if err := k.seed.Destroy(); err != nil {
		return contractError(err)
	}
	return nil
}

// Format prevents private-key disclosure through generic formatting.
func (k SigningKey) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func (k SigningKey) validatedSeed() ([ed25519.SeedSize]byte, error) {
	if err := k.seed.Validate(); err != nil {
		return [ed25519.SeedSize]byte{}, contractError(err)
	}
	count, err := k.seed.ByteCount()
	if err != nil {
		return [ed25519.SeedSize]byte{}, contractError(err)
	}
	size, err := count.Uint64()
	if err != nil || size != ed25519.SeedSize {
		return [ed25519.SeedSize]byte{}, contractError(
			errors.New("keygen signing seed has invalid extent"),
			err,
		)
	}
	if err := k.public.Validate(); err != nil {
		return [ed25519.SeedSize]byte{}, contractError(err)
	}
	raw, err := k.seed.CopyBytes()
	if err != nil {
		return [ed25519.SeedSize]byte{}, contractError(err)
	}
	defer clear(raw)
	var seed [ed25519.SeedSize]byte
	copy(seed[:], raw)
	if err := validateDerivedPublic(seed, k.public); err != nil {
		clear(seed[:])
		return [ed25519.SeedSize]byte{}, err
	}
	return seed, nil
}

func validateDerivedPublic(
	seed [ed25519.SeedSize]byte,
	public core.Ed25519PublicKey,
) error {
	private := ed25519.NewKeyFromSeed(seed[:])
	defer clear(private)
	publicBytes, err := public.Bytes()
	if err != nil {
		return contractError(err)
	}
	defer clear(publicBytes)
	derived := private[ed25519.SeedSize:]
	if subtle.ConstantTimeCompare(derived, publicBytes) != 1 {
		return contractError(errors.New("keygen signing public key does not match its seed"))
	}
	return nil
}
