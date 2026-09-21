package filestore_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
)

func BenchmarkCopyContentExactAgreement(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name string
		size int
	}{
		{name: "within_scratch_window", size: 8 << 10},
		{name: "across_32_scratch_windows", size: 1 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			data := bytes.Repeat([]byte{0x8e}, tc.size)
			want := contentCopyFixture(b, data)
			var scratch [32 << 10]byte
			reader := bytes.NewReader(data)
			request := filestore.CopyContentRequest{Source: reader, Destination: io.Discard, Expected: &want, Buffer: scratch[:]}
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				reader.Reset(data)
				got, err := filestore.CopyContent(b.Context(), request)
				if err != nil || got != want || reader.Len() != 0 {
					b.Fatalf("copy=(%v,%v), remaining=%d, want %v/nil/0", got, err, reader.Len(), want)
				}
			}
		})
	}
}
