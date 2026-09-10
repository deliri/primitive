package exchange

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReceiveStreamUncappedExtentLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                        string
		size                        uint64
		declared                    int64
		terminal, writeErr, wantErr error
		canceled, nilDestination    bool
		wantBytes                   uint64
	}{
		{name: "below copy window", size: 32767, declared: -1, wantBytes: 32767},
		{name: "at copy window", size: 32768, declared: -1, wantBytes: 32768},
		{name: "above copy window", size: 32769, declared: -1, wantBytes: 32769},
		{name: "64 MiB without an acceptance quota", size: 64 << 20, declared: -1, wantBytes: 64 << 20},
		{name: "declared extent streams completely", size: 32769, declared: 32769, wantBytes: 32769},
		{name: "empty EOF"},
		{name: "native read failure preserves prefix", size: 1, declared: -1, terminal: io.ErrUnexpectedEOF, wantErr: io.ErrUnexpectedEOF, wantBytes: 1},
		{name: "native write failure preserves acknowledgment", size: 1, declared: -1, writeErr: io.ErrClosedPipe, wantErr: io.ErrClosedPipe, wantBytes: 1},
		{name: "invalid declaration has no destination effect", size: 1, declared: -2, wantErr: core.ErrExchangeContract},
		{name: "cancellation has no read effect", size: 1, declared: -1, canceled: true, wantErr: context.Canceled},
		{name: "typed nil destination refuses before read", size: 1, declared: -1, nilDestination: true, wantErr: core.ErrExchangeContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := &streamingReviewBody{remaining: tc.size, terminal: tc.terminal}
			sink := &streamingReviewSink{writeErr: tc.writeErr}
			var destination io.Writer = sink
			if tc.nilDestination {
				destination = (*streamingReviewSink)(nil)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", body)
			request.ContentLength = tc.declared
			request.Header.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
			output := httptest.NewRecorder()
			call, err := NewSocketServerCall(output, request)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ReceiveStream(StreamReceiveCall{Call: call, Destination: destination, Route: RouteSemantics{Method: MethodPost, Replay: ReplaySingleAttempt}, ExpectedContentType: core.HTTPMediaTypeOctetStream()})
			if !errors.Is(err, tc.wantErr) || errors.Is(err, core.ErrExchangeBodyLimit) || got.Bytes.Uint64() != tc.wantBytes || sink.bytes != tc.wantBytes || body.closes != 1 {
				t.Fatalf("receive=%+v/%v sink=%d close=%d; want %d/%v and one close", got, err, sink.bytes, body.closes, tc.wantBytes, tc.wantErr)
			}
			if (tc.canceled || tc.nilDestination || tc.declared < -1) && body.reads != 0 {
				t.Fatalf("refused ingress performed %d reads", body.reads)
			}
			if tc.wantErr == nil && (got.Validate() != nil || body.remaining != 0) {
				t.Fatalf("completion=%+v remaining=%d", got, body.remaining)
			}
			if output.Body.Len() != 0 || len(output.Header()) != 0 {
				t.Fatalf("receive response body/header = %d/%v, want no effects", output.Body.Len(), output.Header())
			}
		})
	}
}
