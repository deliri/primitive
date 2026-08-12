package exchange_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/exchange"
)

func mustHeaderValue(t testing.TB, value string) exchange.HeaderValue {
	t.Helper()

	parsed, err := exchange.NewHeaderValue(value)
	if err != nil {
		t.Fatalf("exchange.NewHeaderValue(%d bytes) setup error = %v, want nil", len(value), err)
	}
	return parsed
}
