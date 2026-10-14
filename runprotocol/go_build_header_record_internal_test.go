package runprotocol

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

func TestGoBuildHeaderCoordinateContract(t *testing.T) {
	t.Parallel()
	whole, err := core.NewByteLength(20)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		offset  lineio.SourceByteOffset
		bytes   uint64
		framing lineio.RecordFraming
		refuse  bool
	}{
		{name: "first byte fixed prefix", offset: 10, bytes: 2, framing: lineio.RecordFramingOpaque},
		{name: "last contained prefix", offset: 28, bytes: 2, framing: lineio.RecordFramingOpaque},
		{name: "prefix escapes before record", offset: 9, bytes: 2, framing: lineio.RecordFramingOpaque, refuse: true},
		{name: "prefix escapes after record", offset: 29, bytes: 2, framing: lineio.RecordFramingOpaque, refuse: true},
		{name: "wrong literal extent", offset: 10, bytes: 3, framing: lineio.RecordFramingOpaque, refuse: true},
		{name: "marker cannot carry physical framing", offset: 10, bytes: 2, framing: lineio.RecordFramingLF, refuse: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			size, err := core.NewByteLength(tc.bytes)
			if err != nil {
				t.Fatal(err)
			}
			got := GoFuzzSignalRecord{Record: lineio.RecordRange{Offset: 10, Bytes: whole, Framing: lineio.RecordFramingLF}, BuildHeader: lineio.RecordRange{Offset: tc.offset, Bytes: size, Framing: tc.framing}}
			err = got.Validate()
			if errors.Is(err, core.ErrGoToolchainContract) != tc.refuse {
				t.Fatalf("record/error=%+v/%v,want refusal%t", got, err, tc.refuse)
			}
		})
	}
}
