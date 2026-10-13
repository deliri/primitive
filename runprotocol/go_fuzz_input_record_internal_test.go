package runprotocol

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

func TestGoFuzzInputRecordRejectsForeignCoordinates(t *testing.T) {
	t.Parallel()
	whole, err := core.NewByteLength(20)
	if err != nil {
		t.Fatal(err)
	}
	path, err := core.NewByteLength(4)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name         string
		record, path lineio.RecordRange
		wantValid    bool
	}{
		{"contained exact ending", lineio.RecordRange{Offset: 10, Bytes: whole, Framing: lineio.RecordFramingLF}, lineio.RecordRange{Offset: 26, Bytes: path, Framing: lineio.RecordFramingOpaque}, true},
		{"contained at first byte", lineio.RecordRange{Offset: 10, Bytes: whole, Framing: lineio.RecordFramingEOF}, lineio.RecordRange{Offset: 10, Bytes: path, Framing: lineio.RecordFramingOpaque}, true},
		{"path before physical record", lineio.RecordRange{Offset: 10, Bytes: whole, Framing: lineio.RecordFramingLF}, lineio.RecordRange{Offset: 9, Bytes: path, Framing: lineio.RecordFramingOpaque}, false},
		{"path one byte past physical record", lineio.RecordRange{Offset: 10, Bytes: whole, Framing: lineio.RecordFramingLF}, lineio.RecordRange{Offset: 27, Bytes: path, Framing: lineio.RecordFramingOpaque}, false},
		{"path longer than physical record", lineio.RecordRange{Offset: 10, Bytes: path, Framing: lineio.RecordFramingLF}, lineio.RecordRange{Offset: 10, Bytes: whole, Framing: lineio.RecordFramingOpaque}, false},
		{"path framing cannot become physical authority", lineio.RecordRange{Offset: 10, Bytes: whole, Framing: lineio.RecordFramingLF}, lineio.RecordRange{Offset: 26, Bytes: path, Framing: lineio.RecordFramingLF}, false},
		{"zero physical record", lineio.RecordRange{}, lineio.RecordRange{Offset: 26, Bytes: path, Framing: lineio.RecordFramingOpaque}, false},
		{"zero path", lineio.RecordRange{Offset: 10, Bytes: whole, Framing: lineio.RecordFramingLF}, lineio.RecordRange{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := GoFuzzInputRecord{record: tc.record, path: tc.path}
			if err := got.Validate(); (err == nil) != tc.wantValid {
				t.Fatalf("span %+v validation = %v, want valid %t", got, err, tc.wantValid)
			}
		})
	}
}
