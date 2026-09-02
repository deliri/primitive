package stripe

import (
	"bytes"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
	stripesdk "github.com/stripe/stripe-go/v86"
)

func TestWebhookIngressAttacksTheStripeVerificationBoundary(t *testing.T) {
	t.Parallel()

	body := []byte(`{"id":"evt_123","type":"payment_intent.succeeded"}`)
	observedAt := temporal.InstantFromNanoseconds(1_800_000_000 * int64(temporal.NanosecondsPerSecond))
	tolerance, err := temporal.DurationFromMinutes(5)
	if err != nil {
		t.Fatalf("temporal.DurationFromMinutes() error = %v, want nil", err)
	}
	signed := stripesdk.GenerateTestSignedPayload(&stripesdk.UnsignedPayload{
		Payload: body, Secret: "whsec_test_123", Timestamp: time.Unix(1_800_000_000, 0),
	})

	for _, testCase := range []struct {
		wantErr   error
		name      string
		signature string
		body      []byte
	}{
		{name: "official SDK signature releases exact raw JSON", body: body, signature: signed.Header},
		{name: "one-byte mutation releases nothing", body: append(append([]byte(nil), body[:len(body)-2]...), body[len(body)-2]^1, body[len(body)-1]), signature: signed.Header, wantErr: core.ErrStripeVerification},
		{name: "absent signature releases nothing", body: body, wantErr: core.ErrStripeAuthentication},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			receiver, secret := stripeWebhookTestReceiver(t)
			defer func() { _ = errors.Join(receiver.Close(), secret.Close()) }()
			request := httptest.NewRequest(http.MethodPost, "https://example.test/stripe", bytes.NewReader(testCase.body))
			request.Header.Set("Content-Type", "application/json")
			if testCase.signature != "" {
				request.Header.Set(core.StripeWebhookSignatureHeaderName, testCase.signature)
			}
			call, callErr := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
			if callErr != nil {
				t.Fatalf("exchange.NewSocketServerCall() setup error = %v, want nil", callErr)
			}
			var destination bytes.Buffer
			observation, gotErr := receiver.Receive(WebhookReceiveRequest{
				Call: call, Destination: &destination, ObservedAt: observedAt, Tolerance: tolerance,
			})
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("Receive() error = %v, want %v", gotErr, testCase.wantErr)
			}
			if testCase.wantErr != nil {
				if observation.Validate() == nil || destination.Len() != 0 {
					t.Fatalf("refused webhook = observation:%+v bytes:%d, want zero evidence", observation, destination.Len())
				}
				return
			}
			if observation.Validate() != nil || observation.Bytes.Uint64() != uint64(len(body)) || !bytes.Equal(destination.Bytes(), body) {
				t.Fatalf("accepted webhook = observation:%+v body:%q, want exact body", observation, destination.Bytes())
			}
		})
	}
}

func TestSignatureInstantOverflowRetainsStripeVerificationIdentity(t *testing.T) {
	t.Parallel()

	seconds := math.MaxInt64/int64(temporal.NanosecondsPerSecond) + 1
	got, gotErr := signatureInstant("t=" + strconv.FormatInt(seconds, 10))
	if got != (temporal.Instant{}) || !errors.Is(gotErr, core.ErrStripeVerification) {
		t.Fatalf("signatureInstant(overflow) = (%v, %v), want zero and %v", got, gotErr, core.ErrStripeVerification)
	}
}

func stripeWebhookTestReceiver(t testing.TB) (WebhookReceiver, WebhookSecret) {
	t.Helper()
	secret, err := ParseWebhookSecret([]byte("whsec_test_123"))
	if err != nil {
		t.Fatalf("ParseWebhookSecret() error = %v, want nil", err)
	}
	maximum, err := core.NewByteCount(core.StripeWebhookCustodyMaximumBytes)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	receiver, err := NewWebhookReceiver(secret, maximum)
	if err != nil {
		t.Fatalf("NewWebhookReceiver() error = %v, want nil", err)
	}
	return receiver, secret
}
