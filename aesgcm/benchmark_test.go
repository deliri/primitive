package aesgcm

import (
	"bytes"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const benchmarkPlaintextBytes = 4 << 10

var benchmarkSealSink SealResult

func BenchmarkSeal4KiB(b *testing.B) {
	c := fixtureCipher(b, bytes.Repeat([]byte{0x5a}, core.AES256KeyBytes))
	plaintext := bytes.Repeat([]byte{0xa5}, benchmarkPlaintextBytes)
	additional := []byte("benchmark-additional-data")
	b.SetBytes(benchmarkPlaintextBytes)
	b.ReportAllocs()
	for b.Loop() {
		sealed, err := c.Seal(SealRequest{Plaintext: plaintext, AdditionalData: additional})
		if err != nil {
			b.Fatal(err)
		}
		benchmarkSealSink = sealed
	}
	opened, err := c.Open(OpenRequest{Ciphertext: benchmarkSealSink.Ciphertext, AdditionalData: additional})
	if err != nil || !bytes.Equal(opened.Plaintext, plaintext) {
		b.Fatalf("Open(last sealed) = %d bytes, %v; want the exact plaintext", len(opened.Plaintext), err)
	}
}

func BenchmarkOpen4KiB(b *testing.B) {
	c := fixtureCipher(b, bytes.Repeat([]byte{0x5a}, core.AES256KeyBytes))
	plaintext := bytes.Repeat([]byte{0xa5}, benchmarkPlaintextBytes)
	additional := []byte("benchmark-additional-data")
	sealed, err := c.Seal(SealRequest{Plaintext: plaintext, AdditionalData: additional})
	if err != nil || len(sealed.Ciphertext) != benchmarkPlaintextBytes+core.AESGCMOverheadBytes {
		b.Fatalf("Seal() = %d bytes, %v; want plaintext plus overhead", len(sealed.Ciphertext), err)
	}
	b.SetBytes(benchmarkPlaintextBytes)
	b.ReportAllocs()
	var got OpenResult
	for b.Loop() {
		got, err = c.Open(OpenRequest{Ciphertext: sealed.Ciphertext, AdditionalData: additional})
		if err != nil {
			b.Fatal(err)
		}
	}
	if !bytes.Equal(got.Plaintext, plaintext) {
		b.Fatalf("Open() = %d bytes, want the exact %d-byte plaintext", len(got.Plaintext), benchmarkPlaintextBytes)
	}
}
