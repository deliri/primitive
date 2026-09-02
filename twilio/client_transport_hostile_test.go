package twilio

import (
	"bytes"
	"context"
	"encoding/base64"
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

func TestAccountBoundBasicAuthenticationCrossesTwilioWall(t *testing.T) {
	t.Parallel()

	accountText := "AC" + strings.Repeat("a", 32)
	keyText := "SK" + strings.Repeat("b", 32)
	secretText := "twilio-api-key-secret"
	wantAuthorization := "Basic " + base64.StdEncoding.EncodeToString([]byte(keyText+":"+secretText))
	var calls uint64
	client, err := exchange.NewClient(&http.Client{Transport: transportRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.Method != http.MethodPost || request.URL.Host != core.TwilioAPIHost ||
			request.URL.Path != "/2010-04-01/Accounts/"+accountText+"/Messages.json" ||
			request.Header.Get("Authorization") != wantAuthorization {
			return nil, core.ErrTwilioBinding
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"status":"queued"}`)),
			Request:    request,
		}, nil
	})})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	account, err := ParseAccountSID(accountText)
	if err != nil {
		t.Fatalf("ParseAccountSID() error = %v, want nil", err)
	}
	key, err := ParseAPIKeySID(keyText)
	if err != nil {
		t.Fatalf("ParseAPIKeySID() error = %v, want nil", err)
	}
	credential, err := NewCredential(account, key, []byte(secretText))
	if err != nil {
		t.Fatalf("NewCredential() error = %v, want nil", err)
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

	request := twilioTransportRequest(t, accountText)
	if gotErr := request.Validate(); gotErr != nil {
		t.Fatalf("Request.Validate() error = %v, want nil", gotErr)
	}
	if gotErr := (Request{Stream: request}).Validate(); gotErr != nil {
		t.Fatalf("Twilio Request.Validate() error = %v, want nil", gotErr)
	}
	_, gotErr := provider.RoundTrip(context.Background(), Request{Stream: request}, twilioTransportPolicy(t))
	if gotErr != nil || calls != 1 {
		t.Fatalf("RoundTrip() = error:%v calls:%d, want nil, 1", gotErr, calls)
	}
}

func TestForeignAccountRouteNeverReachesTwilioTransport(t *testing.T) {
	t.Parallel()

	account, _ := ParseAccountSID("AC" + strings.Repeat("a", 32))
	key, _ := ParseAPIKeySID("SK" + strings.Repeat("b", 32))
	credential, err := NewCredential(account, key, []byte("twilio-api-key-secret"))
	if err != nil {
		t.Fatalf("NewCredential() error = %v, want nil", err)
	}
	var calls uint64
	client, err := exchange.NewClient(&http.Client{Transport: transportRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return nil, core.ErrTwilioBinding
	})})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	provider, err := NewClient(client, credential)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}
	defer func() { _ = errors.Join(provider.Close(), credential.Close()) }()

	foreign := "AC" + strings.Repeat("c", 32)
	request := twilioTransportRequest(t, foreign)
	_, gotErr := provider.RoundTrip(context.Background(), Request{Stream: request}, twilioTransportPolicy(t))
	if !errors.Is(gotErr, core.ErrTwilioBinding) || calls != 0 {
		t.Fatalf("RoundTrip() = error:%v calls:%d, want Twilio binding refusal, 0", gotErr, calls)
	}
}

func twilioTransportRequest(t testing.TB, account string) exchange.StreamRoundTripRequest {
	t.Helper()
	target, err := core.ParseHTTPEndpoint("https://" + core.TwilioAPIHost + "/2010-04-01/Accounts/" + account + "/Messages.json")
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint() error = %v, want nil", err)
	}
	requestMedia, err := core.ParseHTTPMediaType("application/x-www-form-urlencoded")
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType(request) error = %v, want nil", err)
	}
	responseMedia, err := core.ParseHTTPMediaType("application/json")
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType(response) error = %v, want nil", err)
	}
	body := []byte("To=%2B15555550100&Body=hello")
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
		Semantics:          exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
		RequestContentType: requestMedia, ExpectedResponseContentType: responseMedia,
		RequestContentLength: length, ResponseBodyLimit: limit, ExpectedStatus: core.HTTPStatusOK(),
	}
}

func twilioTransportPolicy(t testing.TB) exchange.StreamPolicy {
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
