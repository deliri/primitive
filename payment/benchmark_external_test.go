package payment_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/payment"
)

func BenchmarkParseSigningDomainBatch(b *testing.B) {
	const parsesPerBatch = 16
	b.ReportAllocs()
	for b.Loop() {
		for range parsesPerBatch {
			got, err := payment.SigningDomainUnknown.ParseCanonicalText([]byte(payment.SigningDomainReceiptV1Token))
			if err != nil || got != payment.SigningDomainReceiptV1 {
				b.Fatalf("parse = (%v,%v), want receipt domain", got, err)
			}
		}
	}
	b.ReportMetric(parsesPerBatch, "parses/op")
}

func BenchmarkParsePaymentID(b *testing.B) {
	const value = "01234567-89ab-7cde-8f01-23456789abcd"
	var wantErr error
	b.ReportAllocs()
	var last payment.PaymentID
	for b.Loop() {
		got, err := payment.ParsePaymentID(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("payment.ParsePaymentID() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("payment.ParsePaymentID() = %q, want %q", last, value)
	}
}
