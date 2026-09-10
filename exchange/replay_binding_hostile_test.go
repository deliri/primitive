package exchange_test

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type replayBoundDocument struct {
	Operation string `json:"operation"`
}

type replayBoundResponse struct {
	Accepted bool `json:"accepted"`
}

func (r replayBoundResponse) Validate() error {
	if !r.Accepted {
		return core.ErrExchangeContract
	}
	return nil
}

func (r replayBoundResponse) MarshalJSON() ([]byte, error) {
	type wire replayBoundResponse
	return json.Marshal(wire(r))
}

func (d replayBoundDocument) Validate() error {
	_, err := d.IdempotencyKey()
	return err
}

func (d replayBoundDocument) MarshalJSON() ([]byte, error) {
	type wire replayBoundDocument
	return json.Marshal(wire(d))
}

func (d replayBoundDocument) IdempotencyKey() (exchange.IdempotencyKey, error) {
	return exchange.ParseIdempotencyKey(d.Operation)
}

func TestReplayBoundJSONRefusesHeaderBodyIdentityDivergence(t *testing.T) {
	t.Parallel()

	maximumKey := strings.Repeat("m", exchange.IdempotencyKeyMaximumBytes)
	cases := []struct {
		wantErr       error
		name          string
		documentKey   string
		method        string
		wantOperation string
		headerKeys    []string
		bodyOverride  []byte
		wantBinding   bool
	}{
		{name: "one-byte identity binds", documentKey: "a", headerKeys: []string{"a"}, wantOperation: "a"},
		{name: "mixed case identity binds exactly", documentKey: "Operation-Aa", headerKeys: []string{"Operation-Aa"}, wantOperation: "Operation-Aa"},
		{name: "punctuated identity binds", documentKey: "operation:3/path", headerKeys: []string{"operation:3/path"}, wantOperation: "operation:3/path"},
		{name: "one below maximum identity binds", documentKey: maximumKey[:len(maximumKey)-1], headerKeys: []string{maximumKey[:len(maximumKey)-1]}, wantOperation: maximumKey[:len(maximumKey)-1]},
		{name: "maximum identity binds", documentKey: maximumKey, headerKeys: []string{maximumKey}, wantOperation: maximumKey},
		{name: "different one-byte identities are refused", documentKey: "a", headerKeys: []string{"b"}, wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "case difference is refused", documentKey: "Operation", headerKeys: []string{"operation"}, wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "prefix identity is refused", documentKey: "operation-1", headerKeys: []string{"operation"}, wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "suffix identity is refused", documentKey: "operation", headerKeys: []string{"operation-1"}, wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "absent header identity is refused", documentKey: "operation", wantErr: core.ErrExchangeRequest},
		{name: "duplicate header identity is refused", documentKey: "operation", headerKeys: []string{"operation", "operation"}, wantErr: core.ErrExchangeRequest},
		{name: "empty header identity is refused", documentKey: "operation", headerKeys: []string{""}, wantErr: core.ErrExchangeRequest},
		{name: "wrong method is refused before body release", documentKey: "operation", headerKeys: []string{"operation"}, method: http.MethodPut, wantErr: core.ErrExchangeRequest},
		{name: "truncated document is refused before identity comparison", documentKey: "operation", headerKeys: []string{"operation"}, bodyOverride: []byte(`{"operation":`), wantErr: core.ErrExchangeRequest},
		{name: "unknown document member is refused before identity comparison", documentKey: "operation", headerKeys: []string{"operation"}, bodyOverride: []byte(`{"operation":"operation","unknown":true}`), wantErr: core.ErrExchangeRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			document := replayBoundDocument{Operation: tc.documentKey}
			body, gotMarshalErr := document.MarshalJSON()
			if gotMarshalErr != nil {
				t.Fatalf("replayBoundDocument.MarshalJSON() error = %v, want nil", gotMarshalErr)
			}
			if tc.bodyOverride != nil {
				body = tc.bodyOverride
			}
			method := tc.method
			if method == "" {
				method = http.MethodPost
			}
			request := httptest.NewRequest(method, "/", bytes.NewReader(body))
			request.Header.Set(core.HTTPHeaderContentType().String(), mustHTTPMediaType(t, "application/json").String())
			for _, key := range tc.headerKeys {
				request.Header.Add(core.HTTPHeaderIdempotencyKey().String(), key)
			}
			got, gotErr := exchange.ReceiveReplayBoundJSON[
				replayBoundDocument,
				*replayBoundDocument,
			](exchange.JSONReceiveCall{
				Call: socketServerCall(t, request),
				Route: exchange.RouteSemantics{
					Method: exchange.MethodPost,
					Replay: exchange.ReplayIdempotencyKey,
				},
			})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got.Body != nil || !got.IdempotencyKey.IsZero() {
					t.Fatalf("ReceiveReplayBoundJSON() = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, tc.wantErr)
				}
				if tc.wantBinding != errors.Is(gotErr, core.ErrExchangeIdempotencyBinding) {
					t.Fatalf("ReceiveReplayBoundJSON() binding identity = %t, want %t; error = %v", errors.Is(gotErr, core.ErrExchangeIdempotencyBinding), tc.wantBinding, gotErr)
				}
				return
			}
			if gotErr != nil || got.Body == nil || got.Body.Operation != tc.wantOperation || got.IdempotencyKey.String() != tc.wantOperation {
				t.Fatalf("ReceiveReplayBoundJSON() = (%+v, %v), want body and header identity %q", got, gotErr, tc.wantOperation)
			}
		})
	}
}

func TestSendReplayBoundJSONRefusesIdentityDivergenceBeforeNetwork(t *testing.T) {
	t.Parallel()

	maximumKey := strings.Repeat("m", exchange.IdempotencyKeyMaximumBytes)
	cases := []struct {
		name        string
		documentKey string
		headerKey   string
		replay      exchange.ReplayMode
		wantBinding bool
		wantRequest uint64
	}{
		{name: "one-byte identity crosses", documentKey: "a", headerKey: "a", replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "mixed case identity crosses exactly", documentKey: "Operation-Aa", headerKey: "Operation-Aa", replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "punctuated identity crosses", documentKey: "operation:3/path", headerKey: "operation:3/path", replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "one below maximum identity crosses", documentKey: maximumKey[:len(maximumKey)-1], headerKey: maximumKey[:len(maximumKey)-1], replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "maximum identity crosses", documentKey: maximumKey, headerKey: maximumKey, replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "different one-byte identities are refused", documentKey: "a", headerKey: "b", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "case difference is refused", documentKey: "Operation", headerKey: "operation", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "prefix identity is refused", documentKey: "operation-1", headerKey: "operation", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "suffix identity is refused", documentKey: "operation", headerKey: "operation-1", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "single-attempt semantics are refused by bound lane", documentKey: "operation", headerKey: "", replay: exchange.ReplaySingleAttempt, wantBinding: true},
		{name: "first byte divergence is refused", documentKey: "alpha", headerKey: "zlpha", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "middle byte divergence is refused", documentKey: "alpha", headerKey: "alxha", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "last byte divergence is refused", documentKey: "alpha", headerKey: "alphz", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var requests atomic.Uint64
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				requests.Add(1)
				writeErr := exchange.WriteJSON(exchange.JSONWriteCall[replayBoundResponse]{
					Call: socketServerCallFrom(t, writer, request),
					Response: exchange.ServerJSONResponse[replayBoundResponse]{
						Status: core.HTTPStatusOK(), Body: replayBoundResponse{Accepted: true},
					},
				})
				if writeErr != nil {
					t.Errorf("exchange.WriteJSON() error = %v, want nil", writeErr)
				}
			}))
			t.Cleanup(server.Close)

			headerKey := exchange.IdempotencyKey{}
			if tc.headerKey != "" {
				var keyErr error
				headerKey, keyErr = exchange.ParseIdempotencyKey(tc.headerKey)
				if keyErr != nil {
					t.Fatalf("exchange.ParseIdempotencyKey(header) error = %v, want nil", keyErr)
				}
			}
			got, gotErr := exchange.SendReplayBoundJSON[replayBoundDocument, replayBoundResponse](exchange.JSONCall[replayBoundDocument]{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.JSONRequest[replayBoundDocument]{
					Target: mustEndpoint(t, server.URL), Body: replayBoundDocument{Operation: tc.documentKey},
					Semantics:      exchange.RequestSemantics{Method: exchange.MethodPost, Replay: tc.replay, IdempotencyKey: headerKey},
					ExpectedStatus: core.HTTPStatusOK(),
				},
				Policy: exchange.JSONPolicy{
					Operation: singleAttemptOperationPolicy(t),
				},
			})
			if tc.wantRequest == 0 {
				if !errors.Is(gotErr, core.ErrExchangeRequest) ||
					tc.wantBinding != errors.Is(gotErr, core.ErrExchangeIdempotencyBinding) ||
					got.Body != (replayBoundResponse{}) || got.Metadata.Attempts != 0 ||
					got.Metadata.Bytes.Uint64() != 0 || got.Metadata.Status != (core.HTTPStatusCode{}) ||
					len(got.Metadata.Headers.Values) != 0 {
					t.Fatalf("SendReplayBoundJSON() = (%+v, %v), want zero, request identity, and binding=%t", got, gotErr, tc.wantBinding)
				}
			} else if gotErr != nil || !got.Body.Accepted {
				t.Fatalf("SendReplayBoundJSON() = (%+v, %v), want accepted response and nil", got, gotErr)
			}
			if gotRequests := requests.Load(); gotRequests != tc.wantRequest {
				t.Fatalf("network request count = %d, want %d", gotRequests, tc.wantRequest)
			}
		})
	}
}
