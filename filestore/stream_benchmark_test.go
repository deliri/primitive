package filestore

import (
	"bytes"
	"testing"
)

// The workload includes stream copy, exact receipt validation and an exact
// byte comparison. Reader/destination storage is reused outside the timed loop;
// the benchmark exposes production scratch allocation rather than fixture growth.
func BenchmarkStreamCopyExactBytes(b *testing.B) {
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
		for _, window := range []struct {
			name  string
			bytes int
		}{
			{name: "GoAllocated"}, {name: "CallerReused", bytes: min(tc.size, 32<<10)},
		} {
			b.Run(tc.name+"/"+window.name, func(b *testing.B) {
				payload := make([]byte, tc.size)
				for i := range payload {
					payload[i] = byte(i * 131)
				}
				var source bytes.Reader
				var destination bytes.Buffer
				destination.Grow(len(payload))
				request := streamCopyRequest{ctx: b.Context(), source: &source, destination: &destination, kind: streamDestinationCaller, buffer: make([]byte, window.bytes)}
				b.ReportAllocs()
				b.SetBytes(int64(len(payload)))
				for b.Loop() {
					source.Reset(payload)
					destination.Reset()
					got, err := copyStream(request)
					if err != nil || got.Uint64() != uint64(len(payload)) || !bytes.Equal(destination.Bytes(), payload) {
						b.Fatalf("stream copy = (%d,%v,%d destination bytes), want %d exact bytes", got.Uint64(), err, destination.Len(), len(payload))
					}
				}
			})
		}
	}
}
