package paypal

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

type transportRoundTripFunc func(*http.Request) (*http.Response, error)

func (function transportRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestRequestIDCrossesPayPalWallAtPublishedMaximum(t *testing.T) {
	t.Parallel()

	wantRequestID := strings.Repeat("p", core.PayPalRequestIDMaximumBytes)
	var calls uint64
	client, err := exchange.NewClient(&http.Client{Transport: transportRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.Method != http.MethodPost || request.URL.Host != core.PayPalLiveAPIHost ||
			request.URL.Path != "/v2/checkout/orders" ||
			request.Header.Get("Authorization") != "Bearer paypal-access-token" ||
			request.Header.Get(core.PayPalRequestIDHeaderName) != wantRequestID ||
			request.Header.Get(core.HTTPHeaderIdempotencyKey().String()) != "" {
			return nil, core.ErrPayPalBinding
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"status":"CREATED"}`)),
			Request:    request,
		}, nil
	})})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	token, err := ParseAccessToken([]byte("paypal-access-token"))
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v, want nil", err)
	}
	provider, err := NewClient(client, token, false)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}
	defer func() {
		if gotErr := errors.Join(provider.Close(), token.Close()); gotErr != nil {
			t.Errorf("credential cleanup error = %v, want nil", gotErr)
		}
	}()

	request := payPalTransportRequest(t, wantRequestID)
	if gotErr := request.Validate(); gotErr != nil {
		t.Fatalf("Request.Validate() error = %v, want nil", gotErr)
	}
	if gotErr := (Request{Stream: request}).Validate(); gotErr != nil {
		t.Fatalf("PayPal Request.Validate() error = %v, want nil", gotErr)
	}
	_, gotErr := provider.RoundTrip(context.Background(), Request{Stream: request}, payPalTransportPolicy(t))
	if gotErr != nil || calls != 1 {
		t.Fatalf("RoundTrip() = error:%v calls:%d, want nil, 1", gotErr, calls)
	}
}

func TestRequestIDAbovePayPalLimitNeverReachesTransport(t *testing.T) {
	t.Parallel()

	var calls uint64
	client, err := exchange.NewClient(&http.Client{Transport: transportRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return nil, core.ErrPayPalBinding
	})})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	token, err := ParseAccessToken([]byte("paypal-access-token"))
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v, want nil", err)
	}
	provider, err := NewClient(client, token, false)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}
	defer func() { _ = errors.Join(provider.Close(), token.Close()) }()

	request := payPalTransportRequest(t, strings.Repeat("p", core.PayPalRequestIDMaximumBytes+1))
	_, gotErr := provider.RoundTrip(context.Background(), Request{Stream: request}, payPalTransportPolicy(t))
	if !errors.Is(gotErr, core.ErrPayPalBinding) || calls != 0 {
		t.Fatalf("RoundTrip() = error:%v calls:%d, want PayPal binding refusal, 0", gotErr, calls)
	}
}

func TestSandboxRequestValidationDefersHostBindingToSandboxClient(t *testing.T) {
	t.Parallel()

	request := payPalTransportRequestForHost(t, core.PayPalSandboxAPIHost, "sandbox-request")
	if gotErr := (Request{Stream: request}).Validate(); gotErr != nil {
		t.Fatalf("PayPal sandbox Request.Validate() error = %v, want nil", gotErr)
	}
	download, _ := payPalDownloadFixture(t)
	sandboxTarget, targetErr := core.ParseHTTPEndpoint("https://" + core.PayPalSandboxAPIHost + "/v2/checkout/orders/order-123")
	if targetErr != nil {
		t.Fatalf("core.ParseHTTPEndpoint(PayPal sandbox download) error = %v, want nil", targetErr)
	}
	download.Stream.Target = sandboxTarget
	if gotErr := download.Validate(); gotErr != nil {
		t.Fatalf("PayPal sandbox DownloadRequest.Validate() error = %v, want nil", gotErr)
	}

	var calls uint64
	exchangeClient, clientErr := exchange.NewClient(&http.Client{Transport: transportRoundTripFunc(func(got *http.Request) (*http.Response, error) {
		calls++
		if got.URL.Host != core.PayPalSandboxAPIHost {
			return nil, core.ErrPayPalBinding
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"status":"CREATED"}`)),
			Request:    got,
		}, nil
	})})
	if clientErr != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", clientErr)
	}
	token, tokenErr := ParseAccessToken([]byte("paypal-access-token"))
	if tokenErr != nil {
		t.Fatalf("ParseAccessToken() error = %v, want nil", tokenErr)
	}
	provider, providerErr := NewClient(exchangeClient, token, true)
	if providerErr != nil {
		t.Fatalf("NewClient(sandbox) error = %v, want nil", providerErr)
	}
	live, liveErr := NewClient(exchangeClient, token, false)
	if liveErr != nil {
		t.Fatalf("NewClient(live) error = %v, want nil", liveErr)
	}
	t.Cleanup(func() { _ = errors.Join(provider.Close(), live.Close(), token.Close()) })
	liveResponse, liveCallErr := live.RoundTrip(t.Context(), Request{Stream: request}, payPalTransportPolicy(t))
	if liveResponse.Stream.Metadata.Attempts != 0 || liveResponse.Stream.Metadata.Bytes.Uint64() != 0 ||
		len(liveResponse.Stream.Metadata.Headers.Values) != 0 || liveResponse.Stream.RequestBytes.Uint64() != 0 ||
		!errors.Is(liveCallErr, core.ErrPayPalBinding) || calls != 0 {
		t.Fatalf("live RoundTrip(sandbox target) = (response %v, error %v, calls %d), want zero, %v, 0", liveResponse, liveCallErr, calls, core.ErrPayPalBinding)
	}
	response, gotErr := provider.RoundTrip(t.Context(), Request{Stream: request}, payPalTransportPolicy(t))
	if gotErr != nil || response.Validate() != nil || calls != 1 {
		t.Fatalf("sandbox RoundTrip() = (response %v, error %v, calls %d), want valid, nil, 1", response, gotErr, calls)
	}
}

func payPalTransportRequest(t testing.TB, requestID string) exchange.StreamRoundTripRequest {
	return payPalTransportRequestForHost(t, core.PayPalLiveAPIHost, requestID)
}

func payPalTransportRequestForHost(t testing.TB, host, requestID string) exchange.StreamRoundTripRequest {
	t.Helper()
	target, err := core.ParseHTTPEndpoint("https://" + host + "/v2/checkout/orders")
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint() error = %v, want nil", err)
	}
	media, err := core.ParseHTTPMediaType("application/json")
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType() error = %v, want nil", err)
	}
	key, err := exchange.ParseIdempotencyKey(requestID)
	if err != nil {
		t.Fatalf("exchange.ParseIdempotencyKey() error = %v, want nil", err)
	}
	body := []byte(`{"intent":"CAPTURE"}`)
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
		Semantics: exchange.RequestSemantics{Method: exchange.MethodPost,
			Replay: exchange.ReplaySingleAttemptWithIdempotencyKey, IdempotencyKey: key},
		RequestContentType: media, ExpectedResponseContentType: media,
		RequestContentLength: length, ResponseBodyLimit: limit, ExpectedStatus: core.HTTPStatusOK(),
	}
}

func payPalTransportPolicy(t testing.TB) exchange.StreamPolicy {
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
	return exchange.StreamPolicy{OperationTimeout: operation, AttemptTimeout: attempt,
		ErrorBodyLimit: limit, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}
}
