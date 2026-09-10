package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type boundedServerObservation struct {
	receiveErr    error
	writeErr      error
	contentType   string
	body          []byte
	contentLength int64
}

func TestBoundedByteTransportLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact bounded body crosses both real HTTP directions unchanged", func(t *testing.T) {
		t.Parallel()

		body := bytes.Repeat(
			[]byte{0x13, 0x57, 0x9b, 0xdf},
			2*exchange.TransferBufferBytes,
		)
		ok := mustHTTPStatus(t, http.StatusOK)
		observed := make(chan boundedServerObservation, 1)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			call := socketServerCallFrom(t, writer, request)
			received, receiveErr := exchange.ReceiveBounded(
				exchange.BoundedReceiveCall{
					Call: call,
					Route: exchange.RouteSemantics{
						Method: exchange.MethodPost,
						Replay: exchange.ReplaySingleAttempt,
					},

					ExpectedContentType: core.HTTPMediaTypeOctetStream(),
				},
			)
			var writeErr error
			if receiveErr == nil {
				writeErr = exchange.WriteBounded(
					exchange.BoundedWriteCall{
						Call: call,
						Response: exchange.ServerBoundedResponse{
							Body:        received.Body,
							ContentType: core.HTTPMediaTypeOctetStream(),
							Status:      ok,
						},
					},
				)
			}
			observed <- boundedServerObservation{
				receiveErr:    receiveErr,
				writeErr:      writeErr,
				body:          received.Body,
				contentLength: request.ContentLength,
				contentType: request.Header.Get(
					core.HTTPHeaderContentType().String(),
				),
			}
		}))
		defer server.Close()

		got, gotErr := exchange.SendBounded(
			exchange.BoundedCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.BoundedRequest{
					Target: mustEndpoint(t, server.URL),
					Body:   body,
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodPost,
						Replay: exchange.ReplaySingleAttempt,
					},
					RequestContentType:          core.HTTPMediaTypeOctetStream(),
					ExpectedResponseContentType: core.HTTPMediaTypeOctetStream(),
					ExpectedStatus:              ok,
				},
				Policy: exchange.BoundedPolicy{
					Operation: singleAttemptOperationPolicy(t),
				},
			},
		)
		if gotErr != nil {
			t.Fatalf("SendBounded() error = %v, want nil", gotErr)
		}
		if !bytes.Equal(got.Body, body) ||
			got.Metadata.Bytes.Uint64() != uint64(len(body)) {
			t.Fatalf(
				"SendBounded() body parity/bytes = (%t, %d), want (true, %d)",
				bytes.Equal(got.Body, body),
				got.Metadata.Bytes.Uint64(),
				len(body),
			)
		}
		select {
		case serverGot := <-observed:
			if serverGot.receiveErr != nil || serverGot.writeErr != nil {
				t.Fatalf(
					"bounded server receive/write errors = (%v, %v), want (nil, nil)",
					serverGot.receiveErr,
					serverGot.writeErr,
				)
			}
			if !bytes.Equal(serverGot.body, body) ||
				serverGot.contentLength != int64(len(body)) ||
				serverGot.contentType != core.HTTPMediaTypeOctetStream().String() {
				t.Fatalf(
					"bounded server parity/length/type = (%t, %d, %q), want (true, %d, %q)",
					bytes.Equal(serverGot.body, body),
					serverGot.contentLength,
					serverGot.contentType,
					len(body),
					core.HTTPMediaTypeOctetStream(),
				)
			}
		case <-exchangeFixtureBackstop(t, testDeadlockBackstop):
			t.Fatalf(
				"bounded server observation = absent after %v, want one completed observation",
				testDeadlockBackstop,
			)
		}
	})

	t.Run("one byte above former cutoff is delivered in both directions", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Uint64
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			calls.Add(1)
			call := socketServerCallFrom(t, writer, request)
			received, err := exchange.ReceiveBounded(exchange.BoundedReceiveCall{Call: call, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}, ExpectedContentType: core.HTTPMediaTypeOctetStream()})
			if err != nil {
				t.Errorf("receive=%v,want complete bytes", err)
				return
			}
			if err := exchange.WriteBounded(exchange.BoundedWriteCall{Call: call, Response: exchange.ServerBoundedResponse{Body: received.Body, ContentType: core.HTTPMediaTypeOctetStream(), Status: core.HTTPStatusOK()}}); err != nil {
				t.Errorf("echo=%v,want complete bytes", err)
			}
		}))
		defer server.Close()

		ok := mustHTTPStatus(t, http.StatusOK)
		body := bytes.Repeat([]byte{0xa5}, exchange.TransferBufferBytes+1)
		got, gotErr := exchange.SendBounded(
			exchange.BoundedCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.BoundedRequest{
					Target: mustEndpoint(t, server.URL),
					Body:   body,
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodPost,
						Replay: exchange.ReplaySingleAttempt,
					},
					RequestContentType:          core.HTTPMediaTypeOctetStream(),
					ExpectedResponseContentType: core.HTTPMediaTypeOctetStream(),
					ExpectedStatus:              ok,
				},
				Policy: exchange.BoundedPolicy{
					Operation: singleAttemptOperationPolicy(t),
				},
			},
		)
		if gotErr != nil || calls.Load() != 1 || !bytes.Equal(got.Body, body) || got.Metadata.Attempts != 1 || got.Metadata.Bytes.Uint64() != uint64(len(body)) {
			t.Fatalf("whole-byte round trip=%d bytes/%v, calls=%d; want all %d bytes and one call", len(got.Body), gotErr, calls.Load(), len(body))
		}
	})

	t.Run("neutral empty body remains structurally present without fabricated bytes", func(t *testing.T) {
		t.Parallel()

		ok := mustHTTPStatus(t, http.StatusOK)
		observed := make(chan boundedServerObservation, 1)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			call := socketServerCallFrom(t, writer, request)
			received, receiveErr := exchange.ReceiveBounded(
				exchange.BoundedReceiveCall{
					Call: call,
					Route: exchange.RouteSemantics{
						Method: exchange.MethodPost,
						Replay: exchange.ReplaySingleAttempt,
					},

					ExpectedContentType: core.HTTPMediaTypeOctetStream(),
				},
			)
			writeErr := exchange.WriteBounded(exchange.BoundedWriteCall{
				Call: call,
				Response: exchange.ServerBoundedResponse{
					Body:        []byte{},
					ContentType: core.HTTPMediaTypeOctetStream(),
					Status:      ok,
				},
			})
			observed <- boundedServerObservation{
				receiveErr:    receiveErr,
				writeErr:      writeErr,
				body:          received.Body,
				contentLength: request.ContentLength,
				contentType: request.Header.Get(
					core.HTTPHeaderContentType().String(),
				),
			}
		}))
		defer server.Close()

		got, gotErr := exchange.SendBounded(
			exchange.BoundedCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.BoundedRequest{
					Target: mustEndpoint(t, server.URL),
					Body:   []byte{},
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodPost,
						Replay: exchange.ReplaySingleAttempt,
					},
					RequestContentType:          core.HTTPMediaTypeOctetStream(),
					ExpectedResponseContentType: core.HTTPMediaTypeOctetStream(),
					ExpectedStatus:              ok,
				},
				Policy: exchange.BoundedPolicy{
					Operation: singleAttemptOperationPolicy(t),
				},
			},
		)
		if gotErr != nil || len(got.Body) != 0 ||
			got.Metadata.Bytes.Uint64() != 0 {
			t.Fatalf(
				"SendBounded(empty body) = (%+v, %v), want zero bytes and nil",
				got,
				gotErr,
			)
		}
		select {
		case serverGot := <-observed:
			if serverGot.receiveErr != nil || serverGot.writeErr != nil {
				t.Fatalf(
					"empty bounded server receive/write errors = (%v, %v), want (nil, nil)",
					serverGot.receiveErr,
					serverGot.writeErr,
				)
			}
			if len(serverGot.body) != 0 ||
				serverGot.contentLength != 0 ||
				serverGot.contentType != core.HTTPMediaTypeOctetStream().String() {
				t.Fatalf(
					"empty bounded server body/length/type = (%d, %d, %q), want (0, 0, %q)",
					len(serverGot.body),
					serverGot.contentLength,
					serverGot.contentType,
					core.HTTPMediaTypeOctetStream(),
				)
			}
		case <-exchangeFixtureBackstop(t, testDeadlockBackstop):
			t.Fatalf(
				"empty bounded server observation = absent after %v, want one completed observation",
				testDeadlockBackstop,
			)
		}
	})
}

func TestAggregateUnexpectedStatusStillRejectsTransformingContentCoding(t *testing.T) {
	t.Parallel()

	ok := mustHTTPStatus(t, http.StatusOK)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set(core.HTTPHeaderContentEncoding().String(), "br")
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte("compressed provider error bytes"))
	}))
	defer server.Close()

	got, gotErr := exchange.SendNoBodyBounded(exchange.NoBodyBoundedCall{
		Context: context.Background(),
		Client:  mustExchangeClient(t, server.Client()),
		Request: exchange.NoBodyBoundedRequest{
			Target:         mustEndpoint(t, server.URL),
			Semantics:      exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt},
			ExpectedStatus: ok,
		},
		Policy: exchange.NoBodyBoundedPolicy{
			Operation: singleAttemptOperationPolicy(t),
		},
	})
	if !errors.Is(gotErr, core.ErrExchangeResponse) || !errors.Is(gotErr, core.ErrExchangeContentType) {
		t.Fatalf("SendNoBodyBounded(unexpected br response) error = %v, want %v and %v", gotErr, core.ErrExchangeResponse, core.ErrExchangeContentType)
	}
	if len(got.Body) != 0 {
		t.Fatalf("SendNoBodyBounded(unexpected br response) body = %q, want no captured transformed bytes", got.Body)
	}
}
