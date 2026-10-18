package filestore_test

import (
	"context"
	"encoding/binary"
	"sort"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

func FuzzRecordRangeLookupMatchesIndependentForwardOracle(f *testing.F) {
	f.Add([]byte{}, int64(0))
	f.Add([]byte{0, 1, 0, 1, 0, 1, 0, 7}, int64(1))
	f.Add([]byte{255, 255, 0, 0, 0, 1, 0, 2}, int64(-1))
	f.Fuzz(func(t *testing.T, data []byte, target int64) {
		if len(data) > 256 {
			return
		} // Fixture work budget, not an index or byte quota.
		var keys []int64
		for i := 0; i+1 < len(data); i += 2 {
			keys = append(keys, int64(binary.BigEndian.Uint16(data[i:i+2])))
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		want := -1
		for i, key := range keys {
			if key >= target {
				want = i
				break
			}
		}
		file := lookupIndexFile(t, keys)
		got, err := filestore.LowerBoundRecordRangeIndex(t.Context(), filestore.RecordRangeIndexSearchRequest{Source: file, Compare: func(_ context.Context, record lineio.RecordRange) (core.Comparison, error) {
			return lookupComparison(int64(record.Offset), target), nil
		}})
		if err != nil {
			t.Fatal(err)
		}
		if want < 0 {
			if got != (filestore.RecordRangeIndexSearchObservation{}) {
				t.Fatalf("missing candidate=%+v", got)
			}
			return
		}
		if !got.Present || got.Position != filestore.RecordRangeIndexPosition(want) || int64(got.Record.Offset) != keys[want] || got.Record.Bytes.Uint64() != 1 || got.Record.Framing != lineio.RecordFramingOpaque {
			t.Fatalf("candidate=%+v want forward index=%d key=%d", got, want, keys[want])
		}
		at, err := filestore.ReadRecordRangeIndexAt(t.Context(), filestore.RecordRangeIndexReadRequest{Source: file, Position: got.Position})
		if err != nil || at != got.Record {
			t.Fatalf("positional record=%+v/%v candidate=%+v", at, err, got)
		}
	})
}
