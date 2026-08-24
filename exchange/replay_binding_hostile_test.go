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
	key31 := strings.Repeat("a", 31)
	key32 := strings.Repeat("b", 32)
	key33 := strings.Repeat("c", 33)
	key63 := strings.Repeat("d", 63)
	key64 := strings.Repeat("e", 64)
	key65 := strings.Repeat("f", 65)
	key127 := strings.Repeat("g", 127)
	key128 := strings.Repeat("h", 128)
	key129 := strings.Repeat("i", 129)
	key253 := strings.Repeat("j", exchange.IdempotencyKeyMaximumBytes-2)
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
		{name: "two-byte identity binds", documentKey: "ab", headerKeys: []string{"ab"}, wantOperation: "ab"},
		{name: "hyphenated identity binds", documentKey: "operation-1", headerKeys: []string{"operation-1"}, wantOperation: "operation-1"},
		{name: "underscored identity binds", documentKey: "operation_2", headerKeys: []string{"operation_2"}, wantOperation: "operation_2"},
		{name: "uuid identity binds", documentKey: "019ff548-29cb-7451-869e-aa644c0947e6", headerKeys: []string{"019ff548-29cb-7451-869e-aa644c0947e6"}, wantOperation: "019ff548-29cb-7451-869e-aa644c0947e6"},
		{name: "mixed case identity binds exactly", documentKey: "Operation-Aa", headerKeys: []string{"Operation-Aa"}, wantOperation: "Operation-Aa"},
		{name: "punctuated identity binds", documentKey: "operation:3/path", headerKeys: []string{"operation:3/path"}, wantOperation: "operation:3/path"},
		{name: "visible punctuation identity binds", documentKey: "operation~four", headerKeys: []string{"operation~four"}, wantOperation: "operation~four"},
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
		{name: "boundary matching identity one below 32 bytes binds", documentKey: key31, headerKeys: []string{key31}, wantOperation: key31},
		{name: "boundary matching identity at 32 bytes binds", documentKey: key32, headerKeys: []string{key32}, wantOperation: key32},
		{name: "boundary matching identity one above 32 bytes binds", documentKey: key33, headerKeys: []string{key33}, wantOperation: key33},
		{name: "boundary matching identity one below 64 bytes binds", documentKey: key63, headerKeys: []string{key63}, wantOperation: key63},
		{name: "boundary matching identity at 64 bytes binds", documentKey: key64, headerKeys: []string{key64}, wantOperation: key64},
		{name: "boundary matching identity one above 64 bytes binds", documentKey: key65, headerKeys: []string{key65}, wantOperation: key65},
		{name: "boundary matching identity one below 128 bytes binds", documentKey: key127, headerKeys: []string{key127}, wantOperation: key127},
		{name: "boundary matching identity at 128 bytes binds", documentKey: key128, headerKeys: []string{key128}, wantOperation: key128},
		{name: "boundary matching identity one above 128 bytes binds", documentKey: key129, headerKeys: []string{key129}, wantOperation: key129},
		{name: "boundary matching identity two below maximum binds", documentKey: key253, headerKeys: []string{key253}, wantOperation: key253},
		{name: "boundary divergent identity one below 32 bytes is refused", documentKey: key31, headerKeys: []string{key31[:30] + "z"}, wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "boundary divergent identity at 32 bytes is refused", documentKey: key32, headerKeys: []string{key32[:31] + "z"}, wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "boundary divergent identity one above 32 bytes is refused", documentKey: key33, headerKeys: []string{key33[:32] + "z"}, wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "boundary divergent identity one below 64 bytes is refused", documentKey: key63, headerKeys: []string{key63[:62] + "z"}, wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "boundary divergent identity at 64 bytes is refused", documentKey: key64, headerKeys: []string{key64[:63] + "z"}, wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "boundary divergent identity one above 64 bytes is refused", documentKey: key65, headerKeys: []string{key65[:64] + "z"}, wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "boundary divergent identity one below 128 bytes is refused", documentKey: key127, headerKeys: []string{key127[:126] + "z"}, wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "boundary divergent identity at 128 bytes is refused", documentKey: key128, headerKeys: []string{key128[:127] + "z"}, wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "boundary divergent identity one above 128 bytes is refused", documentKey: key129, headerKeys: []string{key129[:128] + "z"}, wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "boundary divergent identity two below maximum is refused", documentKey: key253, headerKeys: []string{key253[:252] + "z"}, wantErr: core.ErrExchangeRequest, wantBinding: true},
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
				Request: request,
				Route: exchange.RouteSemantics{
					Method: exchange.MethodPost,
					Replay: exchange.ReplayIdempotencyKey,
				},
				Policy: exchange.ServerPolicy{RequestBodyLimit: mustByteCount(t, 4*1024)},
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
	key31 := strings.Repeat("a", 31)
	key32 := strings.Repeat("b", 32)
	key33 := strings.Repeat("c", 33)
	key63 := strings.Repeat("d", 63)
	key64 := strings.Repeat("e", 64)
	key65 := strings.Repeat("f", 65)
	key127 := strings.Repeat("g", 127)
	key128 := strings.Repeat("h", 128)
	key129 := strings.Repeat("i", 129)
	key253 := strings.Repeat("j", exchange.IdempotencyKeyMaximumBytes-2)
	cases := []struct {
		name        string
		documentKey string
		headerKey   string
		replay      exchange.ReplayMode
		wantBinding bool
		wantRequest uint64
	}{
		{name: "one-byte identity crosses", documentKey: "a", headerKey: "a", replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "two-byte identity crosses", documentKey: "ab", headerKey: "ab", replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "hyphenated identity crosses", documentKey: "operation-1", headerKey: "operation-1", replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "underscored identity crosses", documentKey: "operation_2", headerKey: "operation_2", replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "uuid identity crosses", documentKey: "019ff548-29cb-7451-869e-aa644c0947e6", headerKey: "019ff548-29cb-7451-869e-aa644c0947e6", replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "mixed case identity crosses exactly", documentKey: "Operation-Aa", headerKey: "Operation-Aa", replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "punctuated identity crosses", documentKey: "operation:3/path", headerKey: "operation:3/path", replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "visible punctuation identity crosses", documentKey: "operation~four", headerKey: "operation~four", replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
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
		{name: "shorter header identity is refused", documentKey: "alpha", headerKey: "alph", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "longer header identity is refused", documentKey: "alpha", headerKey: "alphaa", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "boundary matching identity one below 32 bytes crosses", documentKey: key31, headerKey: key31, replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "boundary matching identity at 32 bytes crosses", documentKey: key32, headerKey: key32, replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "boundary matching identity one above 32 bytes crosses", documentKey: key33, headerKey: key33, replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "boundary matching identity one below 64 bytes crosses", documentKey: key63, headerKey: key63, replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "boundary matching identity at 64 bytes crosses", documentKey: key64, headerKey: key64, replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "boundary matching identity one above 64 bytes crosses", documentKey: key65, headerKey: key65, replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "boundary matching identity one below 128 bytes crosses", documentKey: key127, headerKey: key127, replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "boundary matching identity at 128 bytes crosses", documentKey: key128, headerKey: key128, replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "boundary matching identity one above 128 bytes crosses", documentKey: key129, headerKey: key129, replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "boundary matching identity two below maximum crosses", documentKey: key253, headerKey: key253, replay: exchange.ReplayIdempotencyKey, wantRequest: 1},
		{name: "boundary divergent identity one below 32 bytes is refused", documentKey: key31, headerKey: key31[:30] + "z", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "boundary divergent identity at 32 bytes is refused", documentKey: key32, headerKey: key32[:31] + "z", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "boundary divergent identity one above 32 bytes is refused", documentKey: key33, headerKey: key33[:32] + "z", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "boundary divergent identity one below 64 bytes is refused", documentKey: key63, headerKey: key63[:62] + "z", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "boundary divergent identity at 64 bytes is refused", documentKey: key64, headerKey: key64[:63] + "z", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "boundary divergent identity one above 64 bytes is refused", documentKey: key65, headerKey: key65[:64] + "z", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "boundary divergent identity one below 128 bytes is refused", documentKey: key127, headerKey: key127[:126] + "z", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "boundary divergent identity at 128 bytes is refused", documentKey: key128, headerKey: key128[:127] + "z", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "boundary divergent identity one above 128 bytes is refused", documentKey: key129, headerKey: key129[:128] + "z", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
		{name: "boundary divergent identity two below maximum is refused", documentKey: key253, headerKey: key253[:252] + "z", replay: exchange.ReplayIdempotencyKey, wantBinding: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var requests atomic.Uint64
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				writeErr := exchange.WriteJSON(exchange.JSONWriteCall[replayBoundResponse]{
					Writer: writer,
					Response: exchange.ServerJSONResponse[replayBoundResponse]{
						Status: core.HTTPStatusOK(), Body: replayBoundResponse{Accepted: true},
					},
					Policy: exchange.JSONWritePolicy{ResponseBodyLimit: mustByteCount(t, 4*1024)},
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
					Operation: singleAttemptOperationPolicy(t), RequestBodyLimit: mustByteCount(t, 4*1024), ResponseBodyLimit: mustByteCount(t, 4*1024),
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
