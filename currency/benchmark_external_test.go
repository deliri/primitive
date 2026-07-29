package currency_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/currency"
)

func BenchmarkParseDecimal(b *testing.B) {
	b.ReportAllocs()

	var last currency.Amount
	for b.Loop() {
		value, err := currency.Parse(currency.CodeCLF, "-922337203685477.5808")
		if err != nil {
			b.Fatalf("currency.Parse() error = %v, want nil", err)
		}
		last = value
	}
	_ = last
}

func BenchmarkFormatDecimal(b *testing.B) {
	value, err := currency.New(currency.CodeCLF, -9223372036854775807-1)
	if err != nil {
		b.Fatalf("currency.New() error = %v, want nil", err)
	}
	b.ReportAllocs()

	var last string
	for b.Loop() {
		decimal, gotErr := value.Decimal()
		if gotErr != nil {
			b.Fatalf("Amount.Decimal() error = %v, want nil", gotErr)
		}
		last = decimal
	}
	_ = last
}
