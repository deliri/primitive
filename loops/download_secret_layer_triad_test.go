package loops

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

		credentialValue, err := secretstore.NewValue([]byte("loops_test_key"))
		if err != nil {
			t.Fatalf("secretstore.NewValue(credential) error = %v, want nil", err)
		}
		webhookValue, err := secretstore.NewValue([]byte(officialWebhookSecret))
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

	t.Run("negative unusable secret material is refused at the bridge", func(t *testing.T) {
		t.Parallel()

		value, err := secretstore.NewValue([]byte("whsec_not base64"))
		if err != nil {
			t.Fatalf("secretstore.NewValue() error = %v, want nil", err)
		}
		t.Cleanup(func() { _ = value.Destroy() })
		credential, credentialErr := CredentialFromSecret(value)
		webhook, webhookErr := WebhookSecretFromSecret(value)
		if !errors.Is(credentialErr, core.ErrLoopsContract) || !errors.Is(webhookErr, core.ErrLoopsContract) || credential.Validate() == nil || webhook.Validate() == nil {
			t.Fatalf("bridges = (%v, %v), want contract refusals and zero values", credentialErr, webhookErr)
		}
		destroyed, err := secretstore.NewValue([]byte("loops_test_key"))
		if err != nil {
			t.Fatal(err)
		}
		if err := destroyed.Destroy(); err != nil {
			t.Fatal(err)
		}
		if _, err := CredentialFromSecret(destroyed); !errors.Is(err, core.ErrLoopsContract) {
			t.Fatalf("CredentialFromSecret(destroyed) = %v, want contract refusal", err)
		}
	})

	t.Run("positive authenticated download streams the bounded response", func(t *testing.T) {
		t.Parallel()

		credential, credentialErr := ParseCredential([]byte("loops_test_key"))
		if credentialErr != nil {
			t.Fatalf("ParseCredential() error = %v, want nil", credentialErr)
		}
		t.Cleanup(func() { _ = credential.Close() })
		var calls uint64
		exchangeClient, clientErr := exchange.NewClient(&http.Client{Transport: idempotencyRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if request.Method != http.MethodGet || request.URL.Host != core.LoopsAPIHost || request.URL.Path != "/api/v1/api-key" || request.Header.Get("Authorization") != "Bearer loops_test_key" {
				return nil, core.ErrLoopsBinding
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewBufferString(`{"success":true,"teamName":"Team"}`)), Request: request}, nil
		})})
		if clientErr != nil {
			t.Fatalf("exchange.NewClient() error = %v, want nil", clientErr)
		}
		provider, providerErr := NewClient(exchangeClient, credential)
		if providerErr != nil {
			t.Fatalf("NewClient() error = %v, want nil", providerErr)
		}
		t.Cleanup(func() { _ = provider.Close() })
		request, destination := loopsDownloadFixture(t)
		if gotErr := request.Validate(); gotErr != nil {
			t.Fatalf("DownloadRequest.Validate() error = %v, want nil", gotErr)
		}
		response, gotErr := provider.Download(t.Context(), request, loopsStreamPolicy(t))
		if gotErr != nil || response.Validate() != nil || calls != 1 || destination.String() != `{"success":true,"teamName":"Team"}` {
			t.Fatalf("Client.Download() = (response:%v, error:%v, calls:%d, bytes:%q), want valid, nil, 1, exact JSON", response, gotErr, calls, destination.String())
		}
	})

	t.Run("negative and neutral downloads never reach transport", func(t *testing.T) {
		t.Parallel()

		credential, credentialErr := ParseCredential([]byte("loops_test_key"))
		if credentialErr != nil {
			t.Fatalf("ParseCredential() error = %v, want nil", credentialErr)
		}
		t.Cleanup(func() { _ = credential.Close() })
		var calls uint64
		exchangeClient, clientErr := exchange.NewClient(&http.Client{Transport: idempotencyRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			return nil, core.ErrLoopsBinding
		})})
		if clientErr != nil {
			t.Fatalf("exchange.NewClient() error = %v, want nil", clientErr)
		}
		provider, providerErr := NewClient(exchangeClient, credential)
		if providerErr != nil {
			t.Fatalf("NewClient() error = %v, want nil", providerErr)
		}
		t.Cleanup(func() { _ = provider.Close() })
		keyed, _ := loopsDownloadFixture(t)
		keyed.Stream.Semantics.Replay = exchange.ReplaySingleAttemptWithIdempotencyKey
		keyed.Stream.Semantics.IdempotencyKey = loopsIdempotencyKey(t, "read-key")
		bodyDoor, _ := loopsDownloadFixture(t)
		bodyDoor.Stream.Target, _ = (Route{Operation: SendEvent}).Endpoint()
		for _, request := range []DownloadRequest{{}, keyed, bodyDoor} {
			response, gotErr := provider.Download(t.Context(), request, loopsStreamPolicy(t))
			if !errors.Is(gotErr, core.ErrLoopsBinding) || response.Stream.Metadata.Attempts != 0 ||
				response.Stream.Metadata.Bytes.Uint64() != 0 || len(response.Stream.Metadata.Headers.Values) != 0 || calls != 0 {
				t.Fatalf("Client.Download() = (%v, %v, calls:%d), want zero, errors.Is(..., %v), 0", response, gotErr, calls, core.ErrLoopsBinding)
			}
		}
	})
}

func loopsDownloadFixture(t testing.TB) (DownloadRequest, *bytes.Buffer) {
	t.Helper()
	target, err := (Route{Operation: TestAPIKey}).Endpoint()
	if err != nil {
		t.Fatalf("Route.Endpoint() error = %v, want nil", err)
	}
	destination := &bytes.Buffer{}
	return DownloadRequest{Stream: exchange.DownloadRequest{
		Target: target, Destination: destination,
		Semantics:                   exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt},
		ExpectedResponseContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: core.HTTPStatusOK(),
	}}, destination
}
