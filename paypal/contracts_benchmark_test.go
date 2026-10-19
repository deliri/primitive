package paypal

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func BenchmarkParseAccessToken(b *testing.B) {
	value := []byte("A21AA-test-token")
	b.ReportAllocs()
	parsed := 0
	for b.Loop() {
		token, err := ParseAccessToken(value)
		if err != nil || token.Validate() != nil {
			b.Fatalf("ParseAccessToken() = %v, want a valid token", err)
		}
		if err := token.Close(); err != nil {
			b.Fatal(err)
		}
		parsed++
	}
	if parsed == 0 {
		b.Fatal("ParseAccessToken never ran")
	}
}

// BenchmarkEncodeWebhookVerificationProjection measures the local request
// body PayPal's verify endpoint receives: typed headers plus the original
// event bytes, proven through Core's strict projection encoder.
func BenchmarkEncodeWebhookVerificationProjection(b *testing.B) {
	var event strings.Builder
	event.WriteString(`{"id":"WH-0001","event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{"items":[`)
	for index := range 32 {
		if index > 0 {
			event.WriteByte(',')
		}
		fmt.Fprintf(&event, `{"sku":"item-%04d","quantity":%d,"amount":{"currency_code":"CAD","value":"%d.00"}}`, index, index+1, 10+index)
	}
	event.WriteString(`]}}`)
	webhookID, err := ParsePayPalWebhookID("WHCONFIGURED0001")
	if err != nil {
		b.Fatal(err)
	}
	projection := payPalWebhookVerificationProjection{
		AuthAlgorithm:  "SHA256withRSA",
		CertificateURL: "https://api.paypal.com/v1/notifications/certs/CERT123",
		TransmissionID: "transmission-123",
		Signature:      "c2lnbmF0dXJlLTEyMw==",
		TransmissionAt: "2026-09-02T12:00:00Z",
		WebhookID:      webhookID,
		WebhookEvent:   []byte(event.String()),
	}
	if err := projection.Validate(); err != nil || len(projection.WebhookEvent) < 2048 {
		b.Fatalf("projection fixture = %d event bytes, %v; want a valid multi-kilobyte event", len(projection.WebhookEvent), err)
	}
	limits := core.DefaultStrictJSONLimits()
	b.SetBytes(int64(len(projection.WebhookEvent)))
	b.ReportAllocs()
	var got []byte
	for b.Loop() {
		got, err = core.EncodeValidatedJSON(projection, limits)
		if err != nil {
			b.Fatal(err)
		}
	}
	if !bytes.HasSuffix(got, append([]byte(core.PayPalWebhookEventMember), append(projection.WebhookEvent, '}')...)) {
		b.Fatalf("projection body = %q, want the original event bytes as its final member", got)
	}
}
