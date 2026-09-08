package exchange_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestSocketContextDerivationLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                                                             string
		zeroCall, nilContext, sameContext, cancelOriginal, cancelDerived bool
		wantErr                                                          error
	}{
		{name: "replacement context cannot inherit the old value by accident"},
		{name: "same context still follows Go request-copy ownership", sameContext: true},
		{name: "cancelled old lifetime cannot poison independent replacement", cancelOriginal: true},
		{name: "cancelled replacement remains an exact observable fact", cancelDerived: true},
		{name: "nil replacement cannot release a partial call", nilContext: true, wantErr: core.ErrExchangeContract},
		{name: "unbound socket cannot gain capabilities from a context", zeroCall: true, wantErr: core.ErrExchangeContract},
		{name: "two absent capabilities remain absent", zeroCall: true, nilContext: true, wantErr: core.ErrExchangeContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			type contextKey struct{}
			originalContext, cancelOriginal := context.WithCancel(context.WithValue(t.Context(), contextKey{}, "original"))
			defer cancelOriginal()
			derivedContext, cancelDerived := context.WithCancel(context.WithValue(t.Context(), contextKey{}, "derived"))
			defer cancelDerived()
			if tc.cancelOriginal {
				cancelOriginal()
			}
			if tc.cancelDerived {
				cancelDerived()
			}
			var replacement context.Context = derivedContext
			if tc.sameContext {
				replacement = originalContext
			}
			if tc.nilContext {
				replacement = nil
			}
			request := httptest.NewRequestWithContext(originalContext, http.MethodPost, "https://context-oracle.invalid/scope", nil)
			request.Header.Set(core.HTTPHeaderAccept().String(), core.HTTPMediaTypeJSON().String())
			writer := httptest.NewRecorder()
			call, err := exchange.NewSocketServerCall(writer, request)
			if err != nil {
				t.Fatal(err)
			}
			if tc.zeroCall {
				call = exchange.SocketServerCall{}
			}
			got, err := call.WithContext(replacement)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("context derivation error=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (exchange.SocketServerCall{}) {
					t.Fatalf("refused context derivation=%+v, want zero", got)
				}
				return
			}
			actualContext, err := got.Context()
			if err != nil || actualContext != replacement || actualContext.Value(contextKey{}) != replacement.Value(contextKey{}) || !errors.Is(actualContext.Err(), replacement.Err()) {
				t.Fatalf("derived context=(%v,%v), want exact supplied Go context", actualContext, err)
			}
			var observations int
			var observed *http.Request
			var observedWriter http.ResponseWriter
			err = got.ServeHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { observations++; observed, observedWriter = r, w }))
			if err != nil || observations != 1 || observed == nil {
				t.Fatalf("handler observation=(%v,%d,%v), want one request", err, observations, observed)
			}
			if observed == request || observed.Context() != replacement || observedWriter != writer || observed.Body != request.Body || observed.URL != request.URL || observed.Method != request.Method || observed.Header.Get(core.HTTPHeaderAccept().String()) != request.Header.Get(core.HTTPHeaderAccept().String()) {
				t.Fatalf("request distinct/context/writer/body/URL/method/header retained=%t/%t/%t/%t/%t/%t/%t, want all true", observed != request, observed.Context() == replacement, observedWriter == writer, observed.Body == request.Body, observed.URL == request.URL, observed.Method == request.Method, observed.Header.Get(core.HTTPHeaderAccept().String()) == request.Header.Get(core.HTTPHeaderAccept().String()))
			}
			if request.Context() != originalContext || writer.Body.Len() != 0 || len(writer.Header()) != 0 || writer.Flushed {
				t.Fatalf("context preserved/body/headers/flush=%t/%d/%d/%t, want true/0/0/false", request.Context() == originalContext, writer.Body.Len(), len(writer.Header()), writer.Flushed)
			}
		})
	}
}
