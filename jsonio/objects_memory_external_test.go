package jsonio_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/testserial"
)

type repeatedJSONObjectReader struct {
	record            string
	remaining, offset int
}

func (r *repeatedJSONObjectReader) Read(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := copy(data, r.record[r.offset:])
	r.offset += n
	if r.offset == len(r.record) {
		r.offset = 0
		r.remaining--
	}
	return n, nil
}

func TestJSONObjectStreamRetainedMemoryDoesNotScaleWithRecords(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	object := scalarObject{Name: "one", Count: 1}
	var seed bytes.Buffer
	encoder, err := jsonio.NewObjectEncoder[scalarObject](jsonio.ObjectDestinationRequest{Destination: &seed})
	if err != nil {
		t.Fatal(err)
	}
	if err := encoder.Encode(t.Context(), object); err != nil {
		t.Fatal(err)
	}
	const allowance = 256 << 10
	for _, records := range []int{4096, 65536, 262144} {
		source := &repeatedJSONObjectReader{record: seed.String(), remaining: records}
		before, err := hostfacts.ObserveCollectedGoHeap(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		for got, err := range jsonio.Objects[scalarObject](t.Context(), jsonio.ObjectSourceRequest{Source: source}) {
			if err != nil || got != object {
				t.Fatalf("object=(%+v,%v),want exact %+v", got, err, object)
			}
			seen++
		}
		after, err := hostfacts.ObserveCollectedGoHeap(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if seen != records || source.remaining != 0 {
			t.Fatalf("records=(%d,%d unread),want(%d,0)", seen, source.remaining, records)
		}
		retained := int64(after.Uint64()) - int64(before.Uint64())
		t.Logf("records=%d retained_bytes=%d allowance=%d", records, retained, allowance)
		if retained > allowance {
			t.Fatalf("retained bytes=%d,want <=%d independent of record count", retained, allowance)
		}
	}
}
