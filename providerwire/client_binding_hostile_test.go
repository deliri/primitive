package providerwire

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	stripesdk "github.com/stripe/stripe-go/v86"
)

type providerBindingValidator uint8

const (
	providerBindingStripe providerBindingValidator = iota + 1
	providerBindingPlunk
	providerBindingTwilio
	providerBindingPayPal
)

func TestProviderOutboundBindingHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		host      string
		path      string
		media     string
		response  string
		validator providerBindingValidator
		mutate    func(testing.TB, *exchange.StreamRoundTripRequest)
		wantErr   error
	}{
		{name: "Stripe v1 form authority is admitted", host: StripeAPIHost, path: "/v1/payment_intents", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingStripe},
		{name: "Stripe v1 nested resource authority is admitted", host: StripeAPIHost, path: "/v1/customers/cus_123", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingStripe},
		{name: "Stripe v1 query remains product-owned", host: StripeAPIHost, path: "/v1/payment_intents?limit=1", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingStripe},
		{name: "Stripe v2 JSON authority is admitted", host: StripeAPIHost, path: "/v2/core/accounts", media: "application/json", response: "application/json", validator: providerBindingStripe},
		{name: "Plunk v1 JSON authority is admitted", host: PlunkAPIHost, path: "/v1/send", media: "application/json", response: "application/json", validator: providerBindingPlunk},
		{name: "Plunk v1 contact route is admitted", host: PlunkAPIHost, path: "/v1/contacts", media: "application/json", response: "application/json", validator: providerBindingPlunk},
		{name: "Twilio exact account message route is admitted", host: TwilioAPIHost, path: "/2010-04-01/Accounts/AC0123456789abcdefABCDEF0123456789/Messages.json", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingTwilio},
		{name: "Twilio exact account call route is admitted", host: TwilioAPIHost, path: "/2010-04-01/Accounts/AC0123456789abcdefABCDEF0123456789/Calls.json", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingTwilio},
		{name: "PayPal v1 JSON route is admitted", host: PayPalLiveAPIHost, path: "/v1/vault/payment-tokens", media: "application/json", response: "application/json", validator: providerBindingPayPal},
		{name: "PayPal v2 JSON route is admitted", host: PayPalLiveAPIHost, path: "/v2/checkout/orders", media: "application/json", response: "application/json", validator: providerBindingPayPal},
		{name: "Stripe foreign authority is rejected", host: "example.invalid", path: "/v1/payment_intents", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingStripe, wantErr: core.ErrProviderWireBinding},
		{name: "Stripe explicit default HTTPS port is the same authority", host: StripeAPIHost + ":443", path: "/v1/payment_intents", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingStripe},
		{name: "Plunk explicit default HTTPS port is the same authority", host: PlunkAPIHost + ":443", path: "/v1/send", media: "application/json", response: "application/json", validator: providerBindingPlunk},
		{name: "Twilio explicit default HTTPS port is the same authority", host: TwilioAPIHost + ":443", path: "/2010-04-01/Accounts/AC0123456789abcdefABCDEF0123456789/Messages.json", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingTwilio},
		{name: "PayPal explicit default HTTPS port is the same authority", host: PayPalLiveAPIHost + ":443", path: "/v2/checkout/orders", media: "application/json", response: "application/json", validator: providerBindingPayPal},
		{name: "Stripe HTTP scheme is rejected", host: StripeAPIHost, path: "/v1/payment_intents", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingStripe, mutate: mutateProviderSchemeHTTP, wantErr: core.ErrProviderWireBinding},
		{name: "Stripe encoded raw path is rejected", host: StripeAPIHost, path: "/v1/%70ayment_intents", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingStripe, wantErr: core.ErrProviderWireBinding},
		{name: "Stripe unowned v3 prefix is rejected", host: StripeAPIHost, path: "/v3/payment_intents", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingStripe, wantErr: core.ErrProviderWireBinding},
		{name: "Stripe version lookalike prefix is rejected", host: StripeAPIHost, path: "/void/payment_intents", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingStripe, wantErr: core.ErrProviderWireBinding},
		{name: "Stripe v1 JSON request media is rejected", host: StripeAPIHost, path: "/v1/payment_intents", media: "application/json", response: "application/json", validator: providerBindingStripe, wantErr: core.ErrProviderWireBinding},
		{name: "Stripe v2 form request media is rejected", host: StripeAPIHost, path: "/v2/core/accounts", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingStripe, wantErr: core.ErrProviderWireBinding},
		{name: "Stripe non-JSON response contract is rejected", host: StripeAPIHost, path: "/v1/payment_intents", media: "application/x-www-form-urlencoded", response: "text/plain", validator: providerBindingStripe, wantErr: core.ErrProviderWireBinding},
		{name: "Stripe GET cannot cross the request-body operation", host: StripeAPIHost, path: "/v1/payment_intents", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingStripe, mutate: mutateProviderMethodGET, wantErr: core.ErrProviderWireContract},
		{name: "Stripe product-owned single attempt is admitted without idempotency", host: StripeAPIHost, path: "/v1/payment_intents", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingStripe, mutate: mutateProviderReplayWithoutKey},
		{name: "Caller-supplied authorization header is rejected", host: StripeAPIHost, path: "/v1/payment_intents", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingStripe, mutate: mutateProviderCallerHeader, wantErr: core.ErrProviderWireContract},
		{name: "Nil source is rejected before provider binding", host: StripeAPIHost, path: "/v1/payment_intents", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingStripe, mutate: func(_ testing.TB, request *exchange.StreamRoundTripRequest) { request.Source = nil }, wantErr: core.ErrProviderWireContract},
		{name: "Nil destination is rejected before provider binding", host: StripeAPIHost, path: "/v1/payment_intents", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingStripe, mutate: func(_ testing.TB, request *exchange.StreamRoundTripRequest) { request.Destination = nil }, wantErr: core.ErrProviderWireContract},
		{name: "Plunk obsolete API authority is rejected", host: "api.useplunk.com", path: "/v1/send", media: "application/json", response: "application/json", validator: providerBindingPlunk, wantErr: core.ErrProviderWireBinding},
		{name: "Plunk product-owned single attempt is admitted without idempotency", host: PlunkAPIHost, path: "/v1/send", media: "application/json", response: "application/json", validator: providerBindingPlunk, mutate: mutateProviderReplayWithoutKey},
		{name: "Twilio foreign account route is rejected", host: TwilioAPIHost, path: "/2010-04-01/Accounts/ACffffffffffffffffffffffffffffffff/Messages.json", media: "application/x-www-form-urlencoded", response: "application/json", validator: providerBindingTwilio, wantErr: core.ErrProviderWireBinding},
		{name: "Twilio JSON request media is rejected", host: TwilioAPIHost, path: "/2010-04-01/Accounts/AC0123456789abcdefABCDEF0123456789/Messages.json", media: "application/json", response: "application/json", validator: providerBindingTwilio, wantErr: core.ErrProviderWireBinding},
		{name: "PayPal foreign sandbox authority is rejected by live client", host: PayPalSandboxAPIHost, path: "/v2/checkout/orders", media: "application/json", response: "application/json", validator: providerBindingPayPal, wantErr: core.ErrProviderWireBinding},
		{name: "PayPal nonversioned path is rejected", host: PayPalLiveAPIHost, path: "/checkout/orders", media: "application/json", response: "application/json", validator: providerBindingPayPal, wantErr: core.ErrProviderWireBinding},
		{name: "PayPal version lookalike prefix is rejected", host: PayPalLiveAPIHost, path: "/verification/orders", media: "application/json", response: "application/json", validator: providerBindingPayPal, wantErr: core.ErrProviderWireBinding},
		{name: "PayPal request identity one above its own ceiling is rejected", host: PayPalLiveAPIHost, path: "/v2/checkout/orders", media: "application/json", response: "application/json", validator: providerBindingPayPal, mutate: mutatePayPalRequestIDAboveMaximum, wantErr: core.ErrProviderWireContract},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := providerBindingRequest(t, testCase.host, testCase.path, testCase.media, testCase.response, testCase.validator)
			if testCase.mutate != nil {
				testCase.mutate(t, &request)
			}
			gotErr := validateProviderBindingForTest(testCase.validator, request)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("provider binding validation error = %v, want %v", gotErr, testCase.wantErr)
			}
		})
	}
}

func TestStripeAPIVersionMatchesPinnedOfficialSDK(t *testing.T) {
	t.Parallel()
	if StripeAPIVersion != stripesdk.APIVersion {
		t.Fatalf("StripeAPIVersion = %q, want pinned official SDK %q", StripeAPIVersion, stripesdk.APIVersion)
	}
}

func TestProviderIdempotencyLimitsRemainIndependent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		value      string
		wantStripe bool
		wantPlunk  bool
		wantPayPal bool
	}{
		{name: "one-byte identity crosses every provider", value: "a", wantStripe: true, wantPlunk: true, wantPayPal: true},
		{name: "PayPal one below published ceiling crosses every provider", value: strings.Repeat("p", PayPalRequestIDMaximumBytes-1), wantStripe: true, wantPlunk: true, wantPayPal: true},
		{name: "PayPal exact published ceiling crosses every provider", value: strings.Repeat("p", PayPalRequestIDMaximumBytes), wantStripe: true, wantPlunk: true, wantPayPal: true},
		{name: "PayPal one above published ceiling remains valid only for Stripe and Plunk", value: strings.Repeat("p", PayPalRequestIDMaximumBytes+1), wantStripe: true, wantPlunk: true},
		{name: "Stripe one below published ceiling crosses Stripe and Plunk", value: strings.Repeat("s", StripeIdempotencyKeyMaximumBytes-1), wantStripe: true, wantPlunk: true},
		{name: "Stripe exact published ceiling crosses Stripe and Plunk", value: strings.Repeat("s", StripeIdempotencyKeyMaximumBytes), wantStripe: true, wantPlunk: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			key, err := exchange.ParseIdempotencyKey(testCase.value)
			if err != nil {
				t.Fatalf("exchange.ParseIdempotencyKey() error = %v, want nil", err)
			}
			gotStripe := validProviderIdempotencyKey(key, StripeIdempotencyKeyMaximumBytes)
			gotPlunk := validProviderIdempotencyKey(key, PlunkIdempotencyKeyMaximumBytes)
			gotPayPal := validProviderIdempotencyKey(key, PayPalRequestIDMaximumBytes)
			if gotStripe != testCase.wantStripe || gotPlunk != testCase.wantPlunk || gotPayPal != testCase.wantPayPal {
				t.Fatalf("provider idempotency admission = Stripe:%t Plunk:%t PayPal:%t, want %t/%t/%t", gotStripe, gotPlunk, gotPayPal, testCase.wantStripe, testCase.wantPlunk, testCase.wantPayPal)
			}
		})
	}
}

func providerBindingRequest(t testing.TB, host, requestPath, requestMedia, responseMedia string, validator providerBindingValidator) exchange.StreamRoundTripRequest {
	t.Helper()

	replay := exchange.ReplaySingleAttemptWithIdempotencyKey
	keyText := "provider-request-1"
	if validator == providerBindingTwilio || validator == providerBindingPayPal {
		replay = exchange.ReplaySingleAttempt
		keyText = ""
	}
	target, err := core.ParseHTTPEndpoint("https://" + host + requestPath)
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint() error = %v, want nil", err)
	}
	requestType, err := core.ParseHTTPMediaType(requestMedia)
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType(request) error = %v, want nil", err)
	}
	responseType, err := core.ParseHTTPMediaType(responseMedia)
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType(response) error = %v, want nil", err)
	}
	length, err := core.NewByteLength(2)
	if err != nil {
		t.Fatalf("core.NewByteLength() error = %v, want nil", err)
	}
	limit, err := core.NewByteCount(1 << 10)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	semantics := exchange.RequestSemantics{Method: exchange.MethodPost, Replay: replay}
	if keyText != "" {
		key, keyErr := exchange.ParseIdempotencyKey(keyText)
		if keyErr != nil {
			t.Fatalf("exchange.ParseIdempotencyKey() error = %v, want nil", keyErr)
		}
		semantics.IdempotencyKey = key
	}
	return exchange.StreamRoundTripRequest{
		Target: target, Source: bytes.NewReader([]byte(`{}`)), Destination: &bytes.Buffer{},
		Semantics: semantics, RequestContentType: requestType, ExpectedResponseContentType: responseType,
		RequestContentLength: length, ResponseBodyLimit: limit, ExpectedStatus: core.HTTPStatusOK(),
	}
}

func validateProviderBindingForTest(validator providerBindingValidator, request exchange.StreamRoundTripRequest) error {
	switch validator {
	case providerBindingStripe:
		return validateStripeRequest(request)
	case providerBindingPlunk:
		return validatePlunkRequest(request)
	case providerBindingTwilio:
		return validateTwilioRequest(request, TwilioAccountSID("AC0123456789abcdefABCDEF0123456789"))
	case providerBindingPayPal:
		return validatePayPalRequest(request, PayPalLiveAPIHost)
	}
	return core.ErrProviderWireContract
}

func mutateProviderSchemeHTTP(t testing.TB, request *exchange.StreamRoundTripRequest) {
	t.Helper()
	urlValue := request.Target.HTTPURL()
	urlValue.Scheme = "http"
	target, err := core.ParseHTTPEndpoint(urlValue.String())
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint(HTTP) error = %v, want nil", err)
	}
	request.Target = target
}

func mutateProviderMethodGET(_ testing.TB, request *exchange.StreamRoundTripRequest) {
	request.Semantics = exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}
}

func mutateProviderReplayWithoutKey(_ testing.TB, request *exchange.StreamRoundTripRequest) {
	request.Semantics = exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}
}

func mutatePayPalRequestIDAboveMaximum(t testing.TB, request *exchange.StreamRoundTripRequest) {
	t.Helper()
	key, err := exchange.ParseIdempotencyKey(strings.Repeat("p", PayPalRequestIDMaximumBytes+1))
	if err != nil {
		t.Fatalf("exchange.ParseIdempotencyKey() error = %v, want nil", err)
	}
	request.Semantics = exchange.RequestSemantics{
		Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttemptWithIdempotencyKey, IdempotencyKey: key,
	}
}

func mutateProviderCallerHeader(t testing.TB, request *exchange.StreamRoundTripRequest) {
	t.Helper()
	header, err := providerHeader("X-Test-Caller", "present")
	if err != nil {
		t.Fatalf("providerHeader() error = %v, want nil", err)
	}
	request.Headers = exchange.Headers{Values: []exchange.Header{header}}
}
