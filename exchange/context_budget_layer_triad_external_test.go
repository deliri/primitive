package exchange_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// Deadline expiration is proved with Go's virtual clock in
// TestHTTPTransportFailureHandoffTable. These rows exercise actual loopback
// HTTP and prove the local positive/refusal/neutral response-accounting triad.
func TestContextBudgetLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		payload          []byte
		method           exchange.Method
		maximum          uint64
		zeroPolicy       bool
		wantBody         []byte
		wantCalls        uint64
		wantAttempts     uint64
		wantStatus       core.HTTPStatusCode
		wantHeaderLength uint64
		wantErr          error
	}{
		{name: "exact response ceiling retains binary bytes over real HTTP", payload: []byte{0, 0xff}, method: exchange.MethodGet, maximum: 2, wantBody: []byte{0, 0xff}, wantCalls: 1, wantAttempts: 1, wantStatus: core.HTTPStatusOK(), wantHeaderLength: 2},
		{name: "complete response retains bytes and HTTP facts", payload: []byte{0, 0xff}, method: exchange.MethodGet, maximum: 1, wantBody: []byte{0, 0xff}, wantCalls: 1, wantAttempts: 1, wantStatus: core.HTTPStatusOK(), wantHeaderLength: 2},
		{name: "empty successful HTTP response seals zero bytes without inventing body", method: exchange.MethodGet, maximum: 1, wantCalls: 1, wantAttempts: 1, wantStatus: core.HTTPStatusOK()},
		{name: "HEAD retains declared extent without claiming transferred response bytes", payload: []byte{0, 0xff}, method: exchange.MethodHead, maximum: 2, wantCalls: 1, wantAttempts: 1, wantStatus: core.HTTPStatusOK(), wantHeaderLength: 2},
		{name: "zero operation budget refuses before network execution", payload: []byte{0, 0xff}, method: exchange.MethodGet, maximum: 2, zeroPolicy: true, wantErr: core.ErrExchangeContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Uint64
			handled := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				call, err := exchange.NewSocketServerCall(writer, request)
				if err == nil {
					err = exchange.WriteBounded(exchange.BoundedWriteCall{Call: call, Response: exchange.ServerBoundedResponse{Body: tc.payload, ContentType: core.HTTPMediaTypeOctetStream(), Status: core.HTTPStatusOK()}})
				}
				select {
				case handled <- err:
				default:
				}
			}))
			t.Cleanup(func() {
				server.CloseClientConnections()
				server.Close()
				if got := calls.Load(); got != tc.wantCalls {
					t.Errorf("final server effects = %d, want %d", got, tc.wantCalls)
				}
			})
			client := server.Client()
			t.Cleanup(client.CloseIdleConnections)
			policy := singleAttemptOperationPolicy(t)
			policy.OperationTimeout = mustDurationMilliseconds(t, 10_000)
			policy.AttemptTimeout = mustDurationMilliseconds(t, 10_000)
			if tc.zeroPolicy {
				policy = exchange.OperationPolicy{}
			}
			got, gotErr := exchange.SendNoBodyBounded(exchange.NoBodyBoundedCall{
				Context: t.Context(), Client: mustExchangeClient(t, client),
				Request: exchange.NoBodyBoundedRequest{Target: mustEndpoint(t, server.URL), Semantics: exchange.RequestSemantics{Method: tc.method, Replay: exchange.ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK(), ExpectedResponseContentType: core.HTTPMediaTypeOctetStream(), CaptureHeaders: exchange.HeaderSelection{Names: []core.HTTPHeaderName{core.HTTPHeaderContentLength()}}},
				Policy:  exchange.NoBodyBoundedPolicy{Operation: policy},
			})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("HTTP result error = %v, want %v", gotErr, tc.wantErr)
			}
			if !bytes.Equal(got.Body, tc.wantBody) || (tc.wantBody == nil && got.Body != nil) || got.Metadata.Bytes.Uint64() != uint64(len(tc.wantBody)) || got.Metadata.Attempts != tc.wantAttempts || got.Metadata.Status != tc.wantStatus {
				t.Fatalf("HTTP response = (%x,%+v), want body %x, status %v and %d attempts", got.Body, got.Metadata, tc.wantBody, tc.wantStatus, tc.wantAttempts)
			}
			if tc.wantCalls == 0 {
				if got.Metadata.Headers.Values != nil {
					t.Fatalf("unexecuted header evidence = %v, want nil", got.Metadata.Headers.Values)
				}
				select {
				case event := <-handled:
					t.Fatalf("unexecuted handler emitted %v, want no event", event)
				default:
				}
				return
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("retained HTTP observation validation = %v, want nil", err)
			}
			if len(got.Metadata.Headers.Values) != 1 || got.Metadata.Headers.Values[0].Name != core.HTTPHeaderContentLength() || len(got.Metadata.Headers.Values[0].Values) != 1 {
				t.Fatalf("captured extent = %v, want exactly one Content-Length", got.Metadata.Headers.Values)
			}
			value, err := got.Metadata.Headers.Values[0].Values[0].Value()
			if err != nil || value != strconv.FormatUint(tc.wantHeaderLength, 10) {
				t.Fatalf("declared extent = (%q,%v), want %d", value, err, tc.wantHeaderLength)
			}
			select {
			case err := <-handled:
				if err != nil {
					t.Fatalf("server write error = %v, want nil", err)
				}
			case <-exchangeFixtureBackstop(t, 10*time.Second):
				t.Fatalf("server write completion absent, want completed handler; owned completion channel=%p", handled)
			}
		})
	}
}
