package filestore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type contextHandoffSource struct {
	data   []byte
	cancel context.CancelFunc
	reads  int
}

func (s *contextHandoffSource) Read(buffer []byte) (int, error) {
	s.reads++
	if len(s.data) == 0 {
		return 0, io.EOF
	}
	n := copy(buffer, s.data[:1])
	s.data = s.data[n:]
	if s.cancel != nil {
		s.cancel()
	}
	return n, nil
}

type contextHandoffDestination struct {
	bytes  bytes.Buffer
	cancel context.CancelFunc
	writes int
}

func (w *contextHandoffDestination) Write(buffer []byte) (int, error) {
	w.writes++
	n, err := w.bytes.Write(buffer)
	if w.cancel != nil {
		w.cancel()
	}
	return n, err
}

// Direct bounded-copy ratchet: cancellation occurs at exact reader/writer
// handoffs, never by scheduler timing. Public stream tests cover file effects.
func TestBoundedCopyContextHandoffLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                          string
		payload, want                                 []byte
		maximum                                       uint64
		nilContext, beforeRead, afterRead, afterWrite bool
		wantErr                                       error
		wantReads, wantWrites                         int
	}{
		{name: "active stream retains exact binary bytes through every handoff", payload: []byte{0, 255, 1}, maximum: 4, want: []byte{0, 255, 1}, wantReads: 4, wantWrites: 3},
		{name: "empty stream observes EOF without calling destination", maximum: 1, wantReads: 1},
		{name: "cancellation before first read cannot touch source", payload: []byte{0, 255, 1}, maximum: 4, beforeRead: true, wantErr: context.Canceled},
		{name: "nil context refuses without reading caller material", payload: []byte{0, 255, 1}, maximum: 4, nilContext: true, wantErr: core.ErrNilContext},
		{name: "destination cancellation prevents next source read", payload: []byte{0, 255, 1}, maximum: 4, afterWrite: true, want: []byte{0}, wantErr: context.Canceled, wantReads: 1, wantWrites: 1},
		{name: "cancellation at byte ceiling prevents overflow probe consumption", payload: []byte{0, 255, 1}, maximum: 1, afterWrite: true, want: []byte{0}, wantErr: context.Canceled, wantReads: 1, wantWrites: 1},
		{name: "cancellation during read preserves returned byte count and prefix", payload: []byte{0, 255, 1}, maximum: 4, afterRead: true, want: []byte{0}, wantErr: context.Canceled, wantReads: 1, wantWrites: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.beforeRead {
				cancel()
			}
			if tc.nilContext {
				ctx = nil
			}
			source := contextHandoffSource{data: bytes.Clone(tc.payload)}
			destination := contextHandoffDestination{}
			if tc.afterRead {
				source.cancel = cancel
			}
			if tc.afterWrite {
				destination.cancel = cancel
			}
			maximum, err := core.NewByteCount(tc.maximum)
			if err != nil {
				t.Fatal(err)
			}
			got, gotErr := copyBounded(boundedCopyRequest{ctx: ctx, source: &source, destination: &destination, maximum: maximum, kind: streamDestinationCaller})
			if !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) || errors.Is(gotErr, core.ErrFilestoreDestination) || errors.Is(gotErr, core.ErrFilestoreSize) {
				t.Fatalf("copy error = %v, want exact %v without source/destination/size classification", gotErr, tc.wantErr)
			}
			if got.Uint64() != uint64(len(tc.want)) || !bytes.Equal(destination.bytes.Bytes(), tc.want) || source.reads != tc.wantReads || destination.writes != tc.wantWrites {
				t.Fatalf("receipt/bytes/calls = (%d,%v,%d,%d), want (%d,%v,%d,%d)", got.Uint64(), destination.bytes.Bytes(), source.reads, destination.writes, len(tc.want), tc.want, tc.wantReads, tc.wantWrites)
			}
			if !bytes.Equal(source.data, tc.payload[len(tc.want):]) {
				t.Fatalf("unconsumed source = %v, want %v", source.data, tc.payload[len(tc.want):])
			}
		})
	}
}
