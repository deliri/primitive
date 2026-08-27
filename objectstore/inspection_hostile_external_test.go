package objectstore_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"io"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/zeebo/blake3"
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
			wantBLAKE3 := objectstore.NewBLAKE3Digest(blake3.Sum256(payload))
			wantCRC := core.NewCRC32C(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli)))
			if got.Integrity.Length.Uint64() != uint64(tc.size) || got.Integrity.SHA256 != wantSHA || got.BLAKE3 != wantBLAKE3 || got.Integrity.CRC32C != wantCRC {
				t.Fatalf("objectstore.Inspect() = %+v, want length=%d sha256=%v blake3=%v crc32c=%v", got, tc.size, wantSHA, wantBLAKE3, wantCRC)
			}
		})
	}
}

func TestInspectionContentIdentityLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact source produces the official BLAKE3 content identity", func(t *testing.T) {
		t.Parallel()

		maximum, setupErr := core.NewByteCount(3)
		if setupErr != nil {
			t.Fatalf("core.NewByteCount(3) setup error = %v, want nil", setupErr)
		}
		got, gotErr := objectstore.Inspect(t.Context(), objectstore.InspectionRequest{
			Source: strings.NewReader("abc"), MaximumBytes: maximum,
		})
		want := mustBLAKE3Digest(t, "6437b3ac38465133ffb63b75273a8db548c558465d79db03fd359c6cd5bd9d85")
		if gotErr != nil || got.BLAKE3 != want {
			t.Fatalf("objectstore.Inspect(official vector) = (%v, %v), want BLAKE3 %v and nil", got.BLAKE3, gotErr, want)
		}
	})

	t.Run("negative oversized source returns no partial content identity", func(t *testing.T) {
		t.Parallel()

		maximum, setupErr := core.NewByteCount(2)
		if setupErr != nil {
			t.Fatalf("core.NewByteCount(2) setup error = %v, want nil", setupErr)
		}
		got, gotErr := objectstore.Inspect(t.Context(), objectstore.InspectionRequest{
			Source: strings.NewReader("abc"), MaximumBytes: maximum,
		})
		if got != (objectstore.Inspection{}) || !errors.Is(gotErr, core.ErrObjectStoreSize) {
			t.Fatalf("objectstore.Inspect(oversized) = (%+v, %v), want zero and %v", got, gotErr, core.ErrObjectStoreSize)
		}
	})

	t.Run("neutral empty source returns no invented content identity", func(t *testing.T) {
		t.Parallel()

		maximum, setupErr := core.NewByteCount(1)
		if setupErr != nil {
			t.Fatalf("core.NewByteCount(1) setup error = %v, want nil", setupErr)
		}
		got, gotErr := objectstore.Inspect(t.Context(), objectstore.InspectionRequest{
			Source: strings.NewReader(""), MaximumBytes: maximum,
		})
		if got != (objectstore.Inspection{}) || !errors.Is(gotErr, core.ErrObjectStoreSize) {
			t.Fatalf("objectstore.Inspect(empty) = (%+v, %v), want zero and %v", got, gotErr, core.ErrObjectStoreSize)
		}
	})
}

func mustBLAKE3Digest(t testing.TB, value string) objectstore.BLAKE3Digest {
	t.Helper()

	raw, err := hex.DecodeString(value)
	if err != nil || len(raw) != objectstore.BLAKE3DigestBytes {
		t.Fatalf("BLAKE3 test vector decode = (%d bytes, %v), want (%d, nil)", len(raw), err, objectstore.BLAKE3DigestBytes)
	}
	var digest [objectstore.BLAKE3DigestBytes]byte
	copy(digest[:], raw)
	return objectstore.NewBLAKE3Digest(digest)
}

func TestInspectRefusesInvalidSizeAndSourceBoundariesWithTypedErrors(t *testing.T) {
	t.Parallel()

	sourceFailure := errors.New("hostile source failure")
	one := mustInspectionByteCount(t, 1)
	two := mustInspectionByteCount(t, 2)
	thirtyOne := mustInspectionByteCount(t, 31)
	thirtyTwo := mustInspectionByteCount(t, 32)
	bufferMinusOne := mustInspectionByteCount(t, 32767)
	bufferExact := mustInspectionByteCount(t, 32768)
	maximumUint64 := mustInspectionByteCount(t, math.MaxUint64)
	cases := []struct {
		wantIdentity error
		wantCause    error
		request      objectstore.InspectionRequest
		context      func(testing.TB) context.Context
		name         string
	}{
		{name: "nil source", request: objectstore.InspectionRequest{MaximumBytes: one}, wantIdentity: core.ErrObjectStoreSource},
		{name: "nil source at a larger maximum", request: objectstore.InspectionRequest{MaximumBytes: bufferExact}, wantIdentity: core.ErrObjectStoreSource},
		{name: "unset maximum rejects one source byte", request: objectstore.InspectionRequest{Source: strings.NewReader("x")}, wantIdentity: core.ErrObjectStoreSize},
		{name: "unset maximum rejects an empty source", request: objectstore.InspectionRequest{Source: strings.NewReader("")}, wantIdentity: core.ErrObjectStoreSize},
		{name: "maximum above signed reader extent is rejected before reading", request: objectstore.InspectionRequest{Source: strings.NewReader("x"), MaximumBytes: maximumUint64}, wantIdentity: core.ErrObjectStoreSize},
		{name: "empty source below one-byte maximum", request: objectstore.InspectionRequest{Source: strings.NewReader(""), MaximumBytes: one}, wantIdentity: core.ErrObjectStoreSize},
		{name: "empty source below two-byte maximum", request: objectstore.InspectionRequest{Source: strings.NewReader(""), MaximumBytes: two}, wantIdentity: core.ErrObjectStoreSize},
		{name: "empty source below buffer maximum", request: objectstore.InspectionRequest{Source: strings.NewReader(""), MaximumBytes: bufferExact}, wantIdentity: core.ErrObjectStoreSize},
		{name: "one byte above one-byte maximum", request: objectstore.InspectionRequest{Source: strings.NewReader("xy"), MaximumBytes: one}, wantIdentity: core.ErrObjectStoreSize},
		{name: "two bytes above one-byte maximum", request: objectstore.InspectionRequest{Source: strings.NewReader("xyz"), MaximumBytes: one}, wantIdentity: core.ErrObjectStoreSize},
		{name: "one byte above two-byte maximum", request: objectstore.InspectionRequest{Source: strings.NewReader("xyz"), MaximumBytes: two}, wantIdentity: core.ErrObjectStoreSize},
		{name: "one byte above thirty-one-byte maximum", request: objectstore.InspectionRequest{Source: strings.NewReader(strings.Repeat("x", 32)), MaximumBytes: thirtyOne}, wantIdentity: core.ErrObjectStoreSize},
		{name: "one byte above thirty-two-byte maximum", request: objectstore.InspectionRequest{Source: strings.NewReader(strings.Repeat("x", 33)), MaximumBytes: thirtyTwo}, wantIdentity: core.ErrObjectStoreSize},
		{name: "one byte above buffer-minus-one maximum", request: objectstore.InspectionRequest{Source: strings.NewReader(strings.Repeat("x", 32768)), MaximumBytes: bufferMinusOne}, wantIdentity: core.ErrObjectStoreSize},
		{name: "one byte above exact buffer maximum", request: objectstore.InspectionRequest{Source: strings.NewReader(strings.Repeat("x", 32769)), MaximumBytes: bufferExact}, wantIdentity: core.ErrObjectStoreSize},
		{name: "two bytes above exact buffer maximum", request: objectstore.InspectionRequest{Source: strings.NewReader(strings.Repeat("x", 32770)), MaximumBytes: bufferExact}, wantIdentity: core.ErrObjectStoreSize},
		{name: "two buffers exceed one-buffer maximum", request: objectstore.InspectionRequest{Source: strings.NewReader(strings.Repeat("x", 65536)), MaximumBytes: bufferExact}, wantIdentity: core.ErrObjectStoreSize},
		{name: "many bytes exceed a two-byte maximum", request: objectstore.InspectionRequest{Source: strings.NewReader(strings.Repeat("x", 4096)), MaximumBytes: two}, wantIdentity: core.ErrObjectStoreSize},
		{name: "source fails before first byte", request: objectstore.InspectionRequest{Source: &hostileInspectionReader{err: sourceFailure, failAfter: 0}, MaximumBytes: one}, wantIdentity: core.ErrObjectStoreSource, wantCause: sourceFailure},
		{name: "source fails after one byte below maximum", request: objectstore.InspectionRequest{Source: &hostileInspectionReader{data: []byte("x"), err: sourceFailure, failAfter: 1}, MaximumBytes: two}, wantIdentity: core.ErrObjectStoreSource, wantCause: sourceFailure},
		{name: "source fails after thirty-one bytes below maximum", request: objectstore.InspectionRequest{Source: &hostileInspectionReader{data: bytes.Repeat([]byte("x"), 31), err: sourceFailure, failAfter: 31}, MaximumBytes: thirtyTwo}, wantIdentity: core.ErrObjectStoreSource, wantCause: sourceFailure},
		{name: "source fails one byte below buffer maximum", request: objectstore.InspectionRequest{Source: &hostileInspectionReader{data: bytes.Repeat([]byte("x"), 32767), err: sourceFailure, failAfter: 32767}, MaximumBytes: bufferExact}, wantIdentity: core.ErrObjectStoreSource, wantCause: sourceFailure},
		{name: "source fails during oversize probe", request: objectstore.InspectionRequest{Source: &hostileInspectionReader{data: []byte("x"), err: sourceFailure, failAfter: 1}, MaximumBytes: one}, wantIdentity: core.ErrObjectStoreSource, wantCause: sourceFailure},
		{name: "source fails during exact-buffer oversize probe", request: objectstore.InspectionRequest{Source: &hostileInspectionReader{data: bytes.Repeat([]byte("x"), 32768), err: sourceFailure, failAfter: 32768}, MaximumBytes: bufferExact}, wantIdentity: core.ErrObjectStoreSource, wantCause: sourceFailure},
		{name: "unexpected EOF remains a native source cause", request: objectstore.InspectionRequest{Source: &hostileInspectionReader{err: io.ErrUnexpectedEOF, failAfter: 0}, MaximumBytes: one}, wantIdentity: core.ErrObjectStoreSource, wantCause: io.ErrUnexpectedEOF},
		{name: "closed pipe remains a native source cause", request: objectstore.InspectionRequest{Source: &hostileInspectionReader{err: io.ErrClosedPipe, failAfter: 0}, MaximumBytes: one}, wantIdentity: core.ErrObjectStoreSource, wantCause: io.ErrClosedPipe},
		{name: "source cancellation remains a native source cause", request: objectstore.InspectionRequest{Source: &hostileInspectionReader{err: context.Canceled, failAfter: 0}, MaximumBytes: one}, wantIdentity: core.ErrObjectStoreSource, wantCause: context.Canceled},
		{name: "source deadline remains a native source cause", request: objectstore.InspectionRequest{Source: &hostileInspectionReader{err: context.DeadlineExceeded, failAfter: 0}, MaximumBytes: one}, wantIdentity: core.ErrObjectStoreSource, wantCause: context.DeadlineExceeded},
		{name: "cancelled context refuses before source access", request: objectstore.InspectionRequest{Source: strings.NewReader("x"), MaximumBytes: one}, context: canceledInspectionContext, wantIdentity: context.Canceled},
		{name: "expired deadline refuses before source access", request: objectstore.InspectionRequest{Source: strings.NewReader("x"), MaximumBytes: one}, context: expiredInspectionContext, wantIdentity: context.DeadlineExceeded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			if tc.context != nil {
				ctx = tc.context(t)
			}
			got, gotErr := objectstore.Inspect(ctx, tc.request)
			if got != (objectstore.Inspection{}) || !errors.Is(gotErr, core.ErrObjectStoreContract) || !errors.Is(gotErr, tc.wantIdentity) {
				t.Fatalf("objectstore.Inspect(hostile) = (%+v, %v), want zero and errors.Is(_, %v) beneath %v", got, gotErr, tc.wantIdentity, core.ErrObjectStoreContract)
			}
			if tc.wantCause != nil && !errors.Is(gotErr, tc.wantCause) {
				t.Fatalf("objectstore.Inspect(hostile) error = %v, want native cause %v reachable", gotErr, tc.wantCause)
			}
		})
	}
}

func mustInspectionByteCount(t testing.TB, value uint64) core.ByteCount {
	t.Helper()

	got, gotErr := core.NewByteCount(value)
	if gotErr != nil {
		t.Fatalf("core.NewByteCount(%d) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func canceledInspectionContext(testing.TB) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func expiredInspectionContext(t testing.TB) context.Context {
	t.Helper()

	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	t.Cleanup(cancel)
	return ctx
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
