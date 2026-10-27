package exchange_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestCloseIdleConnectionsTypedNilNativeTransportRefuses(t *testing.T) {
	t.Parallel()
	client, err := exchange.NewClient(&http.Client{Transport: (*http.Transport)(nil)})
	if err != nil {
		t.Fatalf("native client admission = %v, want existing admitted client shape", err)
	}
	if err := client.CloseIdleConnections(); !errors.Is(err, core.ErrExchangeContract) {
		t.Fatalf("typed nil native cleanup = %v, want Exchange contract", err)
	}
}
