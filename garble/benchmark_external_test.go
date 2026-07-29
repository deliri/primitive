package garble_test

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/garble"
)

func BenchmarkDeriveSeed(b *testing.B) {
	material, gotMaterialErr := core.NewSecretMaterial(bytes.Repeat([]byte{0x42}, garble.CustodyBytes))
	if gotMaterialErr != nil {
		b.Fatalf("NewSecretMaterial() error = %v, want nil", gotMaterialErr)
	}
	custody, gotCustodyErr := garble.NewCustody(material)
	if gotCustodyErr != nil {
		b.Fatalf("NewCustody() error = %v, want nil", gotCustodyErr)
	}
	identity, gotIdentityErr := garble.NewDerivationIdentity(
		core.NewSHA256Digest(sha256.Sum256([]byte("benchmark-release"))),
	)
	if gotIdentityErr != nil {
		b.Fatalf("NewDerivationIdentity() error = %v, want nil", gotIdentityErr)
	}
	request := garble.DeriveRequest{
		Custody:    custody,
		Identity:   identity,
		Generation: garble.CurrentDerivationGeneration(),
	}
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, gotErr := garble.Derive(request); gotErr != nil {
			b.Fatalf("Derive() error = %v, want nil", gotErr)
		}
	}
}

func BenchmarkParseSeed(b *testing.B) {
	encoded, gotEncodedErr := garble.NewSeed(
		[garble.SeedBytes]byte{1, 2, 3, 4, 5, 6, 7, 8},
	).Encoded()
	if gotEncodedErr != nil {
		b.Fatalf("Seed.Encoded() error = %v, want nil", gotEncodedErr)
	}
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, gotErr := garble.ParseSeed(encoded); gotErr != nil {
			b.Fatalf("ParseSeed() error = %v, want nil", gotErr)
		}
	}
}
