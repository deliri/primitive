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

func nonZeroSeed() [keygen.SeedSize]byte {
	var seed [keygen.SeedSize]byte
	for index := range seed {
		seed[index] = byte(index + 1)
	}
	return seed
}

func TestAdoptPrivateKeyRejectsEverySingleBitDisagreementLayerTriad(t *testing.T) {
	t.Parallel()
	seed := nonZeroSeed()
	cases := []struct {
		name    string
		bit     int
		wantErr error
	}{{name: "canonical Go key retains exact identity", bit: -1}}
	for bit := range ed25519.PrivateKeySize * 8 {
		cases = append(cases, struct {
			name    string
			bit     int
			wantErr error
		}{name: fmt.Sprintf("changed_private_bit_%d_cannot_silently_change_identity", bit), bit: bit, wantErr: core.ErrKeygenContract})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Parent seed is an immutable value; each subtest derives and owns its
			// own mutable standard-library key and mutation.
			private := ed25519.NewKeyFromSeed(seed[:])
			defer clear(private)
			if tc.bit >= 0 {
				private[tc.bit/8] ^= 1 << uint(tc.bit%8)
			}
			before := bytes.Clone(private)
			defer clear(before)
			derived := ed25519.NewKeyFromSeed(private[:keygen.SeedSize])
			defer clear(derived)
			if tc.bit >= 0 && bytes.Equal(private, derived) {
				t.Fatalf("mutation bit %d canonical equality = true, want false", tc.bit)
			}
			got, err := keygen.AdoptPrivateKey(private)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("AdoptPrivateKey() error = %v, want %v", err, tc.wantErr)
			}
			if !bytes.Equal(private, before) {
				t.Fatalf("AdoptPrivateKey input = %x, want preserved %x", private, before)
			}
			if tc.wantErr != nil {
				if got != (keygen.SigningKey{}) || !errors.Is(err, core.ErrPrimitiveContract) {
					t.Fatalf("AdoptPrivateKey(refused) = (%v, %v), want zero and Core refusal", got, err)
				}
				return
			}
			t.Cleanup(func() {
				if err := got.Destroy(); err != nil {
					t.Fatalf("Destroy() error = %v, want nil", err)
				}
			})
			clear(private)
			projected, err := got.PrivateKey()
			defer clear(projected)
			if err != nil || !bytes.Equal(projected, derived) {
				t.Fatalf("PrivateKey(after caller clear) = (%x, %v), want (%x, nil)", projected, err, derived)
			}
			if err := got.Destroy(); err != nil {
				t.Fatalf("Destroy() error = %v, want nil", err)
			}
			refused, err := got.PrivateKey()
			if refused != nil || !errors.Is(err, core.ErrKeygenContract) {
				t.Fatalf("PrivateKey(destroyed) = (%x, %v), want nil and Core refusal", refused, err)
			}
		})
	}
}

func TestAdoptPrivateKeyExactExtentAndZeroSeed(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                string
		extent              int
		canonicalZero       bool
		wantErr, wantSource error
	}{
		{name: "absent private key", wantErr: core.ErrKeygenContract},
		{name: "seed alone cannot impersonate complete private key", extent: keygen.SeedSize, wantErr: core.ErrKeygenContract},
		{name: "private extent minus one", extent: ed25519.PrivateKeySize - 1, wantErr: core.ErrKeygenContract},
		{name: "private extent plus one", extent: ed25519.PrivateKeySize + 1, wantErr: core.ErrKeygenContract},
		{name: "oversized input refused without copying", extent: core.JSONDocumentMaximumBytes + 1, wantErr: core.ErrKeygenContract},
		{name: "zero seed and zero public remain entropy refusal", extent: ed25519.PrivateKeySize, wantErr: core.ErrKeygenEntropy, wantSource: core.ErrSecretMaterialAllZero},
		{name: "Go canonical zero seed remains entropy refusal", canonicalZero: true, wantErr: core.ErrKeygenEntropy, wantSource: core.ErrSecretMaterialAllZero},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			private := make(ed25519.PrivateKey, tc.extent)
			if tc.canonicalZero {
				private = ed25519.NewKeyFromSeed(make([]byte, keygen.SeedSize))
			}
			defer clear(private)
			before := bytes.Clone(private)
			defer clear(before)
			got, err := keygen.AdoptPrivateKey(private)
			if got != (keygen.SigningKey{}) || !errors.Is(err, tc.wantErr) || !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("AdoptPrivateKey() = (%v, %v), want zero and %v", got, err, tc.wantErr)
			}
			if tc.wantSource != nil && !errors.Is(err, tc.wantSource) {
				t.Fatalf("AdoptPrivateKey source error = %v, want %v", err, tc.wantSource)
			}
			if !bytes.Equal(private, before) {
				t.Fatalf("input preserved = false, want true for %s", tc.name)
			}
		})
	}
}

func TestAdoptSigningKeyPreservesEverySeedBitAndGoIdentity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		seed    [keygen.SeedSize]byte
		wantErr error
	}{
		{name: "zero seed cannot become an active signing key", wantErr: core.ErrKeygenEntropy},
		{name: "every seed position retains its distinct byte", seed: nonZeroSeed()},
	}
	for bit := range keygen.SeedSize * 8 {
		var seed [keygen.SeedSize]byte
		seed[bit/8] = 1 << uint(bit%8)
		cases = append(cases, struct {
			name    string
			seed    [keygen.SeedSize]byte
			wantErr error
		}{name: fmt.Sprintf("single_seed_bit_%d_survives_adoption", bit), seed: seed})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := keygen.AdoptSigningKey(tc.seed)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("AdoptSigningKey() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (keygen.SigningKey{}) || !errors.Is(err, core.ErrSecretMaterialAllZero) {
					t.Fatalf("AdoptSigningKey(zero) = (%v, %v), want zero and all-zero identity", got, err)
				}
				return
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
			if err != nil || seed != tc.seed {
				t.Fatalf("Seed() = (%x, %v), want (%x, nil)", seed, err, tc.seed)
			}
			want := ed25519.NewKeyFromSeed(tc.seed[:])
			defer clear(want)
			private, err := got.PrivateKey()
			defer clear(private)
			if err != nil || !bytes.Equal(private, want) {
				t.Fatalf("PrivateKey() = (%x, %v), want (%x, nil)", private, err, want)
			}
			public, err := got.PublicKey()
			if err != nil {
				t.Fatalf("PublicKey() error = %v, want nil", err)
			}
			raw, err := public.Bytes()
			if err != nil || !bytes.Equal(raw, want[ed25519.SeedSize:]) {
				t.Fatalf("PublicKey.Bytes() = (%x, %v), want Go-derived identity", raw, err)
			}
			message := []byte(core.RedactedValueText)
			if !ed25519.Verify(ed25519.PublicKey(raw), message, ed25519.Sign(private, message)) {
				t.Fatal("Go verification = false, want true")
			}
			readopted, err := keygen.AdoptPrivateKey(private)
			if err != nil {
				t.Fatalf("AdoptPrivateKey(projected) error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := readopted.Destroy(); err != nil {
					t.Fatalf("Destroy(readopted) error = %v, want nil", err)
				}
			})
			second, err := readopted.Seed()
			defer clear(second[:])
			if err != nil || second != seed {
				t.Fatalf("round-trip seed = (%x, %v), want (%x, nil)", second, err, seed)
			}
		})
	}
}
