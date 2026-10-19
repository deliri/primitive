package jsonio

import (
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// referenceRecordExtent classifies every byte, as the record capture contract
// defines: first and one-past-last non-whitespace offsets, or -1 when blank.
func referenceRecordExtent(record string) (int64, int64) {
	first, last := int64(-1), int64(0)
	for index := range len(record) {
		if strings.IndexByte(core.JSONWhitespace, record[index]) < 0 {
			if first < 0 {
				first = int64(index)
			}
			last = int64(index) + 1
		}
	}
	return first, last
}

func FuzzProjectionRecordCaptureExtentAcrossChunks(f *testing.F) {
	f.Add(" \t{\"a\":1} \r\n", uint8(1))
	f.Add("   ", uint8(2))
	f.Add("x", uint8(0))
	f.Add("\n {} \n {} \t", uint8(3))
	f.Fuzz(func(t *testing.T, record string, chunk uint8) {
		width := int(chunk%7) + 1
		capture := projectionRecordCapture{digest: core.NewDigestWriter(), first: -1}
		for start := 0; start < len(record); start += width {
			part := record[start:min(start+width, len(record))]
			if n, err := capture.Write([]byte(part)); n != len(part) || err != nil {
				t.Fatalf("Write = (%d, %v), want full acceptance", n, err)
			}
		}
		first, last := referenceRecordExtent(record)
		if capture.first != first || (first >= 0 && capture.last != last) || capture.consumed != int64(len(record)) {
			t.Fatalf("extent = (%d, %d, consumed %d), want (%d, %d, %d)", capture.first, capture.last, capture.consumed, first, last, len(record))
		}
	})
}
