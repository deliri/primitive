package loops

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// Published Standard Webhooks / Svix verification vector, independent of this
// package: https://docs.svix.com/receiving/verifying-payloads/how-manual
const (
	officialWebhookSecret    = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"
	officialWebhookID        = "msg_p5jXN8AQM9LWM0D4loKWxJek"
	officialWebhookTimestamp = "1614265330"
	officialWebhookBody      = `{"test": 2432232314}`
	officialWebhookSignature = "v1,g0hM9SsE+OTPJTGt/tmIKtSyZlE3uFJELVlNIOLJ1OE="
)

type loopsWebhookHeaders struct {
	contentType string
	id          []string
	timestamp   []string
	signature   []string
}

func officialHeaders() loopsWebhookHeaders {
	return loopsWebhookHeaders{id: []string{officialWebhookID}, timestamp: []string{officialWebhookTimestamp}, signature: []string{officialWebhookSignature}, contentType: "application/json"}
}

func loopsWebhookReceiver(t testing.TB, secretText string) WebhookReceiver {
	t.Helper()
	secret, err := ParseWebhookSecret([]byte(secretText))
	if err != nil {
		t.Fatalf("ParseWebhookSecret() error = %v, want nil", err)
	}
	receiver, err := NewWebhookReceiver(secret)
	if err != nil {
		t.Fatalf("NewWebhookReceiver() error = %v, want nil", err)
	}
	if err := secret.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = receiver.Close() })
	return receiver
}

func loopsSeconds(t testing.TB, seconds int64) temporal.Instant {
	t.Helper()
	instant, err := temporal.InstantFromUnixSeconds(seconds)
	if err != nil {
		t.Fatal(err)
	}
	return instant
}

func loopsWebhookReceive(t testing.TB, receiver WebhookReceiver, headers loopsWebhookHeaders, body io.Reader, destination io.Writer, observedAt temporal.Instant, directory string) (InboundObservation, error) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "https://example.test/loops", body)
	if headers.contentType != "" {
		request.Header.Set("Content-Type", headers.contentType)
	}
	for _, field := range [...]struct {
		name   string
		values []string
	}{{core.LoopsWebhookIDHeaderName, headers.id}, {core.LoopsWebhookTimestampHeaderName, headers.timestamp}, {core.LoopsWebhookSignatureHeaderName, headers.signature}} {
		for _, value := range field.values {
			request.Header.Add(field.name, value)
		}
	}
	call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
	if err != nil {
		t.Fatalf("exchange.NewSocketServerCall() error = %v, want nil", err)
	}
	parent, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	return receiver.Receive(WebhookReceiveRequest{ScratchParent: parent, Call: call, Destination: destination, ObservedAt: observedAt})
}

func requireLoopsScratchEmpty(t testing.TB, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("scratch entries = %d, error = %v, want empty", len(entries), err)
	}
}

func TestWebhookIngressAttacksTheStandardWebhooksBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	signedAt := loopsSeconds(t, 1614265330)
	for _, testCase := range []struct {
		wantErr error
		name    string
		mutate  func(*loopsWebhookHeaders)
		body    string
	}{
		{name: "positive official vector releases exact raw JSON", body: officialWebhookBody},
		{name: "negative one-byte body mutation releases nothing", body: `{"test": 2432232315}`, wantErr: core.ErrLoopsVerification},
		{name: "negative absent signature releases nothing", body: officialWebhookBody, mutate: func(h *loopsWebhookHeaders) { h.signature = nil }, wantErr: core.ErrLoopsAuthentication},
		{name: "neutral empty body under the vector signature releases nothing", mutate: func(*loopsWebhookHeaders) {}, wantErr: core.ErrLoopsVerification},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			receiver := loopsWebhookReceiver(t, officialWebhookSecret)
			headers := officialHeaders()
			if testCase.mutate != nil {
				testCase.mutate(&headers)
			}
			directory := t.TempDir()
			var destination bytes.Buffer
			observation, gotErr := loopsWebhookReceive(t, receiver, headers, bytes.NewReader([]byte(testCase.body)), &destination, signedAt, directory)
			requireLoopsScratchEmpty(t, directory)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("Receive() error = %v, want %v", gotErr, testCase.wantErr)
			}
			if testCase.wantErr != nil {
				if observation.Validate() == nil || destination.Len() != 0 {
					t.Fatalf("refused webhook = observation:%+v bytes:%d, want zero evidence", observation, destination.Len())
				}
				return
			}
			if observation.Validate() != nil || observation.DeliveryID.String() != officialWebhookID || observation.SignedAt != signedAt ||
				observation.ObservedAt != signedAt || observation.Bytes.Uint64() != uint64(len(officialWebhookBody)) || destination.String() != officialWebhookBody {
				t.Fatalf("accepted webhook = observation:%+v body:%q, want exact delivery facts and body", observation, destination.String())
			}
		})
	}
}
