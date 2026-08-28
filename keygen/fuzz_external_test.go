package keygen_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

func FuzzAdoptPrivateKeyAgainstStandardLibraryDerivation(f *testing.F) {
	canonical := canonicalPrivateKeySeed(f)
	f.Add(canonical)
	clear(canonical)
	f.Add([]byte{})
	f.Add(make([]byte, ed25519.SeedSize))
	f.Add(make([]byte, ed25519.PrivateKeySize-1))
	f.Add(make([]byte, ed25519.PrivateKeySize))
	f.Add(make([]byte, ed25519.PrivateKeySize+1))

	f.Fuzz(func(t *testing.T, data []byte) {
		before := append([]byte(nil), data...)
		got, gotErr := keygen.AdoptPrivateKey(ed25519.PrivateKey(data))
		var zeroSeed [keygen.SeedSize]byte
		wantAccepted := len(data) == ed25519.PrivateKeySize && !bytes.Equal(data[:ed25519.SeedSize], zeroSeed[:])

		if !wantAccepted {
			provePrivateKeyAdoptionRejection(t, got, gotErr, data)
			if !bytes.Equal(data, before) {
				t.Fatalf("AdoptPrivateKey(rejected) input = %x, want preserved %x", data, before)
			}
			return
		}

		provePrivateKeyAdoptionClosure(t, got, gotErr, data)
		if !bytes.Equal(data, before) {
			t.Fatalf("AdoptPrivateKey(accepted) input = %x, want preserved %x", data, before)
		}
	})
}

func FuzzAdoptSigningKeyAgainstStandardLibraryDerivation(f *testing.F) {
	seed := canonicalSigningSeed(f)
	f.Add(
		binary.BigEndian.Uint64(seed[0:8]),
		binary.BigEndian.Uint64(seed[8:16]),
		binary.BigEndian.Uint64(seed[16:24]),
		binary.BigEndian.Uint64(seed[24:32]),
	)
	f.Add(uint64(0), uint64(0), uint64(0), uint64(0))

	f.Fuzz(func(t *testing.T, first, second, third, fourth uint64) {
		seed := [keygen.SeedSize]byte{}
		binary.BigEndian.PutUint64(seed[0:8], first)
		binary.BigEndian.PutUint64(seed[8:16], second)
		binary.BigEndian.PutUint64(seed[16:24], third)
		binary.BigEndian.PutUint64(seed[24:32], fourth)

		got, gotErr := keygen.AdoptSigningKey(seed)
		if seed == ([keygen.SeedSize]byte{}) {
			if got != (keygen.SigningKey{}) ||
				!errors.Is(gotErr, core.ErrKeygenEntropy) ||
				!errors.Is(gotErr, core.ErrSecretMaterialAllZero) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("AdoptSigningKey(zero) = (%v, %v), want zero with %v, %v, and %v", got, gotErr, core.ErrKeygenEntropy, core.ErrSecretMaterialAllZero, core.ErrPrimitiveContract)
			}
			return
		}

		proveSigningKeyAdoptionClosure(t, got, gotErr, seed)
		if gotDestroyErr := got.Destroy(); gotDestroyErr != nil {
			t.Fatalf("SigningKey.Destroy() error = %v, want nil", gotDestroyErr)
		}
	})
}

func canonicalPrivateKeySeed(tb testing.TB) []byte {
	tb.Helper()
	key, gotErr := keygen.AdoptSigningKey(nonZeroSeed())
	if gotErr != nil {
		tb.Fatalf("AdoptSigningKey(canonical seed) error = %v, want nil", gotErr)
	}
	private, gotPrivateErr := key.PrivateKey()
	if gotPrivateErr != nil {
		tb.Fatalf("SigningKey.PrivateKey(canonical seed) error = %v, want nil", gotPrivateErr)
	}
	if gotDestroyErr := key.Destroy(); gotDestroyErr != nil {
		tb.Fatalf("SigningKey.Destroy(canonical seed) error = %v, want nil", gotDestroyErr)
	}
	return private
}

func canonicalSigningSeed(tb testing.TB) [keygen.SeedSize]byte {
	tb.Helper()
	key, gotErr := keygen.AdoptSigningKey(nonZeroSeed())
	if gotErr != nil {
		tb.Fatalf("AdoptSigningKey(canonical seed) error = %v, want nil", gotErr)
	}
	seed, gotSeedErr := key.Seed()
	if gotSeedErr != nil {
		tb.Fatalf("SigningKey.Seed(canonical seed) error = %v, want nil", gotSeedErr)
	}
	if gotDestroyErr := key.Destroy(); gotDestroyErr != nil {
		tb.Fatalf("SigningKey.Destroy(canonical seed) error = %v, want nil", gotDestroyErr)
	}
	return seed
}

func provePrivateKeyAdoptionRejection(t *testing.T, got keygen.SigningKey, gotErr error, data []byte) {
	t.Helper()
	if got != (keygen.SigningKey{}) {
		t.Fatalf("AdoptPrivateKey(rejected) key = %v, want zero", got)
	}
	if len(data) != ed25519.PrivateKeySize {
		if !errors.Is(gotErr, core.ErrKeygenContract) || !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("AdoptPrivateKey(%d bytes) error = %v, want %v and %v", len(data), gotErr, core.ErrKeygenContract, core.ErrPrimitiveContract)
		}
		return
	}
	if !errors.Is(gotErr, core.ErrKeygenEntropy) ||
		!errors.Is(gotErr, core.ErrSecretMaterialAllZero) ||
		!errors.Is(gotErr, core.ErrPrimitiveContract) {
		t.Fatalf("AdoptPrivateKey(all-zero seed) error = %v, want %v, %v, and %v", gotErr, core.ErrKeygenEntropy, core.ErrSecretMaterialAllZero, core.ErrPrimitiveContract)
	}
}

func provePrivateKeyAdoptionClosure(t *testing.T, got keygen.SigningKey, gotErr error, data []byte) {
	t.Helper()
	if gotErr != nil {
		t.Fatalf("AdoptPrivateKey(accepted) error = %v, want nil", gotErr)
	}
	var wantSeed [keygen.SeedSize]byte
	copy(wantSeed[:], data[:keygen.SeedSize])
	proveSigningKeyAdoptionClosure(t, got, nil, wantSeed)

	projected, gotProjectedErr := got.PrivateKey()
	if gotProjectedErr != nil {
		t.Fatalf("SigningKey.PrivateKey() error = %v, want nil", gotProjectedErr)
	}
	defer clear(projected)
	wantPrivate := ed25519.NewKeyFromSeed(wantSeed[:])
	defer clear(wantPrivate)
	if !bytes.Equal(projected, wantPrivate) {
		t.Fatalf("SigningKey.PrivateKey() = %x, want standard-library derivation %x", projected, wantPrivate)
	}

	roundTrip, gotRoundTripErr := keygen.AdoptPrivateKey(projected)
	if gotRoundTripErr != nil {
		t.Fatalf("AdoptPrivateKey(canonical projection) error = %v, want nil", gotRoundTripErr)
	}
	second, gotSecondErr := roundTrip.PrivateKey()
	if gotSecondErr != nil {
		t.Fatalf("round-trip SigningKey.PrivateKey() error = %v, want nil", gotSecondErr)
	}
	defer clear(second)
	if !bytes.Equal(second, projected) {
		t.Fatalf("second canonical private projection = %x, want %x", second, projected)
	}
	if gotDestroyErr := got.Destroy(); gotDestroyErr != nil {
		t.Fatalf("SigningKey.Destroy() error = %v, want nil", gotDestroyErr)
	}
	if gotDestroyErr := roundTrip.Destroy(); gotDestroyErr != nil {
		t.Fatalf("round-trip SigningKey.Destroy() error = %v, want nil", gotDestroyErr)
	}
}

func proveSigningKeyAdoptionClosure(t *testing.T, got keygen.SigningKey, gotErr error, wantSeed [keygen.SeedSize]byte) {
	t.Helper()
	if gotErr != nil {
		t.Fatalf("AdoptSigningKey(accepted) error = %v, want nil", gotErr)
	}
	if gotValidateErr := got.Validate(); gotValidateErr != nil {
		t.Fatalf("SigningKey.Validate() error = %v, want nil", gotValidateErr)
	}
	gotSeed, gotSeedErr := got.Seed()
	if gotSeedErr != nil || gotSeed != wantSeed {
		t.Fatalf("SigningKey.Seed() = (%x, %v), want (%x, nil)", gotSeed, gotSeedErr, wantSeed)
	}

	wantPrivate := ed25519.NewKeyFromSeed(wantSeed[:])
	defer clear(wantPrivate)
	wantPublic, gotWantPublicErr := core.NewEd25519PublicKey(ed25519.PublicKey(wantPrivate[ed25519.SeedSize:]))
	if gotWantPublicErr != nil {
		t.Fatalf("core.NewEd25519PublicKey(standard derivation) error = %v, want nil", gotWantPublicErr)
	}
	gotPublic, gotPublicErr := got.PublicKey()
	if gotPublicErr != nil || gotPublic != wantPublic {
		t.Fatalf("SigningKey.PublicKey() = (%v, %v), want (%v, nil)", gotPublic, gotPublicErr, wantPublic)
	}
}
