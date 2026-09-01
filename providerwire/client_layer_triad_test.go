package providerwire

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

type providerRoundTripFunc func(*http.Request) (*http.Response, error)

func (function providerRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type providerRequestObservation struct {
	method          string
	authorization   string
	idempotency     string
	stripeVersion   string
	payPalRequestID string
	host            string
	path            string
	body            []byte
	calls           uint64
}

func TestOutboundProviderWallSocketLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive official provider bindings reach Exchange with exact authentication", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name              string
			body              string
			wantHost          string
			wantPath          string
			wantAuthorization string
			wantIdempotency   string
			wantStripeVersion string
			wantPayPalID      string
			invoke            func(testing.TB, exchange.Client, exchange.StreamRoundTripRequest, exchange.StreamPolicy) error
			request           func(testing.TB, io.Reader, io.Writer) exchange.StreamRoundTripRequest
		}{
			{
				name: "Stripe v1 form stream carries pinned SDK version and idempotency identity",
				body: "amount=100&currency=cad", wantHost: StripeAPIHost, wantPath: "/v1/payment_intents",
				wantAuthorization: "Bearer rk_test_123", wantIdempotency: "stripe-request-1", wantStripeVersion: StripeAPIVersion,
				request: func(tb testing.TB, source io.Reader, destination io.Writer) exchange.StreamRoundTripRequest {
					return providerStreamRequest(tb, StripeAPIHost, "/v1/payment_intents", "application/x-www-form-urlencoded", "stripe-request-1", source, destination, 23)
				},
				invoke: invokeStripeRoundTrip,
			},
			{
				name: "Stripe v1 form stream preserves product-owned single-attempt policy",
				body: "amount=200&currency=cad", wantHost: StripeAPIHost, wantPath: "/v1/payment_intents",
				wantAuthorization: "Bearer rk_test_123", wantStripeVersion: StripeAPIVersion,
				request: func(tb testing.TB, source io.Reader, destination io.Writer) exchange.StreamRoundTripRequest {
					return providerSingleAttemptStreamRequest(tb, StripeAPIHost, "/v1/payment_intents", "application/x-www-form-urlencoded", source, destination, 23)
				},
				invoke: invokeStripeRoundTrip,
			},
			{
				name: "Plunk JSON stream carries documented at-most-once identity",
				body: `{"to":"person@example.com"}`, wantHost: PlunkAPIHost, wantPath: "/v1/send",
				wantAuthorization: "Bearer sk_test_123", wantIdempotency: "plunk-request-1",
				request: func(tb testing.TB, source io.Reader, destination io.Writer) exchange.StreamRoundTripRequest {
					return providerStreamRequest(tb, PlunkAPIHost, "/v1/send", "application/json", "plunk-request-1", source, destination, 27)
				},
				invoke: invokePlunkRoundTrip,
			},
			{
				name: "Plunk JSON stream preserves product-owned single-attempt policy",
				body: `{"to":"other@example.com"}`, wantHost: PlunkAPIHost, wantPath: "/v1/send",
				wantAuthorization: "Bearer sk_test_123",
				request: func(tb testing.TB, source io.Reader, destination io.Writer) exchange.StreamRoundTripRequest {
					return providerSingleAttemptStreamRequest(tb, PlunkAPIHost, "/v1/send", "application/json", source, destination, 26)
				},
				invoke: invokePlunkRoundTrip,
			},
			{
				name: "Twilio form stream binds the exact account and API key",
				body: "To=%2B14165550100&Body=hello", wantHost: TwilioAPIHost,
				wantPath:          "/2010-04-01/Accounts/AC0123456789abcdefABCDEF0123456789/Messages.json",
				wantAuthorization: twilioBasicAuthorizationForTest(),
				request: func(tb testing.TB, source io.Reader, destination io.Writer) exchange.StreamRoundTripRequest {
					return providerSingleAttemptStreamRequest(tb, TwilioAPIHost, "/2010-04-01/Accounts/AC0123456789abcdefABCDEF0123456789/Messages.json", "application/x-www-form-urlencoded", source, destination, 28)
				},
				invoke: invokeTwilioRoundTrip,
			},
			{
				name: "PayPal JSON stream binds live OAuth and request identity",
				body: `{"intent":"CAPTURE"}`, wantHost: PayPalLiveAPIHost, wantPath: "/v2/checkout/orders",
				wantAuthorization: "Bearer paypal_test_token", wantPayPalID: "paypal-request-1",
				request: func(tb testing.TB, source io.Reader, destination io.Writer) exchange.StreamRoundTripRequest {
					return providerStreamRequest(tb, PayPalLiveAPIHost, "/v2/checkout/orders", "application/json", "paypal-request-1", source, destination, 20)
				},
				invoke: invokePayPalRoundTrip,
			},
			{
				name: "PayPal JSON stream preserves product-owned single-attempt policy",
				body: `{"intent":"AUTHORIZE"}`, wantHost: PayPalLiveAPIHost, wantPath: "/v2/checkout/orders",
				wantAuthorization: "Bearer paypal_test_token",
				request: func(tb testing.TB, source io.Reader, destination io.Writer) exchange.StreamRoundTripRequest {
					return providerSingleAttemptStreamRequest(tb, PayPalLiveAPIHost, "/v2/checkout/orders", "application/json", source, destination, 22)
				},
				invoke: invokePayPalRoundTrip,
			},
		}

		for _, testCase := range tests {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				observation := &providerRequestObservation{}
				client := providerExchangeClient(t, observation)
				var destination bytes.Buffer
				request := testCase.request(t, strings.NewReader(testCase.body), &destination)
				gotErr := testCase.invoke(t, client, request, providerStreamPolicy(t))
				if gotErr != nil {
					t.Fatalf("provider RoundTrip() error = %v, want nil", gotErr)
				}
				if observation.calls != 1 || observation.method != http.MethodPost || observation.host != testCase.wantHost || observation.path != testCase.wantPath {
					t.Fatalf("provider request route = calls:%d %s %s%s, want 1 POST %s%s", observation.calls, observation.method, observation.host, observation.path, testCase.wantHost, testCase.wantPath)
				}
				if observation.authorization != testCase.wantAuthorization || observation.idempotency != testCase.wantIdempotency ||
					observation.stripeVersion != testCase.wantStripeVersion || observation.payPalRequestID != testCase.wantPayPalID {
					t.Fatalf("provider authentication = %q/%q/%q/%q, want %q/%q/%q/%q", observation.authorization, observation.idempotency, observation.stripeVersion, observation.payPalRequestID, testCase.wantAuthorization, testCase.wantIdempotency, testCase.wantStripeVersion, testCase.wantPayPalID)
				}
				if gotBody := string(observation.body); gotBody != testCase.body {
					t.Fatalf("provider request body = %q, want %q", gotBody, testCase.body)
				}
				if gotBody := destination.String(); gotBody != `{"accepted":true}` {
					t.Fatalf("provider response body = %q, want %q", gotBody, `{"accepted":true}`)
				}
			})
		}
	})

	t.Run("negative foreign authority is refused before transport", func(t *testing.T) {
		t.Parallel()

		observation := &providerRequestObservation{}
		client := providerExchangeClient(t, observation)
		var destination bytes.Buffer
		request := providerStreamRequest(t, "example.invalid", "/v1/payment_intents", "application/x-www-form-urlencoded", "stripe-request-1", strings.NewReader("amount=1"), &destination, 8)
		gotErr := invokeStripeRoundTrip(t, client, request, providerStreamPolicy(t))
		if !errors.Is(gotErr, core.ErrProviderWireBinding) || observation.calls != 0 || destination.Len() != 0 {
			t.Fatalf("Stripe foreign authority = error:%v calls:%d bytes:%d, want %v, 0, 0", gotErr, observation.calls, destination.Len(), core.ErrProviderWireBinding)
		}
	})

	t.Run("neutral cancelled context performs no provider effect", func(t *testing.T) {
		t.Parallel()

		observation := &providerRequestObservation{}
		client := providerExchangeClient(t, observation)
		credential, err := ParseStripeCredential([]byte("rk_test_123"))
		if err != nil {
			t.Fatalf("ParseStripeCredential() error = %v, want nil", err)
		}
		provider, err := NewStripeClient(client, credential)
		if err != nil {
			t.Fatalf("NewStripeClient() error = %v, want nil", err)
		}
		defer closeStripeClient(t, &provider, &credential)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var destination bytes.Buffer
		request := providerStreamRequest(t, StripeAPIHost, "/v1/payment_intents", "application/x-www-form-urlencoded", "stripe-request-1", strings.NewReader("amount=1"), &destination, 8)
		_, gotErr := provider.RoundTrip(ctx, request, providerStreamPolicy(t))
		if !errors.Is(gotErr, context.Canceled) || observation.calls != 0 || destination.Len() != 0 {
			t.Fatalf("Stripe cancelled request = error:%v calls:%d bytes:%d, want context cancellation, 0, 0", gotErr, observation.calls, destination.Len())
		}
	})
}

func TestProviderAuthenticatedDownloadsUseExactOfficialAuthorities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		host              string
		path              string
		wantAuthorization string
		wantStripeVersion string
		invoke            func(testing.TB, exchange.Client, exchange.DownloadRequest, exchange.StreamPolicy) error
	}{
		{name: "Stripe authenticated GET", host: StripeAPIHost, path: "/v1/payment_intents/pi_123", wantAuthorization: "Bearer rk_test_123", wantStripeVersion: StripeAPIVersion, invoke: invokeStripeDownload},
		{name: "Plunk authenticated GET", host: PlunkAPIHost, path: "/v1/contacts/person", wantAuthorization: "Bearer sk_test_123", invoke: invokePlunkDownload},
		{name: "Twilio account-bound GET", host: TwilioAPIHost, path: "/2010-04-01/Accounts/AC0123456789abcdefABCDEF0123456789/Messages/SM123.json", wantAuthorization: twilioBasicAuthorizationForTest(), invoke: invokeTwilioDownload},
		{name: "PayPal OAuth GET", host: PayPalLiveAPIHost, path: "/v2/checkout/orders/ORDER123", wantAuthorization: "Bearer paypal_test_token", invoke: invokePayPalDownload},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			observation := &providerRequestObservation{}
			client := providerExchangeClient(t, observation)
			var destination bytes.Buffer
			request := providerDownloadRequest(t, testCase.host, testCase.path, &destination)
			gotErr := testCase.invoke(t, client, request, providerStreamPolicy(t))
			if gotErr != nil {
				t.Fatalf("provider Download() error = %v, want nil", gotErr)
			}
			if observation.calls != 1 || observation.method != http.MethodGet || observation.host != testCase.host || observation.path != testCase.path {
				t.Fatalf("provider GET route = calls:%d %s %s%s, want 1 GET %s%s", observation.calls, observation.method, observation.host, observation.path, testCase.host, testCase.path)
			}
			if observation.authorization != testCase.wantAuthorization || observation.stripeVersion != testCase.wantStripeVersion {
				t.Fatalf("provider GET authentication = %q/%q, want %q/%q", observation.authorization, observation.stripeVersion, testCase.wantAuthorization, testCase.wantStripeVersion)
			}
			if gotBody := destination.String(); gotBody != `{"accepted":true}` {
				t.Fatalf("provider GET response = %q, want %q", gotBody, `{"accepted":true}`)
			}
		})
	}
}

func TestVersionedProviderDownloadsRejectEveryVPrefixLookalike(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		host    string
		path    string
		invoke  func(testing.TB, exchange.Client, exchange.DownloadRequest, exchange.StreamPolicy) error
		wantErr error
	}{
		{name: "Stripe void prefix", host: StripeAPIHost, path: "/void/payment_intents", invoke: invokeStripeDownload, wantErr: core.ErrProviderWireBinding},
		{name: "Stripe vault prefix", host: StripeAPIHost, path: "/vault/payment_intents", invoke: invokeStripeDownload, wantErr: core.ErrProviderWireBinding},
		{name: "Stripe verification prefix", host: StripeAPIHost, path: "/verification/payment_intents", invoke: invokeStripeDownload, wantErr: core.ErrProviderWireBinding},
		{name: "PayPal void prefix", host: PayPalLiveAPIHost, path: "/void/checkout/orders", invoke: invokePayPalDownload, wantErr: core.ErrProviderWireBinding},
		{name: "PayPal vault prefix", host: PayPalLiveAPIHost, path: "/vault/payment-tokens", invoke: invokePayPalDownload, wantErr: core.ErrProviderWireBinding},
		{name: "PayPal verification prefix", host: PayPalLiveAPIHost, path: "/verification/orders", invoke: invokePayPalDownload, wantErr: core.ErrProviderWireBinding},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			observation := &providerRequestObservation{}
			client := providerExchangeClient(t, observation)
			var destination bytes.Buffer
			request := providerDownloadRequest(t, testCase.host, testCase.path, &destination)
			gotErr := testCase.invoke(t, client, request, providerStreamPolicy(t))
			if !errors.Is(gotErr, testCase.wantErr) || observation.calls != 0 || destination.Len() != 0 {
				t.Fatalf("provider lookalike Download() = error:%v calls:%d bytes:%d, want %v, 0, 0",
					gotErr, observation.calls, destination.Len(), testCase.wantErr)
			}
		})
	}
}

func providerExchangeClient(t testing.TB, observation *providerRequestObservation) exchange.Client {
	t.Helper()

	client, err := exchange.NewClient(&http.Client{Transport: providerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		observation.calls++
		observation.method = request.Method
		observation.host = request.URL.Host
		observation.path = request.URL.Path
		observation.authorization = request.Header.Get("Authorization")
		observation.idempotency = request.Header.Get("Idempotency-Key")
		observation.stripeVersion = request.Header.Get(StripeVersionHeaderName)
		observation.payPalRequestID = request.Header.Get(PayPalRequestIDHeaderName)
		body, readErr := readExactTestRequestBody(request)
		if readErr != nil {
			return nil, readErr
		}
		observation.body = body
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
	return client
}

func readExactTestRequestBody(request *http.Request) (body []byte, resultErr error) {
	if request.Body == nil {
		return nil, nil
	}
	defer func() { resultErr = errors.Join(resultErr, request.Body.Close()) }()
	if request.ContentLength < 0 || request.ContentLength > 1<<20 {
		return nil, core.ErrProviderWireContract
	}
	body = make([]byte, request.ContentLength)
	if _, err := io.ReadFull(request.Body, body); err != nil {
		return nil, err
	}
	var probe [1]byte
	if _, err := request.Body.Read(probe[:]); !errors.Is(err, io.EOF) {
		return nil, errors.Join(core.ErrProviderWireContract, err)
	}
	return body, nil
}

func providerStreamRequest(t testing.TB, host, requestPath, media, key string, source io.Reader, destination io.Writer, length uint64) exchange.StreamRoundTripRequest {
	t.Helper()

	idempotency, err := exchange.ParseIdempotencyKey(key)
	if err != nil {
		t.Fatalf("exchange.ParseIdempotencyKey() error = %v, want nil", err)
	}
	request := providerSingleAttemptStreamRequest(t, host, requestPath, media, source, destination, length)
	request.Semantics = exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttemptWithIdempotencyKey, IdempotencyKey: idempotency}
	return request
}

func providerSingleAttemptStreamRequest(t testing.TB, host, requestPath, media string, source io.Reader, destination io.Writer, length uint64) exchange.StreamRoundTripRequest {
	t.Helper()

	target, err := core.ParseHTTPEndpoint("https://" + host + requestPath)
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint() error = %v, want nil", err)
	}
	requestMedia, err := core.ParseHTTPMediaType(media)
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType(request) error = %v, want nil", err)
	}
	responseMedia, err := core.ParseHTTPMediaType("application/json")
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType(response) error = %v, want nil", err)
	}
	requestLength, err := core.NewByteLength(length)
	if err != nil {
		t.Fatalf("core.NewByteLength() error = %v, want nil", err)
	}
	responseLimit, err := core.NewByteCount(1 << 10)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	return exchange.StreamRoundTripRequest{
		Target: target, Source: source, Destination: destination,
		Semantics:          exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
		RequestContentType: requestMedia, ExpectedResponseContentType: responseMedia,
		RequestContentLength: requestLength, ResponseBodyLimit: responseLimit, ExpectedStatus: core.HTTPStatusOK(),
	}
}

func providerDownloadRequest(t testing.TB, host, requestPath string, destination io.Writer) exchange.DownloadRequest {
	t.Helper()

	target, err := core.ParseHTTPEndpoint("https://" + host + requestPath)
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint() error = %v, want nil", err)
	}
	media, err := core.ParseHTTPMediaType("application/json")
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType() error = %v, want nil", err)
	}
	limit, err := core.NewByteCount(1 << 10)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	return exchange.DownloadRequest{
		Target: target, Destination: destination,
		Semantics:                   exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt},
		ExpectedResponseContentType: media, ResponseBodyLimit: limit, ExpectedStatus: core.HTTPStatusOK(),
	}
}

func providerStreamPolicy(t testing.TB) exchange.StreamPolicy {
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
	return exchange.StreamPolicy{
		OperationTimeout: operation, AttemptTimeout: attempt, ErrorBodyLimit: limit,
		Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject},
	}
}

func invokeStripeRoundTrip(t testing.TB, client exchange.Client, request exchange.StreamRoundTripRequest, policy exchange.StreamPolicy) error {
	t.Helper()
	credential, err := ParseStripeCredential([]byte("rk_test_123"))
	if err != nil {
		return err
	}
	provider, err := NewStripeClient(client, credential)
	if err != nil {
		return errors.Join(err, credential.Close())
	}
	_, callErr := provider.RoundTrip(context.Background(), request, policy)
	return errors.Join(callErr, provider.Close(), credential.Close())
}

func invokePlunkRoundTrip(t testing.TB, client exchange.Client, request exchange.StreamRoundTripRequest, policy exchange.StreamPolicy) error {
	t.Helper()
	credential, err := ParsePlunkCredential([]byte("sk_test_123"))
	if err != nil {
		return err
	}
	provider, err := NewPlunkClient(client, credential)
	if err != nil {
		return errors.Join(err, credential.Close())
	}
	_, callErr := provider.RoundTrip(context.Background(), request, policy)
	return errors.Join(callErr, provider.Close(), credential.Close())
}

func invokeTwilioRoundTrip(t testing.TB, client exchange.Client, request exchange.StreamRoundTripRequest, policy exchange.StreamPolicy) error {
	t.Helper()
	provider, credential, err := newTwilioTestClient(client)
	if err != nil {
		return err
	}
	_, callErr := provider.RoundTrip(context.Background(), request, policy)
	return errors.Join(callErr, provider.Close(), credential.Close())
}

func invokePayPalRoundTrip(t testing.TB, client exchange.Client, request exchange.StreamRoundTripRequest, policy exchange.StreamPolicy) error {
	t.Helper()
	provider, token, err := newPayPalTestClient(client)
	if err != nil {
		return err
	}
	_, callErr := provider.RoundTrip(context.Background(), request, policy)
	return errors.Join(callErr, provider.Close(), token.Close())
}

func invokeStripeDownload(t testing.TB, client exchange.Client, request exchange.DownloadRequest, policy exchange.StreamPolicy) error {
	t.Helper()
	credential, err := ParseStripeCredential([]byte("rk_test_123"))
	if err != nil {
		return err
	}
	provider, err := NewStripeClient(client, credential)
	if err != nil {
		return errors.Join(err, credential.Close())
	}
	_, callErr := provider.Download(context.Background(), request, policy)
	return errors.Join(callErr, provider.Close(), credential.Close())
}

func invokePlunkDownload(t testing.TB, client exchange.Client, request exchange.DownloadRequest, policy exchange.StreamPolicy) error {
	t.Helper()
	credential, err := ParsePlunkCredential([]byte("sk_test_123"))
	if err != nil {
		return err
	}
	provider, err := NewPlunkClient(client, credential)
	if err != nil {
		return errors.Join(err, credential.Close())
	}
	_, callErr := provider.Download(context.Background(), request, policy)
	return errors.Join(callErr, provider.Close(), credential.Close())
}

func invokeTwilioDownload(t testing.TB, client exchange.Client, request exchange.DownloadRequest, policy exchange.StreamPolicy) error {
	t.Helper()
	provider, credential, err := newTwilioTestClient(client)
	if err != nil {
		return err
	}
	_, callErr := provider.Download(context.Background(), request, policy)
	return errors.Join(callErr, provider.Close(), credential.Close())
}

func invokePayPalDownload(t testing.TB, client exchange.Client, request exchange.DownloadRequest, policy exchange.StreamPolicy) error {
	t.Helper()
	provider, token, err := newPayPalTestClient(client)
	if err != nil {
		return err
	}
	_, callErr := provider.Download(context.Background(), request, policy)
	return errors.Join(callErr, provider.Close(), token.Close())
}

func newTwilioTestClient(client exchange.Client) (TwilioClient, TwilioCredential, error) {
	account, err := ParseTwilioAccountSID("AC0123456789abcdefABCDEF0123456789")
	if err != nil {
		return TwilioClient{}, TwilioCredential{}, err
	}
	key, err := ParseTwilioAPIKeySID(twilioAPIKeySIDTextForTest())
	if err != nil {
		return TwilioClient{}, TwilioCredential{}, err
	}
	credential, err := NewTwilioCredential(account, key, twilioAPIKeySecretForTest())
	if err != nil {
		return TwilioClient{}, TwilioCredential{}, err
	}
	provider, err := NewTwilioClient(client, credential)
	if err != nil {
		return TwilioClient{}, credential, err
	}
	return provider, credential, nil
}

func twilioBasicAuthorizationForTest() string {
	material := twilioAPIKeySIDTextForTest() + ":" + string(twilioAPIKeySecretForTest())
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(material))
}

func newPayPalTestClient(client exchange.Client) (PayPalClient, PayPalAccessToken, error) {
	token, err := ParsePayPalAccessToken([]byte("paypal_test_token"))
	if err != nil {
		return PayPalClient{}, PayPalAccessToken{}, err
	}
	provider, err := NewPayPalClient(client, token, false)
	if err != nil {
		return PayPalClient{}, token, err
	}
	return provider, token, nil
}

func closeStripeClient(t testing.TB, provider *StripeClient, credential *StripeCredential) {
	t.Helper()
	if gotErr := errors.Join(provider.Close(), credential.Close()); gotErr != nil {
		t.Fatalf("Stripe client cleanup error = %v, want nil", gotErr)
	}
}
