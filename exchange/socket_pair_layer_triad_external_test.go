package exchange_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestSocketPairLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive one shared contract carries typed request and response", func(t *testing.T) {
		t.Parallel()

		contract := socketPairContract(t, "/socket", exchange.ReplaySingleAttempt)
		serverSocket, err := exchange.NewServerSocket(contract)
		if err != nil {
			t.Fatalf("exchange.NewServerSocket() error = %v, want nil", err)
		}
		observed := make(chan error, 1)
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			call, callErr := exchange.NewSocketServerCall(writer, request)
			if callErr != nil {
				observed <- callErr
				return
			}
			received, receiveErr := exchange.ReceiveSocketJSON[transportDocument, *transportDocument](serverSocket, call)
			if receiveErr != nil {
				observed <- receiveErr
				return
			}
			if received.Body == nil || received.Body.Message != "candidate" {
				observed <- core.ErrExchangeContract
				return
			}
			observed <- exchange.WriteSocketJSON(serverSocket, call, transportDocument{Message: "accepted"})
		}))
		defer server.Close()
		clientSocket := socketPairClient(t, server, contract)
		response, gotErr := exchange.SendSocketJSON[transportDocument, transportDocument](t.Context(), clientSocket, transportDocument{Message: "candidate"})
		if gotErr != nil || response.Body.Message != "accepted" {
			t.Fatalf("exchange.SendSocketJSON() = (%+v, %v), want accepted and nil", response, gotErr)
		}
		if serverErr := <-observed; serverErr != nil {
			t.Fatalf("socket server error = %v, want nil", serverErr)
		}
	})

	t.Run("positive replay identity is bound at both socket sides", func(t *testing.T) {
		t.Parallel()

		contract := socketPairContract(t, "/mutation", exchange.ReplayIdempotencyKey)
		serverSocket, err := exchange.NewServerSocket(contract)
		if err != nil {
			t.Fatalf("exchange.NewServerSocket() error = %v, want nil", err)
		}
		observed := make(chan error, 1)
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			call, callErr := exchange.NewSocketServerCall(writer, request)
			if callErr != nil {
				observed <- callErr
				return
			}
			received, receiveErr := exchange.ReceiveReplayBoundSocketJSON[replayBoundDocument, *replayBoundDocument](serverSocket, call)
			if receiveErr != nil {
				observed <- receiveErr
				return
			}
			if received.Body == nil || received.Body.Operation != "operation-123" || received.IdempotencyKey.String() != "operation-123" {
				observed <- core.ErrExchangeIdempotencyBinding
				return
			}
			observed <- exchange.WriteSocketJSON(serverSocket, call, transportDocument{Message: "mutated"})
		}))
		defer server.Close()
		clientSocket := socketPairClient(t, server, contract)
		response, gotErr := exchange.SendReplayBoundSocketJSON[replayBoundDocument, transportDocument](t.Context(), clientSocket, replayBoundDocument{Operation: "operation-123"})
		if gotErr != nil || response.Body.Message != "mutated" {
			t.Fatalf("exchange.SendReplayBoundSocketJSON() = (%+v, %v), want mutated and nil", response, gotErr)
		}
		if serverErr := <-observed; serverErr != nil {
			t.Fatalf("replay socket server error = %v, want nil", serverErr)
		}
	})

	t.Run("negative invalid contracts construct neither socket side", func(t *testing.T) {
		t.Parallel()

		client, clientErr := exchange.NewClientSocket(exchange.ClientSocketConfiguration{})
		server, serverErr := exchange.NewServerSocket(exchange.JSONSocketContract{})
		if !errors.Is(clientErr, core.ErrExchangeContract) || client.Validate() == nil ||
			!errors.Is(serverErr, core.ErrExchangeContract) || server.Validate() == nil {
			t.Fatalf("zero socket constructors = (%v, %v, %v, %v), want two zero values and typed refusals", client, clientErr, server, serverErr)
		}
	})
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

func socketPairClient(t testing.TB, server *httptest.Server, contract exchange.JSONSocketContract) exchange.ClientSocket {
	t.Helper()
	configuration := exchange.ClientSocketConfiguration{
		Target: mustEndpoint(t, server.URL+contract.Path.String()), Client: mustExchangeClient(t, server.Client()), Contract: contract, Operation: singleAttemptOperationPolicy(t),
	}
	socket, err := exchange.NewClientSocket(configuration)
	if err != nil {
		t.Fatalf("exchange.NewClientSocket() error = %v, want nil", err)
	}
	return socket
}
