package lineio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/lineio"
	"github.com/deliri/primitive/v2026/testserial"
)

func lastRangeFile(t testing.TB, body []byte) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "native-records")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	return file
}

// The oracle uses standard-library byte search and the actual held file.
// It does not use the production reverse scan or RecordRanges framing.
func proveLastRecordRange(t *testing.T, body []byte, width uint64) {
	t.Helper()
	file := lastRangeFile(t, body)
	before := int64(len(body) / 2)
	if _, err := file.Seek(before, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	extent, err := core.NewByteLength(uint64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	buffer, err := core.NewByteCount(width)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := lineio.ObserveLastRecordRange(t.Context(), lineio.LastRecordRangeRequest{Source: file, Extent: extent, BufferBytes: buffer})
	if err != nil || observed.Validate() != nil {
		t.Fatalf("native tail observation=%+v/%v", observed, err)
	}
	position, err := file.Seek(0, io.SeekCurrent)
	if err != nil || position != before {
		t.Fatalf("positional tail changed sequential cursor=%d/%v want %d", position, err, before)
	}
	if len(body) == 0 {
		if observed.Present || observed.Record != (lineio.RecordRange{}) {
			t.Fatalf("empty source invented record: %+v", observed)
		}
		return
	}
	start := bytes.LastIndexByte(body[:len(body)-1], '\n') + 1
	wantFraming := lineio.RecordFramingEOF
	if body[len(body)-1] == '\n' {
		wantFraming = lineio.RecordFramingLF
	}
	if !observed.Present || int64(observed.Record.Offset) != int64(start) || observed.Record.Bytes.Uint64() != uint64(len(body)-start) || observed.Record.Framing != wantFraming {
		t.Fatalf("tail=%+v, independent window=%d..%d framing=%v", observed, start, len(body), wantFraming)
	}
	actual := make([]byte, len(body)-start)
	n, err := file.ReadAt(actual, int64(observed.Record.Offset))
	if err != nil || n != len(actual) || !bytes.Equal(actual, body[start:]) {
		t.Fatalf("original native coordinates changed bytes: %q/%d/%v", actual, n, err)
	}
}

func TestLastRecordRangeMatchesNativeBytesWithoutChangingCursor(t *testing.T) {
	t.Parallel()
	for _, body := range [][]byte{nil, {'\n'}, {'\n', '\n'}, []byte("a\nb\n"), []byte("a\nfinal"), bytes.Repeat([]byte("z"), 65537), append([]byte("prefix\n"), bytes.Repeat([]byte("x"), 65537)...)} {
		for _, width := range []uint64{1, 7, 4096} {
			proveLastRecordRange(t, body, width)
		}
	}
}

type cancelLastRangeRead struct {
	file   *os.File
	cancel context.CancelCauseFunc
}

func (r cancelLastRangeRead) ReadAt(data []byte, offset int64) (int, error) {
	n, err := r.file.ReadAt(data, offset)
	r.cancel(core.ErrLineIOContract)
	return n, err
}

func TestLastRecordRangeRefusesTruncationCancellationAndTypedNil(t *testing.T) {
	t.Parallel()
	body := []byte("prefix\nlast\n")
	file := lastRangeFile(t, body)
	extent, err := core.NewByteLength(uint64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	buffer, err := core.NewByteCount(4)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(2); err != nil {
		t.Fatal(err)
	}
	observed, err := lineio.ObserveLastRecordRange(t.Context(), lineio.LastRecordRangeRequest{Source: file, Extent: extent, BufferBytes: buffer})
	if observed.Present || !errors.Is(err, core.ErrLineIOScan) || !errors.Is(err, io.EOF) || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("native truncated input became absence/coordinates: %+v/%v", observed, err)
	}
	file = lastRangeFile(t, body)
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	observed, err = lineio.ObserveLastRecordRange(ctx, lineio.LastRecordRangeRequest{Source: cancelLastRangeRead{file: file, cancel: cancel}, Extent: extent, BufferBytes: buffer})
	if observed.Present || !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrLineIOContract) {
		t.Fatalf("cancelled actual read published tail: %+v/%v", observed, err)
	}
	var missing *os.File
	observed, err = lineio.ObserveLastRecordRange(t.Context(), lineio.LastRecordRangeRequest{Source: missing, Extent: extent, BufferBytes: buffer})
	if observed.Present || !errors.Is(err, core.ErrLineIOContract) {
		t.Fatalf("typed nil source admitted: %+v/%v", observed, err)
	}
}

func FuzzLastRecordRangeMatchesIndependentPhysicalTail(f *testing.F) {
	for _, seed := range [][]byte{{}, {'\n'}, []byte("one\ntwo\n"), []byte("one\ntorn"), {0, 255, '\n', 0, 255}, bytes.Repeat([]byte("record\n"), 1025)} {
		f.Add(seed, uint16(7))
	}
	f.Fuzz(func(t *testing.T, body []byte, width uint16) { proveLastRecordRange(t, body, uint64(width%4096)+1) })
}

func TestLastRecordRangeAdmitsOnlyPhysicalObservationShapes(t *testing.T) {
	t.Parallel()
	one, err := core.NewByteLength(1)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []lineio.RecordRangeObservation{
		{Record: lineio.RecordRange{Bytes: one}},
		{Present: true},
		{Present: true, Record: lineio.RecordRange{Bytes: one, Framing: lineio.RecordFramingOpaque}},
		{Present: true, Record: lineio.RecordRange{Offset: -1, Bytes: one, Framing: lineio.RecordFramingLF}},
	} {
		if !errors.Is(value.Validate(), core.ErrLineIOContract) {
			t.Fatalf("invalid physical observation admitted: %+v", value)
		}
	}
	file := lastRangeFile(t, []byte("x\n"))
	observed, err := lineio.ObserveLastRecordRange(t.Context(), lineio.LastRecordRangeRequest{Source: file, Extent: one})
	if err == nil || observed.Present {
		t.Fatalf("unset byte window admitted: %+v/%v", observed, err)
	}
}

func TestLastRecordRangeWorkingMemoryDoesNotFollowRecordExtent(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	var samples [2]uint64
	for lane, size := range []int{64, 4 << 20} {
		body := bytes.Repeat([]byte("x"), size)
		file := lastRangeFile(t, body)
		extent, err := core.NewByteLength(uint64(size))
		if err != nil {
			t.Fatal(err)
		}
		buffer, err := core.NewByteCount(4096)
		if err != nil {
			t.Fatal(err)
		}
		before, err := hostfacts.ObserveGoAllocationTotal(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		observed, err := lineio.ObserveLastRecordRange(t.Context(), lineio.LastRecordRangeRequest{Source: file, Extent: extent, BufferBytes: buffer})
		if err != nil {
			t.Fatal(err)
		}
		after, err := hostfacts.ObserveGoAllocationTotal(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		samples[lane] = after.Uint64() - before.Uint64()
		if !observed.Present || observed.Record.Offset != 0 || observed.Record.Bytes.Uint64() != uint64(size) {
			t.Fatalf("long actual record lost coordinates: %+v", observed)
		}
		t.Logf("last_record_bytes=%d query_allocated_bytes=%d", size, samples[lane])
	}
	if samples[1] > samples[0]+8192 {
		t.Fatalf("tail query allocated record population: %v", samples)
	}
}
