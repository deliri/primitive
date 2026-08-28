package keygen

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// allZero reports whether every byte was cleared. It is a test-side assertion
// about buffer hygiene, not a copy of Core's all-zero rejection rule: production
// asks Core for that decision through core.ErrSecretMaterialAllZero.
func allZero(value []byte) bool {
	var aggregate byte
	for _, part := range value {
		aggregate |= part
	}
	return aggregate == 0
}

func TestGenerateSecretSourceBoundaryPreservesResultErrorIdentityAndClearing(t *testing.T) {
	t.Parallel()

	sourceErr := errors.New("hostile entropy read failure")
	cases := []struct {
		wantErr error
		err     error
		name    string
		size    uint64
		count   int
		fill    byte
	}{
		{name: "minimum exact nonzero fill constructs material and clears temporary", size: core.SecretMaterialMinimumBytes, count: core.SecretMaterialMinimumBytes, fill: 1},
		{name: "maximum exact nonzero fill constructs material and clears temporary", size: core.SecretMaterialMaximumBytes, count: core.SecretMaterialMaximumBytes, fill: 2},
		{name: "minimum full count with source error rejects and clears temporary", size: core.SecretMaterialMinimumBytes, count: core.SecretMaterialMinimumBytes, fill: 3, err: sourceErr, wantErr: core.ErrKeygenEntropy},
		{name: "maximum full count with source error rejects and clears temporary", size: core.SecretMaterialMaximumBytes, count: core.SecretMaterialMaximumBytes, fill: 4, err: sourceErr, wantErr: core.ErrKeygenEntropy},
		{name: "zero count without error rejects and clears temporary", size: core.SecretMaterialMinimumBytes, fill: 5, wantErr: core.ErrKeygenEntropy},
		{name: "one byte count without error rejects and clears temporary", size: core.SecretMaterialMinimumBytes, count: 1, fill: 6, wantErr: core.ErrKeygenEntropy},
		{name: "two below requested count rejects and clears temporary", size: core.SecretMaterialMinimumBytes, count: core.SecretMaterialMinimumBytes - 2, fill: 7, wantErr: core.ErrKeygenEntropy},
		{name: "one below requested count rejects and clears temporary", size: core.SecretMaterialMinimumBytes, count: core.SecretMaterialMinimumBytes - 1, fill: 8, wantErr: core.ErrKeygenEntropy},
		{name: "one below requested count with source error rejects and clears temporary", size: core.SecretMaterialMinimumBytes, count: core.SecretMaterialMinimumBytes - 1, fill: 9, err: sourceErr, wantErr: core.ErrKeygenEntropy},
		{name: "negative count rejects and clears temporary", size: core.SecretMaterialMinimumBytes, count: -1, fill: 10, wantErr: core.ErrKeygenEntropy},
		{name: "one above requested count rejects and clears temporary", size: core.SecretMaterialMinimumBytes, count: core.SecretMaterialMinimumBytes + 1, fill: 11, wantErr: core.ErrKeygenEntropy},
		{name: "maximum integer count rejects and clears temporary", size: core.SecretMaterialMinimumBytes, count: math.MaxInt, fill: 12, wantErr: core.ErrKeygenEntropy},
		{name: "error before any reported byte rejects and clears temporary", size: core.SecretMaterialMinimumBytes, fill: 13, err: sourceErr, wantErr: core.ErrKeygenEntropy},
		{name: "exact all-zero fill rejects and clears temporary", size: core.SecretMaterialMinimumBytes, count: core.SecretMaterialMinimumBytes, wantErr: core.ErrKeygenEntropy},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := SecretRequest{Size: mustInternalByteCount(t, tc.size)}
			var retained []byte
			read := func(destination []byte) (int, error) {
				retained = destination
				for index := range destination {
					destination[index] = tc.fill
				}
				return tc.count, tc.err
			}
			got, gotErr := generateSecretWithRead(request, read)
			if tc.wantErr == nil {
				proveGeneratedSecretFill(t, got, gotErr, tc.size, tc.fill)
			} else {
				proveRejectedGeneratedSecret(t, got, gotErr, tc.wantErr)
			}
			if tc.err != nil && !errors.Is(gotErr, tc.err) {
				t.Fatalf("generateSecretWithRead() error = %v, want wrapped source error %v", gotErr, tc.err)
			}
			if len(retained) != int(tc.size) {
				t.Fatalf("retained entropy destination bytes = %d, want %d", len(retained), tc.size)
			}
			if !allZero(retained) {
				t.Fatalf("retained entropy destination = %x, want cleared zero bytes", retained)
			}
		})
	}
}

func TestGenerateSecretRejectsEveryInvalidRequestBeforeEntropyEffect(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value uint64
		unset bool
	}{
		{name: "unset byte count rejects before effect", unset: true},
		{name: "one byte rejects before effect", value: 1},
		{name: "two below minimum rejects before effect", value: core.SecretMaterialMinimumBytes - 2},
		{name: "one below minimum rejects before effect", value: core.SecretMaterialMinimumBytes - 1},
		{name: "one above maximum rejects before effect", value: core.SecretMaterialMaximumBytes + 1},
		{name: "two above maximum rejects before effect", value: core.SecretMaterialMaximumBytes + 2},
		{name: "maximum uint16 rejects before effect", value: math.MaxUint16},
		{name: "maximum uint32 rejects before effect", value: math.MaxUint32},
		{name: "maximum int64 rejects before effect", value: math.MaxInt64},
		{name: "maximum uint64 rejects before effect", value: math.MaxUint64},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var size core.ByteCount
			if !tc.unset {
				size = mustInternalByteCount(t, tc.value)
			}
			gotCalls := 0
			got, gotErr := generateSecretWithRead(
				SecretRequest{Size: size},
				func([]byte) (int, error) {
					gotCalls++
					return 0, nil
				},
			)
			proveRejectedGeneratedSecret(
				t,
				got,
				gotErr,
				core.ErrKeygenContract,
			)
			if gotCalls != 0 {
				t.Fatalf("entropy read calls = %d, want 0 before validated allocation", gotCalls)
			}
		})
	}
}

func TestGenerateSecretRejectsMissingPrivateEntropyCapability(t *testing.T) {
	t.Parallel()

	request := SecretRequest{
		Size: mustInternalByteCount(t, core.SecretMaterialMinimumBytes),
	}
	got, gotErr := generateSecretWithRead(request, nil)
	proveRejectedGeneratedSecret(t, got, gotErr, core.ErrKeygenContract)
}

// TestSigningKeyInternalForgedCustodyMatrix proves the two structural gates in
// validatedSeed that no external test can reach. GenerateSigningKey is the only
// external producer and it never builds a wrong pairing, so without forging the
// struct the "seed has invalid extent" and "public key unset" arms are carried
// by inspection alone. Core admits 16-to-64-byte material, so a signing key
// holding non-Ed25519-width custody is a representable state that must be
// refused at every boundary rather than silently truncated into a 32-byte seed.
func TestSigningKeyInternalForgedCustodyMatrix(t *testing.T) {
	t.Parallel()

	publicKey, privateKey, generateErr := deterministicEd25519Result(7)
	if generateErr != nil {
		t.Fatalf("deterministicEd25519Result() error = %v, want nil", generateErr)
	}
	ownedPublic, ownedPublicErr := core.NewEd25519PublicKey(publicKey)
	if ownedPublicErr != nil {
		t.Fatalf("core.NewEd25519PublicKey() error = %v, want nil", ownedPublicErr)
	}
	cases := []struct {
		setup   func(testing.TB) SigningKey
		wantErr error
		name    string
	}{
		{
			name:    "exact seed width with unset public rejects",
			wantErr: core.ErrKeygenContract,
			setup: func(t testing.TB) SigningKey {
				return SigningKey{seed: forgedMaterialFixture(t, privateKey.Seed())}
			},
		},
		{
			name:    "minimum core width seed rejects",
			wantErr: core.ErrKeygenContract,
			setup: func(t testing.TB) SigningKey {
				raw := bytes.Repeat([]byte{0x31}, core.SecretMaterialMinimumBytes)
				return SigningKey{seed: forgedMaterialFixture(t, raw), public: ownedPublic}
			},
		},
		{
			name:    "one below seed width rejects",
			wantErr: core.ErrKeygenContract,
			setup: func(t testing.TB) SigningKey {
				raw := bytes.Repeat([]byte{0x32}, ed25519.SeedSize-1)
				return SigningKey{seed: forgedMaterialFixture(t, raw), public: ownedPublic}
			},
		},
		{
			name:    "one above seed width rejects",
			wantErr: core.ErrKeygenContract,
			setup: func(t testing.TB) SigningKey {
				raw := bytes.Repeat([]byte{0x33}, ed25519.SeedSize+1)
				return SigningKey{seed: forgedMaterialFixture(t, raw), public: ownedPublic}
			},
		},
		{
			name:    "maximum core width seed rejects",
			wantErr: core.ErrKeygenContract,
			setup: func(t testing.TB) SigningKey {
				raw := bytes.Repeat([]byte{0x34}, core.SecretMaterialMaximumBytes)
				return SigningKey{seed: forgedMaterialFixture(t, raw), public: ownedPublic}
			},
		},
		{
			name:    "exact seed width with unrelated public rejects",
			wantErr: core.ErrKeygenContract,
			setup: func(t testing.TB) SigningKey {
				raw := bytes.Repeat([]byte{0x35}, ed25519.SeedSize)
				return SigningKey{seed: forgedMaterialFixture(t, raw), public: ownedPublic}
			},
		},
		{
			name: "exact seed width with its derived public validates",
			setup: func(t testing.TB) SigningKey {
				return SigningKey{seed: forgedMaterialFixture(t, privateKey.Seed()), public: ownedPublic}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			key := tc.setup(t)
			if gotErr := key.Validate(); !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("SigningKey.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
			gotPublic, gotPublicErr := key.PublicKey()
			if !errors.Is(gotPublicErr, tc.wantErr) {
				t.Fatalf("SigningKey.PublicKey() error = %v, want %v", gotPublicErr, tc.wantErr)
			}
			gotPrivate, gotPrivateErr := key.PrivateKey()
			if !errors.Is(gotPrivateErr, tc.wantErr) {
				t.Fatalf("SigningKey.PrivateKey() error = %v, want %v", gotPrivateErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if gotPublic != (core.Ed25519PublicKey{}) || gotPrivate != nil {
					t.Fatalf(
						"rejected SigningKey projections = (%v, %v), want (zero, nil)",
						gotPublic, gotPrivate,
					)
				}
				return
			}
			if len(gotPrivate) != ed25519.PrivateKeySize {
				t.Fatalf("SigningKey.PrivateKey() length = %d, want %d", len(gotPrivate), ed25519.PrivateKeySize)
			}
			clear(gotPrivate)
			if gotDestroyErr := key.Destroy(); gotDestroyErr != nil {
				t.Fatalf("SigningKey.Destroy() error = %v, want nil", gotDestroyErr)
			}
			if gotErr := key.Validate(); !errors.Is(gotErr, core.ErrKeygenContract) {
				t.Fatalf("destroyed SigningKey.Validate() error = %v, want %v", gotErr, core.ErrKeygenContract)
			}
		})
	}
}

func forgedMaterialFixture(t testing.TB, raw []byte) core.SecretMaterial {
	t.Helper()
	material, err := core.NewSecretMaterial(raw)
	clear(raw)
	if err != nil {
		t.Fatalf("core.NewSecretMaterial() error = %v, want nil", err)
	}
	return material
}

func TestAdoptGeneratedSigningKeyHostileStandardLibraryResultMatrix(t *testing.T) {
	t.Parallel()

	sourceErr := errors.New("hostile Ed25519 generation failure")
	cases := []struct {
		setup   func() (ed25519.PublicKey, ed25519.PrivateKey, error)
		wantErr error
		name    string
	}{
		{
			name: "valid low deterministic standard result is adopted and cleared",
			setup: func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
				return deterministicEd25519Result(1)
			},
		},
		{
			name: "valid high deterministic standard result is adopted and cleared",
			setup: func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
				return deterministicEd25519Result(math.MaxUint8)
			},
		},
		{
			name:    "source error rejects and clears complete result",
			wantErr: core.ErrKeygenEntropy,
			setup: func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
				public, private, _ := deterministicEd25519Result(2)
				return public, private, sourceErr
			},
		},
		{
			name:    "source error rejects nil result",
			wantErr: core.ErrKeygenEntropy,
			setup: func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
				return nil, nil, sourceErr
			},
		},
		{
			name:    "nil public and private result rejects",
			wantErr: core.ErrKeygenContract,
			setup: func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
				return nil, nil, nil
			},
		},
		{
			name:    "nil public result rejects and clears complete private result",
			wantErr: core.ErrKeygenContract,
			setup: func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
				_, private, _ := deterministicEd25519Result(3)
				return nil, private, nil
			},
		},
		{
			name:    "nil private result rejects and clears complete public result",
			wantErr: core.ErrKeygenContract,
			setup: func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
				public, _, _ := deterministicEd25519Result(4)
				return public, nil, nil
			},
		},
		{
			name:    "one-byte-short public key rejects and clears private result",
			wantErr: core.ErrKeygenContract,
			setup: func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
				_, private, _ := deterministicEd25519Result(5)
				return make(ed25519.PublicKey, ed25519.PublicKeySize-1), private, nil
			},
		},
		{
			name:    "one-byte-long public key rejects and clears private result",
			wantErr: core.ErrKeygenContract,
			setup: func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
				_, private, _ := deterministicEd25519Result(6)
				return make(ed25519.PublicKey, ed25519.PublicKeySize+1), private, nil
			},
		},
		{
			name:    "one-byte-short private key rejects and clears public result",
			wantErr: core.ErrKeygenContract,
			setup: func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
				public, _, _ := deterministicEd25519Result(7)
				return public, make(ed25519.PrivateKey, ed25519.PrivateKeySize-1), nil
			},
		},
		{
			name:    "one-byte-long private key rejects and clears public result",
			wantErr: core.ErrKeygenContract,
			setup: func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
				public, _, _ := deterministicEd25519Result(8)
				return public, make(ed25519.PrivateKey, ed25519.PrivateKeySize+1), nil
			},
		},
		{
			name:    "mismatched public key rejects and clears both results",
			wantErr: core.ErrKeygenContract,
			setup: func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
				public, private, _ := deterministicEd25519Result(9)
				public[0] ^= math.MaxUint8
				return public, private, nil
			},
		},
		{
			name:    "all-zero public key rejects derived relationship and clears both results",
			wantErr: core.ErrKeygenContract,
			setup: func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
				_, private, _ := deterministicEd25519Result(10)
				return make(ed25519.PublicKey, ed25519.PublicKeySize), private, nil
			},
		},
		{
			name:    "all-zero private seed rejects as entropy failure and clears result",
			wantErr: core.ErrKeygenEntropy,
			setup: func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
				private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
				public := append(ed25519.PublicKey(nil), private[ed25519.SeedSize:]...)
				return public, private, nil
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			public, private, gotSourceErr := tc.setup()
			got, gotErr := adoptGeneratedSigningKey(public, private, gotSourceErr)
			if tc.wantErr == nil {
				if gotErr != nil {
					t.Fatalf("adoptGeneratedSigningKey() error = %v, want nil", gotErr)
				}
				if gotValidateErr := got.Validate(); gotValidateErr != nil {
					t.Fatalf("adopted SigningKey.Validate() error = %v, want nil", gotValidateErr)
				}
				if gotDestroyErr := got.Destroy(); gotDestroyErr != nil {
					t.Fatalf("adopted SigningKey.Destroy() error = %v, want nil", gotDestroyErr)
				}
			} else {
				proveRejectedSigningKey(t, got, gotErr, tc.wantErr)
			}
			if gotSourceErr != nil && !errors.Is(gotErr, gotSourceErr) {
				t.Fatalf("adoptGeneratedSigningKey() error = %v, want wrapped source error %v", gotErr, gotSourceErr)
			}
			if !allZero(public) {
				t.Fatalf("adopted standard public result = %x, want cleared zero bytes", public)
			}
			if !allZero(private) {
				t.Fatalf("adopted standard private result = %x, want cleared zero bytes", private)
			}
		})
	}
}

func mustInternalByteCount(t testing.TB, value uint64) core.ByteCount {
	t.Helper()

	count, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("core.NewByteCount(%d) error = %v, want nil", value, err)
	}
	return count
}

func proveGeneratedSecretFill(
	t testing.TB,
	got core.SecretMaterial,
	gotErr error,
	wantSize uint64,
	wantFill byte,
) {
	t.Helper()

	if gotErr != nil {
		t.Fatalf("generateSecretWithRead() error = %v, want nil", gotErr)
	}
	raw, gotRawErr := got.CopyBytes()
	if gotRawErr != nil {
		t.Fatalf("SecretMaterial.CopyBytes() error = %v, want nil", gotRawErr)
	}
	if !bytes.Equal(raw, bytes.Repeat([]byte{wantFill}, int(wantSize))) {
		t.Fatalf("generated secret = %x, want %d bytes of %x", raw, wantSize, wantFill)
	}
	clear(raw)
	if gotDestroyErr := got.Destroy(); gotDestroyErr != nil {
		t.Fatalf("SecretMaterial.Destroy() error = %v, want nil", gotDestroyErr)
	}
}

func proveRejectedGeneratedSecret(
	t testing.TB,
	got core.SecretMaterial,
	gotErr error,
	wantErr error,
) {
	t.Helper()

	if got != (core.SecretMaterial{}) ||
		!errors.Is(gotErr, wantErr) ||
		!errors.Is(gotErr, core.ErrKeygenContract) ||
		!errors.Is(gotErr, core.ErrPrimitiveContract) {
		t.Fatalf(
			"generateSecretWithRead() = (%v, %v), want (zero, %v, %v, and %v)",
			got,
			gotErr,
			wantErr,
			core.ErrKeygenContract,
			core.ErrPrimitiveContract,
		)
	}
}

func proveRejectedSigningKey(
	t testing.TB,
	got SigningKey,
	gotErr error,
	wantErr error,
) {
	t.Helper()

	if got != (SigningKey{}) ||
		!errors.Is(gotErr, wantErr) ||
		!errors.Is(gotErr, core.ErrKeygenContract) ||
		!errors.Is(gotErr, core.ErrPrimitiveContract) {
		t.Fatalf(
			"adoptGeneratedSigningKey() = (%v, %v), want (zero, %v, %v, and %v)",
			got,
			gotErr,
			wantErr,
			core.ErrKeygenContract,
			core.ErrPrimitiveContract,
		)
	}
}

func deterministicEd25519Result(first byte) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	seed := make([]byte, ed25519.SeedSize)
	seed[0] = first
	private := ed25519.NewKeyFromSeed(seed)
	clear(seed)
	public := append(ed25519.PublicKey(nil), private[ed25519.SeedSize:]...)
	return public, private, nil
}

// TestEntropyRejectionCarriesCoreAllZeroIdentityAndSeparatesSourceFailure pins
// what a caller can actually observe about an entropy rejection.
//
// Scope, stated honestly: this test proves Core's all-zero identity survives
// keygen's wrapping and that a short read is not labelled with it. It does not
// prove which predicate selected the branch. SecretRequest.Validate already
// pins the size to Core's exact interval, so an all-zero buffer is the only
// value Core can reject here; a local predicate and Core's identity therefore
// agree on every input this seam can produce, and swapping one for the other is
// invisible at run time. The branch choice is a source-level property, and the
// remaining proof surface is an architecture scan asserting that keygen
// production declares no all-zero predicate of its own.
func TestEntropyRejectionCarriesCoreAllZeroIdentityAndSeparatesSourceFailure(t *testing.T) {
	t.Parallel()

	sizes := []struct {
		name string
		size uint64
	}{
		{name: "minimum admitted extent", size: core.SecretMaterialMinimumBytes},
		{name: "one above the minimum extent", size: core.SecretMaterialMinimumBytes + 1},
		{name: "one below the maximum extent", size: core.SecretMaterialMaximumBytes - 1},
		{name: "maximum admitted extent", size: core.SecretMaterialMaximumBytes},
	}
	for _, tc := range sizes {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := SecretRequest{Size: mustInternalByteCount(t, tc.size)}
			read := func(destination []byte) (int, error) {
				clear(destination)
				return len(destination), nil
			}
			got, gotErr := generateSecretWithRead(request, read)
			if !errors.Is(gotErr, core.ErrSecretMaterialAllZero) {
				t.Fatalf(
					"generateSecretWithRead(%d all-zero bytes) error = %v, want Core identity %v",
					tc.size,
					gotErr,
					core.ErrSecretMaterialAllZero,
				)
			}
			if !errors.Is(gotErr, core.ErrKeygenEntropy) {
				t.Fatalf(
					"generateSecretWithRead(%d all-zero bytes) error = %v, want %v",
					tc.size,
					gotErr,
					core.ErrKeygenEntropy,
				)
			}
			if got != (core.SecretMaterial{}) {
				t.Fatalf("generateSecretWithRead(%d all-zero bytes) = %v, want the zero handle", tc.size, got)
			}

			// A single nonzero byte must produce material, so the entropy
			// verdict follows Core's rule and not the requested extent.
			nonzero := func(destination []byte) (int, error) {
				clear(destination)
				destination[len(destination)-1] = 1
				return len(destination), nil
			}
			accepted, acceptedErr := generateSecretWithRead(request, nonzero)
			if acceptedErr != nil {
				t.Fatalf("generateSecretWithRead(%d bytes, one nonzero) error = %v, want nil", tc.size, acceptedErr)
			}
			if gotValidateErr := accepted.Validate(); gotValidateErr != nil {
				t.Fatalf("generated material Validate() error = %v, want nil", gotValidateErr)
			}
		})
	}

	// A short read is a source failure, not an all-zero rejection. Keeping the
	// two apart proves the Core identity is not attached to every failure.
	shortRead := func(destination []byte) (int, error) {
		clear(destination)
		return len(destination) - 1, nil
	}
	request := SecretRequest{Size: mustInternalByteCount(t, core.SecretMaterialMinimumBytes)}
	_, gotShortErr := generateSecretWithRead(request, shortRead)
	if !errors.Is(gotShortErr, core.ErrKeygenEntropy) {
		t.Fatalf("generateSecretWithRead(short read) error = %v, want %v", gotShortErr, core.ErrKeygenEntropy)
	}
	if errors.Is(gotShortErr, core.ErrSecretMaterialAllZero) {
		t.Fatalf(
			"generateSecretWithRead(short read) error = %v, want no %v match; Core never saw the value",
			gotShortErr,
			core.ErrSecretMaterialAllZero,
		)
	}
}
