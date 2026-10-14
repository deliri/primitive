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
)

// A malformed selected-header agreement cannot become valid by repeating
// identical requests. Both replay owners must stop after the first producer.
func TestNativeMalformedCapturedHeaderDoesNotReplay(t *testing.T) {
	t.Parallel()
	for _, stream := range []bool{false, true} {
		t.Run(headerReplayDoorName(stream), func(t *testing.T) {
			t.Parallel()
			var calls atomic.Uint64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				for range exchange.HeaderValueMaximumCount + 1 {
					w.Header().Add("X-Owned-Correlation", "admitted-native-field-value")
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			client := mustExchangeClient(t, server.Client())
			name, err := core.ParseHTTPHeaderName("X-Owned-Correlation")
			if err != nil {
				t.Fatal(err)
			}
			capture := exchange.HeaderSelection{Names: []core.HTTPHeaderName{name}}
			target := mustEndpoint(t, server.URL)
			policy := retryOperationPolicy(t, 3)
			if stream {
				_, err = exchange.ReplayStream(exchange.StreamReplayCall{Context: t.Context(), Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySafe},
					Policy: exchange.StreamReplayPolicy{OperationTimeout: policy.OperationTimeout, Retry: policy.Retry},
					Attempt: func(ctx context.Context, _ uint64) (exchange.StreamResponse, error) {
						return exchange.Download(exchange.DownloadCall{Context: ctx, Client: client, Policy: singleAttemptStreamPolicy(t), Request: exchange.DownloadRequest{Target: target,
							Destination: io.Discard, CaptureHeaders: capture, ExpectedStatus: core.HTTPStatusOK(), Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}}})
					}})
			} else {
				_, err = exchange.SendNoBodyBounded(exchange.NoBodyBoundedCall{Context: t.Context(), Client: client, Policy: exchange.NoBodyBoundedPolicy{Operation: policy},
					Request: exchange.NoBodyBoundedRequest{Target: target, CaptureHeaders: capture, ExpectedStatus: core.HTTPStatusOK(), Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySafe}}})
			}
			if !errors.Is(err, core.ErrExchangeContract) || errors.Is(err, core.ErrExchangeRetryExhausted) || calls.Load() != 1 {
				t.Fatalf("captured-header refusal = %v calls=%d, want terminal typed agreement refusal and exactly one native request", err, calls.Load())
			}
		})
	}
}

func headerReplayDoorName(stream bool) string {
	if stream {
		return "stream_replay"
	}
	return "aggregate_replay"
}
