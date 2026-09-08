package keygen_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"
	"testing/cryptotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
	"github.com/deliri/primitive/v2026/testserial"
)

// Keygen is the entropy adapter under test. Go's sanctioned cryptotest hook
// resets Go itself; these tests neither replace rand.Reader nor add a provider
// API to production. Expected bytes are drawn from the same installed Go
// implementation after resetting the stream, not pinned across Go releases.
func TestEntropyDoorsMatchGoAndPreserveStreamOnRefusalLayerTriad(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardGlobalRegistry, Scope: core.TestIsolationScopePackageProcess})
	for _, tc := range []struct {
		name    string
		extent  int
		issued  bool
		wantErr error
	}{
		{name: "unissued empty capability cannot become an issued no-op", wantErr: core.ErrKeygenContract},
		{name: "unissued destination refuses without entropy", extent: 1, wantErr: core.ErrKeygenContract},
		{name: "issued empty destination consumes no entropy", issued: true},
		{name: "one byte fills only caller-owned window", extent: 1, issued: true},
		{name: "maximum minus one fills only caller-owned window", extent: core.SecretMaterialMaximumBytes - 1, issued: true},
		{name: "exact maximum fills only caller-owned window", extent: core.SecretMaterialMaximumBytes, issued: true},
		{name: "above maximum preserves destination and next draw", extent: core.SecretMaterialMaximumBytes + 1, issued: true, wantErr: core.ErrKeygenContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardGlobalRegistry, Scope: core.TestIsolationScopePackageProcess})
			const streamSeed uint64 = 0x91a705
			want := bytes.Repeat([]byte{0xa5}, tc.extent+2)
			cryptotest.SetGlobalRandom(t, streamSeed)
			wantCount := 0
			if tc.wantErr == nil {
				var err error
				wantCount, err = rand.Read(want[1 : 1+tc.extent])
				if err != nil {
					t.Fatalf("Go Read(reference) error = %v, want nil", err)
				}
			}
			var wantNext [keygen.SeedSize]byte
			if _, err := rand.Read(wantNext[:]); err != nil {
				t.Fatalf("Go Read(next reference) error = %v, want nil", err)
			}
			cryptotest.SetGlobalRandom(t, streamSeed)
			reader := keygen.EntropyReader{}
			if tc.issued {
				reader = keygen.NewEntropyReader()
			}
			got := bytes.Repeat([]byte{0xa5}, tc.extent+2)
			count, err := reader.Read(got[1 : 1+tc.extent])
			if !errors.Is(err, tc.wantErr) || count != wantCount || !bytes.Equal(got, want) {
				t.Fatalf("Read() = (%x, %d, %v), want (%x, %d, %v)", got, count, err, want, wantCount, tc.wantErr)
			}
			var gotNext [keygen.SeedSize]byte
			if _, err := rand.Read(gotNext[:]); err != nil {
				t.Fatalf("Go Read(next observation) error = %v, want nil", err)
			}
			if gotNext != wantNext {
				t.Fatalf("next entropy = %x, want %x; exact draw extent changed", gotNext, wantNext)
			}
		})
	}
}

func TestRandomTokenEveryExtentMatchesGoAndOwnsItsCopy(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardGlobalRegistry, Scope: core.TestIsolationScopePackageProcess})
	for extent := 1; extent <= keygen.RandomTokenMaximumBytes; extent++ {
		t.Run(fmt.Sprintf("exact_%d_byte_draw_and_independent_projection", extent), func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardGlobalRegistry, Scope: core.TestIsolationScopePackageProcess})
			cryptotest.SetGlobalRandom(t, uint64(extent))
			want := make([]byte, extent)
			if _, err := rand.Read(want); err != nil {
				t.Fatalf("Go Read(reference) error = %v, want nil", err)
			}
			cryptotest.SetGlobalRandom(t, uint64(extent))
			token, err := keygen.RandomToken(keygen.RandomTokenRequest{Size: requireByteCount(t, uint64(extent))})
			if err != nil {
				t.Fatalf("RandomToken() error = %v, want nil", err)
			}
			got, err := token.Bytes()
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("Token.Bytes() = (%x, %v), want (%x, nil)", got, err, want)
			}
			for index := range got {
				got[index] ^= 0xff
			}
			again, err := token.Bytes()
			if err != nil || !bytes.Equal(again, want) {
				t.Fatalf("Token.Bytes(after caller mutation) = (%x, %v), want (%x, nil)", again, err, want)
			}
		})
	}
}

func TestSecretEveryExtentMatchesGoAndOwnsItsCopy(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardGlobalRegistry, Scope: core.TestIsolationScopePackageProcess})
	for extent := core.SecretMaterialMinimumBytes; extent <= core.SecretMaterialMaximumBytes; extent++ {
		t.Run(fmt.Sprintf("exact_%d_byte_secret_and_owned_destruction", extent), func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardGlobalRegistry, Scope: core.TestIsolationScopePackageProcess})
			cryptotest.SetGlobalRandom(t, uint64(extent))
			want := make([]byte, extent)
			defer clear(want)
			if _, err := rand.Read(want); err != nil {
				t.Fatalf("Go Read(reference) error = %v, want nil", err)
			}
			cryptotest.SetGlobalRandom(t, uint64(extent))
			got, err := keygen.GenerateSecret(keygen.SecretRequest{Size: requireByteCount(t, uint64(extent))})
			if err != nil {
				t.Fatalf("GenerateSecret() error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := got.Destroy(); err != nil {
					t.Fatalf("Destroy() error = %v, want nil", err)
				}
			})
			raw, err := got.CopyBytes()
			defer clear(raw)
			if err != nil || !bytes.Equal(raw, want) {
				t.Fatalf("CopyBytes() = (%x, %v), want (%x, nil)", raw, err, want)
			}
			clear(raw)
			second, err := got.CopyBytes()
			defer clear(second)
			if err != nil || !bytes.Equal(second, want) {
				t.Fatalf("CopyBytes(after clear) = (%x, %v), want (%x, nil)", second, err, want)
			}
		})
	}
}

func TestRandomUint64UsesExactGoBytesAndBigEndianProjection(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardGlobalRegistry, Scope: core.TestIsolationScopePackageProcess})
	for _, tc := range []struct {
		name string
		seed uint64
	}{
		{name: "zero stream seed"}, {name: "high stream seed bit", seed: uint64(1) << 63},
		{name: "all stream seed bits", seed: ^uint64(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardGlobalRegistry, Scope: core.TestIsolationScopePackageProcess})
			cryptotest.SetGlobalRandom(t, tc.seed)
			var raw [8]byte
			if _, err := rand.Read(raw[:]); err != nil {
				t.Fatalf("Go Read(reference) error = %v, want nil", err)
			}
			want := binary.BigEndian.Uint64(raw[:])
			cryptotest.SetGlobalRandom(t, tc.seed)
			got, err := keygen.RandomUint64()
			if err != nil || got != want {
				t.Fatalf("RandomUint64() = (%d, %v), want (%d, nil)", got, err, want)
			}
		})
	}
}

func TestGeneratedSigningKeyMatchesInstalledGoDerivation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardGlobalRegistry, Scope: core.TestIsolationScopePackageProcess})
	for _, tc := range []struct {
		name string
		seed uint64
	}{
		{name: "zero stream seed"}, {name: "high stream seed bit", seed: uint64(1) << 63},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardGlobalRegistry, Scope: core.TestIsolationScopePackageProcess})
			cryptotest.SetGlobalRandom(t, tc.seed)
			wantPublic, wantPrivate, err := ed25519.GenerateKey(nil)
			if err != nil {
				t.Fatalf("Go GenerateKey(reference) error = %v, want nil", err)
			}
			defer clear(wantPrivate)
			cryptotest.SetGlobalRandom(t, tc.seed)
			got, err := keygen.GenerateSigningKey()
			if err != nil {
				t.Fatalf("GenerateSigningKey() error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := got.Destroy(); err != nil {
					t.Fatalf("Destroy() error = %v, want nil", err)
				}
			})
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
			if err != nil || !bytes.Equal(raw, wantPublic) {
				t.Fatalf("PublicKey.Bytes() = (%x, %v), want (%x, nil)", raw, err, wantPublic)
			}
		})
	}
}
