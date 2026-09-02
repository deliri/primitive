package paypal

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/secretstore"
)

func TestDownloadAndSecretCustodyLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive secret custody constructs independent redacted provider values", func(t *testing.T) {
		t.Parallel()

		tokenValue, err := secretstore.NewValue([]byte("paypal-access-token"))
		if err != nil {
			t.Fatalf("secretstore.NewValue(token) error = %v, want nil", err)
		}
		clientValue, err := secretstore.NewValue([]byte("paypal-secret"))
		if err != nil {
			t.Fatalf("secretstore.NewValue(client secret) error = %v, want nil", err)
		}
		t.Cleanup(func() { _ = errors.Join(tokenValue.Destroy(), clientValue.Destroy()) })
		token, err := AccessTokenFromSecret(tokenValue)
		if err != nil {
			t.Fatalf("AccessTokenFromSecret() error = %v, want nil", err)
		}
		identity, err := exchange.ParseBasicAuthorizationIdentity("paypal-client")
		if err != nil {
			t.Fatalf("exchange.ParseBasicAuthorizationIdentity() error = %v, want nil", err)
		}
		clientCredential, err := ClientCredentialFromSecret(identity, clientValue)
		if err != nil {
			t.Fatalf("ClientCredentialFromSecret() error = %v, want nil", err)
		}
		t.Cleanup(func() { _ = errors.Join(token.Close(), clientCredential.Close()) })
		if token.Validate() != nil || clientCredential.Validate() != nil || fmt.Sprint(token) != core.RedactedValueText || fmt.Sprint(clientCredential) != core.RedactedValueText {
			t.Fatalf("secret bridges validation/format = (%v, %v, %q, %q), want valid and redacted", token.Validate(), clientCredential.Validate(), fmt.Sprint(token), fmt.Sprint(clientCredential))
		}
	})

	t.Run("positive authenticated download streams the bounded response", func(t *testing.T) {
		t.Parallel()

		token, tokenErr := ParseAccessToken([]byte("paypal-access-token"))
		if tokenErr != nil {
			t.Fatalf("ParseAccessToken() error = %v, want nil", tokenErr)
		}
		t.Cleanup(func() { _ = token.Close() })
		var calls uint64
		exchangeClient, clientErr := exchange.NewClient(&http.Client{Transport: transportRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if request.Method != http.MethodGet || request.URL.Host != core.PayPalLiveAPIHost || request.Header.Get("Authorization") != "Bearer paypal-access-token" {
				return nil, core.ErrPayPalBinding
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewBufferString(`{"status":"COMPLETED"}`)), Request: request}, nil
		})})
		if clientErr != nil {
			t.Fatalf("exchange.NewClient() error = %v, want nil", clientErr)
		}
		provider, providerErr := NewClient(exchangeClient, token, false)
		if providerErr != nil {
			t.Fatalf("NewClient() error = %v, want nil", providerErr)
		}
		t.Cleanup(func() { _ = provider.Close() })
		request, destination := payPalDownloadFixture(t)
		if gotErr := request.Validate(); gotErr != nil {
			t.Fatalf("DownloadRequest.Validate() error = %v, want nil", gotErr)
		}
		response, gotErr := provider.Download(t.Context(), request, payPalTransportPolicy(t))
		if gotErr != nil || response.Validate() != nil || calls != 1 || destination.String() != `{"status":"COMPLETED"}` {
			t.Fatalf("Client.Download() = (response:%v, error:%v, calls:%d, bytes:%q), want valid, nil, 1, exact JSON", response, gotErr, calls, destination.String())
		}
	})

	t.Run("negative zero download never reaches transport", func(t *testing.T) {
		t.Parallel()

		token, tokenErr := ParseAccessToken([]byte("paypal-access-token"))
		if tokenErr != nil {
			t.Fatalf("ParseAccessToken() error = %v, want nil", tokenErr)
		}
		t.Cleanup(func() { _ = token.Close() })
		var calls uint64
		exchangeClient, clientErr := exchange.NewClient(&http.Client{Transport: transportRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			return nil, core.ErrPayPalBinding
		})})
		if clientErr != nil {
			t.Fatalf("exchange.NewClient() error = %v, want nil", clientErr)
		}
		provider, providerErr := NewClient(exchangeClient, token, false)
		if providerErr != nil {
			t.Fatalf("NewClient() error = %v, want nil", providerErr)
		}
		t.Cleanup(func() { _ = provider.Close() })
		response, gotErr := provider.Download(t.Context(), DownloadRequest{}, payPalTransportPolicy(t))
		if !errors.Is(gotErr, core.ErrPayPalBinding) || response.Stream.Metadata.Attempts != 0 ||
			response.Stream.Metadata.Bytes.Uint64() != 0 || len(response.Stream.Metadata.Headers.Values) != 0 || calls != 0 {
			t.Fatalf("Client.Download(zero) = (%v, %v, calls:%d), want zero, errors.Is(..., %v), 0", response, gotErr, calls, core.ErrPayPalBinding)
		}
	})
}

func payPalDownloadFixture(t testing.TB) (DownloadRequest, *bytes.Buffer) {
	t.Helper()
	target, err := core.ParseHTTPEndpoint("https://" + core.PayPalLiveAPIHost + "/v2/checkout/orders/order-123")
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint() error = %v, want nil", err)
	}
	limit, err := core.NewByteCount(1 << 10)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	destination := &bytes.Buffer{}
	return DownloadRequest{Stream: exchange.DownloadRequest{
		Target: target, Destination: destination,
		Semantics:                   exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt},
		ExpectedResponseContentType: core.HTTPMediaTypeJSON(), ResponseBodyLimit: limit, ExpectedStatus: core.HTTPStatusOK(),
	}}, destination
}
