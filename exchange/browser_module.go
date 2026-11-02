package exchange

import (
	_ "embed"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

//go:embed browser_module.js
var browserModuleSource string

// BrowserModule emits Primitive's browser HTTP executor. In the browser,
// native fetch owns the request; the executor admits only same-origin JSON,
// refuses redirects and caching, and carries the caller's AbortSignal. Other
// Primitive browser modules perform HTTP only through this executor.
func BrowserModule() string {
	return strings.NewReplacer(
		"__CONTRACT_IDENTITY__", strconv.Itoa(int(core.ErrExchangeContract)),
		"__RESPONSE_IDENTITY__", strconv.Itoa(int(core.ErrExchangeResponse)),
		"__CONTENT_TYPE_IDENTITY__", strconv.Itoa(int(core.ErrExchangeContentType)),
		"__CANCELLED_IDENTITY__", strconv.Itoa(int(core.ErrExchangeCancelled)),
		"__TRANSPORT_IDENTITY__", strconv.Itoa(int(core.ErrExchangeTransport)),
	).Replace(browserModuleSource)
}
