package objectstore_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"hash/crc32"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
)

type hostileInspectionReader struct {
	err       error
	data      []byte
	chunk     int
	position  int
	failAfter int
}

func (r *hostileInspectionReader) Read(destination []byte) (int, error) {
	if r.failAfter >= 0 && r.position >= r.failAfter {
		return 0, r.err
	}
	if r.position >= len(r.data) {
		return 0, io.EOF
	}
	limit := len(destination)
	if r.chunk > 0 && limit > r.chunk {
		limit = r.chunk
	}
	if remaining := len(r.data) - r.position; limit > remaining {
		limit = remaining
	}
	if r.failAfter >= 0 && r.position+limit > r.failAfter {
		limit = r.failAfter - r.position
	}
	read := copy(destination[:limit], r.data[r.position:r.position+limit])
	r.position += read
	return read, nil
}

func TestInspectStreamsExactIntegrityAcrossHostileChunkBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		size  int
		chunk int
	}{
		{name: "single byte source", size: 1, chunk: 1},
		{name: "two bytes one at a time", size: 2, chunk: 1},
		{name: "three bytes split after two", size: 3, chunk: 2},
		{name: "thirty one bytes below word boundary", size: 31, chunk: 7},
		{name: "thirty two bytes at word boundary", size: 32, chunk: 8},
		{name: "thirty three bytes above word boundary", size: 33, chunk: 9},
		{name: "one below inspection buffer", size: 32767, chunk: 113},
		{name: "exact inspection buffer", size: 32768, chunk: 251},
		{name: "one above inspection buffer", size: 32769, chunk: 4096},
		{name: "many inspection buffers with prime chunks", size: 131101, chunk: 509},
		{name: "reader may fill destination", size: 65536, chunk: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := bytes.Repeat([]byte{byte(tc.size%251 + 1)}, tc.size)
			maximum, err := core.NewByteCount(uint64(tc.size))
			if err != nil {
				t.Fatalf("core.NewByteCount(%d) setup error = %v, want nil", tc.size, err)
			}
			reader := &hostileInspectionReader{data: payload, chunk: tc.chunk, failAfter: -1}
			got, err := objectstore.Inspect(t.Context(), objectstore.InspectionRequest{Source: reader, MaximumBytes: maximum})
			if err != nil {
				t.Fatalf("objectstore.Inspect(%d bytes, chunk %d) error = %v, want nil", tc.size, tc.chunk, err)
			}
			wantSHA := core.NewSHA256Digest(sha256.Sum256(payload))
			wantCRC := core.NewCRC32C(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli)))
			if got.Integrity.Length.Uint64() != uint64(tc.size) || got.Integrity.SHA256 != wantSHA || got.Integrity.CRC32C != wantCRC {
				t.Fatalf("objectstore.Inspect() integrity = %+v, want length=%d sha256=%v crc32c=%v", got.Integrity, tc.size, wantSHA, wantCRC)
			}
		})
	}
}

func TestInspectRefusesInvalidSizeAndSourceBoundariesWithTypedErrors(t *testing.T) {
	t.Parallel()

	sourceFailure := errors.New("hostile source failure")
	one, err := core.NewByteCount(1)
	if err != nil {
		t.Fatalf("core.NewByteCount(1) setup error = %v, want nil", err)
	}
	two, err := core.NewByteCount(2)
	if err != nil {
		t.Fatalf("core.NewByteCount(2) setup error = %v, want nil", err)
	}
	cases := []struct {
		request      objectstore.InspectionRequest
		name         string
		wantIdentity error
		wantCause    error
	}{
		{name: "nil source", request: objectstore.InspectionRequest{MaximumBytes: one}, wantIdentity: core.ErrObjectStoreSource},
		{name: "zero maximum", request: objectstore.InspectionRequest{Source: strings.NewReader("x")}, wantIdentity: core.ErrObjectStoreSize},
		{name: "empty source below positive maximum", request: objectstore.InspectionRequest{Source: strings.NewReader(""), MaximumBytes: one}, wantIdentity: core.ErrObjectStoreSize},
		{name: "one byte above maximum", request: objectstore.InspectionRequest{Source: strings.NewReader("xy"), MaximumBytes: one}, wantIdentity: core.ErrObjectStoreSize},
		{name: "many bytes above maximum", request: objectstore.InspectionRequest{Source: strings.NewReader(strings.Repeat("x", 4096)), MaximumBytes: two}, wantIdentity: core.ErrObjectStoreSize},
		{name: "source fails before first byte", request: objectstore.InspectionRequest{Source: &hostileInspectionReader{err: sourceFailure, failAfter: 0}, MaximumBytes: one}, wantIdentity: core.ErrObjectStoreSource, wantCause: sourceFailure},
		{name: "source fails after one byte below maximum", request: objectstore.InspectionRequest{Source: &hostileInspectionReader{data: []byte("x"), err: sourceFailure, failAfter: 1}, MaximumBytes: two}, wantIdentity: core.ErrObjectStoreSource, wantCause: sourceFailure},
		{name: "source fails during oversize probe", request: objectstore.InspectionRequest{Source: &hostileInspectionReader{data: []byte("x"), err: sourceFailure, failAfter: 1}, MaximumBytes: one}, wantIdentity: core.ErrObjectStoreSource, wantCause: sourceFailure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := objectstore.Inspect(t.Context(), tc.request)
			if got != (objectstore.Inspection{}) || !errors.Is(gotErr, core.ErrObjectStoreContract) || !errors.Is(gotErr, tc.wantIdentity) {
				t.Fatalf("objectstore.Inspect(hostile) = (%+v, %v), want zero and errors.Is(_, %v) beneath %v", got, gotErr, tc.wantIdentity, core.ErrObjectStoreContract)
			}
			if tc.wantCause != nil && !errors.Is(gotErr, tc.wantCause) {
				t.Fatalf("objectstore.Inspect(hostile) error = %v, want native cause %v reachable", gotErr, tc.wantCause)
			}
		})
	}
}

func TestInspectCancellationRefusesBeforeReadingSource(t *testing.T) {
	t.Parallel()

	maximum, err := core.NewByteCount(1)
	if err != nil {
		t.Fatalf("core.NewByteCount(1) setup error = %v, want nil", err)
	}
	reader := &hostileInspectionReader{data: []byte("x"), failAfter: -1}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, gotErr := objectstore.Inspect(ctx, objectstore.InspectionRequest{Source: reader, MaximumBytes: maximum})
	if got != (objectstore.Inspection{}) || !errors.Is(gotErr, context.Canceled) || reader.position != 0 {
		t.Fatalf("objectstore.Inspect(cancelled) = (%+v, %v), bytes read=%d; want zero, context.Canceled, and zero bytes read", got, gotErr, reader.position)
	}
}
