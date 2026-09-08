package exchange

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type declaredPrefixReader struct {
	source    *bytes.Reader
	terminal  error
	readBytes int
	offers    []int
}

func (r *declaredPrefixReader) Read(p []byte) (int, error) {
	r.offers = append(r.offers, len(p))
	n, err := r.source.Read(p)
	r.readBytes += n
	if errors.Is(err, io.EOF) && r.terminal != nil {
		return n, r.terminal
	}
	return n, err
}

// These are transfer invariants, not assertions about the number of internal
// chunks. A declaration cannot truncate actual bytes, enlarge the authorization,
// mask a native terminal cause, or make an aggregate publish partial data.
func TestSmallDeclaredBodyTransferLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		declared    int64
		body, limit int
		terminal    error
		cancelled   bool
		wantRead    int
		wantErr     error
	}{
		{name: "exact small declaration cannot pad or omit body", declared: 3, body: 3, limit: 9, wantRead: 3},
		{name: "overstatement remains a reservation rather than fabricated bytes", declared: 4, body: 3, limit: 9, wantRead: 3},
		{name: "one real byte beyond declaration is not overflow", declared: 3, body: 4, limit: 9, wantRead: 4},
		{name: "understatement continues to authorized limit", declared: 3, body: 9, limit: 9, wantRead: 9},
		{name: "understatement cannot conceal first unauthorized byte", declared: 3, body: 10, limit: 9, wantRead: 10, wantErr: core.ErrExchangeBodyLimit},
		{name: "full source remains unread beyond one overflow probe", declared: 3, body: 90, limit: 9, wantRead: 10, wantErr: core.ErrExchangeBodyLimit},
		{name: "empty source invents neither content nor reservation", declared: 3, limit: 9},
		{name: "absent declaration preserves bounded stream behavior", declared: declaredBodyLengthAbsent, body: 9, limit: 9, wantRead: 9},
		{name: "zero declaration cannot discard real input", body: 3, limit: 9, wantRead: 3},
		{name: "native failure below declaration survives EOF classification", declared: 3, body: 2, limit: 9, terminal: io.ErrClosedPipe, wantRead: 2, wantErr: io.ErrClosedPipe},
		{name: "native failure at declaration cannot become success", declared: 3, body: 3, limit: 9, terminal: io.ErrUnexpectedEOF, wantRead: 3, wantErr: io.ErrUnexpectedEOF},
		{name: "native failure at transition cannot disappear between copies", declared: 3, body: 4, limit: 9, terminal: io.ErrClosedPipe, wantRead: 4, wantErr: io.ErrClosedPipe},
		{name: "native failure after transition retains exact identity", declared: 3, body: 5, limit: 9, terminal: io.ErrUnexpectedEOF, wantRead: 5, wantErr: io.ErrUnexpectedEOF},
		{name: "native failure at authorization ceiling is not clean EOF", declared: 3, body: 9, limit: 9, terminal: io.ErrClosedPipe, wantRead: 9, wantErr: io.ErrClosedPipe},
		{name: "overflow is established before unreachable terminal failure", declared: 3, body: 10, limit: 9, terminal: io.ErrClosedPipe, wantRead: 10, wantErr: core.ErrExchangeBodyLimit},
		{name: "pre-cancellation forbids the first read", declared: 3, body: 3, limit: 9, cancelled: true, wantErr: core.ErrExchangeCancelled},
		{name: "declaration plus probe exactly fills authorization", declared: 8, body: 9, limit: 9, wantRead: 9},
		{name: "declaration exactly fills authorization", declared: 9, body: 9, limit: 9, wantRead: 9},
		{name: "small-reservation ceiling minus one retains exact bytes", declared: boundedBodyInitialReservationMaximumBytes - 1, body: boundedBodyInitialReservationMaximumBytes - 1, limit: 2 * boundedBodyInitialReservationMaximumBytes, wantRead: boundedBodyInitialReservationMaximumBytes - 1},
		{name: "small-reservation ceiling retains exact bytes", declared: boundedBodyInitialReservationMaximumBytes, body: boundedBodyInitialReservationMaximumBytes, limit: 2 * boundedBodyInitialReservationMaximumBytes, wantRead: boundedBodyInitialReservationMaximumBytes},
		{name: "large declaration retains normal bounded copy", declared: boundedBodyInitialReservationMaximumBytes + 1, body: boundedBodyInitialReservationMaximumBytes + 1, limit: 2 * boundedBodyInitialReservationMaximumBytes, wantRead: boundedBodyInitialReservationMaximumBytes + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := bytes.Repeat([]byte{0xa5}, tc.body)
			source := &declaredPrefixReader{source: bytes.NewReader(data), terminal: tc.terminal}
			limit := mustInternalByteCount(t, uint64(tc.limit))
			declared, err := admittedBodyLength(tc.declared, limit)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			got, err := readBoundedBody(boundedBodyRead{context: ctx, source: source, declared: declared, limit: limit})
			if !errors.Is(err, tc.wantErr) || source.readBytes != tc.wantRead {
				t.Fatalf("aggregate=(%v,%d read), want (%v,%d)", err, source.readBytes, tc.wantErr, tc.wantRead)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatalf("refused aggregate published %x", got)
				}
				if tc.cancelled && !errors.Is(err, context.Canceled) {
					t.Fatalf("cancelled read error=%v, want %v", err, context.Canceled)
				}
				return
			}
			if !bytes.Equal(got, data) || cap(got) > tc.limit {
				t.Fatalf("aggregate=%x capacity %d, want exact %x within %d", got, cap(got), data, tc.limit)
			}
		})
	}
}

func TestSmallDeclaredBodyBoundsInitialReadOffer(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		bytes            int
		wantMaximumOffer int
	}{
		{name: "minimum positive declaration cannot solicit the full policy extent", bytes: 1, wantMaximumOffer: 2},
		{name: "small JSON-sized declaration cannot solicit a 32 KiB read", bytes: 128, wantMaximumOffer: 129},
		{name: "reservation ceiling retains one-byte understatement detection", bytes: boundedBodyInitialReservationMaximumBytes, wantMaximumOffer: boundedBodyInitialReservationMaximumBytes + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := bytes.Repeat([]byte{0xa5}, tc.bytes)
			source := &declaredPrefixReader{source: bytes.NewReader(data)}
			limit := mustInternalByteCount(t, uint64(16*boundedBodyInitialReservationMaximumBytes))
			declared, err := admittedBodyLength(int64(len(data)), limit)
			if err != nil {
				t.Fatal(err)
			}
			got, err := readBoundedBody(boundedBodyRead{context: t.Context(), source: source, declared: declared, limit: limit})
			if err != nil || !bytes.Equal(got, data) || source.readBytes != len(data) {
				t.Fatalf("declared aggregate=(%x,%v,%d read), want exact bytes", got, err, source.readBytes)
			}
			if len(source.offers) == 0 || source.offers[0] > tc.wantMaximumOffer {
				t.Fatalf("first source read offers=%v, want at most %d bytes before declaration is disproved", source.offers, tc.wantMaximumOffer)
			}
		})
	}
}
