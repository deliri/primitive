package lease_test

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// TestDeviceIDForPublicKeyPinsThePublishedDerivationVectors fixes the exact
// bytes OGS registers. The vectors are literals, not a second run of the
// production algorithm, so re-truncating, reordering, prefixing, salting, or
// re-encoding the derivation fails here even when the reworked code is
// self-consistent. A registered installation cannot survive a change to these
// values.
func TestDeviceIDForPublicKeyPinsThePublishedDerivationVectors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		keyHex  string
		wantHex string
	}{
		{
			name:    "all zero key bytes still derive a set identity",
			keyHex:  "0000000000000000000000000000000000000000000000000000000000000000",
			wantHex: "66687aadf862bd776c8fc18b8e9f8e20",
		},
		{
			name:    "only the first key byte set",
			keyHex:  "0100000000000000000000000000000000000000000000000000000000000000",
			wantHex: "01d0fabd251fcbbe2b93b4b927b26ad2",
		},
		{
			name:    "only the last key byte set",
			keyHex:  "0000000000000000000000000000000000000000000000000000000000000001",
			wantHex: "ec4916dd28fc4c10d78e287ca5d9cc51",
		},
		{
			name:    "alternating high and low nibble bytes",
			keyHex:  "55aa55aa55aa55aa55aa55aa55aa55aa55aa55aa55aa55aa55aa55aa55aa55aa",
			wantHex: "291f6bb98d3fa955d8212c01d925fbee",
		},
		{
			name:    "every key byte at its maximum",
			keyHex:  "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			wantHex: "af9613760f72635fbdb44a5a0a63c39f",
		},
		{
			name:    "RFC 8032 test 1 public key",
			keyHex:  "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a",
			wantHex: "21fe31dfa154a261626bf854046fd227",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := lease.DeviceIDForPublicKey(publicKeyFromHex(t, tc.keyHex))
			if gotErr != nil || got.String() != tc.wantHex {
				t.Fatalf("DeviceIDForPublicKey(%s) = (%q, %v), want (%q, nil)",
					tc.keyHex, got.String(), gotErr, tc.wantHex)
			}
			repeated, repeatedErr := lease.DeviceIDForPublicKey(publicKeyFromHex(t, tc.keyHex))
			if repeatedErr != nil || repeated != got {
				t.Fatalf("DeviceIDForPublicKey(%s) repeat = (%v, %v), want (%v, nil)",
					tc.keyHex, repeated, repeatedErr, got)
			}
			parsed, parseErr := lease.ParseDeviceID(tc.wantHex)
			if parseErr != nil || parsed != got {
				t.Fatalf("ParseDeviceID(%q) = (%v, %v), want (%v, nil)",
					tc.wantHex, parsed, parseErr, got)
			}
		})
	}
}

// TestDeviceIDForPublicKeyCoversEveryKeyByte proves the derivation reads all
// ed25519.PublicKeySize bytes. A derivation that hashed only the identifier
// width, only the first half, or any other subset would leave at least one
// position inert and two distinct installations sharing one identity.
func TestDeviceIDForPublicKeyCoversEveryKeyByte(t *testing.T) {
	t.Parallel()

	var base [ed25519.PublicKeySize]byte
	baseID, err := lease.DeviceIDForPublicKey(publicKeyFromArray(t, base))
	if err != nil {
		t.Fatalf("DeviceIDForPublicKey(zero key) error = %v, want nil", err)
	}
	const unmutatedKey = "the unmutated key"
	seen := map[lease.DeviceID]string{baseID: unmutatedKey}
	for position := range base {
		mutated := base
		mutated[position] = 0x01
		got, gotErr := lease.DeviceIDForPublicKey(publicKeyFromArray(t, mutated))
		if gotErr != nil {
			t.Fatalf("DeviceIDForPublicKey(key byte %d set) error = %v, want nil", position, gotErr)
		}
		if prior, found := seen[got]; found {
			t.Fatalf("DeviceIDForPublicKey(key byte %d set) = %v, want an identity distinct from %s; key byte %d is inert",
				position, got, prior, position)
		}
		seen[got] = fmt.Sprintf("key byte %d", position)
	}
	if len(seen) != ed25519.PublicKeySize+1 {
		t.Fatalf("distinct identities = %d, want %d", len(seen), ed25519.PublicKeySize+1)
	}
}

// TestDeviceIDForPublicKeyCoversEveryBitOfOneKeyByte proves no key bit is
// masked away inside a covered byte position.
func TestDeviceIDForPublicKeyCoversEveryBitOfOneKeyByte(t *testing.T) {
	t.Parallel()

	seen := make(map[lease.DeviceID]byte, 8)
	for bit := range 8 {
		var key [ed25519.PublicKeySize]byte
		key[ed25519.PublicKeySize-1] = byte(1) << bit
		got, gotErr := lease.DeviceIDForPublicKey(publicKeyFromArray(t, key))
		if gotErr != nil {
			t.Fatalf("DeviceIDForPublicKey(last byte bit %d) error = %v, want nil", bit, gotErr)
		}
		if prior, found := seen[got]; found {
			t.Fatalf("DeviceIDForPublicKey(last byte bit %d) = %v, want an identity distinct from bit %d",
				bit, got, prior)
		}
		seen[got] = byte(bit)
	}
}

// TestDeviceIDForPublicKeyRejectsUnsetKeyWithoutAnIdentity proves the only
// rejectable key produces no identity at all rather than a zero-value one that
// would validate downstream as a real installation.
func TestDeviceIDForPublicKeyRejectsUnsetKeyWithoutAnIdentity(t *testing.T) {
	t.Parallel()

	got, gotErr := lease.DeviceIDForPublicKey(core.Ed25519PublicKey{})
	if got != (lease.DeviceID{}) || !errors.Is(gotErr, core.ErrLeaseContract) {
		t.Fatalf("DeviceIDForPublicKey(zero) = (%v, %v), want (zero, %v)", got, gotErr, core.ErrLeaseContract)
	}
	if got.Validate() == nil {
		t.Fatalf("DeviceIDForPublicKey(zero) identity Validate() = nil, want a rejection")
	}
}

// TestDeviceIDForPublicKeyRejectsEveryOffWidthKey proves no key length other
// than the exact Ed25519 public-key size can reach a derived identity.
func TestDeviceIDForPublicKeyRejectsEveryOffWidthKey(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		length int
	}{
		{name: "empty key material", length: 0},
		{name: "single byte of key material", length: 1},
		{name: "identifier width mistaken for key width", length: lease.IdentifierBytes},
		{name: "one byte below the exact public-key size", length: ed25519.PublicKeySize - 1},
		{name: "one byte above the exact public-key size", length: ed25519.PublicKeySize + 1},
		{name: "private-key width mistaken for public-key width", length: ed25519.PrivateKeySize},
		{name: "whole key pair mistaken for the public key", length: ed25519.PrivateKeySize + ed25519.PublicKeySize},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			key, err := core.NewEd25519PublicKey(make(ed25519.PublicKey, tc.length))
			if !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("NewEd25519PublicKey(%d bytes) error = %v, want %v", tc.length, err, core.ErrPrimitiveContract)
			}
			got, gotErr := lease.DeviceIDForPublicKey(key)
			if got != (lease.DeviceID{}) || !errors.Is(gotErr, core.ErrLeaseContract) {
				t.Fatalf("DeviceIDForPublicKey(%d-byte key) = (%v, %v), want (zero, %v)",
					tc.length, got, gotErr, core.ErrLeaseContract)
			}
		})
	}
}

// TestDeviceIDForPublicKeyBindsRealKeysDistinctly proves the derivation
// separates real Ed25519 installations, not just hand-built byte patterns.
// The seeds are fixed so a failure is a derivation defect, not a draw.
func TestDeviceIDForPublicKeyBindsRealKeysDistinctly(t *testing.T) {
	t.Parallel()

	const installations = 64
	seen := make(map[lease.DeviceID]struct{}, installations)
	for index := range installations {
		var seed [ed25519.SeedSize]byte
		seed[0] = byte(index)
		seed[ed25519.SeedSize-1] = byte(installations - index)
		key, err := core.NewEd25519PublicKey(
			ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey))
		if err != nil {
			t.Fatalf("NewEd25519PublicKey(installation %d) error = %v, want nil", index, err)
		}
		got, gotErr := lease.DeviceIDForPublicKey(key)
		if gotErr != nil || got.Validate() != nil {
			t.Fatalf("DeviceIDForPublicKey(installation %d) = (%v, %v), want a set identity and nil",
				index, got, gotErr)
		}
		if _, found := seen[got]; found {
			t.Fatalf("DeviceIDForPublicKey(installation %d) = %v, want an identity no prior installation holds",
				index, got)
		}
		seen[got] = struct{}{}
	}
}

func publicKeyFromHex(t testing.TB, value string) core.Ed25519PublicKey {
	t.Helper()

	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("hex.DecodeString(%q) error = %v, want nil", value, err)
	}
	key, err := core.NewEd25519PublicKey(ed25519.PublicKey(decoded))
	if err != nil {
		t.Fatalf("NewEd25519PublicKey(%q) error = %v, want nil", value, err)
	}
	return key
}

func publicKeyFromArray(t testing.TB, value [ed25519.PublicKeySize]byte) core.Ed25519PublicKey {
	t.Helper()

	key, err := core.NewEd25519PublicKey(ed25519.PublicKey(value[:]))
	if err != nil {
		t.Fatalf("NewEd25519PublicKey(%x) error = %v, want nil", value, err)
	}
	return key
}
