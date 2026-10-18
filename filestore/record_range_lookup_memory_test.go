package filestore_test

import (
	"bufio"
	"context"
	"math/bits"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/lineio"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestRecordRangeLookupUsesLogarithmicNativeReadsAndConstantHeap(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	const allowance = 512 << 10
	for _, count := range [...]int{257, 8193, 1 << 20} {
		file, err := os.Create(filepath.Join(t.TempDir(), "large-index"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		})
		writer := bufio.NewWriterSize(file, 32<<10)
		extent, err := core.NewByteLength(1)
		if err != nil {
			t.Fatal(err)
		}
		for i := range count {
			if err := filestore.WriteRecordRangeIndex(writer, lineio.RecordRange{Offset: lineio.SourceByteOffset(i * 2), Bytes: extent, Framing: lineio.RecordFramingOpaque}); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Flush(); err != nil {
			t.Fatal(err)
		}
		source := lookupNativeSource{File: file}
		before, err := hostfacts.ObserveCollectedGoHeap(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		var retained int64
		for _, target := range [...]int64{-1, 0, int64(count), int64(count*2 - 2), int64(count * 2)} {
			source.Reads = 0
			got, err := filestore.LowerBoundRecordRangeIndex(t.Context(), filestore.RecordRangeIndexSearchRequest{Source: &source, Compare: func(ctx context.Context, record lineio.RecordRange) (core.Comparison, error) {
				after, err := hostfacts.ObserveCollectedGoHeap(ctx)
				if err != nil {
					return core.ComparisonUnknown, err
				}
				retained = max(retained, int64(after.Uint64())-int64(before.Uint64()))
				return lookupComparison(int64(record.Offset), target), nil
			}})
			want := int64(0)
			if target > 0 {
				want = (target + 1) / 2
			}
			present := want < int64(count)
			if err != nil || got.Present != present || present && int64(got.Position) != want || source.Reads > bits.Len(uint(count))+2 || retained > allowance {
				t.Fatalf("count=%d target=%d result=%+v/%v reads=%d retained=%d want position=%d present=%v", count, target, got, err, source.Reads, retained, want, present)
			}
		}
		t.Logf("index_records=%d index_bytes=%d retained_heap_bytes=%d read_limit=%d", count, count*filestore.RecordRangeIndexBytes, retained, bits.Len(uint(count))+2)
	}
}
