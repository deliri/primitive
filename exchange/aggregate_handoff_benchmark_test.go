package exchange

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// One real aggregate attempt and its classifier handoff, through Go's client
// with an in-memory transport. This measures bounded processing, not latency
// across a real network. Each iteration owns a fresh body and context.
func BenchmarkAggregateCompletedAttemptHandoff(b *testing.B) {
	b.ReportAllocs()
	cases := []struct {
		name            string
		fault           replayHandoffBodyFault
		cancel          bool
		wantProducerErr error
		wantErr         error
	}{
		{name: "binary_complete"},
		{name: "cancel_after_native_close", fault: replayHandoffCloseFailure, cancel: true, wantProducerErr: io.ErrClosedPipe, wantErr: context.Canceled},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			payload := []byte{0, 0xff}
			limit, err := core.NewByteCount(uint64(len(payload)))
			if err != nil {
				b.Fatalf("body ceiling fixture = %v, want nil", err)
			}
			target, err := core.ParseHTTPEndpoint("https://provider.example.test/aggregate")
			if err != nil {
				b.Fatalf("target fixture = %v, want nil", err)
			}
			semantics := RequestSemantics{Method: MethodGet, Replay: ReplaySafe}
			if err := semantics.Validate(); err != nil {
				b.Fatalf("semantics fixture = %v, want nil", err)
			}
			timeout := runtimeAgreementPolicy(b).ReadTimeout
			request := aggregateRequest{target: target, semantics: semantics, expectedStatus: core.HTTPStatusOK()}
			var body *replayHandoffBody
			calls := 0
			client := &http.Client{Transport: replayHandoffTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				body = &replayHandoffBody{reader: bytes.NewReader(payload), fault: tc.fault}
				return &http.Response{StatusCode: http.StatusOK, ContentLength: -1, Request: request, Body: body}, nil
			})}
			b.ReportAllocs()
			for b.Loop() {
				ctx, cancel := context.WithCancel(b.Context())
				before := calls
				produced, producerErr := executeAggregateAttempt(aggregateAttempt{context: ctx, client: client, request: request, timeout: timeout, limit: limit})
				if !errors.Is(producerErr, tc.wantProducerErr) || produced.status != core.HTTPStatusOK() || !bytes.Equal(produced.body, payload) || len(produced.headers.Values) != 0 || produced.retryAfter != "" || calls != before+1 || body == nil || body.readBytes != len(payload) || body.closes != 1 {
					cancel()
					b.Fatalf("producer observation = (%v,%x,%v), want exact binary response, native cause, and one read/close", produced.status, produced.body, producerErr)
				}
				if tc.cancel {
					cancel()
				}
				class, err := classifyAggregateAttempt(aggregateAttemptResult{operationContext: ctx, cause: producerErr, semantics: semantics, response: produced, expected: core.HTTPStatusOK(), attemptsRemaining: 1})
				cancel()
				if class != attemptComplete || !errors.Is(err, tc.wantErr) || (tc.wantProducerErr != nil && !errors.Is(err, tc.wantProducerErr)) || !bytes.Equal(produced.body, payload) {
					b.Fatalf("completed handoff = (%v,%v), want complete with %v and retained %v", class, err, tc.wantErr, tc.wantProducerErr)
				}
			}
		})
	}
}
