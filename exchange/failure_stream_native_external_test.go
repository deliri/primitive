package exchange_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type nativeFailurePatternSource struct{ offset int }

func (r *nativeFailurePatternSource) Read(data []byte) (int, error) {
	for i := range data {
		data[i] = [...]byte{0x00, 0xff, 0x51}[(r.offset+i)%3]
	}
	r.offset += len(data)
	return len(data), nil
}

type nativeFailurePatternSink struct {
	count    int
	mismatch bool
}

func (w *nativeFailurePatternSink) Write(data []byte) (int, error) {
	for i, value := range data {
		if value != [...]byte{0x00, 0xff, 0x51}[(w.count+i)%3] {
			w.mismatch = true
		}
	}
	w.count += len(data)
	return len(data), nil
}

func TestNativeStreamingFailureDestinationsAndSecretHeadersLayerTriad(t *testing.T) {
	t.Parallel()
	for door := range 3 {
		for _, extent := range []int{0, 32767, 32768, 32769, 262151, 4194305} {
			t.Run(fmt.Sprintf("door-%d/failed-response-bytes-%d", door, extent), func(t *testing.T) {
				t.Parallel()
				key := []byte("owned-native-test-key")
				name, err := core.ParseHTTPHeaderName("X-Owned-Secret")
				if err != nil {
					t.Fatal(err)
				}
				observed := make(chan string, 1)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
					observed <- request.Header.Get(name.String())
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = io.CopyN(w, &nativeFailurePatternSource{}, int64(extent))
				}))
				defer server.Close()
				client := mustExchangeClient(t, server.Client())
				secret := []exchange.SecretHeader{{Name: name, Value: key, Scheme: exchange.SecretHeaderLiteral}}
				var failure nativeFailurePatternSink
				var success bytes.Buffer
				target := mustEndpoint(t, server.URL)
				policy := singleAttemptStreamPolicy(t)
				var got exchange.StreamResponse
				switch door {
				case 0:
					got, err = exchange.Upload(exchange.UploadCall{Context: t.Context(), Client: client, Policy: policy, Request: exchange.UploadRequest{Target: target, Source: bytes.NewReader(nil), ContentType: core.HTTPMediaTypeOctetStream(), Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK(), SecretHeaders: secret, FailureDestination: &failure}})
				case 1:
					got, err = exchange.Download(exchange.DownloadCall{Context: t.Context(), Client: client, Policy: policy, Request: exchange.DownloadRequest{Target: target, Destination: &success, Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK(), SecretHeaders: secret, FailureDestination: &failure}})
				case 2:
					response, callErr := exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: t.Context(), Client: client, Policy: policy, Request: exchange.StreamRoundTripRequest{Target: target, Source: bytes.NewReader(nil), Destination: &success, RequestContentType: core.HTTPMediaTypeOctetStream(), Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK(), SecretHeaders: secret, FailureDestination: &failure}})
					got, err = exchange.StreamResponse(response), callErr
				}
				status, ok := errors.AsType[exchange.StatusError](err)
				if !ok || status.Status() != mustHTTPStatus(t, http.StatusUnauthorized) || got.Metadata.Attempts != 1 || got.Metadata.Bytes.Uint64() != uint64(extent) || failure.count != extent || failure.mismatch || success.Len() != 0 {
					t.Fatalf("failure stream = %+v/%v failure=%d mismatch=%t success=%d, want exact %d bytes only in failure sink", got, err, failure.count, failure.mismatch, success.Len(), extent)
				}
				if header := <-observed; header != string(key) {
					t.Fatalf("native secret header = %q, want borrowed test credential", header)
				}
				clear(key)
			})
		}
	}
}

func TestStreamingFailureSinkAndSecretHeaderRefusalsHaveNoEffects(t *testing.T) {
	t.Parallel()
	target := mustEndpoint(t, "https://refusal.invalid/")
	var absent *bytes.Buffer
	name, err := core.ParseHTTPHeaderName("X-Owned-Secret")
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []exchange.SecretHeader{{Name: name, Value: []byte("key\r\n"), Scheme: exchange.SecretHeaderLiteral}, {Name: name, Value: []byte("key"), Scheme: 255}} {
		request := exchange.UploadRequest{Target: target, Source: bytes.NewReader(nil), ContentType: core.HTTPMediaTypeOctetStream(), Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK(), SecretHeaders: []exchange.SecretHeader{secret}}
		if err := request.Validate(); !errors.Is(err, core.ErrExchangeRequest) {
			t.Fatalf("secret ingress refusal = %v, want request refusal", err)
		}
	}
	request := exchange.DownloadRequest{Target: target, Destination: io.Discard, FailureDestination: absent, ExpectedStatus: core.HTTPStatusOK(), Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}}
	if err := request.Validate(); !errors.Is(err, core.ErrExchangeRequest) {
		t.Fatalf("typed nil failure sink = %v, want request refusal", err)
	}
}
