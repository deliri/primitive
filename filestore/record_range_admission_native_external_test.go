package filestore_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

type observedRangeSource struct {
	*os.File
	bytesRead uint64
}

func (s *observedRangeSource) ReadAt(data []byte, offset int64) (int, error) {
	n, err := s.File.ReadAt(data, offset)
	if n > 0 {
		s.bytesRead += uint64(n)
	}
	return n, err
}

// Native files hold the source/index. The observer counts actual positional
// bytes and comparison consumes coordinates only: no second content model.
func TestRecordRangeAdmissionReadsImmutableSourceOnceAcrossWindows(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		records, extent int64
	}{
		{name: "empty index observes no source", extent: 1},
		{name: "one record", records: 1, extent: 1},
		{name: "record below read window", records: 2, extent: 4095},
		{name: "record at read window", records: 2, extent: 4096},
		{name: "record above read window", records: 2, extent: 4097},
		{name: "record many windows and final partial", records: 2, extent: 65537},
		{name: "run below tuple window", records: 4095, extent: 1},
		{name: "run at tuple window", records: 4096, extent: 1},
		{name: "run above tuple window", records: 4097, extent: 1},
		{name: "multiple merge runs final partial", records: 16385, extent: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			file, err := os.Create(filepath.Join(directory, "immutable-source"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := file.Close(); err != nil {
					t.Error(err)
				}
			}()
			if err := file.Truncate(tc.records * tc.extent); err != nil {
				t.Fatal(err)
			}
			source := &observedRangeSource{File: file}
			var scratch [2]*os.File
			for i := range scratch {
				scratch[i], err = os.Create(filepath.Join(directory, fmt.Sprintf("index-%d", i)))
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := scratch[i].Close(); err != nil {
						t.Error(err)
					}
				}()
			}
			extent, err := core.NewByteLength(uint64(tc.extent))
			if err != nil {
				t.Fatal(err)
			}
			for i := tc.records; i > 0; i-- {
				record := lineio.RecordRange{Offset: lineio.SourceByteOffset((i - 1) * tc.extent), Bytes: extent, Framing: lineio.RecordFramingEOF}
				if err := filestore.WriteRecordRangeIndex(scratch[0], record); err != nil {
					t.Fatal(err)
				}
			}
			var visits int64
			request := filestore.RecordRangeSortRequest{Source: source, Files: scratch, Compare: func(_ context.Context, _ lineio.RecordSource, a, b lineio.RecordRange) (core.Comparison, error) {
				if a.Offset < b.Offset {
					return core.ComparisonLess, nil
				}
				if a.Offset > b.Offset {
					return core.ComparisonGreater, nil
				}
				return core.ComparisonEqual, nil
			}, Visit: func(record lineio.RecordRange) error {
				if int64(record.Offset) != visits*tc.extent {
					return core.ErrFilestoreContract
				}
				visits++
				return nil
			}}
			got, err := filestore.SortRecordRanges(t.Context(), request)
			wantBytes := uint64(tc.records * tc.extent)
			if err != nil || got.Records != uint64(tc.records) || visits != tc.records || source.bytesRead != wantBytes {
				t.Fatalf("range sort = (%+v,%v), visits %d, native bytes %d; want %d records and %d bytes exactly once", got, err, visits, source.bytesRead, tc.records, wantBytes)
			}
			if _, err := source.Stat(); errors.Is(err, os.ErrClosed) || err != nil {
				t.Fatalf("borrowed source = %v, want live native file", err)
			}
		})
	}
}
