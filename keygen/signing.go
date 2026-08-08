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

// SeedSize is the exact RFC 8032 seed extent AdoptSigningKey admits and Seed
// returns, named so a consumer holding a persisted seed can state the size of
// keygen's own contract without reaching past keygen for it. The literal is
// held to the standard library's own constant by the compile-time witness
// below, so the two can never drift.
const SeedSize = 32

var _ [SeedSize]byte = [ed25519.SeedSize]byte{}

// Seed projects a caller-owned copy of the live seed, so a product that must
// persist a signing key can store the minimal secret and adopt it back later
// through AdoptSigningKey. Custody rules are unchanged: the copy is the
// caller's to clear, and an unset or destroyed key refuses. Without this door
// the adopt-back path exists but cannot be fed: keygen would hand out only
// the 64-byte standard-library private key while accepting back only the
// seed, and every consumer would bridge that asymmetry with crypto/ed25519
// size arithmetic of its own.
func (k SigningKey) Seed() ([SeedSize]byte, error) {
	return k.validatedSeed()
}

// AdoptSigningKey takes custody of one RFC 8032 seed a product already holds,
// so a key read back from storage reaches the same seed custody, redacted
// formatting, and destruction rules as a generated one.
//
// Without this door a product that persists a signing key has no way to
// produce a SigningKey at all, and is forced to carry a raw
// ed25519.PrivateKey with none of those properties. The seed is the whole key:
// the 64-byte standard-library private key is that seed followed by the public
// half it already determines, so nothing is lost by admitting only the seed and
// a caller cannot supply a pair that disagrees with itself.
func AdoptSigningKey(seed [ed25519.SeedSize]byte) (SigningKey, error) {
	private := ed25519.NewKeyFromSeed(seed[:])
	public := make(ed25519.PublicKey, ed25519.PublicKeySize)
	copy(public, private[ed25519.SeedSize:])
	return adoptGeneratedSigningKey(public, private, nil)
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
