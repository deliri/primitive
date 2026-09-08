package exchange_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// Request entry is synchronized before shutdown: these rows cannot pass by
// closing an idle listener before Go has registered an active connection.
func TestServerRuntimeActiveConnectionOwnershipTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name               string
		cancelGraceful     bool
		completeActive     bool
		wantClientBoundary error
		wantShutdown       error
		wantRequestCause   error
		wantClientCause    error
		wantRequests       int64
		wantMetadata       exchange.ResponseMetadata
	}{
		{name: "force close cancels the active Go request", wantRequestCause: context.Canceled, wantClientCause: io.EOF, wantClientBoundary: core.ErrExchangeTransport, wantRequests: 1},
		{name: "canceled graceful drain preserves active request until force close", cancelGraceful: true, wantShutdown: context.Canceled, wantRequestCause: context.Canceled, wantClientCause: io.EOF, wantClientBoundary: core.ErrExchangeTransport, wantRequests: 1},
		{name: "graceful completion retains empty successful response without force close", completeActive: true, wantRequests: 1, wantMetadata: exchange.ResponseMetadata{Status: core.HTTPStatusOK(), Attempts: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			backstop := exchangeFixtureBackstop(t, 10*time.Second)
			entered := make(chan context.Context, 1)
			handlerDone := make(chan struct{})
			abort, release := context.WithCancel(t.Context())
			var handlerCause error
			var requests atomic.Int64
			handler := http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
				if requests.Add(1) != 1 {
					return
				}
				entered <- request.Context()
				select {
				case <-request.Context().Done():
					handlerCause = context.Cause(request.Context())
				case <-abort.Done():
				}
				close(handlerDone)
			})
			runtime, err := exchange.NewServerRuntime(serverRuntimeConfiguration(t, "127.0.0.1:0"), handler)
			if err != nil {
				t.Fatal(err)
			}
			serveDone := make(chan struct{})
			var serveErr error
			t.Cleanup(func() {
				release()
				if err := runtime.Close(); err != nil {
					t.Errorf("cleanup close = %v", err)
				}
				select {
				case <-serveDone:
				case <-exchangeFixtureBackstop(t, testDeadlockBackstop):
					t.Errorf("serving goroutine did not join; owned completion channel=%p", serveDone)
				}
			})
			go func() { serveErr = runtime.Serve(); close(serveDone) }()
			select {
			case err := <-runtime.Ready():
				if err != nil {
					t.Fatal(err)
				}
			case <-backstop:
				t.Fatalf("listener acquisition did not finish; readiness channel=%p", runtime.Ready())
			}
			address, err := runtime.Address()
			if err != nil {
				t.Fatal(err)
			}
			transport := &http.Transport{Proxy: nil}
			t.Cleanup(transport.CloseIdleConnections)
			client := mustExchangeClient(t, &http.Client{Transport: transport})
			requestContext, cancel := context.WithCancel(t.Context())
			call := exchange.NoBodyBoundedCall{
				Context: requestContext, Client: client,
				Request: exchange.NoBodyBoundedRequest{Target: mustEndpoint(t, "http://"+address.String()+"/"), Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK()},
				Policy:  exchange.NoBodyBoundedPolicy{Operation: singleAttemptOperationPolicy(t), ResponseBodyLimit: mustByteCount(t, 1)},
			}
			if err := call.Validate(); err != nil {
				cancel()
				t.Fatal(err)
			}
			clientDone := make(chan struct{})
			var response exchange.BoundedResponse
			var clientErr error
			t.Cleanup(func() {
				cancel()
				select {
				case <-clientDone:
				case <-exchangeFixtureBackstop(t, testDeadlockBackstop):
					t.Errorf("client goroutine did not join; owned completion channel=%p", clientDone)
				}
			})
			go func() { response, clientErr = exchange.SendNoBodyBounded(call); close(clientDone) }()
			var active context.Context
			select {
			case active = <-entered:
			case <-clientDone:
				t.Fatalf("client returned before request entry: %v", clientErr)
			case <-backstop:
				t.Fatalf("request never reached Go handler; owned completion channel=%p", entered)
			}
			t.Cleanup(func() {
				release()
				if err := runtime.Close(); err != nil {
					t.Errorf("active close = %v", err)
				}
				select {
				case <-handlerDone:
				case <-exchangeFixtureBackstop(t, testDeadlockBackstop):
					t.Errorf("handler goroutine did not join; owned completion channel=%p", handlerDone)
				}
			})
			if tc.cancelGraceful {
				ctx, stop := context.WithCancel(t.Context())
				stop()
				err := runtime.Shutdown(ctx)
				if !errors.Is(err, tc.wantShutdown) || !errors.Is(err, core.ErrExchangeTransport) {
					t.Fatalf("canceled drain = %v, want retained cancellation and transport identity", err)
				}
				if active.Err() != nil {
					t.Fatalf("graceful shutdown canceled active request: %v", active.Err())
				}
				select {
				case <-handlerDone:
					t.Fatalf("graceful shutdown killed active handler; owned completion channel=%p", handlerDone)
				default:
				}
			}
			if tc.completeActive {
				release()
				duration, err := temporal.NewDuration(testDeadlockBackstop)
				if err != nil {
					t.Fatal(err)
				}
				drain, stop, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: duration})
				if err != nil {
					t.Fatal(err)
				}
				err = runtime.Shutdown(drain)
				stop()
				if err != nil {
					t.Fatalf("graceful shutdown=%v,want completed drain", err)
				}
			} else if err := runtime.Close(); err != nil {
				t.Fatalf("force close = %v, want nil", err)
			}
			select {
			case <-handlerDone:
			case <-backstop:
				t.Fatalf("force close did not cancel active handler; owned completion channel=%p", handlerDone)
			}
			if !errors.Is(handlerCause, tc.wantRequestCause) {
				t.Fatalf("handler cause = %v, want %v", handlerCause, tc.wantRequestCause)
			}
			select {
			case <-clientDone:
			case <-backstop:
				t.Fatalf("force close did not release client; owned completion channel=%p", clientDone)
			}
			if !errors.Is(clientErr, tc.wantClientBoundary) || !errors.Is(clientErr, tc.wantClientCause) || response.Metadata.Status != tc.wantMetadata.Status || response.Body != nil || response.Metadata.Bytes != tc.wantMetadata.Bytes || response.Metadata.Attempts != tc.wantMetadata.Attempts || len(response.Metadata.Headers.Values) != 0 {
				t.Fatalf("closed connection observation = (%+v,%v), want exact required transport cause and response evidence", response, clientErr)
			}
			if got := requests.Load(); got != tc.wantRequests {
				t.Fatalf("handler executions = %d, want %d", got, tc.wantRequests)
			}
			select {
			case <-serveDone:
			case <-backstop:
				t.Fatalf("force close did not end Serve; owned completion channel=%p", serveDone)
			}
			if serveErr != nil {
				t.Fatalf("Serve after owned close = %v, want nil", serveErr)
			}
			if retained, err := runtime.Address(); err != nil || retained != address {
				t.Fatalf("retained acquisition = (%v,%v), want (%v,nil)", retained, err, address)
			}
		})
	}
}
