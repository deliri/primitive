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

// TestAdoptPrivateKeyAdmitsExactlyWhatPrivateKeyProjects proves the wire
// door's whole claim: the 64-byte projection a key hands out is adoptable
// back to the same public identity, the trailing half is re-derived rather
// than trusted, and every wrong extent is refused.
func TestAdoptPrivateKeyAdmitsExactlyWhatPrivateKeyProjects(t *testing.T) {
	t.Parallel()

	generated, err := keygen.GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey() error = %v, want nil", err)
	}
	wantPublic, err := generated.PublicKey()
	if err != nil {
		t.Fatalf("PublicKey() error = %v, want nil", err)
	}
	private, err := generated.PrivateKey()
	if err != nil {
		t.Fatalf("PrivateKey() error = %v, want nil", err)
	}
	adopted, err := keygen.AdoptPrivateKey(private)
	if err != nil {
		t.Fatalf("AdoptPrivateKey(PrivateKey()) error = %v, want nil", err)
	}
	gotPublic, err := adopted.PublicKey()
	if err != nil {
		t.Fatalf("adopted PublicKey() error = %v, want nil", err)
	}
	if gotPublic != wantPublic {
		t.Fatalf("adopted PublicKey() = %v, want the projecting identity %v", gotPublic, wantPublic)
	}

	// A tampered trailing half signs as the seed says, never as the tamper
	// says: adoption re-derives, so the forged public half simply vanishes.
	forged := append(ed25519.PrivateKey(nil), private...)
	for i := ed25519.SeedSize; i < len(forged); i++ {
		forged[i] ^= 0xff
	}
	readopted, err := keygen.AdoptPrivateKey(forged)
	if err != nil {
		t.Fatalf("AdoptPrivateKey(forged trailing half) error = %v, want adoption from the seed", err)
	}
	forgedPublic, err := readopted.PublicKey()
	if err != nil {
		t.Fatalf("readopted PublicKey() error = %v, want nil", err)
	}
	if forgedPublic != wantPublic {
		t.Fatalf("forged-half PublicKey() = %v, want the seed-derived %v", forgedPublic, wantPublic)
	}

	for _, extent := range []int{0, 1, ed25519.SeedSize, ed25519.PrivateKeySize - 1, ed25519.PrivateKeySize + 1} {
		got, gotErr := keygen.AdoptPrivateKey(make(ed25519.PrivateKey, extent))
		if !errors.Is(gotErr, core.ErrKeygenContract) {
			t.Fatalf("AdoptPrivateKey(%d bytes) error = %v, want errors.Is %v", extent, gotErr, core.ErrKeygenContract)
		}
		if got != (keygen.SigningKey{}) {
			t.Fatalf("AdoptPrivateKey(%d bytes) = %v, want zero key on refusal", extent, got)
		}
	}
}

// TestSeedRoundTripsThroughAdoption proves the persistence story the door
// exists for: the seed a key hands out is exactly the seed adoption accepts
// back, and the readopted key derives the same public identity.
func TestSeedRoundTripsThroughAdoption(t *testing.T) {
	t.Parallel()

	generated, err := keygen.GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey() error = %v, want nil", err)
	}
	seed, err := generated.Seed()
	if err != nil {
		t.Fatalf("Seed() error = %v, want nil", err)
	}
	wantPublic, err := generated.PublicKey()
	if err != nil {
		t.Fatalf("PublicKey() error = %v, want nil", err)
	}
	readopted, err := keygen.AdoptSigningKey(seed)
	if err != nil {
		t.Fatalf("AdoptSigningKey(Seed()) error = %v, want nil", err)
	}
	gotPublic, err := readopted.PublicKey()
	if err != nil {
		t.Fatalf("readopted PublicKey() error = %v, want nil", err)
	}
	if gotPublic != wantPublic {
		t.Fatalf("readopted PublicKey() = %v, want the generated identity %v", gotPublic, wantPublic)
	}
	roundTrip, err := readopted.Seed()
	if err != nil {
		t.Fatalf("readopted Seed() error = %v, want nil", err)
	}
	if roundTrip != seed {
		t.Fatalf("readopted Seed() = %x, want the persisted seed %x", roundTrip, seed)
	}
}

// TestSeedSizeNamesTheExactContractExtent pins the exported size to the one
// extent AdoptSigningKey admits, so a consumer-declared [keygen.SeedSize]byte
// can never drift from the parameter type.
func TestSeedSizeNamesTheExactContractExtent(t *testing.T) {
	t.Parallel()

	var seed [keygen.SeedSize]byte
	if got, want := len(seed), ed25519.SeedSize; got != want {
		t.Fatalf("len([keygen.SeedSize]byte{}) = %d, want %d", got, want)
	}
	got, gotErr := keygen.AdoptSigningKey(seed)
	if !errors.Is(gotErr, core.ErrKeygenEntropy) || !errors.Is(gotErr, core.ErrSecretMaterialAllZero) {
		t.Fatalf(
			"AdoptSigningKey(zero seed) error = %v, want errors.Is %v and %v",
			gotErr,
			core.ErrKeygenEntropy,
			core.ErrSecretMaterialAllZero,
		)
	}
	if got != (keygen.SigningKey{}) {
		t.Fatalf("AdoptSigningKey(zero seed) = %v, want zero key on refusal", got)
	}
}

// TestSeedRefusesAnUnsetKey holds the custody door: the zero value never
// discloses a seed-shaped answer.
func TestSeedRefusesAnUnsetKey(t *testing.T) {
	t.Parallel()

	var zero keygen.SigningKey
	if _, err := zero.Seed(); !errors.Is(err, core.ErrKeygenContract) {
		t.Fatalf("zero SigningKey Seed() error = %v, want errors.Is %v", err, core.ErrKeygenContract)
	}
}

// TestSeedRefusesADestroyedKey proves destruction reaches the projection: a
// destroyed key must not keep answering with the secret it promised to clear.
func TestSeedRefusesADestroyedKey(t *testing.T) {
	t.Parallel()

	generated, err := keygen.GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey() error = %v, want nil", err)
	}
	if err := generated.Destroy(); err != nil {
		t.Fatalf("Destroy() error = %v, want nil", err)
	}
	seed, gotErr := generated.Seed()
	if !errors.Is(gotErr, core.ErrKeygenContract) {
		t.Fatalf("destroyed SigningKey Seed() error = %v, want errors.Is %v", gotErr, core.ErrKeygenContract)
	}
	if seed != ([keygen.SeedSize]byte{}) {
		t.Fatalf("destroyed SigningKey Seed() = %x, want zero secret on refusal", seed)
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
	if verr := key.Validate(); !errors.Is(verr, core.ErrKeygenContract) {
		t.Fatalf("refused adoption key.Validate() error = %v, want errors.Is %v", verr, core.ErrKeygenContract)
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
	if verr := copied.Validate(); !errors.Is(verr, core.ErrKeygenContract) {
		t.Fatalf("destroyed-copy Validate() error = %v, want errors.Is %v", verr, core.ErrKeygenContract)
	}
	if private, perr := copied.PrivateKey(); private != nil || !errors.Is(perr, core.ErrKeygenContract) {
		t.Fatalf("destroyed-copy PrivateKey() = (%v, %v), want nil and errors.Is %v", private, perr, core.ErrKeygenContract)
	}
}
