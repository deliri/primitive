package keygen_test

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

func TestGenerateSecretExhaustsCompleteAdmittedSizeIntervalThroughProductionCSPRNG(t *testing.T) {
	t.Parallel()

	for size := uint64(core.SecretMaterialMinimumBytes); size <= core.SecretMaterialMaximumBytes; size++ {
		count, gotCountErr := core.NewByteCount(size)
		if gotCountErr != nil {
			t.Fatalf("core.NewByteCount(%d) error = %v, want nil", size, gotCountErr)
		}
		request := keygen.SecretRequest{Size: count}
		if gotValidateErr := request.Validate(); gotValidateErr != nil {
			t.Fatalf("SecretRequest{%d}.Validate() error = %v, want nil", size, gotValidateErr)
		}
		got, gotErr := keygen.GenerateSecret(request)
		if gotErr != nil {
			t.Fatalf("GenerateSecret(%d) error = %v, want nil", size, gotErr)
		}
		if gotValidateErr := got.Validate(); gotValidateErr != nil {
			t.Fatalf("GenerateSecret(%d).Validate() error = %v, want nil", size, gotValidateErr)
		}
		gotCount, gotByteCountErr := got.ByteCount()
		if gotByteCountErr != nil {
			t.Fatalf("GenerateSecret(%d).ByteCount() error = %v, want nil", size, gotByteCountErr)
		}
		gotSize, gotSizeErr := gotCount.Uint64()
		if gotSizeErr != nil || gotSize != size {
			t.Fatalf("GenerateSecret(%d).ByteCount().Uint64() = (%d, %v), want (%d, nil)", size, gotSize, gotSizeErr, size)
		}
		if gotDestroyErr := got.Destroy(); gotDestroyErr != nil {
			t.Fatalf("GenerateSecret(%d).Destroy() error = %v, want nil", size, gotDestroyErr)
		}
	}
}

func TestSecretRequestRejectsEveryAdjacentAndExtremeInvalidSize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value uint64
		zero  bool
	}{
		{name: "unset byte count rejected", zero: true},
		{name: "one byte rejected", value: 1},
		{name: "two below minimum rejected", value: core.SecretMaterialMinimumBytes - 2},
		{name: "one below minimum rejected", value: core.SecretMaterialMinimumBytes - 1},
		{name: "one above maximum rejected", value: core.SecretMaterialMaximumBytes + 1},
		{name: "two above maximum rejected", value: core.SecretMaterialMaximumBytes + 2},
		{name: "maximum uint16 rejected", value: math.MaxUint16},
		{name: "maximum uint32 rejected", value: math.MaxUint32},
		{name: "maximum int64 rejected", value: math.MaxInt64},
		{name: "maximum uint64 rejected", value: math.MaxUint64},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var count core.ByteCount
			if !tc.zero {
				var gotCountErr error
				count, gotCountErr = core.NewByteCount(tc.value)
				if gotCountErr != nil {
					t.Fatalf("core.NewByteCount(%d) error = %v, want nil", tc.value, gotCountErr)
				}
			}
			request := keygen.SecretRequest{Size: count}
			gotValidateErr := request.Validate()
			if !errors.Is(gotValidateErr, core.ErrKeygenContract) ||
				!errors.Is(gotValidateErr, core.ErrPrimitiveContract) {
				t.Fatalf("SecretRequest{%d}.Validate() error = %v, want %v and %v", tc.value, gotValidateErr, core.ErrKeygenContract, core.ErrPrimitiveContract)
			}
			got, gotErr := keygen.GenerateSecret(request)
			if got != (core.SecretMaterial{}) ||
				!errors.Is(gotErr, core.ErrKeygenContract) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("GenerateSecret(%d) = (%v, %v), want (zero, %v and %v)", tc.value, got, gotErr, core.ErrKeygenContract, core.ErrPrimitiveContract)
			}
		})
	}
}

func TestGenerateSigningKeyUsesStandardEd25519ShapeAndProductionCSPRNG(t *testing.T) {
	t.Parallel()

	got, gotErr := keygen.GenerateSigningKey()
	if gotErr != nil {
		t.Fatalf("GenerateSigningKey() error = %v, want nil", gotErr)
	}
	if gotValidateErr := got.Validate(); gotValidateErr != nil {
		t.Fatalf("SigningKey.Validate() error = %v, want nil", gotValidateErr)
	}
	private, gotPrivateErr := got.PrivateKey()
	if gotPrivateErr != nil {
		t.Fatalf("SigningKey.PrivateKey() error = %v, want nil", gotPrivateErr)
	}
	defer clear(private)
	if len(private) != ed25519.PrivateKeySize {
		t.Fatalf("len(SigningKey.PrivateKey()) = %d, want %d", len(private), ed25519.PrivateKeySize)
	}
	public, gotPublicErr := got.PublicKey()
	if gotPublicErr != nil {
		t.Fatalf("SigningKey.PublicKey() error = %v, want nil", gotPublicErr)
	}
	publicBytes, gotPublicBytesErr := public.Bytes()
	if gotPublicBytesErr != nil {
		t.Fatalf("Ed25519PublicKey.Bytes() error = %v, want nil", gotPublicBytesErr)
	}
	if !bytes.Equal(private[ed25519.SeedSize:], publicBytes) {
		t.Fatalf("standard private-key public half = %x, want %x", private[ed25519.SeedSize:], publicBytes)
	}
	message := []byte("primitive-keygen-production-proof")
	signature := ed25519.Sign(private, message)
	if !ed25519.Verify(publicBytes, message, signature) {
		t.Fatal("ed25519.Verify(generated key) = false, want true")
	}
}
