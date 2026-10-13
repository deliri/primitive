package runprotocol

import (
	"errors"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

func TestGoFuzzSignalRecordCoordinatesCannotEscapePhysicalRecord(t *testing.T) {
	t.Parallel()
	extent, err := core.NewByteLength(20)
	if err != nil {
		t.Fatal(err)
	}
	markerExtent, err := core.NewByteLength(6)
	if err != nil {
		t.Fatal(err)
	}
	for _, offset := range []lineio.SourceByteOffset{9, 10, 24, 25, 30} {
		t.Run(strconv.FormatInt(int64(offset), 10), func(t *testing.T) {
			t.Parallel()
			marker := lineio.RecordRange{Offset: offset, Bytes: markerExtent, Framing: lineio.RecordFramingOpaque}
			got := GoFuzzSignalRecord{Record: lineio.RecordRange{Offset: 10, Bytes: extent, Framing: lineio.RecordFramingLF}, Panic: marker}
			err := got.Validate()
			wantRefuse := offset < 10 || offset > 24
			if wantRefuse != errors.Is(err, core.ErrGoToolchainContract) {
				t.Fatalf("offset %d error=%v,want foreign=%t", offset, err, wantRefuse)
			}
		})
	}
}
