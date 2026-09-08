package keygen

import (
	"bytes"
	"crypto/rand"
	"errors"
	"math"
	"testing"
	"testing/cryptotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestRandomRequestsRefuseBeforeGoEntropyLayerTriad(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardGlobalRegistry, Scope: core.TestIsolationScopePackageProcess})
	for _, tc := range []struct {
		name   string
		size   uint64
		secret bool
	}{
		{name: "unissued token request consumes no entropy"},
		{name: "unissued secret request consumes no entropy", secret: true},
		{name: "below secret minimum consumes no entropy", secret: true, size: core.SecretMaterialMinimumBytes - 1},
		{name: "above secret maximum consumes no entropy", secret: true, size: core.SecretMaterialMaximumBytes + 1},
		{name: "above token maximum consumes no entropy", size: RandomTokenMaximumBytes + 1},
		{name: "maximum uint64 cannot narrow into an admitted token", size: math.MaxUint64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardGlobalRegistry, Scope: core.TestIsolationScopePackageProcess})
			var size core.ByteCount
			if tc.size != 0 {
				size = mustInternalByteCount(t, tc.size)
			}
			cryptotest.SetGlobalRandom(t, 1)
			var want [SeedSize]byte
			if _, err := rand.Read(want[:]); err != nil {
				t.Fatalf("Go Read(reference) error = %v, want nil", err)
			}
			cryptotest.SetGlobalRandom(t, 1)
			if tc.secret {
				got, err := GenerateSecret(SecretRequest{Size: size})
				if got != (core.SecretMaterial{}) || !errors.Is(err, core.ErrKeygenContract) {
					t.Fatalf("GenerateSecret(refused) = (%v,%v), want zero and Core refusal", got, err)
				}
			} else {
				got, err := RandomToken(RandomTokenRequest{Size: size})
				if got.bytes != nil || !errors.Is(err, core.ErrKeygenContract) {
					t.Fatalf("RandomToken(refused) = (%x,%v), want nil bytes and Core refusal", got.bytes, err)
				}
			}
			var next [SeedSize]byte
			if _, err := rand.Read(next[:]); err != nil {
				t.Fatalf("Go Read(next) error = %v, want nil", err)
			}
			if next != want {
				t.Fatalf("next entropy = %x, want %x after refusal", next, want)
			}
		})
	}
}

func TestTokenPrivateExtentAndZeroContent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		extent  int
		wantErr error
	}{
		{name: "undrawn storage refuses", wantErr: core.ErrKeygenContract},
		{name: "one zero byte is a legitimate public token", extent: 1},
		{name: "exact maximum zero bytes remain a legitimate public token", extent: RandomTokenMaximumBytes},
		{name: "forged over-limit storage cannot escape through Bytes", extent: RandomTokenMaximumBytes + 1, wantErr: core.ErrKeygenContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw := make([]byte, tc.extent)
			token := Token{bytes: raw}
			if err := token.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Token.Validate() error = %v, want %v", err, tc.wantErr)
			}
			got, err := token.Bytes()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Token.Bytes() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatalf("Token.Bytes(refused) = %x, want nil", got)
				}
				return
			}
			if !bytes.Equal(got, raw) {
				t.Fatalf("Token.Bytes() = %x, want %x", got, raw)
			}
			for index := range got {
				got[index] = 1
			}
			second, err := token.Bytes()
			if err != nil || !bytes.Equal(second, raw) {
				t.Fatalf("Token.Bytes(after mutation) = (%x,%v), want (%x,nil)", second, err, raw)
			}
		})
	}
}
