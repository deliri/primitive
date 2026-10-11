package lineio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

func TestRangeReaderNativePullCoordinatesAndClosure(t *testing.T) {
	t.Parallel()
	for _, width := range [...]uint64{1, 16, 4096, 65536} {
		t.Run(strconv.FormatUint(width, 10), func(t *testing.T) {
			t.Parallel()
			body := []byte("prefix\n\xff\x00\r\n\nlast")
			source := bytes.NewReader(body)
			if _, err := source.Seek(7, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			reader, err := lineio.NewRangeReader(t.Context(), lineio.RecordRangeRequest{Source: source, BufferBytes: recordRangeBuffer(t, width)})
			if err != nil {
				t.Fatal(err)
			}
			offset := 7
			for _, want := range bytes.SplitAfter(body[7:], []byte{'\n'}) {
				record, err := reader.ReadRecord(t.Context())
				framing := lineio.RecordFramingEOF
				if want[len(want)-1] == '\n' {
					framing = lineio.RecordFramingLF
				}
				if err != nil || uint64(record.Offset) != uint64(offset) || record.Bytes.Uint64() != uint64(len(want)) || record.Framing != framing {
					t.Fatalf("pull = (%+v,%v), want offset %d exact %q framing %d", record, err, offset, want, framing)
				}
				actual := make([]byte, len(want))
				if _, err := source.ReadAt(actual, int64(record.Offset)); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(actual, want) {
					t.Fatalf("native replay = %q, want %q", actual, want)
				}
				offset += len(want)
			}
			for range 2 {
				record, err := reader.ReadRecord(t.Context())
				if err != io.EOF || record != (lineio.RecordRange{}) {
					t.Fatalf("native closure = (%+v,%v), want zero and bare EOF", record, err)
				}
			}
		})
	}
}

func TestRangeReaderCancellationDoesNotAdvanceBorrowedSource(t *testing.T) {
	t.Parallel()
	native := bytes.NewReader([]byte("first\nsecond\n"))
	reader, err := lineio.NewRangeReader(t.Context(), lineio.RecordRangeRequest{Source: native, BufferBytes: recordRangeBuffer(t, 16)})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	record, err := reader.ReadRecord(ctx)
	offset, seekErr := native.Seek(0, io.SeekCurrent)
	if !errors.Is(err, context.Canceled) || record != (lineio.RecordRange{}) || offset != 0 || seekErr != nil {
		t.Fatalf("cancelled pull = (%+v,%v), native position (%d,%v), want zero and untouched source", record, err, offset, seekErr)
	}
	record, err = reader.ReadRecord(t.Context())
	if err != nil || record.Offset != 0 || record.Bytes.Uint64() != 6 || record.Framing != lineio.RecordFramingLF {
		t.Fatalf("resumed pull = (%+v,%v), want exact first line", record, err)
	}
}

func TestRangeReaderNilAndZeroRemainTypedRefusals(t *testing.T) {
	t.Parallel()
	for _, reader := range [...]*lineio.RangeReader{nil, {}} {
		record, err := reader.ReadRecord(t.Context())
		if !errors.Is(err, core.ErrLineIOContract) || record != (lineio.RecordRange{}) {
			t.Fatalf("invalid pull = (%+v,%v), want zero and LineIO contract", record, err)
		}
	}
}

func BenchmarkRangeReaderFixedWindow(b *testing.B) {
	for _, size := range [...]int{65535, 65536, 65537, 8<<20 + 1} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			body := bytes.Repeat([]byte{'x'}, size)
			body = append(body, '\n')
			buffer, err := core.NewByteCount(65536)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(body)))
			for b.Loop() {
				reader, err := lineio.NewRangeReader(b.Context(), lineio.RecordRangeRequest{Source: bytes.NewReader(body), BufferBytes: buffer})
				if err != nil {
					b.Fatal(err)
				}
				record, err := reader.ReadRecord(b.Context())
				if err != nil || record.Offset != 0 || record.Bytes.Uint64() != uint64(len(body)) || record.Framing != lineio.RecordFramingLF {
					b.Fatalf("record = (%+v,%v), want exact physical extent", record, err)
				}
				if record, err := reader.ReadRecord(b.Context()); err != io.EOF || record != (lineio.RecordRange{}) {
					b.Fatalf("closure = (%+v,%v), want zero and EOF", record, err)
				}
			}
		})
	}
}
