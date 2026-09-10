package lineio_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

func BenchmarkScanStreaming64Lines(b *testing.B) {
	b.ReportAllocs()
	benchmarkFragments(b, bytes.Repeat([]byte("alpha\n"), 64), 64)
}
func BenchmarkScanStreaming4096Lines(b *testing.B) {
	b.ReportAllocs()
	benchmarkFragments(b, bytes.Repeat([]byte("alpha\n"), 4096), 64)
}

func benchmarkFragments(b *testing.B, payload []byte, buffer uint64) {
	b.Helper()
	capacity := mustByteCount(b, buffer)
	wantLines := bytes.Count(payload, []byte{lineio.Delimiter})
	if len(payload) == 0 {
		b.Fatalf("payload length = %d, want nonempty stream", len(payload))
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	for b.Loop() {
		reader, err := lineio.New(lineio.Request{Source: bytes.NewReader(payload), BufferBytes: capacity})
		if err != nil {
			b.Fatalf("New() = %v, want nil", err)
		}
		gotBytes, gotLines := 0, 0
		for {
			fragment, err := reader.ReadFragment()
			gotBytes += len(fragment.Bytes)
			if len(fragment.Bytes) > 0 && fragment.Bytes[len(fragment.Bytes)-1] == lineio.Delimiter {
				gotLines++
			}
			if err != nil {
				if !errors.Is(err, io.EOF) || errors.Is(err, core.ErrLineIOScan) {
					b.Fatalf("terminal = %v, want EOF", err)
				}
				break
			}
		}
		if gotBytes != len(payload) || gotLines != wantLines {
			b.Fatalf("stream = (%d bytes,%d LF), want (%d,%d)", gotBytes, gotLines, len(payload), wantLines)
		}
	}
}
