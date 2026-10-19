package filestore_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// zipBenchmarkCounter observes the archive extent so the encoder's output is
// checked after the loop instead of being discarded unobserved.
type zipBenchmarkCounter struct{ bytes int64 }

func (c *zipBenchmarkCounter) Write(data []byte) (int, error) {
	c.bytes += int64(len(data))
	return len(data), nil
}

// BenchmarkZIPStoreEntryCardinality measures per-entry framing and copy cost
// for a fixed 128-byte stored payload. Go's ZIP writer retains one central
// directory record per entry, so allocation grows with cardinality by design;
// the per-entry copy window must not.
func BenchmarkZIPStoreEntryCardinality(b *testing.B) {
	for _, count := range []int{1, 1024} {
		b.Run(fmt.Sprintf("entries_%d", count), func(b *testing.B) {
			payload := bytes.Repeat([]byte{0, 255, 31, 8}, 32)
			if len(payload) != 128 {
				b.Fatalf("payload bytes = %d, want 128", len(payload))
			}
			path, err := core.ParseRelativePath("docs/entry.bin")
			if err != nil {
				b.Fatal(err)
			}
			modified := zipFixtureInstant(b)
			source := bytes.NewReader(payload)
			var counter zipBenchmarkCounter
			request := filestore.ZIPWriteRequest{Destination: &counter, Produce: func(_ context.Context, add func(filestore.ZIPEntry) error) error {
				for range count {
					source.Reset(payload)
					if err := add(filestore.ZIPEntry{Source: source, Path: path, Modified: modified, Mode: 0o600, Compression: filestore.ZIPStore}); err != nil {
						return err
					}
					if source.Len() != 0 {
						return fmt.Errorf("ZIP entry left %d unread bytes", source.Len())
					}
				}
				return nil
			}}
			b.ReportAllocs()
			b.SetBytes(int64(count * len(payload)))
			for b.Loop() {
				counter.bytes = 0
				if err := filestore.WriteZIP(b.Context(), request); err != nil {
					b.Fatalf("WriteZIP() error = %v, want nil", err)
				}
			}
			if counter.bytes < int64(count*len(payload)) {
				b.Fatalf("archive bytes = %d, want at least the %d stored payload bytes", counter.bytes, count*len(payload))
			}
		})
	}
}
