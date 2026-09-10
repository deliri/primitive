package exchange

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// The workload is one ReplayStream -> Download handoff through an in-memory
// Go RoundTripper. It includes context ownership and error/byte accounting,
// but measures no DNS, socket, or real-network latency.
func BenchmarkReplayStreamDownloadHandoff(b *testing.B) {
	b.ReportAllocs()
	cases := []struct {
		name            string
		fault           replayHandoffBodyFault
		cancelAtHandoff bool
		wantNative      error
		wantCancelled   bool
	}{
		{name: "binary_success"},
		{name: "cancelled_native_read_failure", fault: replayHandoffReadFailure, cancelAtHandoff: true, wantNative: io.ErrUnexpectedEOF, wantCancelled: true},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			payload := []byte{0x00, 0xff}
			target, err := core.ParseHTTPEndpoint("https://replay-benchmark.invalid/stream")
			if err != nil {
				b.Fatalf("target setup = %v, want nil", err)
			}
			operation := replayDuration(b, 60*int64(temporal.NanosecondsPerSecond))
			attemptTimeout := replayDuration(b, 30*int64(temporal.NanosecondsPerSecond))
			policy := StreamPolicy{OperationTimeout: operation, AttemptTimeout: attemptTimeout, Redirect: RedirectPolicy{Mode: RedirectReject}}
			replayPolicy := StreamReplayPolicy{OperationTimeout: operation, Retry: RetryPolicy{MaximumAttempts: 1}}
			if err := errors.Join(policy.Validate(), replayPolicy.Validate()); err != nil {
				b.Fatalf("policy setup = %v, want nil", err)
			}
			body := &replayHandoffBody{reader: bytes.NewReader(payload), fault: tc.fault}
			var providerCalls int
			client, err := NewClient(&http.Client{Transport: replayHandoffTransport(func(request *http.Request) (*http.Response, error) {
				providerCalls++
				return &http.Response{StatusCode: http.StatusOK, Body: body, ContentLength: -1, Request: request}, nil
			})})
			if err != nil {
				b.Fatalf("client setup = %v, want nil", err)
			}
			var destination bytes.Buffer
			destination.Grow(len(payload))
			request := DownloadRequest{Target: target, Destination: &destination,
				Semantics:      RequestSemantics{Method: MethodGet, Replay: ReplaySingleAttempt},
				ExpectedStatus: core.HTTPStatusOK()}
			if err := request.Validate(); err != nil {
				b.Fatalf("request setup = %v, want nil", err)
			}
			var last StreamResponse
			var lastErr error
			iterations := 0
			b.ReportAllocs()
			b.SetBytes(int64(len(payload)))
			for b.Loop() {
				body.reader.Reset(payload)
				body.closes, body.readBytes, providerCalls = 0, 0, 0
				destination.Reset()
				ctx, cancel := context.WithCancel(b.Context())
				attempts := 0
				last, lastErr = ReplayStream(StreamReplayCall{Context: ctx, Policy: replayPolicy,
					Attempt: func(attemptContext context.Context, index uint64) (StreamResponse, error) {
						attempts++
						if index != 1 || attempts != 1 {
							b.Fatalf("attempt index/calls = %d/%d, want 1/1", index, attempts)
						}
						produced, cause := Download(DownloadCall{Context: attemptContext, Client: client, Request: request, Policy: policy})
						if tc.cancelAtHandoff {
							cancel()
						}
						return produced, cause
					},
				})
				cancel()
				if !errors.Is(lastErr, tc.wantNative) || errors.Is(lastErr, context.Canceled) != tc.wantCancelled || errors.Is(lastErr, core.ErrExchangeCancelled) != tc.wantCancelled || errors.Is(lastErr, core.ErrExchangeResponse) != (tc.wantNative != nil) || errors.Is(lastErr, core.ErrExchangeRetryExhausted) {
					b.Fatalf("handoff cause = %v, want native %v, cancellation %t and no exhaustion", lastErr, tc.wantNative, tc.wantCancelled)
				}
				if attempts != 1 || providerCalls != 1 || body.closes != 1 || body.readBytes != len(payload) || !bytes.Equal(destination.Bytes(), payload) {
					b.Fatalf("handoff attempts/provider/closes/read/output = %d/%d/%d/%d/%x, want 1/1/1/%d/%x", attempts, providerCalls, body.closes, body.readBytes, destination.Bytes(), len(payload), payload)
				}
				if last.Metadata.Status != core.HTTPStatusOK() || last.Metadata.Attempts != 1 || last.Metadata.Bytes.Uint64() != uint64(len(payload)) || last.Metadata.Headers.Values != nil {
					b.Fatalf("handoff metadata = %+v, want one OK attempt, %d bytes, absent headers", last.Metadata, len(payload))
				}
				iterations++
			}
			if iterations == 0 {
				b.Fatal("handoff iterations = 0, want non-vacuous work")
			}
			if err := last.Validate(); err != nil {
				b.Fatalf("retained observation validation = %v, want nil", err)
			}
		})
	}
}
