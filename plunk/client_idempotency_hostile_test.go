package plunk

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

type idempotencyRoundTripFunc func(*http.Request) (*http.Response, error)

func (function idempotencyRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestIdempotencyHeaderCrossesPlunkWallAtPublishedMaximum(t *testing.T) {
	t.Parallel()

	wantKey := strings.Repeat("p", core.PlunkIdempotencyKeyMaximumBytes)
	var gotKey string
	var calls uint64
	client, err := exchange.NewClient(&http.Client{Transport: idempotencyRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		gotKey = request.Header.Get(core.HTTPHeaderIdempotencyKey().String())
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"accepted":true}`)),
			Request:    request,
		}, nil
	})})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	credential, err := ParseCredential([]byte("sk_test_credential"))
	if err != nil {
		t.Fatalf("ParseCredential() error = %v, want nil", err)
	}
	provider, err := NewClient(client, credential)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}
	defer func() {
		if gotErr := errors.Join(provider.Close(), credential.Close()); gotErr != nil {
			t.Errorf("credential cleanup error = %v, want nil", gotErr)
		}
	}()

	request := plunkIdempotencyRequest(t, wantKey)
	if gotErr := provider.Validate(); gotErr != nil {
		t.Fatalf("Client.Validate() error = %v, want nil", gotErr)
	}
	if gotErr := request.Validate(); gotErr != nil {
		t.Fatalf("request.Validate() error = %v, want nil", gotErr)
	}
	if gotErr := (Request{Stream: request}).Validate(); gotErr != nil {
		t.Fatalf("Plunk Request.Validate() error = %v, want nil", gotErr)
	}
	policy := plunkStreamPolicy(t)
	if gotErr := policy.Validate(); gotErr != nil {
		t.Fatalf("policy.Validate() error = %v, want nil", gotErr)
	}
	_, gotErr := provider.RoundTrip(context.Background(), Request{Stream: request}, policy)
	if gotErr != nil || calls != 1 || gotKey != wantKey {
		t.Fatalf("RoundTrip() = error:%v calls:%d idempotency:%q, want nil, 1, %q", gotErr, calls, gotKey, wantKey)
	}
}

func plunkIdempotencyRequest(t testing.TB, keyText string) exchange.StreamRoundTripRequest {
	t.Helper()
	target, err := core.ParseHTTPEndpoint("https://" + core.PlunkAPIHost + "/v1/send")
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint() error = %v, want nil", err)
	}
	media, err := core.ParseHTTPMediaType("application/json")
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType() error = %v, want nil", err)
	}
	key, err := exchange.ParseIdempotencyKey(keyText)
	if err != nil {
		t.Fatalf("exchange.ParseIdempotencyKey() error = %v, want nil", err)
	}
	body := []byte(`{"to":"person@example.com"}`)
	length, err := core.NewByteLength(uint64(len(body)))
	if err != nil {
		t.Fatalf("core.NewByteLength() error = %v, want nil", err)
	}
	limit, err := core.NewByteCount(1 << 10)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	return exchange.StreamRoundTripRequest{
		Target: target, Source: bytes.NewReader(body), Destination: io.Discard,
		Semantics:          exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttemptWithIdempotencyKey, IdempotencyKey: key},
		RequestContentType: media, ExpectedResponseContentType: media,
		RequestContentLength: length, ResponseBodyLimit: limit, ExpectedStatus: core.HTTPStatusOK(),
	}
}

func plunkStreamPolicy(t testing.TB) exchange.StreamPolicy {
	t.Helper()
	operation, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(operation) error = %v, want nil", err)
	}
	attempt, err := temporal.DurationFromSeconds(4)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(attempt) error = %v, want nil", err)
	}
	limit, err := core.NewByteCount(1 << 10)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	return exchange.StreamPolicy{OperationTimeout: operation, AttemptTimeout: attempt, ErrorBodyLimit: limit, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}
}
