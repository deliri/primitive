package webmcp_test

import (
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/webmcp"
)

// This proves the Go publication boundary, not native browser execution. The
// separate browser suite invokes the exact admitted bytes in real ModelContext.
func TestBrowserModulePublishesExactImmutableSource(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("browser_module.js")
	if err != nil {
		t.Fatalf("module source read error = %v, want nil", err)
	}
	got := webmcp.BrowserModule()
	if len(body) == 0 || got != string(body) {
		t.Fatalf("published bytes/equal = %d/%t, want %d/true", len(got), got == string(body), len(body))
	}
	if second := webmcp.BrowserModule(); second != got {
		t.Fatalf("second publication equals first = %t, want true", second == got)
	}
}
