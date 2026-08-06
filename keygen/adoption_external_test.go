package keygen_test

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

// nonZeroSeed builds a seed whose every byte is distinct enough to catch a
// truncated or misaligned copy, without depending on entropy.
func nonZeroSeed() [ed25519.SeedSize]byte {
	var seed [ed25519.SeedSize]byte
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	return seed
}

func TestAdoptedKeyIsIndistinguishableFromAGeneratedOne(t *testing.T) {
	t.Parallel()

	seed := nonZeroSeed()
	key, err := keygen.AdoptSigningKey(seed)
	if err != nil {
		t.Fatalf("AdoptSigningKey() error = %v, want nil", err)
	}
	if err := key.Validate(); err != nil {
		t.Fatalf("adopted key Validate() error = %v, want nil", err)
	}

	// The seed must survive adoption exactly. Anything else means custody took
	// a copy of something other than what the caller persisted, and every
	// signature afterwards would be made by a different key than the operator
	// believes they installed.
	private, err := key.PrivateKey()
	if err != nil {
		t.Fatalf("PrivateKey() error = %v, want nil", err)
	}
	if got := private.Seed(); !bytes.Equal(got, seed[:]) {
		t.Fatalf("PrivateKey().Seed() = %x, want %x", got, seed)
	}

	// The public half must be the one the seed determines, not one supplied
	// alongside it, which is the whole reason this door admits only a seed.
	public, err := key.PublicKey()
	if err != nil {
		t.Fatalf("PublicKey() error = %v, want nil", err)
	}
	publicBytes, err := public.Bytes()
	if err != nil {
		t.Fatalf("PublicKey().Bytes() error = %v, want nil", err)
	}
	wantPublic := ed25519.NewKeyFromSeed(seed[:])[ed25519.SeedSize:]
	if !bytes.Equal(publicBytes, wantPublic) {
		t.Fatalf("PublicKey() = %x, want the seed-derived %x", publicBytes, wantPublic)
	}
}

func TestAdoptSigningKeyRefusesASeedWithNoEntropy(t *testing.T) {
	t.Parallel()

	// An all-zero seed is a real failure mode: it is what a caller gets from a
	// truncated read, a zeroed buffer, or a config key that was never set. It
	// produces a perfectly valid Ed25519 key that every attacker also holds.
	var zero [ed25519.SeedSize]byte
	key, err := keygen.AdoptSigningKey(zero)
	if err == nil {
		t.Fatal("AdoptSigningKey(all-zero seed) error = nil, want refusal")
	}
	if !errors.Is(err, core.ErrSecretMaterialAllZero) {
		t.Fatalf("AdoptSigningKey(all-zero seed) error = %v, want %v", err, core.ErrSecretMaterialAllZero)
	}
	if err := key.Validate(); err == nil {
		t.Fatal("refused adoption returned a key that validates, want the zero value")
	}
}

func TestAdoptedKeyCarriesCoreOwnedDestructionAndRedaction(t *testing.T) {
	t.Parallel()

	seed := nonZeroSeed()
	key, err := keygen.AdoptSigningKey(seed)
	if err != nil {
		t.Fatalf("AdoptSigningKey() error = %v, want nil", err)
	}

	// Redaction is the property a raw ed25519.PrivateKey does not have, and it
	// is the reason adoption exists rather than products holding the slice.
	if got := fmt.Sprintf("%v", key); got != core.RedactedValueText {
		t.Fatalf("formatted adopted key = %q, want %q", got, core.RedactedValueText)
	}

	// A copy shares the Core-owned seed, so destroying either invalidates both.
	// An adopted key that kept a private copy would keep signing after the
	// operator revoked it.
	copied := key
	if err := key.Destroy(); err != nil {
		t.Fatalf("Destroy() error = %v, want nil", err)
	}
	if err := copied.Validate(); err == nil {
		t.Fatal("copy of a destroyed adopted key still validates, want shared destruction")
	}
	if _, err := copied.PrivateKey(); err == nil {
		t.Fatal("copy of a destroyed adopted key still projects a private key, want refusal")
	}
}
