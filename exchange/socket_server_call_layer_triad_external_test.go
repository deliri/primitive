package exchange_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestSocketServerCallLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exchange owns the complete HTTP ingress", func(t *testing.T) {
		t.Parallel()
		request := httptest.NewRequest("POST", "/review", nil)
		call, gotErr := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
		gotContext, contextErr := call.Context()
		if gotErr != nil || contextErr != nil || gotContext != request.Context() {
			t.Fatalf("NewSocketServerCall(complete).Context() = (%v, %v, %v), want (request context, nil, nil)", gotContext, gotErr, contextErr)
		}
	})

	t.Run("negative partial HTTP ingress is refused", func(t *testing.T) {
		t.Parallel()
		request := httptest.NewRequest("POST", "/review", nil)
		_, missingWriterErr := exchange.NewSocketServerCall(nil, request)
		_, missingRequestErr := exchange.NewSocketServerCall(httptest.NewRecorder(), nil)
		if !errors.Is(missingWriterErr, core.ErrExchangeContract) || !errors.Is(missingRequestErr, core.ErrExchangeContract) {
			t.Fatalf("NewSocketServerCall(partial) errors = (%v, %v), want both %v", missingWriterErr, missingRequestErr, core.ErrExchangeContract)
		}
	})

	t.Run("neutral cancelled request preserves cancellation without performing transport", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		request := httptest.NewRequest("POST", "/review", nil).WithContext(ctx)
		call, gotErr := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
		gotContext, contextErr := call.Context()
		if gotErr != nil || contextErr != nil || !errors.Is(gotContext.Err(), context.Canceled) {
			t.Fatalf("NewSocketServerCall(cancelled).Context() = (%v, %v, %v), want context.Canceled with nil construction errors", gotContext.Err(), gotErr, contextErr)
		}
	})
}
