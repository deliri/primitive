package exchange_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestNativeAggregateFailureBytesAndSecretHeadersAcrossClientDoors(t *testing.T) {
	t.Parallel()
	for door := range 4 {
		for _, streamed := range []bool{false, true} {
			if door >= 2 && streamed {
				continue
			} // Whole-value doors return their body.
			t.Run(fmt.Sprintf("door-%d/streamed-failure-%t", door, streamed), func(t *testing.T) {
				t.Parallel()
				payload, err := json.Marshal(transportDocument{Message: "provider refusal"})
				if err != nil {
					t.Fatal(err)
				}
				observed := make(chan string, 1)
				name, err := core.ParseHTTPHeaderName("X-Owned-Secret")
				if err != nil {
					t.Fatal(err)
				}
				key := []byte("native-owned-key")
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
					observed <- request.Header.Get(name.String())
					w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write(payload)
				}))
				defer server.Close()
				client := mustExchangeClient(t, server.Client())
				target := mustEndpoint(t, server.URL)
				secret := []exchange.SecretHeader{{Name: name, Value: key, Scheme: exchange.SecretHeaderLiteral}}
				policy := singleAttemptOperationPolicy(t)
				var metadata exchange.ResponseMetadata
				var retained []byte
				var sink bytes.Buffer
				switch door {
				case 0:
					request := exchange.JSONRequest[transportDocument]{Target: target, Body: transportDocument{Message: "request"}, ExpectedStatus: core.HTTPStatusOK(), Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}, SecretHeaders: secret}
					if streamed {
						request.FailureDestination = &sink
					}
					response, callErr := exchange.SendJSON[transportDocument, transportDocument](exchange.JSONCall[transportDocument]{Context: t.Context(), Client: client, Policy: exchange.JSONPolicy{Operation: policy}, Request: request})
					metadata, retained, err = response.Metadata, response.FailureBody, callErr
					if response.Body != (transportDocument{}) {
						t.Fatalf("failed JSON body admitted = %+v, want zero", response.Body)
					}
				case 1:
					request := exchange.NoBodyRequest{Target: target, ExpectedStatus: core.HTTPStatusOK(), Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}, SecretHeaders: secret}
					if streamed {
						request.FailureDestination = &sink
					}
					response, callErr := exchange.SendNoBodyJSON[transportDocument](exchange.NoBodyJSONCall{Context: t.Context(), Client: client, Policy: exchange.NoBodyJSONPolicy{Operation: policy}, Request: request})
					metadata, retained, err = response.Metadata, response.FailureBody, callErr
					if response.Body != (transportDocument{}) {
						t.Fatalf("failed JSON body admitted = %+v, want zero", response.Body)
					}
				case 2:
					response, callErr := exchange.SendBounded(exchange.BoundedCall{Context: t.Context(), Client: client, Policy: exchange.BoundedPolicy{Operation: policy}, Request: exchange.BoundedRequest{Target: target, Body: []byte("owned request"), RequestContentType: core.HTTPMediaTypeOctetStream(), ExpectedStatus: core.HTTPStatusOK(), Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}, SecretHeaders: secret}})
					metadata, retained, err = response.Metadata, response.Body, callErr
				case 3:
					response, callErr := exchange.SendNoBodyBounded(exchange.NoBodyBoundedCall{Context: t.Context(), Client: client, Policy: exchange.NoBodyBoundedPolicy{Operation: policy}, Request: exchange.NoBodyBoundedRequest{Target: target, ExpectedStatus: core.HTTPStatusOK(), Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}, SecretHeaders: secret}})
					metadata, retained, err = response.Metadata, response.Body, callErr
				}
				status, ok := errors.AsType[exchange.StatusError](err)
				if !ok || status.Status() != mustHTTPStatus(t, http.StatusBadRequest) || metadata.Bytes.Uint64() != uint64(len(payload)) || metadata.Attempts != 1 {
					t.Fatalf("provider refusal = %+v/%v, want status 400 and exact body observation", metadata, err)
				}
				if streamed {
					if retained != nil || !bytes.Equal(sink.Bytes(), payload) {
						t.Fatalf("streamed refusal = retained %q sink %q, want only sink bytes %q", retained, sink.Bytes(), payload)
					}
				} else if !bytes.Equal(retained, payload) || sink.Len() != 0 {
					t.Fatalf("whole-value refusal = %q, sink %d, want exact %q", retained, sink.Len(), payload)
				}
				if got := <-observed; got != string(key) {
					t.Fatalf("native secret = %q, want borrowed test bytes", got)
				}
				clear(key)
			})
		}
	}
}
