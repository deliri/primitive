package lineio_test

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestRecordFieldsRetainedMemoryDoesNotFollowTokenExtent(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, size := range []int64{64*1024 + 1, 64*1024*1024 + 1} {
		dir := t.TempDir()
		file, err := os.Create(filepath.Join(dir, "native-token"))
		if err != nil {
			t.Fatal(err)
		}
		var chunk [32768]byte
		for i := range chunk {
			chunk[i] = 'x'
		}
		for left := size; left > 0; {
			n := min(left, int64(len(chunk)))
			if _, err := file.Write(chunk[:n]); err != nil {
				file.Close()
				t.Fatal(err)
			}
			left -= n
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			file.Close()
			t.Fatal(err)
		}
		length, err := core.NewByteLength(uint64(size))
		if err != nil {
			file.Close()
			t.Fatal(err)
		}
		window, err := core.NewByteCount(4096)
		if err != nil {
			file.Close()
			t.Fatal(err)
		}
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		count := 0
		var observed lineio.RecordRange
		for got, err := range lineio.RecordFields(t.Context(), lineio.RecordFragmentRequest{Source: file, Record: lineio.RecordRange{Bytes: length, Framing: lineio.RecordFramingOpaque}, BufferBytes: window}) {
			if err != nil {
				file.Close()
				t.Fatal(err)
			}
			count++
			observed = got
		}
		if count != 1 || observed.Offset != 0 || observed.Bytes != length {
			file.Close()
			t.Fatalf("native fields=%d/%+v,wantoneexact%d-byte field", count, observed, size)
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		runtime.KeepAlive(observed)
		if after.HeapAlloc > before.HeapAlloc+4*1024*1024 {
			file.Close()
			t.Fatalf("source%d retained growth%d,wantfixedworkbelow4MiB", size, after.HeapAlloc-before.HeapAlloc)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
