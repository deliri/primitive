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

// The constructor admits two capabilities. Exhaust all missing/typed-nil
// writer and missing request combinations, then preserve cancellation as data.
func TestSocketServerCallLayerTriad(t *testing.T) {
	t.Parallel()
	type writerPresence uint8
	const (
		presentWriter writerPresence = iota
		absentWriter
		typedNilWriter
	)
	cases := []struct {
		name             string
		writer           writerPresence
		absentRequest    bool
		cancelled        bool
		wantErr          error
		wantContextErr   error
		wantContextCause error
		wantCalls        int
	}{
		{name: "positive both capabilities retain exact identity", wantCalls: 1},
		{name: "negative missing request cannot retain writer", absentRequest: true, wantErr: core.ErrExchangeContract, wantContextErr: core.ErrExchangeContract},
		{name: "negative missing writer cannot retain request", writer: absentWriter, wantErr: core.ErrExchangeContract, wantContextErr: core.ErrExchangeContract},
		{name: "negative neither capability cannot construct", writer: absentWriter, absentRequest: true, wantErr: core.ErrExchangeContract, wantContextErr: core.ErrExchangeContract},
		{name: "negative typed nil writer cannot retain request", writer: typedNilWriter, wantErr: core.ErrExchangeContract, wantContextErr: core.ErrExchangeContract},
		{name: "negative typed nil writer and missing request stay zero", writer: typedNilWriter, absentRequest: true, wantErr: core.ErrExchangeContract, wantContextErr: core.ErrExchangeContract},
		{name: "neutral cancellation remains a fact of the original context", cancelled: true, wantContextCause: context.Canceled, wantCalls: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/review", nil)
			if tc.absentRequest {
				request = nil
			}
			recorder := httptest.NewRecorder()
			var writer http.ResponseWriter = recorder
			switch tc.writer {
			case absentWriter:
				writer = nil
			case typedNilWriter:
				writer = (*httptest.ResponseRecorder)(nil)
			}
			got, gotErr := exchange.NewSocketServerCall(writer, request)
			gotContext, gotContextErr := got.Context()
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotContextErr, tc.wantContextErr) {
				t.Fatalf("constructor/context errors = (%v, %v), want (%v, %v)", gotErr, gotContextErr, tc.wantErr, tc.wantContextErr)
			}
			if tc.wantErr != nil {
				if got != (exchange.SocketServerCall{}) || gotContext != nil {
					t.Fatalf("refused call/context = (%+v, %v), want exact zero and nil", got, gotContext)
				}
				return
			}
			if gotContext != ctx || !errors.Is(gotContext.Err(), tc.wantContextCause) {
				t.Fatalf("context/cause = (%v, %v), want (%v, %v)", gotContext, gotContext.Err(), ctx, tc.wantContextCause)
			}
			var calls int
			var observedRequest *http.Request
			var observedWriter http.ResponseWriter
			gotServeErr := got.ServeHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				observedRequest, observedWriter = r, w
			}))
			if gotServeErr != nil || calls != tc.wantCalls || observedRequest != request || observedWriter != writer {
				t.Fatalf("ServeHTTP error/calls/request identity/writer identity = (%v, %d, %t, %t), want (nil, %d, true, true)", gotServeErr, calls, observedRequest == request, observedWriter == writer, tc.wantCalls)
			}
			if recorder.Body.Len() != 0 || len(recorder.Header()) != 0 {
				t.Fatalf("constructor effects body/headers = (%q, %v), want absent", recorder.Body.Bytes(), recorder.Header())
			}
		})
	}
}
