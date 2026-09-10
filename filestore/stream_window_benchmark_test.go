package filestore

import (
	"bytes"
	"io"
	"testing"
)

const benchmarkStreamPattern byte = 0xa5

type benchmarkGeneratedStream struct{}

func (benchmarkGeneratedStream) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = benchmarkStreamPattern
	}
	return len(p), nil
}

type benchmarkObservedStream struct {
	count   int64
	changed bool
}

func (w *benchmarkObservedStream) Write(p []byte) (int, error) {
	w.count += int64(len(p))
	w.changed = w.changed || bytes.Count(p, []byte{benchmarkStreamPattern}) != len(p)
	return len(p), nil
}

// The generator, byte-verifying sink and Go copy are all measured. The input
// is never materialized: only the same 32 KiB caller buffer is retained.
// These finite workloads prove window reuse, not execution of a terabyte run.
func BenchmarkStreamFixedWindow(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name   string
		extent int64
	}{
		{name: "1MiB", extent: 1 << 20},
		{name: "64MiB", extent: 64 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			source := io.LimitedReader{R: benchmarkGeneratedStream{}}
			var destination benchmarkObservedStream
			request := streamCopyRequest{ctx: b.Context(), source: &source, destination: &destination, kind: streamDestinationCaller, buffer: make([]byte, 32<<10)}
			b.ReportAllocs()
			b.SetBytes(tc.extent)
			for b.Loop() {
				source.N = tc.extent
				destination.count = 0
				destination.changed = false
				got, err := copyStream(request)
				if err != nil || got.Uint64() != uint64(tc.extent) || source.N != 0 || destination.count != tc.extent || destination.changed {
					b.Fatalf("stream = (%d,%v,%d remaining,%d observed,changed %t), want %d exact bytes", got.Uint64(), err, source.N, destination.count, destination.changed, tc.extent)
				}
			}
		})
	}
}
