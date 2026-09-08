package core_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func BenchmarkParsePathComponent(b *testing.B) {
	const value = "entry"
	var wantErr error
	b.ReportAllocs()
	var last core.PathComponent
	for b.Loop() {
		component, err := core.ParsePathComponent(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("core.ParsePathComponent() error = %v, want %v", err, wantErr)
		}
		last = component
	}
	if last.String() != value {
		b.Fatalf("core.ParsePathComponent() = %q, want %q", last, value)
	}
}

func BenchmarkParseRelativePath(b *testing.B) {
	value := filepath.Join("stable", "child")
	var wantErr error
	b.ReportAllocs()
	var last core.RelativePath
	for b.Loop() {
		path, err := core.ParseRelativePath(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("core.ParseRelativePath() error = %v, want %v", err, wantErr)
		}
		last = path
	}
	if last.String() != value {
		b.Fatalf("core.ParseRelativePath() = %q, want %q", last, value)
	}
}

func BenchmarkDecodeCanonicalHexSHA256(b *testing.B) {
	const value = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	destination := make([]byte, core.SHA256DigestBytes)
	want, err := hex.DecodeString(value)
	if err != nil {
		b.Fatal(err)
	}
	var wantErr error
	b.ReportAllocs()
	for b.Loop() {
		err := core.DecodeCanonicalHex(destination, value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("core.DecodeCanonicalHex() error = %v, want %v", err, wantErr)
		}
	}
	if !bytes.Equal(destination, want) {
		b.Fatalf("decoded=%x; want %x", destination, want)
	}
}

func BenchmarkDigestWriter(b *testing.B) {
	b.ReportAllocs()
	cases := []struct {
		name string
		size int
	}{
		{name: "1KiB", size: 1 << 10},
		{name: "1MiB", size: 1 << 20},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			payload := bytes.Repeat([]byte{0xa5}, tc.size)
			want := core.NewSHA256Digest(sha256.Sum256(payload))
			b.ReportAllocs()
			b.SetBytes(int64(tc.size))
			var got core.SHA256Digest
			var count core.ByteLength
			for b.Loop() {
				writer := core.NewDigestWriter()
				n, err := writer.Write(payload)
				if err != nil || n != tc.size {
					b.Fatalf("write=%d, %v; want %d, nil", n, err, tc.size)
				}
				got, count, err = writer.Seal()
				if err != nil {
					b.Fatal(err)
				}
			}
			if got != want || count.Uint64() != uint64(tc.size) {
				b.Fatalf("sealed=%v, %v; want %v, %d", got, count, want, tc.size)
			}
		})
	}
}
