package exchange_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type jsonServerObservation struct {
	receiveErr     error
	writeErr       error
	message        string
	acceptEncoding string
	contentLength  int64
}

func TestJSONTransportLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive typed JSON crosses a real HTTP client and server", func(t *testing.T) {
		t.Parallel()

		responseHeader, gotHeaderErr := core.ParseHTTPHeaderName("X-Exchange-Result")
		if gotHeaderErr != nil {
			t.Fatalf("ParseHTTPHeaderName() setup error = %v, want nil", gotHeaderErr)
		}
		created := mustHTTPStatus(t, http.StatusCreated)
		serverPolicy := exchange.ServerPolicy{
			RequestBodyLimit: mustByteCount(t, 4*1024),
		}
		writePolicy := exchange.JSONWritePolicy{
			ResponseBodyLimit: mustByteCount(t, 4*1024),
		}
		observed := make(chan jsonServerObservation, 1)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			serverCall := socketServerCallFrom(t, writer, request)
			received, receiveErr := exchange.ReceiveJSON[
				transportDocument,
				*transportDocument,
			](exchange.JSONReceiveCall{
				Call: serverCall,
				Route: exchange.RouteSemantics{
					Method: exchange.MethodPost,
					Replay: exchange.ReplaySingleAttempt,
				},
				Policy: serverPolicy,
			})
			observation := jsonServerObservation{
				receiveErr: receiveErr,
				acceptEncoding: request.Header.Get(
					core.HTTPHeaderAcceptEncoding().String(),
				),
				contentLength: request.ContentLength,
			}
			if receiveErr == nil {
				observation.message = received.Body.Message
				observation.writeErr = exchange.WriteJSON(
					exchange.JSONWriteCall[transportDocument]{
						Call: serverCall,
						Response: exchange.ServerJSONResponse[transportDocument]{
							Body: transportDocument{Message: "accepted"},
							Headers: exchange.ResponseHeaders{
								Values: []exchange.Header{{
									Name: responseHeader, Values: []exchange.HeaderValue{mustHeaderValue(t, "sealed")},
								}},
							},
							Status: created,
						},
						Policy: writePolicy,
					},
				)
			}
			observed <- observation
		}))
		defer server.Close()

		got, gotErr := exchange.SendJSON[
			transportDocument,
			transportDocument,
		](exchange.JSONCall[transportDocument]{
			Context: context.Background(),
			Client:  mustExchangeClient(t, server.Client()),
			Request: exchange.JSONRequest[transportDocument]{
				Target: mustEndpoint(t, server.URL),
				Body:   transportDocument{Message: "candidate"},
				Semantics: exchange.RequestSemantics{
					Method: exchange.MethodPost,
					Replay: exchange.ReplaySingleAttempt,
				},
				CaptureHeaders: exchange.HeaderSelection{
					Names: []core.HTTPHeaderName{responseHeader},
				},
				ExpectedStatus: created,
			},
			Policy: exchange.JSONPolicy{
				Operation:         singleAttemptOperationPolicy(t),
				RequestBodyLimit:  mustByteCount(t, 4*1024),
				ResponseBodyLimit: mustByteCount(t, 4*1024),
			},
		})
		if gotErr != nil {
			t.Fatalf("exchange.SendJSON() error = %v, want nil", gotErr)
		}
		if got.Body.Message != "accepted" {
			t.Fatalf("exchange.SendJSON() body message = %q, want %q", got.Body.Message, "accepted")
		}
		if got.Metadata.Status != created || got.Metadata.Attempts != 1 {
			t.Fatalf(
				"exchange.SendJSON() metadata status/attempts = (%v, %d), want (%v, 1)",
				got.Metadata.Status,
				got.Metadata.Attempts,
				created,
			)
		}
		capturedValue := ""
		var capturedValueErr error
		if len(got.Metadata.Headers.Values) == 1 &&
			len(got.Metadata.Headers.Values[0].Values) == 1 {
			capturedValue, capturedValueErr = got.Metadata.Headers.Values[0].Values[0].Value()
		}
		if len(got.Metadata.Headers.Values) != 1 ||
			got.Metadata.Headers.Values[0].Name != responseHeader ||
			len(got.Metadata.Headers.Values[0].Values) != 1 ||
			capturedValueErr != nil || capturedValue != "sealed" {
			t.Fatalf(
				"exchange.SendJSON() captured headers = %+v, want one sealed %s",
				got.Metadata.Headers,
				responseHeader,
			)
		}

		select {
		case serverGot := <-observed:
			if serverGot.receiveErr != nil || serverGot.writeErr != nil {
				t.Fatalf(
					"server Exchange errors = (%v, %v), want (nil, nil)",
					serverGot.receiveErr,
					serverGot.writeErr,
				)
			}
			if serverGot.message != "candidate" {
				t.Fatalf("server received message = %q, want %q", serverGot.message, "candidate")
			}
			if serverGot.acceptEncoding != identityContentCoding {
				t.Fatalf(
					"server Accept-Encoding = %q, want %q",
					serverGot.acceptEncoding,
					identityContentCoding,
				)
			}
			if serverGot.contentLength <= 0 {
				t.Fatalf("server ContentLength = %d, want positive typed JSON body", serverGot.contentLength)
			}
		case <-time.After(testDeadlockBackstop):
			t.Fatalf(
				"JSON server observation = absent after %v, want one completed observation",
				testDeadlockBackstop,
			)
		}
	})

	t.Run("negative malformed JSON response keeps response and JSON identities", func(t *testing.T) {
		t.Parallel()

		ok := mustHTTPStatus(t, http.StatusOK)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			writer.Header().Set(
				core.HTTPHeaderContentType().String(),
				mustHTTPMediaType(t, "application/json").String(),
			)
			writer.WriteHeader(http.StatusOK)
			if _, err := writer.Write(
				[]byte(`{"message":"accepted","unknown":true}`),
			); err != nil {
				return
			}
		}))
		defer server.Close()

		got, gotErr := exchange.SendNoBodyJSON[transportDocument](
			exchange.NoBodyJSONCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.NoBodyRequest{
					Target: mustEndpoint(t, server.URL),
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodGet,
						Replay: exchange.ReplaySingleAttempt,
					},
					ExpectedStatus: ok,
				},
				Policy: exchange.NoBodyJSONPolicy{
					Operation:         singleAttemptOperationPolicy(t),
					ResponseBodyLimit: mustByteCount(t, 4*1024),
				},
			},
		)
		if !errors.Is(gotErr, core.ErrExchangeResponse) ||
			!errors.Is(gotErr, core.ErrJSONContract) {
			t.Fatalf(
				"exchange.SendNoBodyJSON() error = %v, want %v and %v",
				gotErr,
				core.ErrExchangeResponse,
				core.ErrJSONContract,
			)
		}
		if got.Body != (transportDocument{}) {
			t.Fatalf("exchange.SendNoBodyJSON() body = %+v, want zero", got.Body)
		}
		if got.Metadata.Status != ok || got.Metadata.Attempts != 1 {
			t.Fatalf(
				"exchange.SendNoBodyJSON() metadata = %+v, want status %v and one attempt",
				got.Metadata,
				ok,
			)
		}
	})

	t.Run("negative invalid captured metadata never escapes the response boundary", func(t *testing.T) {
		t.Parallel()

		ok := mustHTTPStatus(t, http.StatusOK)
		capturedName, gotNameErr := core.ParseHTTPHeaderName("X-Exchange-Captured")
		if gotNameErr != nil {
			t.Fatalf("ParseHTTPHeaderName() setup error = %v, want nil", gotNameErr)
		}
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			writer.Header().Set(
				core.HTTPHeaderContentType().String(),
				mustHTTPMediaType(t, "application/json").String(),
			)
			writer.Header().Set(
				capturedName.String(),
				strings.Repeat("v", exchange.HeaderValueMaximumBytes+1),
			)
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(`{"message":"accepted"}`))
		}))
		defer server.Close()

		got, gotErr := exchange.SendNoBodyJSON[transportDocument](
			exchange.NoBodyJSONCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.NoBodyRequest{
					Target: mustEndpoint(t, server.URL),
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodGet,
						Replay: exchange.ReplaySingleAttempt,
					},
					CaptureHeaders: exchange.HeaderSelection{
						Names: []core.HTTPHeaderName{capturedName},
					},
					ExpectedStatus: ok,
				},
				Policy: exchange.NoBodyJSONPolicy{
					Operation:         singleAttemptOperationPolicy(t),
					ResponseBodyLimit: mustByteCount(t, 4*1024),
				},
			},
		)
		if !errors.Is(gotErr, core.ErrExchangeResponse) {
			t.Fatalf(
				"exchange.SendNoBodyJSON(invalid metadata) error = %v, want %v",
				gotErr,
				core.ErrExchangeResponse,
			)
		}
		if got.Body != (transportDocument{}) ||
			got.Metadata.Attempts != 0 ||
			got.Metadata.Bytes.Uint64() != 0 ||
			len(got.Metadata.Headers.Values) != 0 ||
			got.Metadata.Status != (core.HTTPStatusCode{}) {
			t.Fatalf(
				"exchange.SendNoBodyJSON(invalid metadata) = %+v, want zero",
				got,
			)
		}
	})

	t.Run("neutral no-body request does not synthesize an empty JSON body", func(t *testing.T) {
		t.Parallel()

		ok := mustHTTPStatus(t, http.StatusOK)
		writePolicy := exchange.JSONWritePolicy{
			ResponseBodyLimit: mustByteCount(t, 4*1024),
		}
		observed := make(chan jsonServerObservation, 1)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			serverCall := socketServerCallFrom(t, writer, request)
			_, receiveErr := exchange.ReceiveNoBody(
				exchange.NoBodyReceiveCall{
					Call: serverCall,
					Route: exchange.RouteSemantics{
						Method: exchange.MethodGet,
						Replay: exchange.ReplaySingleAttempt,
					},
				},
			)
			writeErr := exchange.WriteJSON(
				exchange.JSONWriteCall[transportDocument]{
					Call: serverCall,
					Response: exchange.ServerJSONResponse[transportDocument]{
						Body:   transportDocument{Message: "body absent"},
						Status: ok,
					},
					Policy: writePolicy,
				},
			)
			observed <- jsonServerObservation{
				receiveErr:    receiveErr,
				writeErr:      writeErr,
				contentLength: request.ContentLength,
			}
		}))
		defer server.Close()

		got, gotErr := exchange.SendNoBodyJSON[transportDocument](
			exchange.NoBodyJSONCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.NoBodyRequest{
					Target: mustEndpoint(t, server.URL),
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodGet,
						Replay: exchange.ReplaySingleAttempt,
					},
					ExpectedStatus: ok,
				},
				Policy: exchange.NoBodyJSONPolicy{
					Operation:         singleAttemptOperationPolicy(t),
					ResponseBodyLimit: mustByteCount(t, 4*1024),
				},
			},
		)
		if gotErr != nil || got.Body.Message != "body absent" {
			t.Fatalf(
				"exchange.SendNoBodyJSON() = (%+v, %v), want body-absent response and nil",
				got,
				gotErr,
			)
		}
		select {
		case serverGot := <-observed:
			if serverGot.receiveErr != nil || serverGot.writeErr != nil {
				t.Fatalf(
					"body-absent server errors = (%v, %v), want (nil, nil)",
					serverGot.receiveErr,
					serverGot.writeErr,
				)
			}
			if serverGot.contentLength != 0 {
				t.Fatalf("body-absent request ContentLength = %d, want 0", serverGot.contentLength)
			}
		case <-time.After(testDeadlockBackstop):
			t.Fatalf(
				"body-absent server observation = absent after %v, want one completed observation",
				testDeadlockBackstop,
			)
		}
	})
}
