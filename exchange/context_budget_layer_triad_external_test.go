package exchange_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestContextBudgetLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive active context completes inside both typed budgets", func(t *testing.T) {
		t.Parallel()

		ok := mustHTTPStatus(t, http.StatusOK)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			writer.Header().Set(
				core.HTTPHeaderContentType().String(),
				core.HTTPMediaTypeTextPlain().String(),
			)
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("inside budget"))
		}))
		defer server.Close()

		got, gotErr := exchange.SendNoBodyBounded(
			exchange.NoBodyBoundedCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.NoBodyBoundedRequest{
					Target: mustEndpoint(t, server.URL),
					Semantics: exchange.RequestSemantics{
						Method: core.HTTPMethodGet,
						Replay: exchange.ReplaySingleAttempt,
					},
					ExpectedResponseContentType: core.HTTPMediaTypeTextPlain(),
					ExpectedStatus:              ok,
				},
				Policy: exchange.NoBodyBoundedPolicy{
					Operation:         singleAttemptOperationPolicy(t),
					ResponseBodyLimit: mustByteCount(t, 4*1024),
				},
			},
		)
		if gotErr != nil || string(got.Body) != "inside budget" {
			t.Fatalf(
				"bounded active-context result = (%q, %v), want (%q, nil)",
				got.Body,
				gotErr,
				"inside budget",
			)
		}
	})

	t.Run("negative attempt deadline cancels the real HTTP request and handler", func(t *testing.T) {
		t.Parallel()

		handlerDone := make(chan error, 1)
		server := httptest.NewServer(http.HandlerFunc(func(
			_ http.ResponseWriter,
			request *http.Request,
		) {
			<-request.Context().Done()
			handlerDone <- request.Context().Err()
		}))
		defer server.Close()

		ok := mustHTTPStatus(t, http.StatusOK)
		policy := singleAttemptOperationPolicy(t)
		policy.OperationTimeout = mustDurationMilliseconds(t, 5_000)
		// Five milliseconds let the deadline fire before a race-instrumented
		// request reached the real handler, making "handler was cancelled"
		// impossible to observe. One second still proves the attempt budget owns
		// cancellation while leaving enough ingress margin under the full gate.
		policy.AttemptTimeout = mustDurationMilliseconds(t, 1_000)
		got, gotErr := exchange.SendNoBodyBounded(
			exchange.NoBodyBoundedCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.NoBodyBoundedRequest{
					Target: mustEndpoint(t, server.URL),
					Semantics: exchange.RequestSemantics{
						Method: core.HTTPMethodGet,
						Replay: exchange.ReplaySingleAttempt,
					},
					ExpectedStatus: ok,
				},
				Policy: exchange.NoBodyBoundedPolicy{
					Operation:         policy,
					ResponseBodyLimit: mustByteCount(t, 4*1024),
				},
			},
		)
		if !errors.Is(gotErr, core.ErrExchangeCancelled) ||
			!errors.Is(gotErr, context.DeadlineExceeded) {
			t.Fatalf(
				"attempt deadline error = %v, want %v and %v",
				gotErr,
				core.ErrExchangeCancelled,
				context.DeadlineExceeded,
			)
		}
		if got.Metadata.Attempts != 0 || len(got.Body) != 0 {
			t.Fatalf("attempt deadline response = %+v, want zero", got)
		}
		select {
		case gotHandlerErr := <-handlerDone:
			if !errors.Is(gotHandlerErr, context.Canceled) {
				t.Fatalf("handler context error = %v, want %v", gotHandlerErr, context.Canceled)
			}
		case <-time.After(testDeadlockBackstop):
			t.Fatalf(
				"attempt-deadline handler completion = absent after %v, want context-bound exit",
				testDeadlockBackstop,
			)
		}
	})

	t.Run("neutral pre-cancelled context transmits no request", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Uint64
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			calls.Add(1)
			writer.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ok := mustHTTPStatus(t, http.StatusOK)
		got, gotErr := exchange.SendNoBodyBounded(
			exchange.NoBodyBoundedCall{
				Context: ctx,
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.NoBodyBoundedRequest{
					Target: mustEndpoint(t, server.URL),
					Semantics: exchange.RequestSemantics{
						Method: core.HTTPMethodGet,
						Replay: exchange.ReplaySingleAttempt,
					},
					ExpectedStatus: ok,
				},
				Policy: exchange.NoBodyBoundedPolicy{
					Operation:         singleAttemptOperationPolicy(t),
					ResponseBodyLimit: mustByteCount(t, 4*1024),
				},
			},
		)
		if !errors.Is(gotErr, core.ErrExchangeRequest) ||
			!errors.Is(gotErr, core.ErrExchangeCancelled) ||
			!errors.Is(gotErr, context.Canceled) {
			t.Fatalf(
				"pre-cancelled ingress error = %v, want %v, %v, and %v",
				gotErr,
				core.ErrExchangeRequest,
				core.ErrExchangeCancelled,
				context.Canceled,
			)
		}
		if calls.Load() != 0 {
			t.Fatalf("pre-cancelled server calls = %d, want 0", calls.Load())
		}
		if got.Metadata.Attempts != 0 || len(got.Body) != 0 {
			t.Fatalf("pre-cancelled response = %+v, want zero", got)
		}
	})
}
