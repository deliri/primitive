package lineio_test

import (
	"bytes"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

func BenchmarkScanStreaming64Lines(b *testing.B) {
	b.ReportAllocs()
	benchmarkScanStreaming(b, 64, 64)
}

func BenchmarkScanStreaming4096Lines(b *testing.B) {
	b.ReportAllocs()
	benchmarkScanStreaming(b, 4096, 64)
}

func benchmarkScanStreaming(b *testing.B, lines, initial int) {
	b.Helper()

	payload := bytes.Repeat([]byte("alpha\n"), lines)
	initialBytes, err := core.NewByteCount(uint64(initial))
	if err != nil {
		b.Fatalf("core.NewByteCount(%d) error = %v, want nil", initial, err)
	}
	maximumBytes, err := core.NewByteCount(lineio.MaximumLineBytes)
	if err != nil {
		b.Fatalf("core.NewByteCount(MaximumLineBytes) error = %v, want nil", err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	var got int
	for b.Loop() {
		scanner, err := lineio.New(lineio.Request{
			Source: bytes.NewReader(payload),
			Buffer: lineio.BufferPolicy{InitialBytes: initialBytes, MaximumLineBytes: maximumBytes},
		})
		if err != nil {
			b.Fatalf("lineio.New() error = %v, want nil", err)
		}
		got = 0
		for scanner.Scan() {
			if len(scanner.Bytes()) == 0 {
				b.Fatalf("Scanner.Bytes()=%q, want the fixture line", scanner.Bytes())
			}
			got++
		}
		if err := scanner.Err(); err != nil {
			b.Fatalf("Scanner.Err() = %v, want nil", err)
		}
		if got != lines {
			b.Fatalf("Scanner lines = %d, want %d", got, lines)
		}
	}
}
