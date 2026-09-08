package keygen_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

// Public external ingress: AdoptPrivateKey -> this target; AdoptSigningKey ->
// FuzzAdoptSigningKeyAgainstStandardLibraryDerivation. Entropy draws have no
// caller-supplied representation to decode; their exact effects have the Go
// cryptotest differential tables. Core.ByteCount owns numeric reconstruction.
var (
	_ func(ed25519.PrivateKey) (keygen.SigningKey, error)    = keygen.AdoptPrivateKey
	_ func([keygen.SeedSize]byte) (keygen.SigningKey, error) = keygen.AdoptSigningKey
)

func FuzzAdoptPrivateKeyAgainstStandardLibraryDerivation(f *testing.F) {
	canonical := canonicalPrivateKeySeed(f)
	f.Add(canonical)
	forged := bytes.Clone(canonical)
	forged[len(forged)-1] ^= 1
	f.Add(forged)
	f.Add([]byte{})
	f.Add(canonical[:ed25519.PrivateKeySize-1])
	f.Add(append(bytes.Clone(canonical), 0))
	f.Add(make([]byte, ed25519.PrivateKeySize))
	f.Add([]byte(ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))))
	f.Fuzz(func(t *testing.T, data []byte) {
		// Hashing keeps input preservation proof O(1) in auxiliary memory even
		// when the fuzzer supplies a rejected oversized document.
		before := sha256.Sum256(data)
		got, gotErr := keygen.AdoptPrivateKey(ed25519.PrivateKey(data))
		var wantSeed [keygen.SeedSize]byte
		var wantPrivate ed25519.PrivateKey
		var wantErr, wantSource error
		switch {
		case len(data) != ed25519.PrivateKeySize:
			wantErr = core.ErrKeygenContract
		default:
			copy(wantSeed[:], data[:keygen.SeedSize])
			if wantSeed == ([keygen.SeedSize]byte{}) {
				wantErr = core.ErrKeygenEntropy
				wantSource = core.ErrSecretMaterialAllZero
			} else {
				wantPrivate = ed25519.NewKeyFromSeed(wantSeed[:])
				if !bytes.Equal(data, wantPrivate) {
					wantErr = core.ErrKeygenContract
				}
			}
		}
		defer clear(wantPrivate)
		defer clear(wantSeed[:])
		if sha256.Sum256(data) != before {
			t.Fatalf("AdoptPrivateKey input hash = %x, want preserved %x", sha256.Sum256(data), before)
		}
		if wantErr != nil {
			if got != (keygen.SigningKey{}) || !errors.Is(gotErr, wantErr) || !errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("AdoptPrivateKey(rejected) = (%v, %v), want (zero, %v)", got, gotErr, wantErr)
			}
			if wantSource != nil && !errors.Is(gotErr, wantSource) {
				t.Fatalf("AdoptPrivateKey source error = %v, want %v", gotErr, wantSource)
			}
			return
		}
		if gotErr != nil {
			t.Fatalf("AdoptPrivateKey(Go canonical key) error = %v, want nil", gotErr)
		}
		t.Cleanup(func() {
			if err := got.Destroy(); err != nil {
				t.Fatalf("Destroy() error = %v, want nil", err)
			}
		})
		if err := got.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
		seed, err := got.Seed()
		defer clear(seed[:])
		if err != nil || seed != wantSeed {
			t.Fatalf("Seed() = (%x, %v), want (%x, nil)", seed, err, wantSeed)
		}
		private, err := got.PrivateKey()
		defer clear(private)
		if err != nil || !bytes.Equal(private, wantPrivate) {
			t.Fatalf("PrivateKey() = (%x, %v), want (%x, nil)", private, err, wantPrivate)
		}
		public, err := got.PublicKey()
		if err != nil {
			t.Fatalf("PublicKey() error = %v, want nil", err)
		}
		publicBytes, err := public.Bytes()
		if err != nil || !bytes.Equal(publicBytes, wantPrivate[ed25519.SeedSize:]) {
			t.Fatalf("PublicKey.Bytes() = (%x, %v), want Go seed-derived bytes", publicBytes, err)
		}
		message := []byte(core.RedactedValueText)
		if !ed25519.Verify(ed25519.PublicKey(publicBytes), message, ed25519.Sign(private, message)) {
			t.Fatal("Go verification of adopted projection = false, want true")
		}
		round, err := keygen.AdoptPrivateKey(private)
		if err != nil {
			t.Fatalf("AdoptPrivateKey(canonical) error = %v, want nil", err)
		}
		t.Cleanup(func() {
			if err := round.Destroy(); err != nil {
				t.Fatalf("Destroy(round trip) error = %v, want nil", err)
			}
		})
		second, err := round.PrivateKey()
		defer clear(second)
		if err != nil || !bytes.Equal(second, private) {
			t.Fatalf("second PrivateKey() = (%x, %v), want (%x, nil)", second, err, private)
		}
	})
}

func FuzzAdoptSigningKeyAgainstStandardLibraryDerivation(f *testing.F) {
	seed := canonicalSigningSeed(f)
	f.Add(binary.BigEndian.Uint64(seed[0:8]), binary.BigEndian.Uint64(seed[8:16]), binary.BigEndian.Uint64(seed[16:24]), binary.BigEndian.Uint64(seed[24:32]))
	f.Add(uint64(0), uint64(0), uint64(0), uint64(0))
	f.Add(uint64(1)<<63, uint64(0), uint64(0), uint64(0))
	f.Add(uint64(0), uint64(0), uint64(0), uint64(1))
	f.Fuzz(func(t *testing.T, first, second, third, fourth uint64) {
		var seed [keygen.SeedSize]byte
		binary.BigEndian.PutUint64(seed[0:8], first)
		binary.BigEndian.PutUint64(seed[8:16], second)
		binary.BigEndian.PutUint64(seed[16:24], third)
		binary.BigEndian.PutUint64(seed[24:32], fourth)
		defer clear(seed[:])
		got, err := keygen.AdoptSigningKey(seed)
		if seed == ([keygen.SeedSize]byte{}) {
			if got != (keygen.SigningKey{}) || !errors.Is(err, core.ErrKeygenEntropy) || !errors.Is(err, core.ErrSecretMaterialAllZero) || !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("AdoptSigningKey(zero) = (%v, %v), want zero and typed entropy/all-zero refusal", got, err)
			}
			return
		}
		if err != nil {
			t.Fatalf("AdoptSigningKey(nonzero) error = %v, want nil", err)
		}
		t.Cleanup(func() {
			if err := got.Destroy(); err != nil {
				t.Fatalf("Destroy() error = %v, want nil", err)
			}
		})
		if err := got.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
		gotSeed, err := got.Seed()
		defer clear(gotSeed[:])
		if err != nil || gotSeed != seed {
			t.Fatalf("Seed() = (%x, %v), want (%x, nil)", gotSeed, err, seed)
		}
		wantPrivate := ed25519.NewKeyFromSeed(seed[:])
		defer clear(wantPrivate)
		private, err := got.PrivateKey()
		defer clear(private)
		if err != nil || !bytes.Equal(private, wantPrivate) {
			t.Fatalf("PrivateKey() = (%x, %v), want (%x, nil)", private, err, wantPrivate)
		}
		public, err := got.PublicKey()
		if err != nil {
			t.Fatalf("PublicKey() error = %v, want nil", err)
		}
		raw, err := public.Bytes()
		if err != nil || !bytes.Equal(raw, wantPrivate[ed25519.SeedSize:]) {
			t.Fatalf("PublicKey.Bytes() = (%x, %v), want Go-derived identity", raw, err)
		}
		message := []byte(core.RedactedValueText)
		if !ed25519.Verify(ed25519.PublicKey(raw), message, ed25519.Sign(private, message)) {
			t.Fatal("Go verification of adopted seed = false, want true")
		}
		round, err := keygen.AdoptSigningKey(gotSeed)
		if err != nil {
			t.Fatalf("AdoptSigningKey(projected seed) error = %v, want nil", err)
		}
		t.Cleanup(func() {
			if err := round.Destroy(); err != nil {
				t.Fatalf("Destroy(round trip) error = %v, want nil", err)
			}
		})
		roundSeed, err := round.Seed()
		defer clear(roundSeed[:])
		if err != nil || roundSeed != seed {
			t.Fatalf("second Seed() = (%x, %v), want (%x, nil)", roundSeed, err, seed)
		}
	})
}

func canonicalPrivateKeySeed(tb testing.TB) []byte {
	tb.Helper()
	key, err := keygen.AdoptSigningKey(nonZeroSeed())
	if err != nil {
		tb.Fatalf("AdoptSigningKey(seed) error = %v, want nil", err)
	}
	defer func() {
		if err := key.Destroy(); err != nil {
			tb.Fatalf("Destroy(seed) error = %v, want nil", err)
		}
	}()
	if err := key.Validate(); err != nil {
		tb.Fatalf("Validate(seed) error = %v, want nil", err)
	}
	private, err := key.PrivateKey()
	if err != nil {
		tb.Fatalf("PrivateKey(seed) error = %v, want nil", err)
	}
	return private
}

func canonicalSigningSeed(tb testing.TB) [keygen.SeedSize]byte {
	tb.Helper()
	key, err := keygen.AdoptSigningKey(nonZeroSeed())
	if err != nil {
		tb.Fatalf("AdoptSigningKey(seed) error = %v, want nil", err)
	}
	defer func() {
		if err := key.Destroy(); err != nil {
			tb.Fatalf("Destroy(seed) error = %v, want nil", err)
		}
	}()
	if err := key.Validate(); err != nil {
		tb.Fatalf("Validate(seed) error = %v, want nil", err)
	}
	seed, err := key.Seed()
	if err != nil {
		tb.Fatalf("Seed(seed) error = %v, want nil", err)
	}
	return seed
}
