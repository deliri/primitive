package payment_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/payment"
)

func BenchmarkParseSigningDomain(b *testing.B) {
	const value = payment.SigningDomainReceiptV1Token
	var wantErr error
	b.ReportAllocs()
	var last payment.SigningDomain
	for b.Loop() {
		got, err := payment.SigningDomainUnknown.ParseCanonicalText([]byte(value))
		if !errors.Is(err, wantErr) {
			b.Fatalf("payment.ParseCanonicalText() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("payment.ParseCanonicalText() = %q, want %q", last, value)
	}
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
