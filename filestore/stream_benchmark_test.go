package filestore

import (
	"bytes"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// The workload includes bounded copy, exact receipt validation and an exact
// byte comparison. Reader/destination storage is reused outside the timed loop;
// the benchmark exposes production scratch allocation rather than fixture growth.
func BenchmarkBoundedCopyExactBytes(b *testing.B) {
	b.ReportAllocs()
	cases := []struct {
		name string
		size int
	}{
		{name: "128B", size: 128},
		{name: "32KiB", size: 32 << 10},
		{name: "1MiB", size: 1 << 20},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			payload := make([]byte, tc.size)
			for i := range payload {
				payload[i] = byte(i * 131)
			}
			maximum, err := core.NewByteCount(uint64(len(payload)))
			if err != nil {
				b.Fatal(err)
			}
			var source bytes.Reader
			var destination bytes.Buffer
			destination.Grow(len(payload))
			request := boundedCopyRequest{ctx: b.Context(), source: &source, destination: &destination, maximum: maximum, kind: streamDestinationCaller}
			b.ReportAllocs()
			b.SetBytes(int64(len(payload)))
			for b.Loop() {
				source.Reset(payload)
				destination.Reset()
				got, err := copyBounded(request)
				if err != nil || got.Uint64() != uint64(len(payload)) || !bytes.Equal(destination.Bytes(), payload) {
					b.Fatalf("bounded copy = (%d,%v,%d destination bytes), want %d exact bytes", got.Uint64(), err, destination.Len(), len(payload))
				}
			}
		})
	}
}
