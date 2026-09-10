package exchange_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const testJSONIngressLimitBytes = 128

// ingressObservation is what one real server boundary observed for one real
// request. It is filled inside the handler and read by the client goroutine.
type ingressObservation struct {
	err     error
	body    *transportDocument
	message string
	key     string
	bytes   uint64
}

// jsonIngressRequest is one hostile or admitted request crafted with net/http
// directly, because Exchange's own client cannot emit these shapes.
type jsonIngressRequest struct {
	method          string
	contentType     string
	contentEncoding string
	idempotencyKey  string
	body            []byte
	omitContentType bool
	duplicateType   bool
}

func startJSONIngressServer(
	t *testing.T,
	route exchange.RouteSemantics,
	observed chan<- ingressObservation,
) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		serverCall := socketServerCallFrom(t, writer, request)
		received, receiveErr := exchange.ReceiveJSON[
			transportDocument,
			*transportDocument,
		](exchange.JSONReceiveCall{
			Call:  serverCall,
			Route: route,
		})
		observation := ingressObservation{err: receiveErr, body: received.Body, key: received.IdempotencyKey.String()}
		if received.Body != nil {
			observation.message = received.Body.Message
		}
		observed <- observation
		writer.WriteHeader(http.StatusOK)
	}))
}

func sendRawRequest(
	t *testing.T,
	server *httptest.Server,
	input jsonIngressRequest,
) {
	t.Helper()

	var body io.Reader
	if input.body != nil {
		body = bytes.NewReader(input.body)
	}
	request, gotErr := http.NewRequestWithContext(
		context.Background(),
		input.method,
		server.URL,
		body,
	)
	if gotErr != nil {
		t.Fatalf("http.NewRequestWithContext(%q) setup error = %v, want nil", input.method, gotErr)
	}
	if !input.omitContentType {
		contentType := input.contentType
		if contentType == "" {
			contentType = mustHTTPMediaType(t, "application/json").String()
		}
		request.Header.Set(core.HTTPHeaderContentType().String(), contentType)
		if input.duplicateType {
			request.Header.Add(
				core.HTTPHeaderContentType().String(),
				mustHTTPMediaType(t, "application/json").String(),
			)
		}
	}
	if input.contentEncoding != "" {
		request.Header.Set(
			core.HTTPHeaderContentEncoding().String(),
			input.contentEncoding,
		)
	}
	if input.idempotencyKey != "" {
		request.Header.Set(
			core.HTTPHeaderIdempotencyKey().String(),
			input.idempotencyKey,
		)
	}
	response, gotSendErr := server.Client().Do(request)
	if gotSendErr != nil || response == nil || response.Body == nil {
		t.Fatalf("server.Client().Do() = (%v, %v), want a real response and nil", response, gotSendErr)
		return
	}
	_, gotDrainErr := io.Copy(io.Discard, response.Body)
	gotCloseErr := response.Body.Close()
	if gotDrainErr != nil || gotCloseErr != nil {
		t.Fatalf(
			"raw response drain/close errors = (%v, %v), want (nil, nil)",
			gotDrainErr,
			gotCloseErr,
		)
	}
}

func awaitIngressObservation(
	t *testing.T,
	observed <-chan ingressObservation,
) ingressObservation {
	t.Helper()

	select {
	case observation := <-observed:
		return observation
	case <-exchangeFixtureBackstop(t, testDeadlockBackstop):
		t.Fatalf(
			"server ingress observation = absent after %v, want one completed observation",
			testDeadlockBackstop,
		)
		return ingressObservation{}
	}
}

func TestJSONIngressGuardHostileTable(t *testing.T) {
	t.Parallel()

	admitted := []byte(`{"message":"candidate"}`)
	overLimit := append(
		[]byte(`{"message":"`),
		append(
			bytes.Repeat([]byte{'a'}, testJSONIngressLimitBytes),
			[]byte(`"}`)...,
		)...,
	)
	// wantAbsent pins cases whose admitted parent identity is too coarse on its
	// own: Core makes every exchange identity a child of ErrExchangeContract, so
	// a rejection must also prove which sibling guard did not fire.
	cases := []struct {
		wantErr     error
		wantAbsent  error
		name        string
		wantMessage string
		request     jsonIngressRequest
	}{
		{
			name:        "admitted canonical JSON body decodes",
			request:     jsonIngressRequest{method: http.MethodPost, body: admitted},
			wantMessage: "candidate",
		},
		{
			name: "admitted JSON parameter shares the required base",
			request: jsonIngressRequest{
				method:      http.MethodPost,
				contentType: "application/json; charset=utf-8",
				body:        admitted,
			},
			wantMessage: "candidate",
		},
		{
			name: "admitted identity content coding is transparent",
			request: jsonIngressRequest{
				method:          http.MethodPost,
				contentEncoding: identityContentCoding,
				body:            admitted,
			},
			wantMessage: "candidate",
		},
		{
			name:       "method outside the route contract is refused",
			request:    jsonIngressRequest{method: http.MethodPut, body: admitted},
			wantErr:    core.ErrExchangeRequest,
			wantAbsent: core.ErrExchangeContentType,
		},
		{
			name: "absent content type is refused",
			request: jsonIngressRequest{
				method:          http.MethodPost,
				omitContentType: true,
				body:            admitted,
			},
			wantErr: core.ErrExchangeContentType,
		},
		{
			name: "foreign content type is refused",
			request: jsonIngressRequest{
				method:      http.MethodPost,
				contentType: mustHTTPMediaType(t, "text/plain").String(),
				body:        admitted,
			},
			wantErr: core.ErrExchangeContentType,
		},
		{
			name: "duplicate content type is refused as ambiguous",
			request: jsonIngressRequest{
				method:        http.MethodPost,
				duplicateType: true,
				body:          admitted,
			},
			wantErr: core.ErrExchangeContentType,
		},
		{
			name: "malformed content type is refused",
			request: jsonIngressRequest{
				method:      http.MethodPost,
				contentType: "application//json",
				body:        admitted,
			},
			wantErr: core.ErrExchangeContentType,
		},
		{
			name: "transforming content coding is refused",
			request: jsonIngressRequest{
				method:          http.MethodPost,
				contentEncoding: "gzip",
				body:            admitted,
			},
			wantErr: core.ErrExchangeContentType,
		},
		{
			name: "unsolicited idempotency key is refused",
			request: jsonIngressRequest{
				method:         http.MethodPost,
				idempotencyKey: "unsolicited",
				body:           admitted,
			},
			wantErr:    core.ErrExchangeRequest,
			wantAbsent: core.ErrJSONContract,
		},
		{
			name: "value beyond former route cutoff is complete",
			request: jsonIngressRequest{
				method: http.MethodPost,
				body:   overLimit,
			},
			wantMessage: strings.Repeat("a", testJSONIngressLimitBytes),
		},
		{
			name: "unknown JSON member is refused by the strict grammar",
			request: jsonIngressRequest{
				method: http.MethodPost,
				body:   []byte(`{"message":"candidate","extra":1}`),
			},
			wantErr: core.ErrJSONContract,
		},
		{
			name: "structurally invalid body is refused before validation",
			request: jsonIngressRequest{
				method: http.MethodPost,
				body:   []byte(`{"message":`),
			},
			wantErr: core.ErrJSONContract,
		},
		{
			name: "body failing the owning type contract is refused",
			request: jsonIngressRequest{
				method: http.MethodPost,
				body:   []byte(`{"message":""}`),
			},
			wantErr: errTransportDocumentContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			observed := make(chan ingressObservation, 1)
			server := startJSONIngressServer(
				t,
				exchange.RouteSemantics{
					Method: exchange.MethodPost,
					Replay: exchange.ReplaySingleAttempt,
				},
				observed,
			)
			defer server.Close()

			sendRawRequest(t, server, tc.request)
			got := awaitIngressObservation(t, observed)
			if !errors.Is(got.err, tc.wantErr) {
				t.Fatalf("exchange.ReceiveJSON() error = %v, want %v", got.err, tc.wantErr)
			}
			if tc.wantAbsent != nil && errors.Is(got.err, tc.wantAbsent) {
				t.Fatalf(
					"exchange.ReceiveJSON() error = %v, want %v absent",
					got.err,
					tc.wantAbsent,
				)
			}
			if tc.wantErr != nil {
				if got.body != nil || got.message != "" || got.key != "" {
					t.Fatalf("refused ingress leaked = %+v, want zero", got)
				}
				return
			}
			if got.body == nil || got.message != tc.wantMessage {
				t.Fatalf("received message = %q, want %q", got.message, tc.wantMessage)
			}
		})
	}
}

func TestNoBodyIngressLayerTriad(t *testing.T) {
	t.Parallel()

	route := exchange.RouteSemantics{
		Method: exchange.MethodGet,
		Replay: exchange.ReplaySafe,
	}
	newServer := func(observed chan<- ingressObservation) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			serverCall := socketServerCallFrom(t, writer, request)
			received, receiveErr := exchange.ReceiveNoBody(
				exchange.NoBodyReceiveCall{Call: serverCall, Route: route},
			)
			observed <- ingressObservation{
				err: receiveErr, key: received.IdempotencyKey.String(),
			}
			writer.WriteHeader(http.StatusOK)
		}))
	}

	t.Run("positive a body-absent request yields the typed no-body value", func(t *testing.T) {
		t.Parallel()

		observed := make(chan ingressObservation, 1)
		server := newServer(observed)
		defer server.Close()

		sendRawRequest(t, server, jsonIngressRequest{
			method: http.MethodGet, omitContentType: true,
		})
		if got := awaitIngressObservation(t, observed); got.err != nil || got.key != "" {
			t.Fatalf("exchange.ReceiveNoBody() = %+v, want no error and no key", got)
		}
	})

	t.Run("negative a body-bearing request is refused before any read", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			request jsonIngressRequest
		}{
			{
				name: "declared body bytes",
				request: jsonIngressRequest{
					method: http.MethodGet, body: []byte(`{"message":"smuggled"}`),
				},
			},
			{
				name:    "content type without bytes",
				request: jsonIngressRequest{method: http.MethodGet},
			},
			{
				name: "content encoding without bytes",
				request: jsonIngressRequest{
					method:          http.MethodGet,
					omitContentType: true,
					contentEncoding: identityContentCoding,
				},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				observed := make(chan ingressObservation, 1)
				server := newServer(observed)
				defer server.Close()

				sendRawRequest(t, server, tc.request)
				got := awaitIngressObservation(t, observed)
				if !errors.Is(got.err, core.ErrExchangeRequest) ||
					!errors.Is(got.err, core.ErrExchangeContract) {
					t.Fatalf(
						"exchange.ReceiveNoBody(body bearing) error = %v, want %v and %v",
						got.err,
						core.ErrExchangeRequest,
						core.ErrExchangeContract,
					)
				}
				if got.key != "" {
					t.Fatalf("refused no-body ingress key = %q, want empty", got.key)
				}
			})
		}
	})

	t.Run("neutral every declared key route observes the real request key", func(t *testing.T) {
		t.Parallel()

		for _, replay := range [...]exchange.ReplayMode{
			exchange.ReplayIdempotencyKey,
			exchange.ReplaySingleAttemptWithIdempotencyKey,
		} {
			t.Run(replay.String(), func(t *testing.T) {
				t.Parallel()

				keyRoute := exchange.RouteSemantics{Method: exchange.MethodPost, Replay: replay}
				observed := make(chan ingressObservation, 1)
				server := startJSONIngressServer(t, keyRoute, observed)
				defer server.Close()

				sendRawRequest(t, server, jsonIngressRequest{
					method:         http.MethodPost,
					idempotencyKey: "01JD-EXCHANGE-KEY",
					body:           []byte(`{"message":"once"}`),
				})
				got := awaitIngressObservation(t, observed)
				if got.err != nil || got.key != "01JD-EXCHANGE-KEY" {
					t.Fatalf("%s ingress = (%v, %q), want (nil, %q)", replay, got.err, got.key, "01JD-EXCHANGE-KEY")
				}

				missingObserved := make(chan ingressObservation, 1)
				missingServer := startJSONIngressServer(t, keyRoute, missingObserved)
				defer missingServer.Close()
				sendRawRequest(t, missingServer, jsonIngressRequest{
					method: http.MethodPost, body: []byte(`{"message":"once"}`),
				})
				gotMissing := awaitIngressObservation(t, missingObserved)
				if !errors.Is(gotMissing.err, core.ErrExchangeRequest) {
					t.Fatalf("%s without a key error = %v, want %v", replay, gotMissing.err, core.ErrExchangeRequest)
				}
			})
		}
	})
}

func TestStreamIngressExtentLayerTriad(t *testing.T) {
	t.Parallel()
	const formerLimit = 3 * exchange.TransferBufferBytes
	cases := []struct {
		name    string
		size    int
		chunked bool
	}{
		{name: "below former cutoff", size: formerLimit - 1},
		{name: "at former cutoff", size: formerLimit},
		{name: "declared above former cutoff", size: formerLimit + 1},
		{name: "chunked above former cutoff", size: formerLimit + 1, chunked: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := bytes.Repeat([]byte{0xa5}, tc.size)
			want := sha256.Sum256(body)
			path := filepath.Join(t.TempDir(), "received.bin")
			destination, err := createExchangeFixtureFile(t, path)
			if err != nil {
				t.Fatal(err)
			}
			observed := make(chan ingressObservation, 1)
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				received, err := exchange.ReceiveStream(exchange.StreamReceiveCall{
					Call: socketServerCallFrom(t, writer, request), Destination: destination,
					Route:               exchange.RouteSemantics{Method: exchange.MethodPut, Replay: exchange.ReplaySingleAttempt},
					ExpectedContentType: core.HTTPMediaTypeOctetStream(), Buffer: make([]byte, exchange.TransferBufferBytes),
				})
				observed <- ingressObservation{err: err, bytes: received.Bytes.Uint64()}
				writer.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			var source io.Reader = bytes.NewReader(body)
			if tc.chunked {
				source = io.NopCloser(source)
			}
			request, err := http.NewRequestWithContext(t.Context(), http.MethodPut, server.URL, source)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
			response, sendErr := server.Client().Do(request)
			if sendErr != nil {
				t.Fatal(sendErr)
			}
			_, drainErr := io.Copy(io.Discard, response.Body)
			closeErr := response.Body.Close()
			got := awaitIngressObservation(t, observed)
			fileCloseErr := destination.Close()
			if err := errors.Join(got.err, drainErr, closeErr, fileCloseErr); err != nil {
				t.Fatal(err)
			}
			if got.bytes != uint64(tc.size) || sha256File(t, path) != want {
				t.Fatalf("receive bytes=%d want %d with exact digest", got.bytes, tc.size)
			}
		})
	}
}
