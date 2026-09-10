package exchange

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type streamingReviewBody struct {
	remaining     uint64
	terminal      error
	closes, reads int
}

func (r *streamingReviewBody) Read(p []byte) (int, error) {
	r.reads++
	if r.remaining == 0 {
		if r.terminal != nil {
			return 0, r.terminal
		}
		return 0, io.EOF
	}
	n := min(uint64(len(p)), r.remaining)
	for i := range int(n) {
		p[i] = 0xa5
	}
	r.remaining -= n
	return int(n), nil
}
func (r *streamingReviewBody) Close() error { r.closes++; return nil }

type streamingReviewSink struct {
	bytes    uint64
	writeErr error
}

func (w *streamingReviewSink) Write(p []byte) (int, error) {
	for _, value := range p {
		if value != 0xa5 {
			return 0, core.ErrExchangeContract
		}
	}
	w.bytes += uint64(len(p))
	return len(p), w.writeErr
}

func TestDownloadUncappedExtentLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                        string
		size                        uint64
		declared                    int64
		terminal, writeErr, wantErr error
		canceled                    bool
		wantBytes                   uint64
	}{
		{name: "unknown length below the Go window completes", size: 32767, declared: -1, wantBytes: 32767},
		{name: "unknown length at the Go window completes", size: 32768, declared: -1, wantBytes: 32768},
		{name: "unknown length above the Go window continues", size: 32769, declared: -1, wantBytes: 32769},
		{name: "64 MiB generated stream needs no transfer quota", size: 64 << 20, declared: -1, wantBytes: 64 << 20},
		{name: "declared length does not require a second acceptance quota", size: 32769, declared: 32769, wantBytes: 32769},
		{name: "empty EOF produces zero-byte completion", declared: 0},
		{name: "native read failure preserves the accepted prefix", size: 1, declared: -1, terminal: io.ErrUnexpectedEOF, wantErr: io.ErrUnexpectedEOF, wantBytes: 1},
		{name: "native write failure retains acknowledged bytes", size: 1, declared: -1, writeErr: io.ErrClosedPipe, wantErr: io.ErrClosedPipe, wantBytes: 1},
		{name: "unrepresentable declared length is refused before copying", size: 1, declared: -2, wantErr: core.ErrExchangeContract},
		{name: "cancellation never reaches transport", size: 1, declared: -1, canceled: true, wantErr: context.Canceled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := &streamingReviewBody{remaining: tc.size, terminal: tc.terminal}
			sink := &streamingReviewSink{writeErr: tc.writeErr}
			calls := 0
			client, err := NewClient(&http.Client{Transport: replayHandoffTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: http.StatusOK, Body: body, ContentLength: tc.declared, Request: request}, nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			target, err := core.ParseHTTPEndpoint("https://stream-review.invalid/body")
			if err != nil {
				t.Fatal(err)
			}
			timeout := runtimeAgreementPolicy(t).ReadTimeout
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			got, err := Download(DownloadCall{Context: ctx, Client: client, Request: DownloadRequest{
				Target: target, Destination: sink, Semantics: RequestSemantics{Method: MethodGet, Replay: ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK(),
			}, Policy: StreamPolicy{OperationTimeout: timeout, AttemptTimeout: timeout, Redirect: RedirectPolicy{Mode: RedirectReject}}})
			if !errors.Is(err, tc.wantErr) || got.Metadata.Bytes.Uint64() != tc.wantBytes || sink.bytes != tc.wantBytes {
				t.Fatalf("download = %+v/%v, sink=%d; want %d bytes and %v", got, err, sink.bytes, tc.wantBytes, tc.wantErr)
			}
			wantCalls := 1
			if tc.canceled {
				wantCalls = 0
			}
			if calls != wantCalls || body.closes != wantCalls {
				t.Fatalf("transport/close = %d/%d, want %d/%d", calls, body.closes, wantCalls, wantCalls)
			}
			if tc.wantErr == nil && (got.Validate() != nil || body.remaining != 0 || got.Metadata.Attempts != 1 || got.Metadata.Status != core.HTTPStatusOK()) {
				t.Fatalf("completed download = %+v, remaining=%d; want exact valid completion", got, body.remaining)
			}
		})
	}
}

type reviewReaderFromPanic struct {
	calls, writes int
	consume       bool
	consumed      int
}

func (w *reviewReaderFromPanic) Write([]byte) (int, error) { w.writes++; return 0, io.ErrClosedPipe }
func (w *reviewReaderFromPanic) ReadFrom(r io.Reader) (int64, error) {
	w.calls++
	if w.consume {
		var one [1]byte
		n, err := r.Read(one[:])
		w.consumed = n
		if err != nil {
			return int64(n), err
		}
	}
	panic(io.ErrClosedPipe)
}

func TestReaderFromPanicRetainsOnlyReturnedAcknowledgmentsLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		consume      bool
		wantConsumed int
	}{
		{name: "panic before any effect cannot fabricate bytes"},
		{name: "panic after an unacknowledged effect still has no returned count", consume: true, wantConsumed: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			destination := &reviewReaderFromPanic{consume: tc.consume}
			limit, err := core.NewByteCount(2)
			if err != nil {
				t.Fatal(err)
			}
			got, err := copyDownload(downloadCopyRequest{context: t.Context(), source: &streamingReviewBody{remaining: 2}, destination: destination, limit: &limit})
			if got != 0 || !errors.Is(err, core.ErrExchangeContract) || destination.calls != 1 || destination.writes != 0 || destination.consumed != tc.wantConsumed {
				t.Fatalf("copy=%d/%v, ReadFrom/Write/consumed=%d/%d/%d; want 0/contract, 1/0/%d", got, err, destination.calls, destination.writes, destination.consumed, tc.wantConsumed)
			}
		})
	}
}
