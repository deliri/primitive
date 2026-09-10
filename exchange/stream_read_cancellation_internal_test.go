package exchange

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type cancellationReadStep struct {
	payload string
	err     error
	cancel  bool
}

type cancellationStepReader struct {
	steps     []cancellationReadStep
	cancel    context.CancelFunc
	calls     int
	readBytes int
}

func (r *cancellationStepReader) Read(p []byte) (int, error) {
	index := r.calls
	r.calls++
	if index >= len(r.steps) {
		return 0, io.EOF
	}
	step := r.steps[index]
	if len(step.payload) > len(p) {
		panic("fixture step exceeds the requested read buffer")
	}
	n := copy(p, step.payload)
	r.readBytes += n
	if step.cancel {
		r.cancel()
	}
	return n, step.err
}

func TestStreamReadCancellationLayerTriad(t *testing.T) {
	t.Parallel()
	// Every row pins a distinct read/error/probe ordering. In particular, an
	// observed excess byte is stronger evidence than cancellation after Read.
	cases := []struct {
		name          string
		steps         []cancellationReadStep
		limit         uint64
		cancelBefore  bool
		wantErr       error
		wantCancelled bool
		wantBody      string
		wantWritten   uint64
		wantReadBytes int
		wantReadCalls int
	}{
		{name: "cancelled before reading cannot consume a byte", steps: []cancellationReadStep{{payload: "a"}}, limit: 2, cancelBefore: true, wantErr: context.Canceled, wantCancelled: true},
		{name: "cancel after a short successful read retains its byte", steps: []cancellationReadStep{{payload: "a", cancel: true}}, limit: 2, wantErr: context.Canceled, wantCancelled: true, wantBody: "a", wantWritten: 1, wantReadBytes: 1, wantReadCalls: 1},
		{name: "cancel after the exact limit retains the complete write", steps: []cancellationReadStep{{payload: "ab", cancel: true}}, limit: 2, wantErr: context.Canceled, wantCancelled: true, wantBody: "ab", wantWritten: 2, wantReadBytes: 2, wantReadCalls: 1},
		{name: "cancel during excess probe cannot hide the excess byte", steps: []cancellationReadStep{{payload: "ab"}, {payload: "c", cancel: true}}, limit: 2, wantErr: core.ErrExchangeBodyLimit, wantBody: "ab", wantWritten: 2, wantReadBytes: 3, wantReadCalls: 2},
		{name: "cancel plus native failure retains returned data and native identity", steps: []cancellationReadStep{{payload: "a", err: io.ErrUnexpectedEOF, cancel: true}}, limit: 2, wantErr: io.ErrUnexpectedEOF, wantBody: "a", wantWritten: 1, wantReadBytes: 1, wantReadCalls: 1},
		{name: "cancel plus EOF preserves data before reporting cancellation", steps: []cancellationReadStep{{payload: "a", err: io.EOF, cancel: true}}, limit: 2, wantErr: context.Canceled, wantCancelled: true, wantBody: "a", wantWritten: 1, wantReadBytes: 1, wantReadCalls: 1},
		{name: "empty cancelled read cannot invent progress", steps: []cancellationReadStep{{cancel: true}}, limit: 2, wantErr: context.Canceled, wantCancelled: true, wantReadCalls: 1},
		{name: "later cancellation cannot erase an earlier acknowledged chunk", steps: []cancellationReadStep{{payload: "a"}, {payload: "b", cancel: true}}, limit: 3, wantErr: context.Canceled, wantCancelled: true, wantBody: "ab", wantWritten: 2, wantReadBytes: 2, wantReadCalls: 2},
		{name: "EOF carrying the final byte remains a successful transfer", steps: []cancellationReadStep{{payload: "a", err: io.EOF}}, limit: 2, wantBody: "a", wantWritten: 1, wantReadBytes: 1, wantReadCalls: 1},
		{name: "an empty EOF creates no destination evidence", steps: []cancellationReadStep{{err: io.EOF}}, limit: 2, wantReadCalls: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelBefore {
				cancel()
			}
			limit, err := core.NewByteCount(tc.limit)
			if err != nil {
				t.Fatalf("limit setup error = %v, want nil", err)
			}
			source := &cancellationStepReader{steps: tc.steps, cancel: cancel}
			destination := &retainingWriter{}
			written, gotErr := copyDownload(downloadCopyRequest{context: ctx, source: source, destination: destination, limit: &limit})
			if !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrExchangeCancelled) != tc.wantCancelled {
				t.Fatalf("copy error/cancellation = (%v, %t), want (%v, %t)", gotErr, errors.Is(gotErr, core.ErrExchangeCancelled), tc.wantErr, tc.wantCancelled)
			}
			if written != tc.wantWritten || destination.written.String() != tc.wantBody || source.readBytes != tc.wantReadBytes || source.calls != tc.wantReadCalls {
				t.Fatalf("acknowledged bytes/body/consumed bytes/read calls = (%d, %q, %d, %d), want (%d, %q, %d, %d)", written, destination.written.String(), source.readBytes, source.calls, tc.wantWritten, tc.wantBody, tc.wantReadBytes, tc.wantReadCalls)
			}
		})
	}
}
