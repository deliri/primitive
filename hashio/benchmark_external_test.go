package hashio_test

import (
	"hash/fnv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/hashio"
)

var benchmarkFNVSink uint64

// Three framed parts of a fixed 1 KiB total, the shape callers fingerprint.
func BenchmarkFNV1a64TextParts(b *testing.B) {
	parts := []string{strings.Repeat("k", 256), "\x00", strings.Repeat("v", 767)}
	oracle := fnv.New64a()
	_, _ = oracle.Write([]byte(strings.Join(parts, "")))
	want := oracle.Sum64()
	b.ReportAllocs()
	b.SetBytes(1024)
	for b.Loop() {
		benchmarkFNVSink = hashio.FNV1a64TextParts(parts...)
	}
	if benchmarkFNVSink != want {
		b.Fatalf("FNV1a64TextParts() = %x, want native %x", benchmarkFNVSink, want)
	}
}

var benchmarkEqualSink bool

func BenchmarkEqualBytes(b *testing.B) {
	first := []byte(strings.Repeat("e", 64))
	second := []byte(strings.Repeat("e", 64))
	b.ReportAllocs()
	b.SetBytes(int64(len(first)))
	for b.Loop() {
		benchmarkEqualSink = hashio.EqualBytes(first, second)
	}
	if !benchmarkEqualSink {
		b.Fatal("EqualBytes(equal 64-byte inputs) = false, want true")
	}
}
