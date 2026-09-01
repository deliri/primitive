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
	reads int
	data  []byte
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

	type exactReaderCase struct {
		name          string
		declared      uint64
		sourceBytes   int
		wantDelivered int
		wantErr       bool
		wantEOF       bool
	}
	cases := []exactReaderCase{
		{name: "neutral empty source proves declared zero", declared: 0, sourceBytes: 0},
		{name: "minimum positive extent delivers one byte", declared: 1, sourceBytes: 1, wantDelivered: 1},
		{name: "two byte extent delivers exactly", declared: 2, sourceBytes: 2, wantDelivered: 2},
		{name: "three byte extent delivers exactly", declared: 3, sourceBytes: 3, wantDelivered: 3},
		{name: "seven byte extent delivers exactly", declared: 7, sourceBytes: 7, wantDelivered: 7},
		{name: "one below small power boundary delivers exactly", declared: 31, sourceBytes: 31, wantDelivered: 31},
		{name: "small power boundary delivers exactly", declared: 32, sourceBytes: 32, wantDelivered: 32},
		{name: "one above small power boundary delivers exactly", declared: 33, sourceBytes: 33, wantDelivered: 33},
		{name: "one below kibibyte boundary delivers exactly", declared: 1023, sourceBytes: 1023, wantDelivered: 1023},
		{name: "kibibyte boundary delivers exactly", declared: 1024, sourceBytes: 1024, wantDelivered: 1024},
		{name: "one above kibibyte boundary delivers exactly", declared: 1025, sourceBytes: 1025, wantDelivered: 1025},
		{name: "one below internal buffer boundary delivers exactly", declared: 32767, sourceBytes: 32767, wantDelivered: 32767},
		{name: "internal buffer boundary delivers exactly", declared: 32768, sourceBytes: 32768, wantDelivered: 32768},
		{name: "one above internal buffer boundary delivers exactly", declared: 32769, sourceBytes: 32769, wantDelivered: 32769},

		{name: "declared zero rejects one source byte", declared: 0, sourceBytes: 1, wantErr: true},
		{name: "declared zero rejects two source bytes", declared: 0, sourceBytes: 2, wantErr: true},
		{name: "minimum positive extent rejects empty source", declared: 1, sourceBytes: 0, wantErr: true, wantEOF: true},
		{name: "minimum positive extent rejects one extra byte", declared: 1, sourceBytes: 2, wantErr: true},
		{name: "minimum positive extent rejects two extra bytes", declared: 1, sourceBytes: 3, wantErr: true},
		{name: "two byte extent rejects empty source", declared: 2, sourceBytes: 0, wantErr: true, wantEOF: true},
		{name: "two byte extent rejects one byte short", declared: 2, sourceBytes: 1, wantDelivered: 1, wantErr: true, wantEOF: true},
		{name: "two byte extent rejects one byte long and withholds final declared byte", declared: 2, sourceBytes: 3, wantDelivered: 1, wantErr: true},
		{name: "two byte extent rejects two bytes long and withholds final declared byte", declared: 2, sourceBytes: 4, wantDelivered: 1, wantErr: true},
		{name: "three byte extent rejects empty source", declared: 3, sourceBytes: 0, wantErr: true, wantEOF: true},
		{name: "three byte extent rejects two bytes short", declared: 3, sourceBytes: 1, wantDelivered: 1, wantErr: true, wantEOF: true},
		{name: "three byte extent rejects one byte short", declared: 3, sourceBytes: 2, wantDelivered: 2, wantErr: true, wantEOF: true},
		{name: "three byte extent rejects one byte long and withholds final declared byte", declared: 3, sourceBytes: 4, wantDelivered: 2, wantErr: true},
		{name: "three byte extent rejects two bytes long and withholds final declared byte", declared: 3, sourceBytes: 5, wantDelivered: 2, wantErr: true},
		{name: "one below small power rejects one byte short", declared: 31, sourceBytes: 30, wantDelivered: 30, wantErr: true, wantEOF: true},
		{name: "one below small power rejects one byte long", declared: 31, sourceBytes: 32, wantDelivered: 30, wantErr: true},
		{name: "small power rejects one byte short", declared: 32, sourceBytes: 31, wantDelivered: 31, wantErr: true, wantEOF: true},
		{name: "small power rejects one byte long", declared: 32, sourceBytes: 33, wantDelivered: 31, wantErr: true},
		{name: "one above small power rejects one byte short", declared: 33, sourceBytes: 32, wantDelivered: 32, wantErr: true, wantEOF: true},
		{name: "one above small power rejects one byte long", declared: 33, sourceBytes: 34, wantDelivered: 32, wantErr: true},
		{name: "one below internal buffer rejects one byte short", declared: 32767, sourceBytes: 32766, wantDelivered: 32766, wantErr: true, wantEOF: true},
		{name: "one below internal buffer rejects one byte long", declared: 32767, sourceBytes: 32768, wantDelivered: 32766, wantErr: true},
		{name: "internal buffer rejects one byte short", declared: 32768, sourceBytes: 32767, wantDelivered: 32767, wantErr: true, wantEOF: true},
		{name: "internal buffer rejects one byte long", declared: 32768, sourceBytes: 32769, wantDelivered: 32767, wantErr: true},
		{name: "one above internal buffer rejects one byte short", declared: 32769, sourceBytes: 32768, wantDelivered: 32768, wantErr: true, wantEOF: true},
		{name: "one above internal buffer rejects one byte long", declared: 32769, sourceBytes: 32770, wantDelivered: 32768, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			source := bytes.Repeat([]byte{0xa5}, tc.sourceBytes)
			reader, gotErr := objectstore.NewExactReader(
				bytes.NewReader(source),
				mustByteLength(t, tc.declared),
			)
			if gotErr != nil || reader == nil {
				t.Fatalf("NewExactReader() = (%v, %v), want (non-nil, nil)", reader, gotErr)
			}

			var destination bytes.Buffer
			if tc.declared == 0 {
				gotErr = reader.ProveEmpty()
			} else {
				for {
					var chunk [1]byte
					gotCount, readErr := reader.Read(chunk[:])
					if gotCount > 0 {
						_, _ = destination.Write(chunk[:gotCount])
					}
					if readErr != nil {
						gotErr = readErr
						break
					}
					if gotCount == 0 {
						t.Fatal("ExactReader.Read() = (0, nil), want progress or terminal error")
					}
				}
				if errors.Is(gotErr, io.EOF) && reader.Failure() == nil {
					gotErr = nil
				}
			}
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrObjectStoreSource) ||
					!errors.Is(gotErr, core.ErrObjectStoreIntegrity) ||
					!errors.Is(reader.Failure(), core.ErrObjectStoreSource) ||
					!errors.Is(reader.Failure(), core.ErrObjectStoreIntegrity) {
					t.Fatalf(
						"exact stream refusal = (error %v, failure %v), want errors.Is %v and %v",
						gotErr,
						reader.Failure(),
						core.ErrObjectStoreSource,
						core.ErrObjectStoreIntegrity,
					)
				}
				if gotEOF := errors.Is(gotErr, io.EOF); gotEOF != tc.wantEOF {
					t.Fatalf("exact stream refusal errors.Is(io.EOF) = %t, want %t", gotEOF, tc.wantEOF)
				}
			} else if gotErr != nil || reader.Failure() != nil {
				t.Fatalf("exact stream = (error %v, failure %v), want (nil, nil)", gotErr, reader.Failure())
			}
			if got := destination.Len(); got != tc.wantDelivered {
				t.Fatalf("exact stream delivered bytes = %d, want %d", got, tc.wantDelivered)
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
