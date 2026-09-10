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
}

func (r *declaredPrefixReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	r.readBytes += n
	if errors.Is(err, io.EOF) && r.terminal != nil {
		return n, r.terminal
	}
	return n, err
}

// These are transfer invariants, not assertions about the number of internal
// chunks. A declaration cannot truncate actual bytes, alter actual content,
// mask a native terminal cause, or make an aggregate publish partial data.
func TestSmallDeclaredBodyTransferLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		declared  int64
		body      int
		terminal  error
		cancelled bool
		wantRead  int
		wantErr   error
	}{
		{name: "exact small declaration cannot pad or omit body", declared: 3, body: 3, wantRead: 3},
		{name: "overstatement remains a reservation rather than fabricated bytes", declared: 4, body: 3, wantRead: 3},
		{name: "one real byte beyond declaration is not overflow", declared: 3, body: 4, wantRead: 4},
		{name: "understatement continues to complete source", declared: 3, body: 9, wantRead: 9},
		{name: "understatement cannot discard bytes beyond former cutoff", declared: 3, body: 10, wantRead: 10},
		{name: "full source is read through EOF", declared: 3, body: 90, wantRead: 90},
		{name: "empty source invents neither content nor reservation", declared: 3},
		{name: "absent declaration preserves complete stream behavior", declared: declaredBodyLengthAbsent, body: 9, wantRead: 9},
		{name: "zero declaration cannot discard real input", body: 3, wantRead: 3},
		{name: "native failure below declaration survives EOF classification", declared: 3, body: 2, terminal: io.ErrClosedPipe, wantRead: 2, wantErr: io.ErrClosedPipe},
		{name: "native failure at declaration cannot become success", declared: 3, body: 3, terminal: io.ErrUnexpectedEOF, wantRead: 3, wantErr: io.ErrUnexpectedEOF},
		{name: "native failure at transition cannot disappear between copies", declared: 3, body: 4, terminal: io.ErrClosedPipe, wantRead: 4, wantErr: io.ErrClosedPipe},
		{name: "native failure after transition retains exact identity", declared: 3, body: 5, terminal: io.ErrUnexpectedEOF, wantRead: 5, wantErr: io.ErrUnexpectedEOF},
		{name: "native failure at authorization ceiling is not clean EOF", declared: 3, body: 9, terminal: io.ErrClosedPipe, wantRead: 9, wantErr: io.ErrClosedPipe},
		{name: "late native failure cannot be hidden by a former quota", declared: 3, body: 10, terminal: io.ErrClosedPipe, wantRead: 10, wantErr: io.ErrClosedPipe},
		{name: "pre-cancellation forbids the first read", declared: 3, body: 3, cancelled: true, wantErr: core.ErrExchangeCancelled},
		{name: "declaration plus probe exactly fills authorization", declared: 8, body: 9, wantRead: 9},
		{name: "declaration exactly fills authorization", declared: 9, body: 9, wantRead: 9},
		{name: "small-reservation ceiling minus one retains exact bytes", declared: TransferBufferBytes - 1, body: TransferBufferBytes - 1, wantRead: TransferBufferBytes - 1},
		{name: "small-reservation ceiling retains exact bytes", declared: TransferBufferBytes, body: TransferBufferBytes, wantRead: TransferBufferBytes},
		{name: "large declaration retains normal bounded copy", declared: TransferBufferBytes + 1, body: TransferBufferBytes + 1, wantRead: TransferBufferBytes + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := bytes.Repeat([]byte{0xa5}, tc.body)
			source := &declaredPrefixReader{source: bytes.NewReader(data), terminal: tc.terminal}
			declared, err := parseDeclaredBodyLength(tc.declared)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			got, err := readWholeBody(wholeBodyRead{context: ctx, source: source, declared: declared})
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
			if !bytes.Equal(got, data) {
				t.Fatalf("aggregate=%x capacity %d, want exact %x ", got, cap(got), data)
			}
		})
	}
}
