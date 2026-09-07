package exchange_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type unreadUploadSource struct {
	reader *bytes.Reader
	reads  atomic.Int64
}

func (s *unreadUploadSource) Read(p []byte) (int, error) { s.reads.Add(1); return s.reader.Read(p) }

// Go's real Expect/continue exchange can finish with an HTTP response and no
// body read. A declared extent cannot become evidence that those bytes moved.
func TestUploadEarlyResponseDeclarationLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		status        int
		wantErr       error
		wantReadCalls int64
		roundTrip     bool
	}{
		{name: "successful early response cannot prove unconsumed request bytes", status: http.StatusOK},
		{name: "refused early response cannot prove unconsumed request bytes", status: http.StatusRequestEntityTooLarge, wantErr: core.ErrExchangeResponse},
		{name: "round trip successful early response preserves declaration without delivery claim", status: http.StatusOK, roundTrip: true},
		{name: "round trip refused early response preserves declaration without delivery claim", status: http.StatusRequestEntityTooLarge, wantErr: core.ErrExchangeResponse, roundTrip: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			status := mustHTTPStatus(t, tc.status)
			handled := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				call, err := exchange.NewSocketServerCall(w, r)
				if err == nil {
					err = exchange.WriteBounded(exchange.BoundedWriteCall{Call: call, Response: exchange.ServerBoundedResponse{Status: status, ContentType: core.HTTPMediaTypeOctetStream()}})
				}
				handled <- err
			}))
			t.Cleanup(server.Close)
			transport := http.DefaultTransport.(*http.Transport).Clone()
			duration := mustDurationMilliseconds(t, 10_000)
			timeout, err := duration.Stdlib()
			if err != nil {
				t.Fatal(err)
			}
			transport.ExpectContinueTimeout = timeout
			t.Cleanup(transport.CloseIdleConnections)
			client := mustExchangeClient(t, &http.Client{Transport: transport})
			source := &unreadUploadSource{reader: bytes.NewReader([]byte{0, 0xff})}
			expect, err := exchange.NewHeaderValue(core.HTTPExpectContinueValue)
			if err != nil {
				t.Fatal(err)
			}
			request := exchange.UploadRequest{Target: mustEndpoint(t, server.URL), Source: source, ContentLength: mustByteLength(t, 2), ContentType: core.HTTPMediaTypeOctetStream(), ExpectedStatus: core.HTTPStatusOK(), Semantics: exchange.RequestSemantics{Method: exchange.MethodPut, Replay: exchange.ReplaySingleAttempt}, Headers: exchange.Headers{Values: []exchange.Header{{Name: core.HTTPHeaderExpect(), Values: []exchange.HeaderValue{expect}}}}}
			var got exchange.StreamResponse
			if tc.roundTrip {
				var destination bytes.Buffer
				response, callErr := exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: t.Context(), Client: client, Policy: singleAttemptStreamPolicy(t), Request: exchange.StreamRoundTripRequest{Target: request.Target, Source: source, Destination: &destination, RequestContentLength: request.ContentLength, RequestContentType: request.ContentType, ExpectedStatus: request.ExpectedStatus, Semantics: request.Semantics, Headers: request.Headers, ResponseBodyLimit: mustByteCount(t, 2)}})
				got = exchange.StreamResponse{Metadata: response.Metadata, DeclaredRequestBytes: response.DeclaredRequestBytes}
				err = callErr
				if destination.Len() != 0 {
					t.Fatalf("early response destination = %x, want empty", destination.Bytes())
				}
			} else {
				got, err = exchange.Upload(exchange.UploadCall{Context: t.Context(), Client: client, Request: request, Policy: singleAttemptStreamPolicy(t)})
			}
			if got.DeclaredRequestBytes.Uint64() != 2 {
				t.Fatalf("declared request extent = %d, want exact 2", got.DeclaredRequestBytes.Uint64())
			}
			if validationErr := got.Validate(); validationErr != nil {
				t.Fatalf("response validation = %v, want nil", validationErr)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("early response error = %v, want %v", err, tc.wantErr)
			}
			if got.Metadata.Status != status || got.Metadata.Attempts != 1 || got.Metadata.Headers.Values != nil {
				t.Fatalf("early response metadata = %+v, want exact status, one attempt and absent captured headers", got.Metadata)
			}
			if source.reads.Load() != tc.wantReadCalls || source.reader.Len() != 2 {
				t.Fatalf("source read calls/remaining = %d/%d, want %d/2", source.reads.Load(), source.reader.Len(), tc.wantReadCalls)
			}
			if got.Metadata.Bytes != (core.ByteLength{}) {
				t.Errorf("observed extent = %d, want zero; no request body was read", got.Metadata.Bytes.Uint64())
			}
			select {
			case err := <-handled:
				if err != nil {
					t.Fatalf("server response = %v, want nil", err)
				}
			case <-exchangeFixtureBackstop(t, timeout):
				t.Fatal("Go server did not complete its response")
			}
		})
	}
}

var _ io.Reader = (*unreadUploadSource)(nil)
