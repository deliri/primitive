package providerwire

import (
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

func TestPayPalOAuthAcquisitionLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive documented client-credentials response produces a bounded grant", func(t *testing.T) {
		t.Parallel()

		responseBody, err := (payPalOAuthResponse{
			Scope: "openid", AccessToken: "paypal_access_token", TokenType: payPalOAuthTokenTypeBearer,
			AppID: "APP-123", ExpiresIn: 28800, Nonce: "nonce-123",
		}).MarshalJSON()
		if err != nil {
			t.Fatalf("payPalOAuthResponse.MarshalJSON() error = %v, want nil", err)
		}
		calls := uint64(0)
		client := payPalOAuthExchangeClient(t, &calls, responseBody)
		credential := payPalOAuthCredential(t)
		defer closePayPalOAuthCredential(t, &credential)
		grant, gotErr := AcquirePayPalAccessGrant(context.Background(), PayPalAccessGrantRequest{Client: client, Credential: credential, Policy: providerOperationPolicy(t)})
		if gotErr != nil {
			t.Fatalf("AcquirePayPalAccessGrant() error = %v, want nil", gotErr)
		}
		defer closePayPalAccessGrant(t, &grant)
		wantLifetime, err := temporal.DurationFromSeconds(28800)
		if err != nil {
			t.Fatalf("temporal.DurationFromSeconds() error = %v, want nil", err)
		}
		if calls != 1 || grant.Validate() != nil {
			t.Fatalf("PayPal access grant = calls:%d value:%v validation:%v, want 1, redacted, nil", calls, grant, grant.Validate())
		}
		comparison, err := grant.ExpiresIn.Compare(wantLifetime)
		if err != nil || comparison != core.ComparisonEqual {
			t.Fatalf("PayPal access lifetime comparison = (%v, %v), want equal and nil", comparison, err)
		}
	})

	t.Run("negative unknown provider member is a typed closed-protocol refusal", func(t *testing.T) {
		t.Parallel()

		responseBody := []byte(`{"scope":"openid","access_token":"paypal_access_token","token_type":"Bearer","app_id":"APP-123","expires_in":28800,"nonce":"nonce-123","unknown":true}`)
		calls := uint64(0)
		client := payPalOAuthExchangeClient(t, &calls, responseBody)
		credential := payPalOAuthCredential(t)
		defer closePayPalOAuthCredential(t, &credential)
		grant, gotErr := AcquirePayPalAccessGrant(context.Background(), PayPalAccessGrantRequest{Client: client, Credential: credential, Policy: providerOperationPolicy(t)})
		if !errors.Is(gotErr, core.ErrProviderWireContract) || !errors.Is(gotErr, core.ErrJSONContract) || grant.Validate() == nil || calls != 1 {
			t.Fatalf("PayPal unknown response = grant:%v error:%v calls:%d, want zero, provider+JSON contract, 1", grant, gotErr, calls)
		}
	})

	t.Run("neutral cancelled acquisition performs no OAuth request", func(t *testing.T) {
		t.Parallel()

		calls := uint64(0)
		client := payPalOAuthExchangeClient(t, &calls, []byte(`{}`))
		credential := payPalOAuthCredential(t)
		defer closePayPalOAuthCredential(t, &credential)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		grant, gotErr := AcquirePayPalAccessGrant(ctx, PayPalAccessGrantRequest{Client: client, Credential: credential, Policy: providerOperationPolicy(t)})
		if !errors.Is(gotErr, context.Canceled) || grant.Validate() == nil || calls != 0 {
			t.Fatalf("PayPal cancelled acquisition = grant:%v error:%v calls:%d, want zero, context cancellation, 0", grant, gotErr, calls)
		}
	})
}

func TestPayPalClientCredentialHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		clientID string
		secret   []byte
		wantErr  error
	}{
		{name: "ordinary provider credential is copied", clientID: "paypal-client", secret: []byte("paypal-secret")},
		{name: "one-byte client identity is admitted", clientID: "p", secret: []byte("s")},
		{name: "client identity exact custody ceiling is admitted", clientID: strings.Repeat("p", PayPalClientIDMaximumBytes), secret: []byte("s")},
		{name: "empty client identity is rejected", clientID: "", secret: []byte("s"), wantErr: core.ErrProviderWireContract},
		{name: "client identity colon conflicts with Basic framing", clientID: "pay:pal", secret: []byte("s"), wantErr: core.ErrProviderWireContract},
		{name: "one-byte client secret is admitted", clientID: "p", secret: []byte("s")},
		{name: "client secret exact custody ceiling is admitted", clientID: "p", secret: []byte(strings.Repeat("s", PayPalClientSecretMaximumBytes))},
		{name: "empty client secret is rejected", clientID: "p", secret: []byte{}, wantErr: core.ErrProviderWireContract},
		{name: "client secret one above custody ceiling is rejected", clientID: "p", secret: []byte(strings.Repeat("s", PayPalClientSecretMaximumBytes+1)), wantErr: core.ErrProviderWireContract},
		{name: "client secret malformed UTF-8 is rejected", clientID: "p", secret: []byte{0xff}, wantErr: core.ErrProviderWireContract},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			identity, identityErr := exchange.ParseBasicAuthorizationIdentity(testCase.clientID)
			credential, gotErr := NewPayPalClientCredential(identity, append([]byte(nil), testCase.secret...))
			gotErr = errors.Join(identityErr, gotErr)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("PayPal client credential error = %v, want %v", gotErr, testCase.wantErr)
			}
			if testCase.wantErr != nil {
				if credential.Validate() == nil {
					t.Fatalf("rejected PayPal client credential Validate() error = nil, want non-nil")
				}
				return
			}
			if gotValidateErr := credential.Validate(); gotValidateErr != nil {
				t.Fatalf("PayPal client credential Validate() error = %v, want nil", gotValidateErr)
			}
			closePayPalOAuthCredential(t, &credential)
		})
	}
}

func payPalOAuthExchangeClient(t testing.TB, calls *uint64, responseBody []byte) exchange.Client {
	t.Helper()

	client, err := exchange.NewClient(&http.Client{Transport: providerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		*calls++
		wantAuthorization := "Basic " + base64.StdEncoding.EncodeToString([]byte("paypal-client:paypal-secret"))
		if request.Method != http.MethodPost || request.URL.Host != PayPalLiveAPIHost || request.URL.Path != payPalOAuthPath ||
			request.Header.Get("Authorization") != wantAuthorization || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			return nil, core.ErrProviderWireBinding
		}
		body, readErr := readExactTestRequestBody(request)
		if readErr != nil {
			return nil, readErr
		}
		if string(body) != payPalOAuthGrantBody {
			return nil, core.ErrProviderWireContract
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(string(responseBody))),
			Request:    request,
		}, nil
	})})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	return client
}

func payPalOAuthCredential(t testing.TB) PayPalClientCredential {
	t.Helper()

	identity, err := exchange.ParseBasicAuthorizationIdentity("paypal-client")
	if err != nil {
		t.Fatalf("exchange.ParseBasicAuthorizationIdentity() error = %v, want nil", err)
	}
	credential, err := NewPayPalClientCredential(identity, []byte("paypal-secret"))
	if err != nil {
		t.Fatalf("NewPayPalClientCredential() error = %v, want nil", err)
	}
	return credential
}

func providerOperationPolicy(t testing.TB) exchange.OperationPolicy {
	t.Helper()

	operation, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(operation) error = %v, want nil", err)
	}
	attempt, err := temporal.DurationFromSeconds(4)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(attempt) error = %v, want nil", err)
	}
	return exchange.OperationPolicy{
		OperationTimeout: operation,
		AttemptTimeout:   attempt,
		Retry:            exchange.RetryPolicy{MaximumAttempts: 1},
		Redirect:         exchange.RedirectPolicy{Mode: exchange.RedirectReject},
	}
}

func closePayPalOAuthCredential(t testing.TB, credential *PayPalClientCredential) {
	t.Helper()
	if gotErr := credential.Close(); gotErr != nil {
		t.Fatalf("PayPalClientCredential.Close() error = %v, want nil", gotErr)
	}
}

func closePayPalAccessGrant(t testing.TB, grant *PayPalAccessGrant) {
	t.Helper()
	if gotErr := grant.Close(); gotErr != nil {
		t.Fatalf("PayPalAccessGrant.Close() error = %v, want nil", gotErr)
	}
}
