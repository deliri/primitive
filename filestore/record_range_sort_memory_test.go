package filestore_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/lineio"
	"github.com/deliri/primitive/v2026/testserial"
)

type rangeSortPaddingReader struct{ remaining int64 }

func (p *rangeSortPaddingReader) Read(buffer []byte) (int, error) {
	if p.remaining == 0 {
		return 0, io.EOF
	}
	n := int(min(int64(len(buffer)), p.remaining))
	for i := range n {
		buffer[i] = 'x'
	}
	p.remaining -= int64(n)
	return n, nil
}
func writeRangeSortMemoryFixture(t testing.TB, source *os.File, index *os.File, count int, padding int64) {
	t.Helper()
	writer := bufio.NewWriterSize(source, 32<<10)
	for i := range count {
		if _, err := fmt.Fprintf(writer, "%08d", count-i); err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(writer, &rangeSortPaddingReader{remaining: padding}); err != nil {
			t.Fatal(err)
		}
		if err := writer.WriteByte('\n'); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	buffer, err := core.NewByteCount(4096)
	if err != nil {
		t.Fatal(err)
	}
	for record, err := range lineio.RecordRanges(context.Background(), lineio.RecordRangeRequest{Source: source, BufferBytes: buffer}) {
		if err != nil {
			t.Fatal(err)
		}
		if err := filestore.WriteRecordRangeIndex(index, record); err != nil {
			t.Fatal(err)
		}
	}
}
func TestRecordRangeSortRetainsConstantMemoryAcrossCardinalityAndExtent(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	const allowance = 512 << 10
	for _, shape := range []struct {
		count   int
		padding int64
	}{{256, 0}, {8192, 0}, {65536, 0}, {2, 4096}, {2, 6 << 20}, {2, 12 << 20}} {
		source, files := recordRangeSortFiles(t)
		writeRangeSortMemoryFixture(t, source, files[0], shape.count, shape.padding)
		before, err := hostfacts.ObserveCollectedGoHeap(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		var retained int64
		seen := 0
		observation, err := filestore.SortRecordRanges(t.Context(), filestore.RecordRangeSortRequest{Source: source, Files: files, Compare: recordRangeSortPrefixComparison, Visit: func(record lineio.RecordRange) error {
			seen++
			if int64(record.Offset) != int64(shape.count-seen)*(shape.padding+9) {
				return core.ErrFilestoreContract
			}
			if seen == shape.count {
				after, err := hostfacts.ObserveCollectedGoHeap(t.Context())
				if err != nil {
					return err
				}
				retained = int64(after.Uint64()) - int64(before.Uint64())
			}
			return nil
		}})
		t.Logf("records=%d record_padding=%d retained_bytes=%d allowance=%d", shape.count, shape.padding, retained, allowance)
		if err != nil || observation.Records != uint64(shape.count) || seen != shape.count || retained > allowance {
			t.Fatalf("observation%+v/%v effects%d retained%d want%d <=%d", observation, err, seen, retained, shape.count, allowance)
		}
	}
}
