package core_test

import (
	"errors"
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
	const value = "stable/child"
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
	var wantErr error
	b.ReportAllocs()
	for b.Loop() {
		err := core.DecodeCanonicalHex(destination, value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("core.DecodeCanonicalHex() error = %v, want %v", err, wantErr)
		}
	}
}

func BenchmarkDigestWriter1KiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkDigestWriter(b, 1<<10)
}

func BenchmarkDigestWriter1MiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkDigestWriter(b, 1<<20)
}

func benchmarkDigestWriter(b *testing.B, size int) {
	b.Helper()
	payload := make([]byte, size)
	for index := range payload {
		payload[index] = 0xa5
	}
	var wantErr error
	b.ReportAllocs()
	b.SetBytes(int64(size))
	var last core.SHA256Digest
	for b.Loop() {
		writer := core.NewDigestWriter()
		n, err := writer.Write(payload)
		if !errors.Is(err, wantErr) || n != size {
			b.Fatalf("DigestWriter.Write() = (%d, %v), want %d and %v", n, err, size, wantErr)
		}
		digest, _, err := writer.Seal()
		if !errors.Is(err, wantErr) {
			b.Fatalf("DigestWriter.Seal() error = %v, want %v", err, wantErr)
		}
		last = digest
	}
	if last == (core.SHA256Digest{}) {
		b.Fatalf("sealed digest=%v, want the hashed payload", last)
	}
}
