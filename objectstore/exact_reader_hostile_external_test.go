package objectstore_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
)

type exactReaderEmptySource struct{ reads int }

func (r *exactReaderEmptySource) Read([]byte) (int, error) {
	r.reads++
	return 0, nil
}

type exactReaderUnclosedSource struct {
	data  []byte
	reads int
}

func (r *exactReaderUnclosedSource) Read(destination []byte) (int, error) {
	r.reads++
	if r.reads > 1 {
		panic("ExactReader attempted a blocking overflow probe")
	}
	return copy(destination, r.data), nil
}

func TestExactReaderConstructionRejectsNilSource(t *testing.T) {
	t.Parallel()

	var typedNil *bytes.Reader
	cases := []struct {
		source io.Reader
		name   string
	}{
		{name: "nil interface", source: nil},
		{name: "typed nil pointer", source: typedNil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := objectstore.NewExactReader(testCase.source, mustByteLength(t, 0))
			if got != nil ||
				!errors.Is(gotErr, core.ErrObjectStoreContract) ||
				!errors.Is(gotErr, core.ErrObjectStoreSource) {
				t.Fatalf(
					"NewExactReader() = (%v, %v), want (nil, errors including %v and %v)",
					got,
					gotErr,
					core.ErrObjectStoreContract,
					core.ErrObjectStoreSource,
				)
			}
		})
	}
}

func TestExactReaderExtentLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		source, want  []byte
		declared      uint64
		chunk         int
		wantErr       error
		wantRemaining int
	}{
		{name: "empty source requires empty proof", chunk: 1},
		{name: "minimum object survives an oversized destination", source: []byte{0x81}, want: []byte{0x81}, declared: 1, chunk: 3},
		{name: "partial final chunk preserves exact byte order", source: []byte{0, 0x81, 0xff}, want: []byte{0, 0x81, 0xff}, declared: 3, chunk: 2},
		{name: "exact chunk boundary preserves terminal bytes", source: []byte{0x81, 0}, want: []byte{0x81, 0}, declared: 2, chunk: 2},
		{name: "empty declaration refuses without consuming a byte", source: []byte{0x81}, chunk: 1, wantErr: core.ErrObjectStoreSource, wantRemaining: 1},
		{name: "short empty source cannot prove a nonempty object", declared: 1, chunk: 1, wantErr: io.EOF},
		{name: "short stream preserves exact delivered prefix", source: []byte{0, 0x81, 0xff}, want: []byte{0, 0x81, 0xff}, declared: 4, chunk: 2, wantErr: io.EOF},
		{name: "overlong first chunk is withheld", source: []byte{0x81, 0xff}, declared: 1, chunk: 2, wantErr: core.ErrObjectStoreSource, wantRemaining: 1},
		{name: "overlong final partial chunk is withheld after exact prefix", source: []byte{0, 0x81, 0xff, 0x42}, want: []byte{0, 0x81}, declared: 3, chunk: 2, wantErr: core.ErrObjectStoreSource, wantRemaining: 1},
		{name: "overlong final full chunk is withheld", source: []byte{0, 0x81, 0xff, 0x42, 0x10}, want: []byte{0, 0x81}, declared: 4, chunk: 2, wantErr: core.ErrObjectStoreSource, wantRemaining: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := bytes.NewReader(tc.source)
			reader, err := objectstore.NewExactReader(source, mustByteLength(t, tc.declared))
			if err != nil {
				t.Fatal(err)
			}
			var got []byte
			buffer := make([]byte, tc.chunk)
			if tc.declared == 0 {
				err = reader.ProveEmpty()
			} else {
				for attempts := uint64(0); ; attempts++ {
					if attempts > tc.declared+1 {
						t.Fatalf("read attempts = %d, want at most declared extent %d plus one EOF read", attempts, tc.declared)
					}
					n, readErr := reader.Read(buffer)
					if n < 0 || n > len(buffer) {
						t.Fatalf("read count = %d, capacity = %d", n, len(buffer))
					}
					got = append(got, buffer[:n]...)
					if readErr != nil {
						err = readErr
						break
					}
				}
				if reader.Failure() == nil && errors.Is(err, io.EOF) {
					err = nil
				}
			}
			if !errors.Is(err, tc.wantErr) || !bytes.Equal(got, tc.want) || source.Len() != tc.wantRemaining {
				t.Fatalf("stream = (%x, %v), remaining=%d; want (%x, %v), remaining=%d", got, err, source.Len(), tc.want, tc.wantErr, tc.wantRemaining)
			}
			if tc.wantErr != nil && (!errors.Is(reader.Failure(), core.ErrObjectStoreSource) || !errors.Is(reader.Failure(), core.ErrObjectStoreIntegrity)) {
				t.Fatalf("failure = %v, want source and integrity identities", reader.Failure())
			}
			if tc.wantErr == nil && reader.Failure() != nil {
				t.Fatalf("successful stream retained failure %v", reader.Failure())
			}
			remaining := source.Len()
			n, again := reader.Read(buffer)
			wantTerminal := io.EOF
			if tc.wantErr != nil {
				wantTerminal = tc.wantErr
			}
			if n != 0 || !errors.Is(again, wantTerminal) || source.Len() != remaining {
				t.Fatalf("terminal replay = (%d, %v), remaining=%d; want no bytes, %v, unchanged source", n, again, source.Len(), wantTerminal)
			}
		})
	}
}

func TestExactReaderSignedLengthCeilingFailsShortWithoutAllocation(t *testing.T) {
	t.Parallel()

	reader, gotErr := objectstore.NewExactReader(
		bytes.NewReader(nil),
		mustByteLength(t, math.MaxInt64),
	)
	if gotErr != nil || reader == nil {
		t.Fatalf("NewExactReader(MaxInt64) = (%v, %v), want (non-nil, nil)", reader, gotErr)
	}
	var destination bytes.Buffer
	_, gotErr = io.CopyBuffer(&destination, reader, make([]byte, 1))
	if !errors.Is(gotErr, core.ErrObjectStoreSource) ||
		!errors.Is(gotErr, core.ErrObjectStoreIntegrity) ||
		!errors.Is(gotErr, io.EOF) ||
		destination.Len() != 0 {
		t.Fatalf(
			"MaxInt64 empty source = (%d bytes, %v), want (0 bytes, errors.Is %v, %v, and io.EOF)",
			destination.Len(),
			gotErr,
			core.ErrObjectStoreSource,
			core.ErrObjectStoreIntegrity,
		)
	}
}

func TestExactReaderRefusesConsecutiveEmptyReadsWithTypedSourceFailure(t *testing.T) {
	t.Parallel()

	source := &exactReaderEmptySource{}
	reader, gotErr := objectstore.NewExactReader(source, mustByteLength(t, 1))
	if gotErr != nil {
		t.Fatalf("NewExactReader(empty reader) error = %v, want nil", gotErr)
	}
	var destination bytes.Buffer
	_, gotErr = io.CopyBuffer(&destination, reader, make([]byte, 1))
	if !errors.Is(gotErr, io.ErrNoProgress) ||
		!errors.Is(gotErr, core.ErrObjectStoreSource) ||
		!errors.Is(gotErr, core.ErrObjectStoreIntegrity) ||
		destination.Len() != 0 || source.reads > core.ReaderConsecutiveEmptyReadMaximum {
		t.Fatalf(
			"ExactReader(empty source) = (error %v, bytes %d, reads %d), want %v with typed source integrity and at most %d reads",
			gotErr,
			destination.Len(),
			source.reads,
			io.ErrNoProgress,
			core.ReaderConsecutiveEmptyReadMaximum,
		)
	}
}

func TestExactReaderRefusesUnprovableEndWithoutASecondRead(t *testing.T) {
	t.Parallel()

	source := &exactReaderUnclosedSource{data: []byte{0x7a}}
	reader, err := objectstore.NewExactReader(source, mustByteLength(t, 1))
	if err != nil {
		t.Fatalf("NewExactReader() error = %v, want nil", err)
	}
	var destination [1]byte
	got, gotErr := reader.Read(destination[:])
	if got != 0 || source.reads != 1 || !errors.Is(gotErr, io.ErrNoProgress) ||
		!errors.Is(gotErr, core.ErrObjectStoreSource) {
		t.Fatalf("ExactReader.Read(unclosed exact source) = (%d, %v, %d source reads), want (0, typed no-progress, 1 read)", got, gotErr, source.reads)
	}

	empty := &exactReaderUnclosedSource{}
	reader, err = objectstore.NewExactReader(empty, mustByteLength(t, 0))
	if err != nil {
		t.Fatalf("NewExactReader(empty) error = %v, want nil", err)
	}
	gotErr = reader.ProveEmpty()
	if empty.reads != 0 || !errors.Is(gotErr, io.ErrNoProgress) || !errors.Is(gotErr, core.ErrObjectStoreSource) {
		t.Fatalf("ExactReader.ProveEmpty(unprovable source) = (%v, %d reads), want typed no-progress without reading", gotErr, empty.reads)
	}
}
