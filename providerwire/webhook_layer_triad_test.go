package providerwire

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
	stripesdk "github.com/stripe/stripe-go/v86"
)

func TestStripeWebhookIngressLayerTriad(t *testing.T) {
	t.Parallel()

	body := []byte(`{"id":"evt_123","type":"payment_intent.succeeded"}`)
	observedAt := providerWebhookInstant(t, 1_800_000_000)
	tolerance, err := temporal.DurationFromMinutes(5)
	if err != nil {
		t.Fatalf("temporal.DurationFromMinutes() error = %v, want nil", err)
	}
	signed := stripesdk.GenerateTestSignedPayload(&stripesdk.UnsignedPayload{
		Payload: body, Secret: "whsec_test_123", Timestamp: time.Unix(1_800_000_000, 0),
	})

	t.Run("positive official SDK signature releases exact raw JSON", func(t *testing.T) {
		t.Parallel()

		receiver, secret := stripeWebhookTestReceiver(t)
		defer closeStripeWebhookTestReceiver(t, &receiver, &secret)
		request := webhookRequest(t, "https://api.example.test/v1/stripe", body, "application/json")
		request.Header.Set(StripeWebhookSignatureHeaderName, signed.Header)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(StripeWebhookReceiveRequest{
			Request: request, Destination: &destination, ObservedAt: observedAt, Tolerance: tolerance,
		})
		if gotErr != nil || observation.Provider != ProviderStripe || observation.Bytes.Uint64() != uint64(len(body)) || !bytes.Equal(destination.Bytes(), body) {
			t.Fatalf("Stripe Receive() = observation:%+v body:%q error:%v, want exact %d-byte Stripe body and nil", observation, destination.Bytes(), gotErr, len(body))
		}
	})

	t.Run("negative one-byte body mutation preserves typed verification refusal", func(t *testing.T) {
		t.Parallel()

		receiver, secret := stripeWebhookTestReceiver(t)
		defer closeStripeWebhookTestReceiver(t, &receiver, &secret)
		mutated := append([]byte(nil), body...)
		mutated[len(mutated)-2] ^= 1
		request := webhookRequest(t, "https://api.example.test/v1/stripe", mutated, "application/json")
		request.Header.Set(StripeWebhookSignatureHeaderName, signed.Header)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(StripeWebhookReceiveRequest{
			Request: request, Destination: &destination, ObservedAt: observedAt, Tolerance: tolerance,
		})
		if !errors.Is(gotErr, core.ErrProviderWireVerification) || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("Stripe mutated Receive() = observation:%+v bytes:%d error:%v, want zero, 0, %v", observation, destination.Len(), gotErr, core.ErrProviderWireVerification)
		}
	})

	t.Run("neutral absent signature releases no provider observation", func(t *testing.T) {
		t.Parallel()

		receiver, secret := stripeWebhookTestReceiver(t)
		defer closeStripeWebhookTestReceiver(t, &receiver, &secret)
		request := webhookRequest(t, "https://api.example.test/v1/stripe", body, "application/json")
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(StripeWebhookReceiveRequest{
			Request: request, Destination: &destination, ObservedAt: observedAt, Tolerance: tolerance,
		})
		if !errors.Is(gotErr, core.ErrProviderWireAuthentication) || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("Stripe unsigned Receive() = observation:%+v bytes:%d error:%v, want zero, 0, %v", observation, destination.Len(), gotErr, core.ErrProviderWireAuthentication)
		}
	})
}

func TestTwilioWebhookIngressLayerTriad(t *testing.T) {
	t.Parallel()

	const (
		publicURL = "https://mycompany.com/myapp.php?foo=1&bar=2"
		body      = "property=value&boolean=true"
	)
	signature := twilioFormTestSignature(t, publicURL)
	t.Run("positive official SDK vector releases exact form body", func(t *testing.T) {
		t.Parallel()

		receiver, token := twilioWebhookTestReceiver(t, publicURL, TwilioWebhookRepresentationForm)
		defer closeTwilioWebhookTestReceiver(t, &receiver, &token)
		request := webhookRequest(t, publicURL, []byte(body), "application/x-www-form-urlencoded")
		request.Header.Set(TwilioWebhookSignatureHeaderName, signature)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination)
		if gotErr != nil || observation.Provider != ProviderTwilio || observation.Bytes.Uint64() != uint64(len(body)) || destination.String() != body {
			t.Fatalf("Twilio Receive() = observation:%+v body:%q error:%v, want exact %d-byte Twilio body and nil", observation, destination.String(), gotErr, len(body))
		}
	})

	t.Run("negative body mutation fails official SDK verification", func(t *testing.T) {
		t.Parallel()

		receiver, token := twilioWebhookTestReceiver(t, publicURL, TwilioWebhookRepresentationForm)
		defer closeTwilioWebhookTestReceiver(t, &receiver, &token)
		request := webhookRequest(t, publicURL, []byte(body+"x"), "application/x-www-form-urlencoded")
		request.Header.Set(TwilioWebhookSignatureHeaderName, signature)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination)
		if !errors.Is(gotErr, core.ErrProviderWireVerification) || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("Twilio mutated Receive() = observation:%+v bytes:%d error:%v, want zero, 0, %v", observation, destination.Len(), gotErr, core.ErrProviderWireVerification)
		}
	})

	t.Run("neutral foreign query binding is refused before body release", func(t *testing.T) {
		t.Parallel()

		receiver, token := twilioWebhookTestReceiver(t, publicURL, TwilioWebhookRepresentationForm)
		defer closeTwilioWebhookTestReceiver(t, &receiver, &token)
		request := webhookRequest(t, "https://mycompany.com/myapp.php?foo=changed", []byte(body), "application/x-www-form-urlencoded")
		request.Header.Set(TwilioWebhookSignatureHeaderName, signature)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination)
		if !errors.Is(gotErr, core.ErrProviderWireBinding) || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("Twilio foreign target Receive() = observation:%+v bytes:%d error:%v, want zero, 0, %v", observation, destination.Len(), gotErr, core.ErrProviderWireBinding)
		}
	})
}

func TestTwilioJSONWebhookIngressLayerTriad(t *testing.T) {
	t.Parallel()

	const publicURL = "https://mycompany.com/json-callback?tenant=one"
	body := []byte(`{"CallSid":"CA123","CallStatus":"completed"}`)
	target, signature := twilioJSONTestSignature(t, publicURL, body)

	t.Run("positive official SDK JSON body hash releases exact raw body", func(t *testing.T) {
		t.Parallel()

		receiver, token := twilioWebhookTestReceiver(t, publicURL, TwilioWebhookRepresentationJSON)
		defer closeTwilioWebhookTestReceiver(t, &receiver, &token)
		request := webhookRequest(t, target, body, "application/json")
		request.Header.Set(TwilioWebhookSignatureHeaderName, signature)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination)
		if gotErr != nil || observation.Provider != ProviderTwilio || observation.Bytes.Uint64() != uint64(len(body)) || !bytes.Equal(destination.Bytes(), body) {
			t.Fatalf("Twilio JSON Receive() = observation:%+v body:%q error:%v, want exact %d-byte Twilio body and nil", observation, destination.Bytes(), gotErr, len(body))
		}
	})

	t.Run("negative one-byte JSON mutation fails body digest verification", func(t *testing.T) {
		t.Parallel()

		receiver, token := twilioWebhookTestReceiver(t, publicURL, TwilioWebhookRepresentationJSON)
		defer closeTwilioWebhookTestReceiver(t, &receiver, &token)
		mutated := append([]byte(nil), body...)
		mutated[len(mutated)-2] ^= 1
		request := webhookRequest(t, target, mutated, "application/json")
		request.Header.Set(TwilioWebhookSignatureHeaderName, signature)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination)
		if !errors.Is(gotErr, core.ErrProviderWireVerification) || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("Twilio mutated JSON Receive() = observation:%+v bytes:%d error:%v, want zero, 0, %v", observation, destination.Len(), gotErr, core.ErrProviderWireVerification)
		}
	})

	t.Run("neutral missing body digest query releases no observation", func(t *testing.T) {
		t.Parallel()

		receiver, token := twilioWebhookTestReceiver(t, publicURL, TwilioWebhookRepresentationJSON)
		defer closeTwilioWebhookTestReceiver(t, &receiver, &token)
		request := webhookRequest(t, publicURL, body, "application/json")
		request.Header.Set(TwilioWebhookSignatureHeaderName, signature)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination)
		if !errors.Is(gotErr, core.ErrProviderWireBinding) || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("Twilio JSON without digest Receive() = observation:%+v bytes:%d error:%v, want zero, 0, %v", observation, destination.Len(), gotErr, core.ErrProviderWireBinding)
		}
	})
}

func TestPlunkWebhookIngressLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive configured shared secret streams opaque JSON exactly", func(t *testing.T) {
		t.Parallel()

		receiver, secret := plunkWebhookTestReceiver(t)
		defer closePlunkWebhookTestReceiver(t, &receiver, &secret)
		body := []byte(`{"event":"delivered"}`)
		request := webhookRequest(t, "https://api.example.test/v1/plunk", body, "application/json")
		request.Header.Set("Authorization", "Bearer plunk-shared-secret")
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination)
		if gotErr != nil || observation.Provider != ProviderPlunk || observation.Bytes.Uint64() != uint64(len(body)) || !bytes.Equal(destination.Bytes(), body) {
			t.Fatalf("Plunk Receive() = observation:%+v body:%q error:%v, want exact %d-byte Plunk body and nil", observation, destination.Bytes(), gotErr, len(body))
		}
	})

	t.Run("negative foreign bearer is refused before body release", func(t *testing.T) {
		t.Parallel()

		receiver, secret := plunkWebhookTestReceiver(t)
		defer closePlunkWebhookTestReceiver(t, &receiver, &secret)
		request := webhookRequest(t, "https://api.example.test/v1/plunk", []byte(`{"event":"delivered"}`), "application/json")
		request.Header.Set("Authorization", "Bearer foreign-secret")
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination)
		if !errors.Is(gotErr, core.ErrProviderWireAuthentication) || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("Plunk foreign bearer Receive() = observation:%+v bytes:%d error:%v, want zero, 0, %v", observation, destination.Len(), gotErr, core.ErrProviderWireAuthentication)
		}
	})

	t.Run("neutral empty authenticated callback stays an exact zero-byte observation", func(t *testing.T) {
		t.Parallel()

		receiver, secret := plunkWebhookTestReceiver(t)
		defer closePlunkWebhookTestReceiver(t, &receiver, &secret)
		request := webhookRequest(t, "https://api.example.test/v1/plunk", []byte{}, "application/json")
		request.Header.Set("Authorization", "Bearer plunk-shared-secret")
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination)
		if gotErr != nil || observation.Provider != ProviderPlunk || observation.Bytes.Uint64() != 0 || destination.Len() != 0 {
			t.Fatalf("Plunk empty Receive() = observation:%+v bytes:%d error:%v, want Plunk zero-byte observation, 0, nil", observation, destination.Len(), gotErr)
		}
	})
}

func TestPayPalWebhookIngressLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive provider verification success releases exact raw event", func(t *testing.T) {
		t.Parallel()

		verificationCalls := uint64(0)
		receiver, token := payPalWebhookTestReceiver(t, &verificationCalls, PayPalWebhookVerificationSuccess)
		defer closePayPalWebhookTestReceiver(t, &receiver, &token)
		body := []byte(`{"id":"WH-EVENT-123","event_type":"PAYMENT.CAPTURE.COMPLETED"}`)
		request := payPalWebhookRequest(t, body, true)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination, providerOperationPolicy(t))
		if gotErr != nil || verificationCalls != 1 || observation.Provider != ProviderPayPal || observation.Bytes.Uint64() != uint64(len(body)) || !bytes.Equal(destination.Bytes(), body) {
			t.Fatalf("PayPal Receive() = calls:%d observation:%+v body:%q error:%v, want 1 and exact %d-byte PayPal body", verificationCalls, observation, destination.Bytes(), gotErr, len(body))
		}
	})

	t.Run("negative provider verification failure releases no event", func(t *testing.T) {
		t.Parallel()

		verificationCalls := uint64(0)
		receiver, token := payPalWebhookTestReceiver(t, &verificationCalls, PayPalWebhookVerificationFailure)
		defer closePayPalWebhookTestReceiver(t, &receiver, &token)
		request := payPalWebhookRequest(t, []byte(`{"id":"WH-EVENT-123"}`), true)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination, providerOperationPolicy(t))
		if !errors.Is(gotErr, core.ErrProviderWireVerification) || verificationCalls != 1 || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("PayPal refused Receive() = calls:%d observation:%+v bytes:%d error:%v, want 1, zero, 0, %v", verificationCalls, observation, destination.Len(), gotErr, core.ErrProviderWireVerification)
		}
	})

	t.Run("neutral missing provider header performs no verification call", func(t *testing.T) {
		t.Parallel()

		verificationCalls := uint64(0)
		receiver, token := payPalWebhookTestReceiver(t, &verificationCalls, PayPalWebhookVerificationSuccess)
		defer closePayPalWebhookTestReceiver(t, &receiver, &token)
		request := payPalWebhookRequest(t, []byte(`{"id":"WH-EVENT-123"}`), false)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination, providerOperationPolicy(t))
		if !errors.Is(gotErr, core.ErrProviderWireAuthentication) || verificationCalls != 0 || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("PayPal missing-header Receive() = calls:%d observation:%+v bytes:%d error:%v, want 0, zero, 0, %v", verificationCalls, observation, destination.Len(), gotErr, core.ErrProviderWireAuthentication)
		}
	})
}

func webhookRequest(t testing.TB, target string, body []byte, media string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	request.Header.Set("Content-Type", media)
	return request
}

func twilioFormTestSignature(t testing.TB, publicURL string) string {
	t.Helper()
	// The independently generated fixture follows Twilio's documented form
	// signing input. Production acceptance still goes through Twilio's SDK.
	hash := hmac.New(sha1.New, []byte("12345123451234512345123451234512"))
	if _, err := hash.Write([]byte(publicURL + "booleantruepropertyvalue")); err != nil {
		t.Fatalf("Twilio fixture HMAC write error = %v, want nil", err)
	}
	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

func stripeWebhookMaximum(t testing.TB) core.ByteCount {
	t.Helper()
	maximum, err := core.NewByteCount(StripeWebhookMaximumBytes)
	if err != nil {
		t.Fatalf("core.NewByteCount(StripeWebhookMaximumBytes) error = %v, want nil", err)
	}
	return maximum
}

func twilioWebhookMaximum(t testing.TB) core.ByteCount {
	t.Helper()
	maximum, err := core.NewByteCount(TwilioWebhookMaximumBytes)
	if err != nil {
		t.Fatalf("core.NewByteCount(TwilioWebhookMaximumBytes) error = %v, want nil", err)
	}
	return maximum
}

func plunkWebhookMaximum(t testing.TB) core.ByteCount {
	t.Helper()
	maximum, err := core.NewByteCount(PlunkWebhookMaximumBytes)
	if err != nil {
		t.Fatalf("core.NewByteCount(PlunkWebhookMaximumBytes) error = %v, want nil", err)
	}
	return maximum
}

func payPalWebhookMaximum(t testing.TB) core.ByteCount {
	t.Helper()
	maximum, err := core.NewByteCount(PayPalWebhookMaximumBytes)
	if err != nil {
		t.Fatalf("core.NewByteCount(PayPalWebhookMaximumBytes) error = %v, want nil", err)
	}
	return maximum
}

func stripeWebhookTestReceiver(t testing.TB) (StripeWebhookReceiver, StripeWebhookSecret) {
	t.Helper()
	secret, err := ParseStripeWebhookSecret([]byte("whsec_test_123"))
	if err != nil {
		t.Fatalf("ParseStripeWebhookSecret() error = %v, want nil", err)
	}
	receiver, err := NewStripeWebhookReceiver(secret, stripeWebhookMaximum(t))
	if err != nil {
		t.Fatalf("NewStripeWebhookReceiver() error = %v, want nil", err)
	}
	return receiver, secret
}

func twilioWebhookTestReceiver(t testing.TB, publicURL string, representation TwilioWebhookRepresentation) (TwilioWebhookReceiver, TwilioAuthToken) {
	t.Helper()
	token, err := ParseTwilioAuthToken([]byte("12345123451234512345123451234512"))
	if err != nil {
		t.Fatalf("ParseTwilioAuthToken() error = %v, want nil", err)
	}
	endpoint, err := core.ParseHTTPEndpoint(publicURL)
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint() error = %v, want nil", err)
	}
	receiver, err := NewTwilioWebhookReceiver(TwilioWebhookReceiverRequest{
		Token: token, PublicEndpoint: endpoint, Representation: representation, Maximum: twilioWebhookMaximum(t),
	})
	if err != nil {
		t.Fatalf("NewTwilioWebhookReceiver() error = %v, want nil", err)
	}
	return receiver, token
}

func twilioJSONTestSignature(t testing.TB, publicURL string, body []byte) (string, string) {
	t.Helper()
	digest := sha256.Sum256(body)
	separator := "?"
	if strings.Contains(publicURL, separator) {
		separator = "&"
	}
	target := publicURL + separator + TwilioBodySHA256QueryName + "=" + hex.EncodeToString(digest[:])
	hash := hmac.New(sha1.New, []byte("12345123451234512345123451234512"))
	if _, err := hash.Write([]byte(target)); err != nil {
		t.Fatalf("Twilio JSON fixture HMAC write error = %v, want nil", err)
	}
	return target, base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

func plunkWebhookTestReceiver(t testing.TB) (PlunkWebhookReceiver, PlunkWebhookSecret) {
	t.Helper()
	secret, err := ParsePlunkWebhookSecret([]byte("plunk-shared-secret"))
	if err != nil {
		t.Fatalf("ParsePlunkWebhookSecret() error = %v, want nil", err)
	}
	receiver, err := NewPlunkWebhookReceiver(secret, plunkWebhookMaximum(t))
	if err != nil {
		t.Fatalf("NewPlunkWebhookReceiver() error = %v, want nil", err)
	}
	return receiver, secret
}

func providerWebhookInstant(t testing.TB, seconds int64) temporal.Instant {
	t.Helper()
	return temporal.InstantFromNanoseconds(seconds * int64(temporal.NanosecondsPerSecond))
}

func payPalWebhookRequest(t testing.TB, body []byte, complete bool) *http.Request {
	t.Helper()
	request := webhookRequest(t, "https://api.example.test/v1/paypal", body, "application/json")
	if complete {
		request.Header.Set(PayPalAuthAlgorithmHeaderName, "SHA256withRSA")
	}
	request.Header.Set(PayPalCertificateURLHeaderName, "https://api.paypal.com/v1/notifications/certs/CERT-123")
	request.Header.Set(PayPalTransmissionIDHeaderName, "transmission-123")
	request.Header.Set(PayPalTransmissionSignatureHeaderName, "signature-123")
	request.Header.Set(PayPalTransmissionTimeHeaderName, "2026-08-31T20:00:00Z")
	return request
}

func payPalWebhookTestReceiver(t testing.TB, calls *uint64, status PayPalWebhookVerificationStatus) (PayPalWebhookReceiver, PayPalAccessToken) {
	t.Helper()
	client, err := exchange.NewClient(&http.Client{Transport: providerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		*calls++
		if request.Method != http.MethodPost || request.URL.Host != PayPalLiveAPIHost || request.URL.Path != payPalWebhookVerificationPath || request.Header.Get("Authorization") != "Bearer paypal_test_token" {
			return nil, core.ErrProviderWireBinding
		}
		body, readErr := readExactTestRequestBody(request)
		if readErr != nil {
			return nil, readErr
		}
		limit, limitErr := core.NewByteCount(core.JSONDocumentMaximumBytes)
		if limitErr != nil {
			return nil, limitErr
		}
		limits := core.DefaultStrictJSONLimits()
		limits.DocumentMaximumBytes = limit
		projection, decodeErr := core.DecodeStrictJSONStructure[payPalWebhookVerificationProjection](body, limits)
		if decodeErr != nil || projection.Validate() != nil {
			return nil, errors.Join(decodeErr, projection.Validate())
		}
		response, marshalErr := (payPalWebhookVerificationResponse{Status: status}).MarshalJSON()
		if marshalErr != nil {
			return nil, marshalErr
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(response)), Request: request,
		}, nil
	})})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	provider, token, err := newPayPalTestClient(client)
	if err != nil {
		t.Fatalf("newPayPalTestClient() error = %v, want nil", err)
	}
	identifier, err := ParsePayPalWebhookID("WH123456789")
	if err != nil {
		t.Fatalf("ParsePayPalWebhookID() error = %v, want nil", err)
	}
	receiver, err := NewPayPalWebhookReceiver(provider, identifier, payPalWebhookMaximum(t))
	if err != nil {
		t.Fatalf("NewPayPalWebhookReceiver() error = %v, want nil", err)
	}
	if err := provider.Close(); err != nil {
		t.Fatalf("PayPalClient.Close() error = %v, want nil", err)
	}
	return receiver, token
}

func closeStripeWebhookTestReceiver(t testing.TB, receiver *StripeWebhookReceiver, secret *StripeWebhookSecret) {
	t.Helper()
	if gotErr := errors.Join(receiver.Close(), secret.Close()); gotErr != nil {
		t.Fatalf("Stripe webhook cleanup error = %v, want nil", gotErr)
	}
}

func closeTwilioWebhookTestReceiver(t testing.TB, receiver *TwilioWebhookReceiver, token *TwilioAuthToken) {
	t.Helper()
	if gotErr := errors.Join(receiver.Close(), token.Close()); gotErr != nil {
		t.Fatalf("Twilio webhook cleanup error = %v, want nil", gotErr)
	}
}

func closePlunkWebhookTestReceiver(t testing.TB, receiver *PlunkWebhookReceiver, secret *PlunkWebhookSecret) {
	t.Helper()
	if gotErr := errors.Join(receiver.Close(), secret.Close()); gotErr != nil {
		t.Fatalf("Plunk webhook cleanup error = %v, want nil", gotErr)
	}
}

func closePayPalWebhookTestReceiver(t testing.TB, receiver *PayPalWebhookReceiver, token *PayPalAccessToken) {
	t.Helper()
	if gotErr := errors.Join(receiver.Close(), token.Close()); gotErr != nil {
		t.Fatalf("PayPal webhook cleanup error = %v, want nil", gotErr)
	}
}
