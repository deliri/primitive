package process_test

import (
	"bytes"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// FuzzTailBufferExternalIngress compares each independently partitioned stream
// with a finite full-source oracle. Native retention is bounded; the oracle is
// deliberately test-only. Bytes include invalid UTF-8 and embedded zeroes.
func FuzzTailBufferExternalIngress(f *testing.F) {
	f.Add([]byte{}, uint16(1), uint16(1))
	f.Add([]byte("prefix\x00tail\xff"), uint16(4), uint16(3))
	f.Add([]byte("one long chunk then a wrapping suffix"), uint16(7), uint16(19))
	f.Add([]byte("exactly8"), uint16(8), uint16(8))
	f.Fuzz(func(t *testing.T, source []byte, windowSeed, chunkSeed uint16) {
		// These finite test generators bound oracle work, never accepted input.
		window := 1 + int(windowSeed)%257
		chunk := 1 + int(chunkSeed)%513
		count, err := core.NewByteCount(uint64(window))
		if err != nil {
			t.Fatal(err)
		}
		buffer, err := process.NewTailBuffer(count)
		if err != nil {
			t.Fatal(err)
		}
		checkPrefix := func(consumed int) {
			t.Helper()
			got, err := buffer.Snapshot()
			want := source[max(0, consumed-window):consumed]
			if err != nil || !bytes.Equal(got.Data, want) || got.Received.Uint64() != uint64(consumed) || got.Dropped.Uint64() != uint64(consumed-len(want)) {
				t.Fatalf("consumed=%d,window=%d,chunk=%d,snapshot=%+v,%v,want suffix=%x", consumed, window, chunk, got, err, want)
			}
			if err := got.Validate(); err != nil {
				t.Fatal(err)
			}
		}
		checkPrefix(0)
		for start := 0; start < len(source); start += chunk {
			end := min(len(source), start+chunk)
			if n, err := buffer.Write(source[start:end]); n != end-start || err != nil {
				t.Fatalf("Write=%d,%v", n, err)
			}
			checkPrefix(end)
		}
		if err := buffer.Reset(); err != nil {
			t.Fatal(err)
		}
		checkPrefix(0)
		if n, err := buffer.Write(source); n != len(source) || err != nil {
			t.Fatalf("single chunk=%d,%v", n, err)
		}
		checkPrefix(len(source))
	})
}
