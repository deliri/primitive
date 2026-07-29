package exchange_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const testJSONIngressLimitBytes = 128

// ingressObservation is what one real server boundary observed for one real
// request. It is filled inside the handler and read by the client goroutine.
type ingressObservation struct {
	err     error
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

	policy := exchange.ServerPolicy{
		RequestBodyLimit: mustByteCount(t, testJSONIngressLimitBytes),
	}
	return httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		received, receiveErr := exchange.ReceiveJSON[
			transportDocument,
			*transportDocument,
		](exchange.JSONReceiveCall{
			Request: request,
			Route:   route,
			Policy:  policy,
		})
		observation := ingressObservation{err: receiveErr}
		if receiveErr == nil {
			observation.message = received.Body.Message
			observation.key = received.IdempotencyKey.String()
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
			contentType = core.HTTPMediaTypeJSON().String()
		}
		request.Header.Set(core.HTTPHeaderContentType().String(), contentType)
		if input.duplicateType {
			request.Header.Add(
				core.HTTPHeaderContentType().String(),
				core.HTTPMediaTypeJSON().String(),
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
	case <-time.After(testDeadlockBackstop):
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
				contentEncoding: core.HTTPContentCodingIdentity().String(),
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
				contentType: core.HTTPMediaTypeTextPlain().String(),
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
			name: "declared length above the route bound is refused",
			request: jsonIngressRequest{
				method: http.MethodPost,
				body:   overLimit,
			},
			wantErr: core.ErrExchangeBodyLimit,
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
					Method: core.HTTPMethodPost,
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
				if got.message != "" || got.key != "" {
					t.Fatalf("refused ingress leaked = %+v, want zero", got)
				}
				return
			}
			if got.message != tc.wantMessage {
				t.Fatalf("received message = %q, want %q", got.message, tc.wantMessage)
			}
		})
	}
}

func TestNoBodyIngressLayerTriad(t *testing.T) {
	t.Parallel()

	route := exchange.RouteSemantics{
		Method: core.HTTPMethodGet,
		Replay: exchange.ReplaySafe,
	}
	newServer := func(observed chan<- ingressObservation) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			received, receiveErr := exchange.ReceiveNoBody(
				exchange.NoBodyReceiveCall{Request: request, Route: route},
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
					contentEncoding: core.HTTPContentCodingIdentity().String(),
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

	t.Run("neutral a declared key route observes the real request key", func(t *testing.T) {
		t.Parallel()

		keyRoute := exchange.RouteSemantics{
			Method: core.HTTPMethodPost,
			Replay: exchange.ReplayIdempotencyKey,
		}
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
			t.Fatalf(
				"keyed ingress = (%v, %q), want (nil, %q)",
				got.err,
				got.key,
				"01JD-EXCHANGE-KEY",
			)
		}

		missingObserved := make(chan ingressObservation, 1)
		missingServer := startJSONIngressServer(t, keyRoute, missingObserved)
		defer missingServer.Close()

		sendRawRequest(t, missingServer, jsonIngressRequest{
			method: http.MethodPost, body: []byte(`{"message":"once"}`),
		})
		gotMissing := awaitIngressObservation(t, missingObserved)
		if !errors.Is(gotMissing.err, core.ErrExchangeRequest) {
			t.Fatalf(
				"keyed route without a key error = %v, want %v",
				gotMissing.err,
				core.ErrExchangeRequest,
			)
		}
	})
}

func TestStreamIngressBoundLayerTriad(t *testing.T) {
	t.Parallel()

	const limit = 3 * exchange.TransferBufferBytes
	newServer := func(
		t *testing.T,
		destination io.Writer,
		observed chan<- ingressObservation,
	) *httptest.Server {
		t.Helper()

		policy := exchange.ServerStreamPolicy{
			RequestBodyLimit: mustByteCount(t, limit),
		}
		return httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			received, receiveErr := exchange.ReceiveStream(
				exchange.StreamReceiveCall{
					Request:             request,
					Destination:         destination,
					Route:               exchange.RouteSemantics{Method: core.HTTPMethodPut, Replay: exchange.ReplaySingleAttempt},
					Policy:              policy,
					ExpectedContentType: core.HTTPMediaTypeOctetStream(),
				},
			)
			observed <- ingressObservation{
				err: receiveErr, bytes: received.Bytes.Uint64(),
			}
			writer.WriteHeader(http.StatusOK)
		}))
	}

	t.Run("positive the exact bound streams into a real file with digest parity", func(t *testing.T) {
		t.Parallel()

		body := bytes.Repeat([]byte{0x11, 0x22, 0x33, 0x44}, limit/4)
		want := sha256.Sum256(body)
		path := filepath.Join(t.TempDir(), "received.bin")
		destination, gotCreateErr := os.Create(path)
		if gotCreateErr != nil {
			t.Fatalf("os.Create(%q) setup error = %v, want nil", path, gotCreateErr)
		}
		observed := make(chan ingressObservation, 1)
		server := newServer(t, destination, observed)
		defer server.Close()

		sendRawRequest(t, server, jsonIngressRequest{
			method:      http.MethodPut,
			contentType: core.HTTPMediaTypeOctetStream().String(),
			body:        body,
		})
		got := awaitIngressObservation(t, observed)
		gotCloseErr := destination.Close()
		if got.err != nil || gotCloseErr != nil {
			t.Fatalf(
				"exchange.ReceiveStream()/destination.Close() = (%v, %v), want (nil, nil)",
				got.err,
				gotCloseErr,
			)
		}
		if got.bytes != limit {
			t.Fatalf("streamed bytes = %d, want %d", got.bytes, limit)
		}
		if gotDigest := sha256File(t, path); gotDigest != want {
			t.Fatalf("received SHA256 = %x, want %x", gotDigest, want)
		}
	})

	t.Run("negative one byte above the bound is refused before the destination grows", func(t *testing.T) {
		t.Parallel()

		destination := bytes.NewBuffer(nil)
		observed := make(chan ingressObservation, 1)
		server := newServer(t, destination, observed)
		defer server.Close()

		sendRawRequest(t, server, jsonIngressRequest{
			method:      http.MethodPut,
			contentType: core.HTTPMediaTypeOctetStream().String(),
			body:        bytes.Repeat([]byte{0xa5}, limit+1),
		})
		got := awaitIngressObservation(t, observed)
		if !errors.Is(got.err, core.ErrExchangeRequest) ||
			!errors.Is(got.err, core.ErrExchangeBodyLimit) {
			t.Fatalf(
				"exchange.ReceiveStream(one over) error = %v, want %v and %v",
				got.err,
				core.ErrExchangeRequest,
				core.ErrExchangeBodyLimit,
			)
		}
		if got.bytes != 0 || destination.Len() != 0 {
			t.Fatalf(
				"declared-over destination bytes/reported = (%d, %d), want (0, 0)",
				destination.Len(),
				got.bytes,
			)
		}
	})

	t.Run("neutral an unknown-length overrun writes exactly the bound and rejects the rest", func(t *testing.T) {
		t.Parallel()

		body := bytes.Repeat([]byte{0x5a}, limit+1)
		destination := bytes.NewBuffer(nil)
		observed := make(chan ingressObservation, 1)
		server := newServer(t, destination, observed)
		defer server.Close()

		request, gotErr := http.NewRequestWithContext(
			context.Background(),
			http.MethodPut,
			server.URL,
			io.NopCloser(bytes.NewReader(body)),
		)
		if gotErr != nil {
			t.Fatalf("http.NewRequestWithContext() setup error = %v, want nil", gotErr)
		}
		request.ContentLength = -1
		request.Header.Set(
			core.HTTPHeaderContentType().String(),
			core.HTTPMediaTypeOctetStream().String(),
		)
		response, gotSendErr := server.Client().Do(request)
		if gotSendErr == nil && response != nil && response.Body != nil {
			_, _ = io.Copy(io.Discard, response.Body)
			if gotCloseErr := response.Body.Close(); gotCloseErr != nil {
				t.Fatalf("chunked response close error = %v, want nil", gotCloseErr)
			}
		}
		got := awaitIngressObservation(t, observed)
		if !errors.Is(got.err, core.ErrExchangeBodyLimit) {
			t.Fatalf(
				"exchange.ReceiveStream(chunked one over) error = %v, want %v",
				got.err,
				core.ErrExchangeBodyLimit,
			)
		}
		if got.bytes != limit || destination.Len() != limit ||
			!bytes.Equal(destination.Bytes(), body[:limit]) {
			t.Fatalf(
				"chunked overrun reported/written/prefix = (%d, %d, %t), want (%d, %d, true)",
				got.bytes,
				destination.Len(),
				bytes.Equal(destination.Bytes(), body[:limit]),
				limit,
				limit,
			)
		}
	})
}
