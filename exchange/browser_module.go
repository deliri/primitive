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
// native fetch owns the request. sendJSON admits same-origin JSON. The explicit
// createBrowserExchange capability binds a trusted API origin and optional
// application-owned CSRF header, with native CORS and cookies authoritative.
// Both paths refuse redirects/caching and carry the caller's AbortSignal.
// Other Primitive browser modules perform HTTP only through this executor.
func BrowserModule() string {
	return strings.NewReplacer(
		"__IDEMPOTENCY_HEADER__", core.HTTPHeaderIdempotencyKey().String(),
		"__CONTRACT_IDENTITY__", strconv.Itoa(int(core.ErrExchangeContract)),
		"__RESPONSE_IDENTITY__", strconv.Itoa(int(core.ErrExchangeResponse)),
		"__CONTENT_TYPE_IDENTITY__", strconv.Itoa(int(core.ErrExchangeContentType)),
		"__CANCELLED_IDENTITY__", strconv.Itoa(int(core.ErrExchangeCancelled)),
		"__TRANSPORT_IDENTITY__", strconv.Itoa(int(core.ErrExchangeTransport)),
		"__REDIRECT_IDENTITY__", strconv.Itoa(int(core.ErrExchangeRedirect)),
	).Replace(browserModuleSource)
}
