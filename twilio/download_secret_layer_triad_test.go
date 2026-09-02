package twilio

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/secretstore"
)

func TestDownloadAndSecretCustodyLayerTriad(t *testing.T) {
	t.Parallel()

	accountText := "AC" + strings.Repeat("a", 32)
	keyText := "SK" + strings.Repeat("b", 32)

	t.Run("positive secret custody constructs independent redacted provider values", func(t *testing.T) {
		t.Parallel()

		account, accountErr := ParseAccountSID(accountText)
		key, keyErr := ParseAPIKeySID(keyText)
		if accountErr != nil || keyErr != nil || key.String() != keyText {
			t.Fatalf("Twilio SID parsing = (%q, %v, %q, %v), want exact account/key and nil", account, accountErr, key.String(), keyErr)
		}
		credentialValue, err := secretstore.NewValue([]byte("twilio-api-key-secret"))
		if err != nil {
			t.Fatalf("secretstore.NewValue(credential) error = %v, want nil", err)
		}
		tokenValue, err := secretstore.NewValue([]byte("twilio-auth-token"))
		if err != nil {
			t.Fatalf("secretstore.NewValue(auth token) error = %v, want nil", err)
		}
		t.Cleanup(func() { _ = errors.Join(credentialValue.Destroy(), tokenValue.Destroy()) })
		credential, err := CredentialFromSecret(account, key, credentialValue)
		if err != nil {
			t.Fatalf("CredentialFromSecret() error = %v, want nil", err)
		}
		token, err := AuthTokenFromSecret(tokenValue)
		if err != nil {
			t.Fatalf("AuthTokenFromSecret() error = %v, want nil", err)
		}
		t.Cleanup(func() { _ = errors.Join(credential.Close(), token.Close()) })
		if credential.Validate() != nil || token.Validate() != nil || fmt.Sprint(credential) != core.RedactedValueText || fmt.Sprint(token) != core.RedactedValueText {
			t.Fatalf("secret bridges validation/format = (%v, %v, %q, %q), want valid and redacted", credential.Validate(), token.Validate(), fmt.Sprint(credential), fmt.Sprint(token))
		}
	})

	t.Run("positive authenticated download streams the account-bound response", func(t *testing.T) {
		t.Parallel()

		account, _ := ParseAccountSID(accountText)
		key, _ := ParseAPIKeySID(keyText)
		credential, credentialErr := NewCredential(account, key, []byte("twilio-api-key-secret"))
		if credentialErr != nil {
			t.Fatalf("NewCredential() error = %v, want nil", credentialErr)
		}
		t.Cleanup(func() { _ = credential.Close() })
		wantAuthorization := "Basic " + base64.StdEncoding.EncodeToString([]byte(keyText+":"+"twilio-api-key-secret"))
		var calls uint64
		exchangeClient, clientErr := exchange.NewClient(&http.Client{Transport: transportRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if request.Method != http.MethodGet || request.URL.Host != core.TwilioAPIHost || request.Header.Get("Authorization") != wantAuthorization {
				return nil, core.ErrTwilioBinding
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewBufferString(`{"status":"delivered"}`)), Request: request}, nil
		})})
		if clientErr != nil {
			t.Fatalf("exchange.NewClient() error = %v, want nil", clientErr)
		}
		provider, providerErr := NewClient(exchangeClient, credential)
		if providerErr != nil {
			t.Fatalf("NewClient() error = %v, want nil", providerErr)
		}
		t.Cleanup(func() { _ = provider.Close() })
		request, destination := twilioDownloadFixture(t, accountText)
		if gotErr := request.Validate(); gotErr != nil {
			t.Fatalf("DownloadRequest.Validate() error = %v, want nil", gotErr)
		}
		response, gotErr := provider.Download(t.Context(), request, twilioTransportPolicy(t))
		if gotErr != nil || response.Validate() != nil || calls != 1 || destination.String() != `{"status":"delivered"}` {
			t.Fatalf("Client.Download() = (response:%v, error:%v, calls:%d, bytes:%q), want valid, nil, 1, exact JSON", response, gotErr, calls, destination.String())
		}
	})

	t.Run("negative zero download never reaches transport", func(t *testing.T) {
		t.Parallel()

		account, _ := ParseAccountSID(accountText)
		key, _ := ParseAPIKeySID(keyText)
		credential, credentialErr := NewCredential(account, key, []byte("twilio-api-key-secret"))
		if credentialErr != nil {
			t.Fatalf("NewCredential() error = %v, want nil", credentialErr)
		}
		t.Cleanup(func() { _ = credential.Close() })
		var calls uint64
		exchangeClient, clientErr := exchange.NewClient(&http.Client{Transport: transportRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			return nil, core.ErrTwilioBinding
		})})
		if clientErr != nil {
			t.Fatalf("exchange.NewClient() error = %v, want nil", clientErr)
		}
		provider, providerErr := NewClient(exchangeClient, credential)
		if providerErr != nil {
			t.Fatalf("NewClient() error = %v, want nil", providerErr)
		}
		t.Cleanup(func() { _ = provider.Close() })
		response, gotErr := provider.Download(t.Context(), DownloadRequest{}, twilioTransportPolicy(t))
		if !errors.Is(gotErr, core.ErrTwilioBinding) || response.Stream.Metadata.Attempts != 0 ||
			response.Stream.Metadata.Bytes.Uint64() != 0 || len(response.Stream.Metadata.Headers.Values) != 0 || calls != 0 {
			t.Fatalf("Client.Download(zero) = (%v, %v, calls:%d), want zero, errors.Is(..., %v), 0", response, gotErr, calls, core.ErrTwilioBinding)
		}
	})

	for _, tc := range []struct {
		name       string
		wantString string
		value      WebhookRepresentation
		wantValid  bool
	}{
		{name: "zero representation is refused", value: WebhookRepresentationUnknown},
		{name: "form representation is admitted", value: WebhookRepresentationForm, wantValid: true, wantString: "form"},
		{name: "JSON representation is admitted", value: WebhookRepresentationJSON, wantValid: true, wantString: "json"},
		{name: "next representation is refused", value: WebhookRepresentationJSON + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.value.IsValid() != tc.wantValid || tc.value.String() != tc.wantString {
				t.Fatalf("WebhookRepresentation(%d) valid/string = (%t, %q), want (%t, %q)", tc.value, tc.value.IsValid(), tc.value.String(), tc.wantValid, tc.wantString)
			}
			tc.value.OffWireEnum()
		})
	}
}

func twilioDownloadFixture(t testing.TB, account string) (DownloadRequest, *bytes.Buffer) {
	t.Helper()
	target, err := core.ParseHTTPEndpoint("https://" + core.TwilioAPIHost + "/2010-04-01/Accounts/" + account + "/Messages/message-123.json")
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
