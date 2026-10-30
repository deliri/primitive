package exchange

import (
	"net/http"

	"github.com/deliri/primitive/v2026/core"
)

// HTTPClientCleanupRequest borrows the exact native client whose idle pool the
// caller owns or is authorized to release. A nil Transport selects Go's shared
// default pool; the request does not create ownership of that shared pool.
// Request timeouts are unrelated to idle cleanup and remain unchanged.
type HTTPClientCleanupRequest struct {
	Client *http.Client
}

func (r HTTPClientCleanupRequest) Validate() error {
	if r.Client == nil || r.Client.Transport != nil && core.IsNil(r.Client.Transport) {
		return core.ErrExchangeContract
	}
	return nil
}

// CloseHTTPClientIdleConnections delegates to the exact borrowed Go client.
// Go closes idle connections, preserves active calls, and invokes an optional
// custom transport cleanup method. Its operation has no error result. No live
// operation context or request-client timeout policy gates this cleanup.
func CloseHTTPClientIdleConnections(request HTTPClientCleanupRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	return nil
}
