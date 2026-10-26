package filestore_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/filestore"
)

// cacheTrimBenchmarkFiles shapes a Go build cache: 256 two-hex-digit
// directories holding 1 KiB entries with distinct mtimes.
const (
	cacheTrimBenchmarkFiles = 20_000
	cacheTrimBenchmarkBytes = 1 << 10
)

func cacheTrimBenchmarkFixture(b *testing.B, directory string) {
	b.Helper()
	body := make([]byte, cacheTrimBenchmarkBytes)
	for i := range cacheTrimBenchmarkFiles {
		name := filepath.Join(directory, "cache", fmt.Sprintf("%02x", i%256), fmt.Sprintf("%016x-a", i))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(name, body, 0o600); err != nil {
			b.Fatal(err)
		}
		stamp := cacheTrimBase.Add(-time.Duration(cacheTrimBenchmarkFiles-i) * time.Second)
		if err := os.Chtimes(name, stamp, stamp); err != nil {
			b.Fatal(err)
		}
	}
}

func cacheTrimBenchmarkRoot(b *testing.B, directory string) *os.Root {
	b.Helper()
	root, err := os.OpenRoot(directory)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = root.Close() })
	return root
}

// The common case: one walk finds the cache within its share, no sort.
func BenchmarkTrimCacheWithinLimit(b *testing.B) {
	directory := b.TempDir()
	cacheTrimBenchmarkFixture(b, directory)
	root := cacheTrimBenchmarkRoot(b, directory)
	request := cacheTrimRequest(b, root, cacheTrimBenchmarkFiles*cacheTrimBenchmarkBytes)
	b.ReportAllocs()
	for b.Loop() {
		got, err := filestore.TrimCache(b.Context(), request)
		if err != nil || got.RemovedFiles != 0 {
			b.Fatalf("TrimCache = %+v/%v, want nothing removed", got, err)
		}
	}
	b.ReportMetric(cacheTrimBenchmarkFiles, "files/op")
}

// Over the share: walk, external sort and evict the older half.
func BenchmarkTrimCacheEvictHalf(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		directory := b.TempDir()
		cacheTrimBenchmarkFixture(b, directory)
		root := cacheTrimBenchmarkRoot(b, directory)
		request := cacheTrimRequest(b, root, cacheTrimBenchmarkFiles*cacheTrimBenchmarkBytes/2)
		b.StartTimer()
		got, err := filestore.TrimCache(b.Context(), request)
		if err != nil || got.RemovedFiles != cacheTrimBenchmarkFiles/2 {
			b.Fatalf("TrimCache = %+v/%v, want half removed", got, err)
		}
	}
	b.ReportMetric(cacheTrimBenchmarkFiles, "files/op")
}
