package paypal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestWebhookIngressAttacksThePayPalProviderVerificationBoundary(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		wantErr       error
		name          string
		status        string
		wantCalls     uint64
		wantBodyBytes int
		complete      bool
	}{
		{name: "provider success releases exact raw event", status: payPalWebhookVerificationSuccessText, complete: true, wantCalls: 1, wantBodyBytes: len(`{"id":"WH-123","event_type":"PAYMENT.CAPTURE.COMPLETED"}`)},
		{name: "provider failure releases nothing", status: payPalWebhookVerificationFailureText, complete: true, wantErr: core.ErrPayPalVerification, wantCalls: 1},
		{name: "missing signed header never calls provider", status: payPalWebhookVerificationSuccessText, wantErr: core.ErrPayPalAuthentication},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			var calls uint64
			receiver, client, token := payPalWebhookTestReceiver(t, &calls, testCase.status)
			defer func() { _ = errors.Join(receiver.Close(), client.Close(), token.Close()) }()

			body := []byte(`{"id":"WH-123","event_type":"PAYMENT.CAPTURE.COMPLETED"}`)
			request := payPalIncomingWebhookRequest(t, body, testCase.complete)
			observedAt, err := temporal.ParseRFC3339("2026-09-02T12:00:00Z")
			if err != nil {
				t.Fatalf("temporal.ParseRFC3339() error = %v, want nil", err)
			}
			tolerance, err := temporal.DurationFromMinutes(5)
			if err != nil {
				t.Fatalf("temporal.DurationFromMinutes() error = %v, want nil", err)
			}
			call, callErr := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
			if callErr != nil {
				t.Fatalf("exchange.NewSocketServerCall() setup error = %v, want nil", callErr)
			}
			var destination bytes.Buffer
			observation, gotErr := receiver.Receive(PayPalWebhookReceiveRequest{
				Call: call, Destination: &destination, ObservedAt: observedAt,
				Tolerance: tolerance, Policy: payPalOAuthTestPolicy(t),
			})
			if !errors.Is(gotErr, testCase.wantErr) || calls != testCase.wantCalls {
				t.Fatalf("Receive() = error:%v calls:%d, want %v, %d", gotErr, calls, testCase.wantErr, testCase.wantCalls)
			}
			if testCase.wantErr != nil {
				if observation.Validate() == nil || destination.Len() != 0 {
					t.Fatalf("refused webhook = observation:%+v bytes:%d, want zero evidence", observation, destination.Len())
				}
				return
			}
			if observation.Validate() != nil || observation.Bytes.Uint64() != uint64(testCase.wantBodyBytes) || !bytes.Equal(destination.Bytes(), body) {
				t.Fatalf("accepted webhook = observation:%+v body:%q, want exact body", observation, destination.Bytes())
			}
		})
	}
}

func payPalWebhookTestReceiver(t testing.TB, calls *uint64, status string) (PayPalWebhookReceiver, Client, AccessToken) {
	t.Helper()
	exchangeClient, err := exchange.NewClient(&http.Client{Transport: transportRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		*calls++
		if request.Method != http.MethodPost || request.URL.Host != core.PayPalLiveAPIHost ||
			request.URL.Path != payPalWebhookVerificationPath ||
			request.Header.Get("Authorization") != "Bearer paypal-access-token" {
			return nil, core.ErrPayPalBinding
		}
		body, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			return nil, readErr
		}
		projection, decodeErr := core.DecodeStrictJSONStructure[payPalWebhookVerificationProjection](body, core.DefaultStrictJSONLimits())
		if decodeErr != nil || projection.Validate() != nil || projection.WebhookID != PayPalWebhookID("WEBHOOK123") {
			return nil, errors.Join(core.ErrPayPalContract, decodeErr, projection.Validate())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"verification_status":"` + status + `"}`)),
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
	client, err := NewClient(exchangeClient, token, false)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}
	webhookID, err := ParsePayPalWebhookID("WEBHOOK123")
	if err != nil {
		t.Fatalf("ParsePayPalWebhookID() error = %v, want nil", err)
	}
	maximum, err := core.NewByteCount(core.PayPalWebhookEventCustodyMaximumBytes)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	receiver, err := NewPayPalWebhookReceiver(client, webhookID, maximum)
	if err != nil {
		t.Fatalf("NewPayPalWebhookReceiver() error = %v, want nil", err)
	}
	return receiver, client, token
}

func payPalIncomingWebhookRequest(t testing.TB, body []byte, complete bool) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.test/paypal", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if complete {
		request.Header.Set(core.PayPalAuthAlgorithmHeaderName, "SHA256withRSA")
	}
	request.Header.Set(core.PayPalCertificateURLHeaderName, "https://api.paypal.com/v1/notifications/certs/CERT123")
	request.Header.Set(core.PayPalTransmissionIDHeaderName, "transmission-123")
	request.Header.Set(core.PayPalTransmissionSignatureHeaderName, "signature-123")
	request.Header.Set(core.PayPalTransmissionTimeHeaderName, "2026-09-02T12:00:00Z")
	return request
}
