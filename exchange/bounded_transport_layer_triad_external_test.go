package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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
		serverPolicy := exchange.ServerBoundedPolicy{
			RequestBodyLimit: mustByteCount(t, uint64(len(body))),
		}
		observed := make(chan boundedServerObservation, 1)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			received, receiveErr := exchange.ReceiveBounded(
				exchange.BoundedReceiveCall{
					Request: request,
					Route: exchange.RouteSemantics{
						Method: exchange.MethodPost,
						Replay: exchange.ReplaySingleAttempt,
					},
					Policy:              serverPolicy,
					ExpectedContentType: core.HTTPMediaTypeOctetStream(),
				},
			)
			var writeErr error
			if receiveErr == nil {
				writeErr = exchange.WriteBounded(
					exchange.BoundedWriteCall{
						Context: request.Context(),
						Writer:  writer,
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
					Operation:         singleAttemptOperationPolicy(t),
					RequestBodyLimit:  mustByteCount(t, uint64(len(body))),
					ResponseBodyLimit: mustByteCount(t, uint64(len(body))),
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
		case <-time.After(testDeadlockBackstop):
			t.Fatalf(
				"bounded server observation = absent after %v, want one completed observation",
				testDeadlockBackstop,
			)
		}
	})

	t.Run("negative one byte above the caller bound transmits no request", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Uint64
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			calls.Add(1)
			writer.WriteHeader(http.StatusOK)
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
					Operation:         singleAttemptOperationPolicy(t),
					RequestBodyLimit:  mustByteCount(t, exchange.TransferBufferBytes),
					ResponseBodyLimit: mustByteCount(t, exchange.TransferBufferBytes),
				},
			},
		)
		if !errors.Is(gotErr, core.ErrExchangeRequest) ||
			!errors.Is(gotErr, core.ErrExchangeBodyLimit) {
			t.Fatalf(
				"SendBounded(one over) error = %v, want %v and %v",
				gotErr,
				core.ErrExchangeRequest,
				core.ErrExchangeBodyLimit,
			)
		}
		if calls.Load() != 0 {
			t.Fatalf("bounded one-over server calls = %d, want 0", calls.Load())
		}
		if len(got.Body) != 0 || got.Metadata.Attempts != 0 {
			t.Fatalf("SendBounded(one over) response = %+v, want zero", got)
		}
	})

	t.Run("neutral empty body remains structurally present without fabricated bytes", func(t *testing.T) {
		t.Parallel()

		ok := mustHTTPStatus(t, http.StatusOK)
		serverPolicy := exchange.ServerBoundedPolicy{
			RequestBodyLimit: mustByteCount(t, 1),
		}
		observed := make(chan boundedServerObservation, 1)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			received, receiveErr := exchange.ReceiveBounded(
				exchange.BoundedReceiveCall{
					Request: request,
					Route: exchange.RouteSemantics{
						Method: exchange.MethodPost,
						Replay: exchange.ReplaySingleAttempt,
					},
					Policy:              serverPolicy,
					ExpectedContentType: core.HTTPMediaTypeOctetStream(),
				},
			)
			writeErr := exchange.WriteBounded(exchange.BoundedWriteCall{
				Context: request.Context(),
				Writer:  writer,
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
					Operation:         singleAttemptOperationPolicy(t),
					RequestBodyLimit:  mustByteCount(t, 1),
					ResponseBodyLimit: mustByteCount(t, 1),
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
		case <-time.After(testDeadlockBackstop):
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
			Operation:         singleAttemptOperationPolicy(t),
			ResponseBodyLimit: mustByteCount(t, 1024),
		},
	})
	if !errors.Is(gotErr, core.ErrExchangeResponse) || !errors.Is(gotErr, core.ErrExchangeContentType) {
		t.Fatalf("SendNoBodyBounded(unexpected br response) error = %v, want %v and %v", gotErr, core.ErrExchangeResponse, core.ErrExchangeContentType)
	}
	if len(got.Body) != 0 {
		t.Fatalf("SendNoBodyBounded(unexpected br response) body = %q, want no captured transformed bytes", got.Body)
	}
}
