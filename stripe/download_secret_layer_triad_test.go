package stripe

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

		credentialValue, err := secretstore.NewValue([]byte("rk_test_credential"))
		if err != nil {
			t.Fatalf("secretstore.NewValue(credential) error = %v, want nil", err)
		}
		webhookValue, err := secretstore.NewValue([]byte("whsec_verified-webhook-secret"))
		if err != nil {
			t.Fatalf("secretstore.NewValue(webhook) error = %v, want nil", err)
		}
		t.Cleanup(func() { _ = errors.Join(credentialValue.Destroy(), webhookValue.Destroy()) })
		credential, err := CredentialFromSecret(credentialValue)
		if err != nil {
			t.Fatalf("CredentialFromSecret() error = %v, want nil", err)
		}
		webhook, err := WebhookSecretFromSecret(webhookValue)
		if err != nil {
			t.Fatalf("WebhookSecretFromSecret() error = %v, want nil", err)
		}
		t.Cleanup(func() { _ = errors.Join(credential.Close(), webhook.Close()) })
		if credential.Validate() != nil || webhook.Validate() != nil || fmt.Sprint(credential) != core.RedactedValueText || fmt.Sprint(webhook) != core.RedactedValueText {
			t.Fatalf("secret bridges validation/format = (%v, %v, %q, %q), want valid and redacted", credential.Validate(), webhook.Validate(), fmt.Sprint(credential), fmt.Sprint(webhook))
		}
	})

	t.Run("positive authenticated download streams the bounded response", func(t *testing.T) {
		t.Parallel()

		credential, credentialErr := ParseCredential([]byte("rk_test_credential"))
		if credentialErr != nil {
			t.Fatalf("ParseCredential() error = %v, want nil", credentialErr)
		}
		t.Cleanup(func() { _ = credential.Close() })
		var calls uint64
		exchangeClient, clientErr := exchange.NewClient(&http.Client{Transport: idempotencyRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if request.Method != http.MethodGet || request.URL.Host != core.StripeAPIHost || request.Header.Get("Authorization") != "Bearer rk_test_credential" || request.Header.Get(core.StripeVersionHeaderName) != core.StripeAPIVersion {
				return nil, core.ErrStripeBinding
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewBufferString(`{"object":"file"}`)), Request: request}, nil
		})})
		if clientErr != nil {
			t.Fatalf("exchange.NewClient() error = %v, want nil", clientErr)
		}
		provider, providerErr := NewClient(exchangeClient, credential)
		if providerErr != nil {
			t.Fatalf("NewClient() error = %v, want nil", providerErr)
		}
		t.Cleanup(func() {
			if gotErr := provider.Close(); gotErr != nil {
				t.Errorf("Client.Close() error = %v, want nil", gotErr)
			}
		})
		request, destination := stripeDownloadFixture(t)
		if gotErr := request.Validate(); gotErr != nil {
			t.Fatalf("DownloadRequest.Validate() error = %v, want nil", gotErr)
		}
		response, gotErr := provider.Download(t.Context(), request, stripeStreamPolicy(t))
		if gotErr != nil || response.Validate() != nil || calls != 1 || destination.String() != `{"object":"file"}` {
			t.Fatalf("Client.Download() = (response:%v, error:%v, calls:%d, bytes:%q), want valid, nil, 1, exact JSON", response, gotErr, calls, destination.String())
		}
	})

	t.Run("negative zero download never reaches transport", func(t *testing.T) {
		t.Parallel()

		credential, credentialErr := ParseCredential([]byte("rk_test_credential"))
		if credentialErr != nil {
			t.Fatalf("ParseCredential() error = %v, want nil", credentialErr)
		}
		t.Cleanup(func() { _ = credential.Close() })
		var calls uint64
		exchangeClient, clientErr := exchange.NewClient(&http.Client{Transport: idempotencyRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			return nil, core.ErrStripeBinding
		})})
		if clientErr != nil {
			t.Fatalf("exchange.NewClient() error = %v, want nil", clientErr)
		}
		provider, providerErr := NewClient(exchangeClient, credential)
		if providerErr != nil {
			t.Fatalf("NewClient() error = %v, want nil", providerErr)
		}
		t.Cleanup(func() { _ = provider.Close() })
		response, gotErr := provider.Download(t.Context(), DownloadRequest{}, stripeStreamPolicy(t))
		if !errors.Is(gotErr, core.ErrStripeBinding) || response.Stream.Metadata.Attempts != 0 ||
			response.Stream.Metadata.Bytes.Uint64() != 0 || len(response.Stream.Metadata.Headers.Values) != 0 || calls != 0 {
			t.Fatalf("Client.Download(zero) = (%v, %v, calls:%d), want zero, errors.Is(..., %v), 0", response, gotErr, calls, core.ErrStripeBinding)
		}
	})
}

func stripeDownloadFixture(t testing.TB) (DownloadRequest, *bytes.Buffer) {
	t.Helper()
	target, err := core.ParseHTTPEndpoint("https://" + core.StripeAPIHost + "/v1/files/file_123/contents")
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
