package exchange_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestSocketPairLayerTriad(t *testing.T) {
	t.Parallel()
	intent := replayBoundDocument{Operation: "operation-A"}
	reply := transportDocument{Message: "accepted-fact"}
	cases := []struct {
		name                            string
		replay                          exchange.ReplayMode
		intent                          replayBoundDocument
		zeroContract, dormant           bool
		wantCalls                       int64
		wantKey                         string
		wantConstructorErr, wantSendErr error
	}{
		{name: "single attempt crosses the shared agreement without inventing replay identity", replay: exchange.ReplaySingleAttempt, intent: intent, wantCalls: 1},
		{name: "bound replay identity survives both independently constructed socket sides", replay: exchange.ReplayIdempotencyKey, intent: intent, wantCalls: 1, wantKey: intent.Operation},
		{name: "invalid caller document cannot cause an HTTP effect", replay: exchange.ReplaySingleAttempt, wantSendErr: core.ErrExchangeRequest},
		{name: "unbound agreement constructs neither side", replay: exchange.ReplaySingleAttempt, intent: intent, zeroContract: true, wantConstructorErr: core.ErrExchangeContract},
		{name: "valid dormant socket construction performs no request", replay: exchange.ReplaySingleAttempt, intent: intent, dormant: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			contract := socketPairContract(t, "/socket", tc.replay)
			if tc.zeroContract {
				contract = exchange.JSONSocketContract{}
			}
			serverSocket, serverErr := exchange.NewServerSocket(contract)
			type observation struct {
				intent replayBoundDocument
				key    exchange.IdempotencyKey
				err    error
			}
			observed := make(chan observation, 1)
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if calls.Add(1) != 1 {
					return
				}
				call, err := exchange.NewSocketServerCall(writer, request)
				if err != nil {
					observed <- observation{err: err}
					return
				}
				var received exchange.Received[*replayBoundDocument]
				if tc.replay == exchange.ReplayIdempotencyKey {
					received, err = exchange.ReceiveReplayBoundSocketJSON[replayBoundDocument, *replayBoundDocument](serverSocket, call)
				} else {
					received, err = exchange.ReceiveSocketJSON[replayBoundDocument, *replayBoundDocument](serverSocket, call)
				}
				fact := observation{key: received.IdempotencyKey, err: err}
				if received.Body != nil {
					fact.intent = *received.Body
				}
				if err == nil {
					fact.err = exchange.WriteSocketJSON(serverSocket, call, reply)
				}
				observed <- fact
			}))
			defer server.Close()
			clientSocket, clientErr := exchange.NewClientSocket(exchange.ClientSocketConfiguration{Target: mustEndpoint(t, server.URL+"/socket"), Client: mustExchangeClient(t, server.Client()), Contract: contract, Operation: singleAttemptOperationPolicy(t)})
			if !errors.Is(serverErr, tc.wantConstructorErr) || !errors.Is(clientErr, tc.wantConstructorErr) {
				t.Fatalf("socket constructors=(%v,%v), want %v", serverErr, clientErr, tc.wantConstructorErr)
			}
			if tc.wantConstructorErr != nil {
				if !reflect.ValueOf(clientSocket).IsZero() || serverSocket != (exchange.ServerSocket{}) || calls.Load() != 0 {
					t.Fatalf("refused client/server/calls=%+v/%+v/%d, want zero/zero/0", clientSocket, serverSocket, calls.Load())
				}
				return
			}
			if clientSocket.Validate() != nil || serverSocket.Validate() != nil {
				t.Fatalf("client/server validation=%v/%v, want nil/nil", clientSocket.Validate(), serverSocket.Validate())
			}
			var response exchange.JSONResponse[transportDocument]
			var sendErr error
			if !tc.dormant {
				if tc.replay == exchange.ReplayIdempotencyKey {
					response, sendErr = exchange.SendReplayBoundSocketJSON[replayBoundDocument, transportDocument](t.Context(), clientSocket, tc.intent)
				} else {
					response, sendErr = exchange.SendSocketJSON[replayBoundDocument, transportDocument](t.Context(), clientSocket, tc.intent)
				}
			}
			if !errors.Is(sendErr, tc.wantSendErr) || calls.Load() != tc.wantCalls {
				t.Fatalf("socket send=(%v,%d requests), want (%v,%d)", sendErr, calls.Load(), tc.wantSendErr, tc.wantCalls)
			}
			if tc.wantCalls == 0 {
				if response.Body != (transportDocument{}) || response.Metadata.Status != (core.HTTPStatusCode{}) || response.Metadata.Attempts != 0 || response.Metadata.Bytes != (core.ByteLength{}) || response.Metadata.Headers.Values != nil {
					t.Fatalf("unexecuted socket produced response %+v", response)
				}
				select {
				case fact := <-observed:
					t.Fatalf("unexecuted socket produced server observation %+v", fact)
				default:
				}
				return
			}
			if response.Body != reply || response.Metadata.Status != contract.SuccessStatus || response.Metadata.Attempts != 1 || response.Validate() != nil {
				t.Fatalf("socket response=%+v, want exact shared reply and one attempt", response)
			}
			select {
			case fact := <-observed:
				if fact.err != nil || fact.intent != tc.intent || fact.key.String() != tc.wantKey {
					t.Fatalf("paired server facts=%+v, want exact intent %+v and replay key %q", fact, tc.intent, tc.wantKey)
				}
			case <-exchangeFixtureBackstop(t, 10*time.Second):
				t.Fatalf("paired server observation did not arrive; owned completion channel=%p", observed)
			}
		})
	}
}

func socketPairContract(t testing.TB, path string, replay exchange.ReplayMode) exchange.JSONSocketContract {
	t.Helper()
	route, err := exchange.ParseSocketRoutePath(path)
	if err != nil {
		t.Fatalf("exchange.ParseSocketRoutePath(%q) error = %v, want nil", path, err)
	}
	status, err := exchange.HTTPStatusAccepted()
	if err != nil {
		t.Fatalf("exchange.HTTPStatusAccepted() error = %v, want nil", err)
	}
	contract := exchange.JSONSocketContract{
		Path: route, RequestBodyLimit: mustByteCount(t, 4*1024), ResponseBodyLimit: mustByteCount(t, 4*1024),
		SuccessStatus: status, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: replay},
	}
	if err := contract.Validate(); err != nil {
		t.Fatalf("JSONSocketContract.Validate() error = %v, want nil", err)
	}
	return contract
}
