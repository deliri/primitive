package exchange_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// This uses the actual HTTP response producer and replay owner. The wait-budget
// outcome proves the hint was consumed without guessing elapsed wall time.
func TestNativeReplayRetryAfterBudgetPreservesLastStatus(t *testing.T) {
	t.Parallel()
	var calls atomic.Uint64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := mustExchangeClient(t, server.Client())
	one, err := temporal.DurationFromNanoseconds(1)
	if err != nil {
		t.Fatal(err)
	}
	window, err := temporal.DurationFromMilliseconds(1)
	if err != nil {
		t.Fatal(err)
	}
	budget, err := temporal.DurationFromSeconds(30)
	if err != nil {
		t.Fatal(err)
	}
	target := mustEndpoint(t, server.URL)
	got, err := exchange.ReplayStream(exchange.StreamReplayCall{
		Context: t.Context(), Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySafe},
		Policy: exchange.StreamReplayPolicy{OperationTimeout: budget, Retry: exchange.RetryPolicy{MaximumAttempts: 3,
			BaseDelay: one, MaximumDelay: one, MaximumJitter: one, MaximumRetryAfter: window, MaximumWait: window}},
		Attempt: func(ctx context.Context, _ uint64) (exchange.StreamResponse, error) {
			return exchange.Download(exchange.DownloadCall{Context: ctx, Client: client, Policy: singleAttemptStreamPolicy(t),
				Request: exchange.DownloadRequest{Target: target, Destination: io.Discard, ExpectedStatus: core.HTTPStatusOK(),
					Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}}})
		},
	})
	status, ok := errors.AsType[exchange.StatusError](err)
	if !errors.Is(err, core.ErrExchangeRetryExhausted) || !ok || status.Status() != mustHTTPStatus(t, http.StatusTooManyRequests) || calls.Load() != 2 || got.Metadata.Attempts != 2 {
		t.Fatalf("native retry hint = %+v/%v calls=%d, want last 429 retained and exactly two calls before wait budget refuses third", got, err, calls.Load())
	}
}
