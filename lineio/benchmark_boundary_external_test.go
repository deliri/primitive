package lineio_test

import (
	"bytes"
	"testing"

	"github.com/deliri/primitive/v2026/lineio"
)

func BenchmarkFragmentStreaming(b *testing.B) {
	b.ReportAllocs()
	cases := []struct {
		name   string
		extent int
		buffer uint64
		suffix string
	}{
		{name: "Line64KiBBuffer64", extent: 64 << 10, buffer: 64},
		{name: "Line64KiBBuffer64KiB", extent: 64 << 10, buffer: 64 << 10},
		{name: "Line1MiBBuffer64KiB", extent: 1 << 20, buffer: 64 << 10},
		{name: "CRLFSplit64KiB", extent: (64 << 10) - 1, buffer: 64 << 10, suffix: "\r\n"},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			payload := append(bytes.Repeat([]byte{'x'}, tc.extent), tc.suffix...)
			benchmarkFragments(b, payload, tc.buffer)
		})
	}
}
func BenchmarkReaderConstruction(b *testing.B) {
	request := lineio.Request{Source: bytes.NewReader(nil), BufferBytes: mustByteCount(b, 64)}
	if err := request.Validate(); err != nil {
		b.Fatalf("request = %v, want nil", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		reader, err := lineio.New(request)
		if err != nil || reader == nil {
			b.Fatalf("New() = (%v,%v), want reader and nil", reader, err)
		}
		if err := reader.Validate(); err != nil {
			b.Fatalf("reader = %v, want nil", err)
		}
	}
}
