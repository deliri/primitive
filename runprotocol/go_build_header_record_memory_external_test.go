package runprotocol_test

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runprotocol"
	"github.com/deliri/primitive/v2026/testserial"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGoBuildHeaderRetainedMemoryIndependentOfExtent(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, extent := range []int64{4095, 4096, 4097, 64*1024*1024 + 1} {
		dir := t.TempDir()
		file, err := os.Create(filepath.Join(dir, "native"))
		if err != nil {
			t.Fatal(err)
		}
		var window [32768]byte
		for i := range window {
			window[i] = ' '
		}
		for remaining := extent; remaining > 0; {
			size := min(remaining, int64(len(window)))
			if _, err := file.Write(window[:size]); err != nil {
				file.Close()
				t.Fatal(err)
			}
			remaining -= size
		}
		if _, err := file.WriteString("# example.invalid/pkg\n"); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			file.Close()
			t.Fatal(err)
		}
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		count := 0
		var got runprotocol.GoFuzzSignalRecord
		for observed, err := range runprotocol.GoFuzzSignalRecords(t.Context(), runprotocol.GoFuzzSignalSourceRequest{Source: file}) {
			if err != nil {
				file.Close()
				t.Fatal(err)
			}
			count++
			got = observed
		}
		if count != 1 || int64(got.BuildHeader.Offset) != extent || got.BuildHeader.Bytes.Uint64() != 2 {
			file.Close()
			t.Fatalf("native marker at extent%d=%+v,count%d,want exact completed coordinate", extent, got, count)
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		runtime.KeepAlive(got)
		if after.HeapAlloc > before.HeapAlloc+4*1024*1024 {
			file.Close()
			t.Fatalf("extent%d retained%d,want fixed work below4MiB", extent, after.HeapAlloc-before.HeapAlloc)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
