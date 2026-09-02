package plunk

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestWebhookIngressAttacksThePlunkBearerBoundary(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		wantErr error
		name    string
		bearer  string
	}{
		{name: "configured bearer releases exact body", bearer: "plunk-shared-secret"},
		{name: "foreign bearer releases nothing", bearer: "foreign-secret", wantErr: core.ErrPlunkVerification},
		{name: "missing bearer releases nothing", wantErr: core.ErrPlunkAuthentication},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			receiver, secret := plunkWebhookTestReceiver(t)
			defer func() { _ = errors.Join(receiver.Close(), secret.Close()) }()
			body := []byte(`{"event":"delivered"}`)
			request := httptest.NewRequest(http.MethodPost, "https://example.test/plunk", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			if testCase.bearer != "" {
				request.Header.Set("Authorization", "Bearer "+testCase.bearer)
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
			if observation.Validate() != nil || observation.Bytes.Uint64() != uint64(len(body)) || !bytes.Equal(destination.Bytes(), body) {
				t.Fatalf("accepted webhook = observation:%+v body:%q, want exact body", observation, destination.Bytes())
			}
		})
	}
}

func plunkWebhookTestReceiver(t testing.TB) (WebhookReceiver, WebhookSecret) {
	t.Helper()
	secret, err := ParseWebhookSecret([]byte("plunk-shared-secret"))
	if err != nil {
		t.Fatalf("ParseWebhookSecret() error = %v, want nil", err)
	}
	maximum, err := core.NewByteCount(core.PlunkWebhookCustodyMaximumBytes)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	receiver, err := NewWebhookReceiver(secret, maximum)
	if err != nil {
		t.Fatalf("NewWebhookReceiver() error = %v, want nil", err)
	}
	return receiver, secret
}
