package filestore

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// The buffered fast path must refuse exactly as the unbuffered reader loop does,
// and must consume exactly one tuple per admitted record.
func TestBufferedIndexRecordMatchesUnbufferedRefusals(t *testing.T) {
	t.Parallel()
	entry := ContentIndexEntry{Digest: core.NewSHA256Digest([core.SHA256DigestBytes]byte{31: 7}), Extent: mustByteLengthForTest(t, 9)}
	var content bytes.Buffer
	if err := WriteContentIndexEntry(&content, entry); err != nil {
		t.Fatal(err)
	}
	record := lineio.RecordRange{Offset: 3, Bytes: mustByteLengthForTest(t, 5), Framing: lineio.RecordFramingLF}
	var ranges bytes.Buffer
	if err := WriteRecordRangeIndex(&ranges, record); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		source func([]byte) io.Reader
		want   []error
	}{
		{name: "clean end", source: func([]byte) io.Reader { return bytes.NewReader(nil) }, want: []error{io.EOF}},
		{name: "partial tuple", source: func(data []byte) io.Reader { return bytes.NewReader(data[:len(data)-1]) }, want: []error{core.ErrFilestoreSource, io.ErrUnexpectedEOF}},
		{name: "native refusal", source: func(data []byte) io.Reader {
			return io.MultiReader(bytes.NewReader(data[:2]), iotest.ErrReader(io.ErrClosedPipe))
		}, want: []error{core.ErrFilestoreSource, io.ErrClosedPipe}},
		{name: "no progress", source: func([]byte) io.Reader { return &indexNoProgressSource{} }, want: []error{core.ErrFilestoreSource, io.ErrNoProgress}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, buffered := range []bool{false, true} {
				wrap := func(data []byte) io.Reader {
					if buffered {
						return bufio.NewReaderSize(tc.source(data), 64)
					}
					return tc.source(data)
				}
				gotEntry, entryErr := ReadContentIndexEntry(wrap(content.Bytes()))
				gotRange, rangeErr := ReadRecordRangeIndex(wrap(ranges.Bytes()))
				for _, want := range tc.want {
					if !errors.Is(entryErr, want) || !errors.Is(rangeErr, want) {
						t.Fatalf("buffered=%t refusals = (%v,%v), want %v", buffered, entryErr, rangeErr, tc.want)
					}
				}
				if tc.want[0] == io.EOF && (entryErr != io.EOF || rangeErr != io.EOF) {
					t.Fatalf("buffered=%t clean end = (%v,%v), want bare io.EOF", buffered, entryErr, rangeErr)
				}
				if gotEntry != (ContentIndexEntry{}) || gotRange != (lineio.RecordRange{}) {
					t.Fatalf("buffered=%t refused records = (%+v,%+v), want zero", buffered, gotEntry, gotRange)
				}
			}
		})
	}
	t.Run("consecutive tuples", func(t *testing.T) {
		t.Parallel()
		contentReader := bufio.NewReaderSize(io.MultiReader(bytes.NewReader(content.Bytes()), bytes.NewReader(content.Bytes())), 64)
		rangeReader := bufio.NewReaderSize(io.MultiReader(bytes.NewReader(ranges.Bytes()), bytes.NewReader(ranges.Bytes())), 32)
		for range 2 {
			gotEntry, entryErr := ReadContentIndexEntry(contentReader)
			gotRange, rangeErr := ReadRecordRangeIndex(rangeReader)
			if entryErr != nil || rangeErr != nil || gotEntry != entry || gotRange != record {
				t.Fatalf("buffered tuples = (%+v,%v),(%+v,%v), want exact records", gotEntry, entryErr, gotRange, rangeErr)
			}
		}
		_, entryErr := ReadContentIndexEntry(contentReader)
		_, rangeErr := ReadRecordRangeIndex(rangeReader)
		if entryErr != io.EOF || rangeErr != io.EOF {
			t.Fatalf("buffered end = (%v,%v), want bare io.EOF after exactly two tuples", entryErr, rangeErr)
		}
	})
	t.Run("window smaller than tuple uses reader loop", func(t *testing.T) {
		t.Parallel()
		got, err := ReadContentIndexEntry(bufio.NewReaderSize(bytes.NewReader(content.Bytes()), 16))
		if err != nil || got != entry {
			t.Fatalf("small-window record = (%+v,%v), want exact record", got, err)
		}
	})
	t.Run("buffered writer encodes in place", func(t *testing.T) {
		t.Parallel()
		var encoded bytes.Buffer
		writer := bufio.NewWriterSize(&encoded, 64)
		if err := errors.Join(WriteContentIndexEntry(writer, entry), WriteRecordRangeIndex(writer, record), WriteRecordRangeIndex(writer, record), writer.Flush()); err != nil {
			t.Fatal(err)
		}
		want := append(append(append([]byte(nil), content.Bytes()...), ranges.Bytes()...), ranges.Bytes()...)
		if !bytes.Equal(encoded.Bytes(), want) {
			t.Fatalf("buffered encoding = %x, want %x", encoded.Bytes(), want)
		}
	})
}

func mustByteLengthForTest(t *testing.T, value uint64) core.ByteLength {
	t.Helper()
	length, err := core.NewByteLength(value)
	if err != nil {
		t.Fatal(err)
	}
	return length
}
