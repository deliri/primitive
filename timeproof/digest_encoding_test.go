package timeproof

import (
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestDigestSetEncodingLayerTriad(t *testing.T) {
	t.Parallel()
	oid, err := asn1.Marshal(oidSHA256())
	if err != nil {
		t.Fatalf("asn1.Marshal(SHA256) error = %v, want nil", err)
	}
	sequence := func(body []byte) []byte { return derTagged(byte(asn1.TagSequence)|derConstructed, body) }
	set := func(body []byte) []byte { return derTagged(byte(asn1.TagSet)|derConstructed, body) }
	cases := []struct {
		name    string
		in      []byte
		wantErr error
	}{
		{name: "algorithm without optional parameters", in: set(sequence(oid))},
		{name: "algorithm with NULL parameters", in: set(sequence(append(bytes.Clone(oid), asn1.NullBytes...)))},
		{name: "empty declaration set is structurally present", in: set(nil)},
		{name: "missing outer set", wantErr: core.ErrTimeProofInvalid},
		{name: "sequence cannot replace declaration set", in: sequence(sequence(oid)), wantErr: core.ErrTimeProofInvalid},
		{name: "bare identifier cannot replace algorithm sequence", in: set(oid), wantErr: core.ErrTimeProofInvalid},
		{name: "algorithm sequence cannot omit identifier", in: set(sequence(nil)), wantErr: core.ErrTimeProofInvalid},
		{name: "NULL cannot replace identifier", in: set(sequence(asn1.NullBytes)), wantErr: core.ErrTimeProofInvalid},
		{name: "identifier body cannot be empty", in: set(sequence([]byte{6, 0})), wantErr: core.ErrTimeProofInvalid},
		{name: "identifier arc cannot end mid encoding", in: set(sequence([]byte{6, 1, 0x81})), wantErr: core.ErrTimeProofInvalid},
		{name: "identifier arc cannot have a redundant leading group", in: set(sequence([]byte{6, 2, 0x80, 1})), wantErr: core.ErrTimeProofInvalid},
		{name: "parameter cannot be truncated", in: set(sequence(append(bytes.Clone(oid), 5))), wantErr: core.ErrTimeProofInvalid},
		{name: "second parameter cannot disappear", in: set(sequence(append(append(bytes.Clone(oid), asn1.NullBytes...), asn1.NullBytes...))), wantErr: core.ErrTimeProofInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, rest, err := consumeAlgorithmSet(tc.in)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("consumeAlgorithmSet() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if len(got.FullBytes) != 0 || len(got.Bytes) != 0 || len(rest) != 0 {
					t.Fatalf("rejected declaration bytes/rest = %x/%x, want no partial result", got.FullBytes, rest)
				}
				return
			}
			if len(rest) != 0 || !bytes.Equal(got.FullBytes, tc.in) || &got.FullBytes[0] != &tc.in[0] {
				t.Fatalf("declaration span/rest = %x/%x, want exact borrowed input and no rest", got.FullBytes, rest)
			}
		})
	}
}

// This direct parser ratchet compares representation admission against the
// standard library's arbitrary-width OID decoder. Provider fuzz targets cover
// the complete signed response and independent authentic agreement.
func FuzzDigestOIDRepresentation(f *testing.F) {
	fixture := loadAuthenticFixture(f)
	encoded, err := asn1.Marshal(oidSHA256())
	if err != nil {
		f.Fatalf("asn1.Marshal(seed) error = %v, want nil", err)
	}
	seed := rawValueFromDER(f, encoded)
	var seedOID x509.OID
	if err := seedOID.UnmarshalBinary(seed.Bytes); err != nil {
		f.Fatalf("x509.OID seed error = %v, want nil", err)
	}
	f.Add(seed.Bytes)
	f.Add([]byte{})
	f.Add([]byte{0x80, 0})
	f.Add([]byte{0x81})
	f.Fuzz(func(t *testing.T, data []byte) {
		// The oracle is bounded by the public response admission contract.
		// Larger representations belong to the future streaming public API.
		if len(data) > ResponseMaximumBytes {
			got, err := Verify(VerifyRequest{Response: data, Request: fixture.request, ExpectedDigest: fixture.digest})
			if !errors.Is(err, core.ErrTimeProofContract) || !timestampHasNoProof(got) {
				t.Fatalf("Verify(oversized representation) = (%+v, %v), want zero proof and typed contract rejection", got, err)
			}
			return
		}
		oidDER := derTagged(byte(asn1.TagOID), data)
		algorithm := derTagged(byte(asn1.TagSequence)|derConstructed, oidDER)
		input := derTagged(byte(asn1.TagSet)|derConstructed, algorithm)
		got, rest, err := consumeAlgorithmSet(input)
		var oracle x509.OID
		wantErr := oracle.UnmarshalBinary(data)
		if (err == nil) != (wantErr == nil) {
			t.Fatalf("OID admission error = %v, want agreement with x509.OID error %v", err, wantErr)
		}
		if err != nil {
			if !errors.Is(err, core.ErrTimeProofInvalid) || len(got.FullBytes) != 0 || len(rest) != 0 {
				t.Fatalf("OID rejection = (%x, %x, %v), want typed refusal and zero spans", got.FullBytes, rest, err)
			}
			return
		}
		if gotMatch, wantMatch := digestAlgorithmDeclared(got, oidSHA256()), oracle.Equal(seedOID); gotMatch != wantMatch {
			t.Fatalf("digest declaration match = %t, want %t from independent OID identity", gotMatch, wantMatch)
		}
		canonical, err := oracle.MarshalBinary()
		if err != nil || !bytes.Equal(canonical, data) || !bytes.Equal(got.FullBytes, input) || len(rest) != 0 {
			t.Fatalf("OID canonical bytes/error = (%x, %v), want %x and nil with exact source span", canonical, err, data)
		}
	})
}

func BenchmarkDigestDeclarationScan(b *testing.B) {
	b.ReportAllocs()
	for _, count := range []int{4, 4096} {
		b.Run("declarations_"+strconv.Itoa(count), func(b *testing.B) {
			input := digestSetFixture(b, count, 1)
			got, rest, err := consumeAlgorithmSet(input)
			if err != nil || len(rest) != 0 || len(got.Bytes) == 0 {
				b.Fatalf("digest workload = (%d, %d, %v), want nonempty complete set", len(got.Bytes), len(rest), err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			for b.Loop() {
				got, rest, err := consumeAlgorithmSet(input)
				if err != nil || len(rest) != 0 || len(got.FullBytes) != len(input) {
					b.Fatalf("consumeAlgorithmSet() = (%d, %d, %v), want (%d, 0, nil)", len(got.FullBytes), len(rest), err, len(input))
				}
			}
		})
	}
}
