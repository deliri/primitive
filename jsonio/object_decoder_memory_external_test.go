package jsonio_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestJSONObjectDecoderNativeResetRetainsConstantMemory(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	data := []byte(`{"Name":"one","Count":1}`)
	path := filepath.Join(t.TempDir(), "object")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	const allowance = 256 << 10
	for _, count := range []int{4096, 65536, 262144} {
		decoder, err := jsonio.NewObjectDecoder[scalarObject](jsonio.ObjectSourceRequest{Source: io.NewSectionReader(file, 0, int64(len(data)))})
		if err != nil {
			t.Fatal(err)
		}
		before, err := hostfacts.ObserveCollectedGoHeap(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < count; i++ {
			if err := decoder.Reset(t.Context(), jsonio.ObjectSourceRequest{Source: io.NewSectionReader(file, 0, int64(len(data)))}); err != nil {
				t.Fatal(err)
			}
			object, err := decoder.Decode(t.Context())
			if err != nil || object != (scalarObject{Name: "one", Count: 1}) {
				t.Fatalf("row%d %+v/%v", i, object, err)
			}
			if i == count-1 {
				after, err := hostfacts.ObserveCollectedGoHeap(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				retained := int64(after.Uint64()) - int64(before.Uint64())
				t.Logf("native_resets=%d retained_bytes=%d allowance=%d", count, retained, allowance)
				if retained > allowance {
					t.Fatalf("retained%d exceeds%d", retained, allowance)
				}
			}
			if _, err := decoder.Decode(t.Context()); err != io.EOF {
				t.Fatalf("row%d native closure%v", i, err)
			}
		}
	}
}
