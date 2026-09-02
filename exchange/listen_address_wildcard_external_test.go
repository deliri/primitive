package exchange_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestParseListenAddressRefusesEmptyHostWildcard(t *testing.T) {
	t.Parallel()

	got, gotErr := exchange.ParseListenAddress(":8080")
	if !errors.Is(gotErr, core.ErrExchangeContract) || got != (exchange.ListenAddress{}) {
		t.Fatalf("exchange.ParseListenAddress(empty host) = (%v, %v), want zero and %v", got, gotErr, core.ErrExchangeContract)
	}
}
