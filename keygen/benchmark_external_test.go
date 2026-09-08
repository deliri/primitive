package keygen_test

import (
	"bytes"
	"crypto/ed25519"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

func benchmarkSigningFixture(b *testing.B) keygen.SigningKey {
	b.Helper()
	key, err := keygen.AdoptSigningKey(nonZeroSeed())
	if err != nil {
		b.Fatalf("AdoptSigningKey(fixture) error = %v, want nil", err)
	}
	b.Cleanup(func() {
		if err := key.Destroy(); err != nil {
			b.Fatalf("Destroy(fixture) error = %v, want nil", err)
		}
	})
	return key
}

// Generation includes destruction of each owned result. The untimed probe
// checks the real generated key against Go's signing and verification APIs.
func BenchmarkGenerateSigningKey(b *testing.B) {
	probe, err := keygen.GenerateSigningKey()
	if err != nil {
		b.Fatalf("GenerateSigningKey(probe) error = %v, want nil", err)
	}
	private, err := probe.PrivateKey()
	if err != nil {
		b.Fatalf("PrivateKey(probe) error = %v, want nil", err)
	}
	message := []byte(core.RedactedValueText)
	if !ed25519.Verify(ed25519.PublicKey(private[ed25519.SeedSize:]), message, ed25519.Sign(private, message)) {
		b.Fatal("generated signing proof = false, want true")
	}
	clear(private)
	if err := probe.Destroy(); err != nil {
		b.Fatalf("Destroy(probe) error = %v, want nil", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		key, err := keygen.GenerateSigningKey()
		if err != nil {
			b.Fatalf("GenerateSigningKey() error = %v, want nil", err)
		}
		if err := key.Destroy(); err != nil {
			b.Fatalf("Destroy() error = %v, want nil", err)
		}
	}
}

func BenchmarkGenerateMaximumSecret(b *testing.B) {
	size, err := core.NewByteCount(core.SecretMaterialMaximumBytes)
	if err != nil {
		b.Fatalf("NewByteCount() error = %v, want nil", err)
	}
	request := keygen.SecretRequest{Size: size}
	if err := request.Validate(); err != nil {
		b.Fatalf("SecretRequest.Validate() error = %v, want nil", err)
	}
	probe, err := keygen.GenerateSecret(request)
	if err != nil {
		b.Fatalf("GenerateSecret(probe) error = %v, want nil", err)
	}
	count, err := probe.ByteCount()
	if err != nil || count != size {
		b.Fatalf("ByteCount(probe) = (%v, %v), want (%v, nil)", count, err, size)
	}
	if err := probe.Destroy(); err != nil {
		b.Fatalf("Destroy(probe) error = %v, want nil", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		material, err := keygen.GenerateSecret(request)
		if err != nil {
			b.Fatalf("GenerateSecret() error = %v, want nil", err)
		}
		if err := material.Destroy(); err != nil {
			b.Fatalf("Destroy() error = %v, want nil", err)
		}
	}
}

func BenchmarkAdoptPrivateKey(b *testing.B) {
	key := benchmarkSigningFixture(b)
	input, err := key.PrivateKey()
	if err != nil || len(input) != ed25519.PrivateKeySize {
		b.Fatalf("PrivateKey(fixture) = (%d bytes, %v), want (%d, nil)", len(input), err, ed25519.PrivateKeySize)
	}
	defer clear(input)
	b.ReportAllocs()
	for b.Loop() {
		got, err := keygen.AdoptPrivateKey(input)
		if err != nil {
			b.Fatalf("AdoptPrivateKey() error = %v, want nil", err)
		}
		if err := got.Destroy(); err != nil {
			b.Fatalf("Destroy() error = %v, want nil", err)
		}
	}
}

func BenchmarkSigningKeyValidate(b *testing.B) {
	key := benchmarkSigningFixture(b)
	if err := key.Validate(); err != nil {
		b.Fatalf("Validate(fixture) error = %v, want nil", err)
	}
	var gotErr error
	b.ReportAllocs()
	for b.Loop() {
		gotErr = key.Validate()
	}
	if gotErr != nil {
		b.Fatalf("Validate() error = %v, want nil", gotErr)
	}
}

func BenchmarkSigningKeySeed(b *testing.B) {
	key := benchmarkSigningFixture(b)
	want := nonZeroSeed()
	var got [keygen.SeedSize]byte
	var err error
	b.ReportAllocs()
	for b.Loop() {
		got, err = key.Seed()
	}
	if err != nil || got != want {
		b.Fatalf("Seed() = (%x, %v), want (%x, nil)", got, err, want)
	}
	clear(got[:])
}

func BenchmarkSigningKeyPrivate(b *testing.B) {
	key := benchmarkSigningFixture(b)
	seed := nonZeroSeed()
	want := ed25519.NewKeyFromSeed(seed[:])
	defer clear(want)
	var got ed25519.PrivateKey
	var err error
	b.ReportAllocs()
	for b.Loop() {
		clear(got)
		got, err = key.PrivateKey()
	}
	defer clear(got)
	if err != nil || !bytes.Equal(got, want) {
		b.Fatalf("PrivateKey() = (%x, %v), want (%x, nil)", got, err, want)
	}
}

func BenchmarkSigningKeyPublic(b *testing.B) {
	key := benchmarkSigningFixture(b)
	seed := nonZeroSeed()
	private := ed25519.NewKeyFromSeed(seed[:])
	defer clear(private)
	want, err := core.NewEd25519PublicKey(ed25519.PublicKey(private[ed25519.SeedSize:]))
	if err != nil {
		b.Fatalf("NewEd25519PublicKey() error = %v, want nil", err)
	}
	var got core.Ed25519PublicKey
	b.ReportAllocs()
	for b.Loop() {
		got, err = key.PublicKey()
	}
	if err != nil || got != want {
		b.Fatalf("PublicKey() = (%v, %v), want (%v, nil)", got, err, want)
	}
}

func BenchmarkRandomTokenMaximum(b *testing.B) {
	size, err := core.NewByteCount(keygen.RandomTokenMaximumBytes)
	if err != nil {
		b.Fatalf("NewByteCount() error = %v, want nil", err)
	}
	request := keygen.RandomTokenRequest{Size: size}
	if err := request.Validate(); err != nil {
		b.Fatalf("Validate(fixture) error = %v, want nil", err)
	}
	var got keygen.Token
	b.ReportAllocs()
	for b.Loop() {
		got, err = keygen.RandomToken(request)
	}
	if err != nil {
		b.Fatalf("RandomToken() error = %v, want nil", err)
	}
	raw, err := got.Bytes()
	if err != nil || len(raw) != keygen.RandomTokenMaximumBytes {
		b.Fatalf("Token.Bytes() = (%d bytes, %v), want (%d, nil)", len(raw), err, keygen.RandomTokenMaximumBytes)
	}
}

func BenchmarkEntropyReadMaximum(b *testing.B) {
	reader := keygen.NewEntropyReader()
	if err := reader.Validate(); err != nil {
		b.Fatalf("Validate(fixture) error = %v, want nil", err)
	}
	var raw [core.SecretMaterialMaximumBytes]byte
	var count int
	var err error
	b.ReportAllocs()
	for b.Loop() {
		count, err = reader.Read(raw[:])
	}
	if err != nil || count != len(raw) {
		b.Fatalf("Read() = (%d, %v), want (%d, nil)", count, err, len(raw))
	}
	clear(raw[:])
}

func BenchmarkRandomUint64(b *testing.B) {
	var got uint64
	var err error
	b.ReportAllocs()
	for b.Loop() {
		got, err = keygen.RandomUint64()
	}
	// Every uint64 bit pattern is admitted; entropy correctness is proved by
	// the deterministic Go cryptotest differential table, not statistical guesses.
	if err != nil {
		b.Fatalf("RandomUint64() = (%d, %v), want a uint64 and nil", got, err)
	}
}
