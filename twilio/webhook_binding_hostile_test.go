package twilio

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestWebhookIngressAttacksTheTwilioSignatureAndBindingBoundary(t *testing.T) {
	t.Parallel()

	const (
		publicURL = "https://mycompany.com/myapp.php?foo=1&bar=2"
		body      = "property=value&boolean=true"
	)
	signature := twilioFormTestSignature(t, publicURL)
	for _, testCase := range []struct {
		wantErr   error
		name      string
		target    string
		body      string
		signature string
	}{
		{name: "valid provider signature releases exact form", target: publicURL, body: body, signature: signature},
		{name: "body mutation releases nothing", target: publicURL, body: body + "x", signature: signature, wantErr: core.ErrTwilioVerification},
		{name: "foreign public target releases nothing", target: "https://mycompany.com/myapp.php?foo=changed", body: body, signature: signature, wantErr: core.ErrTwilioBinding},
		{name: "missing signature releases nothing", target: publicURL, body: body, wantErr: core.ErrTwilioAuthentication},
		{name: "wrong-length signature remains an authentication refusal", target: publicURL, body: body, signature: "short", wantErr: core.ErrTwilioAuthentication},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			receiver, token := twilioWebhookTestReceiver(t, publicURL)
			defer func() { _ = errors.Join(receiver.Close(), token.Close()) }()
			request := httptest.NewRequest(http.MethodPost, testCase.target, bytes.NewBufferString(testCase.body))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if testCase.signature != "" {
				request.Header.Set(core.TwilioWebhookSignatureHeaderName, testCase.signature)
			}
			call, callErr := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
			if callErr != nil {
				t.Fatalf("exchange.NewSocketServerCall() setup error = %v, want nil", callErr)
			}
			var destination bytes.Buffer
			observation, gotErr := receiver.Receive(WebhookReceiveRequest{
				Call: call, Destination: &destination,
				ObservedAt: temporal.InstantFromNanoseconds(1_800_000_000 * int64(temporal.NanosecondsPerSecond)),
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
			if observation.Validate() != nil || observation.Bytes.Uint64() != uint64(len(body)) || destination.String() != body {
				t.Fatalf("accepted webhook = observation:%+v body:%q, want exact body", observation, destination.String())
			}
		})
	}
}

func twilioFormTestSignature(t testing.TB, publicURL string) string {
	t.Helper()
	hash := hmac.New(sha1.New, []byte("12345123451234512345123451234512"))
	if _, err := hash.Write([]byte(publicURL + "booleantruepropertyvalue")); err != nil {
		t.Fatalf("Twilio fixture HMAC write error = %v, want nil", err)
	}
	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

func twilioWebhookTestReceiver(t testing.TB, publicURL string) (WebhookReceiver, AuthToken) {
	t.Helper()
	token, err := ParseAuthToken([]byte("12345123451234512345123451234512"))
	if err != nil {
		t.Fatalf("ParseAuthToken() error = %v, want nil", err)
	}
	endpoint, err := core.ParseHTTPEndpoint(publicURL)
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint() error = %v, want nil", err)
	}
	maximum, err := core.NewByteCount(core.TwilioWebhookCustodyMaximumBytes)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	receiver, err := NewWebhookReceiver(WebhookReceiverRequest{
		Token: token, PublicEndpoint: endpoint, Maximum: maximum, Representation: WebhookRepresentationForm,
	})
	if err != nil {
		t.Fatalf("NewWebhookReceiver() error = %v, want nil", err)
	}
	return receiver, token
}
