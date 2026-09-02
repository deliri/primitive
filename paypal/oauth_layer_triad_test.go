package paypal

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

func TestOAuthAcquisitionAttacksThePayPalTransportBoundary(t *testing.T) {
	t.Parallel()

	t.Run("documented client credentials exchange produces a bounded grant", func(t *testing.T) {
		t.Parallel()
		calls := uint64(0)
		client := payPalOAuthTestClient(t, &calls,
			`{"scope":"openid","access_token":"paypal_access_token","token_type":"Bearer","app_id":"APP-123","expires_in":28800,"nonce":"nonce-123"}`)
		credential := payPalOAuthTestCredential(t)
		defer func() { _ = credential.Close() }()

		grant, gotErr := AcquirePayPalAccessGrant(context.Background(), PayPalAccessGrantRequest{
			Client: client, Credential: credential, Policy: payPalOAuthTestPolicy(t), Sandbox: true,
		})
		if gotErr != nil {
			t.Fatalf("AcquirePayPalAccessGrant() error = %v, want nil", gotErr)
		}
		defer func() { _ = grant.Close() }()
		wantLifetime, err := temporal.DurationFromSeconds(28800)
		if err != nil {
			t.Fatalf("temporal.DurationFromSeconds() error = %v, want nil", err)
		}
		comparison, err := grant.ExpiresIn.Compare(wantLifetime)
		if calls != 1 || grant.Validate() != nil || err != nil || comparison != core.ComparisonEqual {
			t.Fatalf("OAuth grant = calls:%d validation:%v lifetime:(%v,%v), want 1, nil, equal, nil",
				calls, grant.Validate(), comparison, err)
		}
	})

	t.Run("unknown provider response member is a typed closed protocol refusal", func(t *testing.T) {
		t.Parallel()
		calls := uint64(0)
		client := payPalOAuthTestClient(t, &calls,
			`{"scope":"openid","access_token":"paypal_access_token","token_type":"Bearer","app_id":"APP-123","expires_in":28800,"nonce":"nonce-123","unknown":true}`)
		credential := payPalOAuthTestCredential(t)
		defer func() { _ = credential.Close() }()

		grant, gotErr := AcquirePayPalAccessGrant(context.Background(), PayPalAccessGrantRequest{
			Client: client, Credential: credential, Policy: payPalOAuthTestPolicy(t), Sandbox: true,
		})
		if !errors.Is(gotErr, core.ErrPayPalContract) || !errors.Is(gotErr, core.ErrJSONContract) ||
			grant.Validate() == nil || calls != 1 {
			t.Fatalf("OAuth unknown member = grant:%v error:%v calls:%d, want zero, PayPal+JSON contract, 1",
				grant, gotErr, calls)
		}
	})

	t.Run("cancelled acquisition performs no provider request", func(t *testing.T) {
		t.Parallel()
		calls := uint64(0)
		client := payPalOAuthTestClient(t, &calls, `{}`)
		credential := payPalOAuthTestCredential(t)
		defer func() { _ = credential.Close() }()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		grant, gotErr := AcquirePayPalAccessGrant(ctx, PayPalAccessGrantRequest{
			Client: client, Credential: credential, Policy: payPalOAuthTestPolicy(t), Sandbox: true,
		})
		if !errors.Is(gotErr, context.Canceled) || grant.Validate() == nil || calls != 0 {
			t.Fatalf("cancelled OAuth = grant:%v error:%v calls:%d, want zero, cancellation, 0", grant, gotErr, calls)
		}
	})
}

func TestPayPalAccessGrantRejectsZeroLifetime(t *testing.T) {
	t.Parallel()

	token, tokenErr := ParseAccessToken([]byte("paypal-access-token"))
	if tokenErr != nil {
		t.Fatalf("ParseAccessToken() error = %v, want nil", tokenErr)
	}
	t.Cleanup(func() { _ = token.Close() })
	grant := PayPalAccessGrant{Token: token}
	if gotErr := grant.Validate(); !errors.Is(gotErr, core.ErrPayPalContract) {
		t.Fatalf("PayPalAccessGrant.Validate(zero lifetime) error = %v, want %v", gotErr, core.ErrPayPalContract)
	}
}

func payPalOAuthTestClient(t testing.TB, calls *uint64, responseBody string) exchange.Client {
	t.Helper()
	client, err := exchange.NewClient(&http.Client{Transport: transportRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		*calls++
		wantAuthorization := "Basic " + base64.StdEncoding.EncodeToString([]byte("paypal-client:paypal-secret"))
		if request.Method != http.MethodPost || request.URL.Host != core.PayPalSandboxAPIHost || request.URL.Path != payPalOAuthPath ||
			request.Header.Get("Authorization") != wantAuthorization ||
			request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			return nil, core.ErrPayPalBinding
		}
		body, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			return nil, readErr
		}
		if string(body) != payPalOAuthGrantBody {
			return nil, core.ErrPayPalContract
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	return client
}

func payPalOAuthTestCredential(t testing.TB) ClientCredential {
	t.Helper()
	identity, err := exchange.ParseBasicAuthorizationIdentity("paypal-client")
	if err != nil {
		t.Fatalf("exchange.ParseBasicAuthorizationIdentity() error = %v, want nil", err)
	}
	credential, err := NewClientCredential(identity, []byte("paypal-secret"))
	if err != nil {
		t.Fatalf("NewClientCredential() error = %v, want nil", err)
	}
	return credential
}

func payPalOAuthTestPolicy(t testing.TB) exchange.OperationPolicy {
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
		OperationTimeout: operation, AttemptTimeout: attempt,
		Retry:    exchange.RetryPolicy{MaximumAttempts: 1},
		Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject},
	}
}
