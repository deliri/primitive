package paypal

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestWebhookPostbackPreservesOriginalJSONThroughTLS(t *testing.T) {
	t.Parallel()
	bodies := [...]string{
		`{"id":"WH-123","href":"https://example.test/a"}`,
		`{"id":"WH-123","href":"https:\/\/example.test\/a"}`,
		"{\n  \"id\": \"WH-123\",\n  \"amount\": 9007199254740993\n}",
		`{"id":"WH-123","escape":"\u0061\/\u2028"}`,
		`{"id":"WH-123","nested": { "items": [ 1, 2, 3 ] }}`,
		`{"id":"WH-123","amount":1.0000000000000000000000001}`,
		"{\t\"id\"\t:\t\"WH-123\"\t}",
		`{"id":"WH-123","text":"東京 ☃"}`,
	}
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			body := []byte(strings.Repeat(" \n", index/8) + bodies[index%8] + strings.Repeat("\t ", index/8))
			var calls atomic.Uint64
			provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				posted, err := io.ReadAll(r.Body)
				var projection payPalWebhookVerificationProjection
				decodeErr := json.Unmarshal(posted, &projection)
				if err != nil || decodeErr != nil || projection.Validate() != nil || !bytes.Equal(bytes.TrimSpace(projection.WebhookEvent), bytes.TrimSpace(body)) || !bytes.Contains(posted, append(append([]byte(core.PayPalWebhookEventMember), body...), '}')) {
					t.Errorf("postback changed signed bytes: %q: %v/%v", posted, err, decodeErr)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"verification_status":"SUCCESS","provider_extension":{"version":2}}`)
			}))
			defer provider.Close()
			transport := provider.Client().Transport.(*http.Transport).Clone()
			transport.TLSClientConfig.ServerName = "example.com"
			transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
				return new(net.Dialer).DialContext(ctx, network, provider.Listener.Addr().String())
			}
			defer transport.CloseIdleConnections()
			socket, err := exchange.NewClient(&http.Client{Transport: transport})
			if err != nil {
				t.Fatal(err)
			}
			token, err := ParseAccessToken([]byte("paypal-access-token"))
			if err != nil {
				t.Fatal(err)
			}
			client, err := NewClient(socket, token, false)
			if err != nil {
				t.Fatal(err)
			}
			receiver, err := NewPayPalWebhookReceiver(client, PayPalWebhookID("WEBHOOK123"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := errors.Join(receiver.Close(), client.Close(), token.Close()); err != nil {
					t.Error(err)
				}
			}()
			request := payPalIncomingWebhookRequest(t, body, true)
			call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
			if err != nil {
				t.Fatal(err)
			}
			observed, err := temporal.ParseRFC3339("2026-09-02T12:00:00Z")
			if err != nil {
				t.Fatal(err)
			}
			tolerance, err := temporal.DurationFromMinutes(5)
			if err != nil {
				t.Fatal(err)
			}
			var destination bytes.Buffer
			result, err := receiver.Receive(PayPalWebhookReceiveRequest{Call: call, Destination: &destination, ObservedAt: observed, Tolerance: tolerance, Policy: payPalOAuthTestPolicy(t)})
			if err != nil || calls.Load() != 1 || result.Bytes.Uint64() != uint64(len(body)) || !bytes.Equal(destination.Bytes(), body) {
				t.Fatalf("raw custody: calls=%d err=%v body=%q", calls.Load(), err, destination.Bytes())
			}
		})
	}
}
