package exchange

import "net/http"

// OfficialSDKResponseTransport applies a provider-owned response boundary to
// this client's transport. The SDK owns its request construction; Exchange
// retains transport ownership, response limits, and redirect refusal at the
// SDK client boundary. The admitted client is never mutated.
func (c Client) OfficialSDKResponseTransport(boundary OfficialSDKResponseBoundary) (http.RoundTripper, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	base := c.http.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	return NewOfficialSDKResponseTransport(OfficialSDKResponseTransportRequest{Base: base, Boundary: boundary})
}
