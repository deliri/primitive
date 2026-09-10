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

// This table drives the real aggregate producer through Go's http.Client and
// an in-memory RoundTripper, then calls the production classifier. Cancellation
// is injected only after the producer facts have been checked. This isolates
// the handoff from cancellation during an unfinished body read.
func TestAggregateCompletedAttemptCancellationLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name            string
		payload         []byte
		fault           replayHandoffBodyFault
		cancel          bool
		wantBody        []byte
		wantRead        int
		wantProducerErr error
		wantClass       attemptDisposition
		wantErr         error
		wantNative      error
	}{
		{name: "empty successful producer retains zero body and no retry", wantClass: attemptComplete},
		{name: "one byte below ceiling is not rounded to absence", payload: []byte{0xff}, wantBody: []byte{0xff}, wantRead: 1, wantClass: attemptComplete},
		{name: "exact ceiling preserves binary body", payload: []byte{0, 0xff}, wantBody: []byte{0, 0xff}, wantRead: 2, wantClass: attemptComplete},
		{name: "above former cutoff retains all bytes", payload: []byte{0, 0xff, 1}, wantBody: []byte{0, 0xff, 1}, wantRead: 3, wantClass: attemptComplete},
		{name: "many windows retain all bytes", payload: bytes.Repeat([]byte{0xff}, 1<<16), wantBody: bytes.Repeat([]byte{0xff}, 1<<16), wantRead: 1 << 16, wantClass: attemptComplete},
		{name: "native read refusal withholds partial aggregate and requests retry", payload: []byte{0xff}, fault: replayHandoffReadFailure, wantRead: 1, wantProducerErr: io.ErrUnexpectedEOF, wantClass: attemptRetry},
		{name: "native close refusal retains completed bytes and requests retry", payload: []byte{0, 0xff}, fault: replayHandoffCloseFailure, wantBody: []byte{0, 0xff}, wantRead: 2, wantProducerErr: io.ErrClosedPipe, wantClass: attemptRetry},
		{name: "canceling empty completed producer cannot invent body evidence", cancel: true, wantClass: attemptComplete, wantErr: context.Canceled},
		{name: "canceling completed exact body preserves producer facts", payload: []byte{0, 0xff}, wantBody: []byte{0, 0xff}, wantRead: 2, cancel: true, wantClass: attemptComplete, wantErr: context.Canceled},
		{name: "cancellation retains all completed bytes beyond former cutoff", payload: []byte{0, 0xff, 1}, wantBody: []byte{0, 0xff, 1}, wantRead: 3, cancel: true, wantClass: attemptComplete, wantErr: context.Canceled},
		{name: "cancellation cannot erase prior native read refusal", payload: []byte{0xff}, fault: replayHandoffReadFailure, wantRead: 1, wantProducerErr: io.ErrUnexpectedEOF, cancel: true, wantClass: attemptComplete, wantErr: context.Canceled, wantNative: io.ErrUnexpectedEOF},
		{name: "cancellation cannot erase prior native close refusal", payload: []byte{0, 0xff}, fault: replayHandoffCloseFailure, wantBody: []byte{0, 0xff}, wantRead: 2, wantProducerErr: io.ErrClosedPipe, cancel: true, wantClass: attemptComplete, wantErr: context.Canceled, wantNative: io.ErrClosedPipe},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			target, err := core.ParseHTTPEndpoint("https://provider.example.test/aggregate")
			if err != nil {
				t.Fatalf("endpoint fixture = %v, want nil", err)
			}
			body := &replayHandoffBody{reader: bytes.NewReader(tc.payload), fault: tc.fault}
			calls := 0
			client := &http.Client{Transport: replayHandoffTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: http.StatusOK, ContentLength: -1, Body: body, Request: request}, nil
			})}
			semantics := RequestSemantics{Method: MethodGet, Replay: ReplaySafe}
			if err := semantics.Validate(); err != nil {
				t.Fatalf("replay semantics fixture = %v, want nil", err)
			}
			produced, producerErr := executeAggregateAttempt(aggregateAttempt{context: ctx, client: client,
				request: aggregateRequest{target: target, semantics: semantics, expectedStatus: core.HTTPStatusOK()},
				timeout: runtimeAgreementPolicy(t).ReadTimeout})
			if produced.status != core.HTTPStatusOK() || !bytes.Equal(produced.body, tc.wantBody) || len(produced.headers.Values) != 0 || produced.retryAfter != "" {
				t.Fatalf("producer = (%v,%x,%v,%q), want exact OK/%x and absent headers/retry-after", produced.status, produced.body, produced.headers.Values, produced.retryAfter, tc.wantBody)
			}
			if (produced.body == nil) != (tc.wantBody == nil) {
				t.Fatalf("producer body nil = %t, want %t", produced.body == nil, tc.wantBody == nil)
			}
			if !errors.Is(producerErr, tc.wantProducerErr) {
				t.Fatalf("producer error = %v, want %v", producerErr, tc.wantProducerErr)
			}
			if calls != 1 || body.closes != 1 || body.readBytes != tc.wantRead {
				t.Fatalf("producer call/close/read = %d/%d/%d, want 1/1/%d", calls, body.closes, body.readBytes, tc.wantRead)
			}
			retained := bytes.Clone(produced.body)
			if tc.cancel {
				cancel()
			}
			gotClass, gotErr := classifyAggregateAttempt(aggregateAttemptResult{operationContext: ctx, cause: producerErr, semantics: semantics, response: produced, expected: core.HTTPStatusOK(), attemptsRemaining: 1})
			if gotClass != tc.wantClass || !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("classification = (%v,%v), want (%v,%v)", gotClass, gotErr, tc.wantClass, tc.wantErr)
			}
			if tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("retained producer cause = %v, want %v", gotErr, tc.wantNative)
			}
			if errors.Is(gotErr, core.ErrExchangeCancelled) != tc.cancel {
				t.Fatalf("cancellation identity = %t, want %t", errors.Is(gotErr, core.ErrExchangeCancelled), tc.cancel)
			}
			if !bytes.Equal(produced.body, retained) || calls != 1 || body.closes != 1 || body.readBytes != tc.wantRead {
				t.Fatalf("classifier changed producer evidence or performed another effect: body %x calls %d closes %d read %d", produced.body, calls, body.closes, body.readBytes)
			}
		})
	}
}
