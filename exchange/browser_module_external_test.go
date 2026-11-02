package exchange_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestBrowserModuleEmitsCoreIdentities(t *testing.T) {
	t.Parallel()
	module := exchange.BrowserModule()
	for _, row := range []struct {
		name     string
		identity core.ErrorIdentity
	}{
		{name: "contract", identity: core.ErrExchangeContract},
		{name: "response", identity: core.ErrExchangeResponse},
		{name: "contentType", identity: core.ErrExchangeContentType},
		{name: "cancelled", identity: core.ErrExchangeCancelled},
		{name: "transport", identity: core.ErrExchangeTransport},
		{name: "redirect", identity: core.ErrExchangeRedirect},
	} {
		if fragment := row.name + ": " + strconv.Itoa(int(row.identity)) + ","; !strings.Contains(module, fragment) {
			t.Fatalf("BrowserModule() lacks %q", fragment)
		}
	}
	for _, required := range []string{"redirect: 'manual'", "credentials: 'same-origin'", "cache: 'no-store'", "export async function sendJSON"} {
		if !strings.Contains(module, required) {
			t.Fatalf("BrowserModule() lacks %q", required)
		}
	}
	if strings.Contains(module, "__") {
		t.Fatal("BrowserModule() has unreplaced placeholders")
	}
}
