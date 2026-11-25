package objectstore

import (
	"errors"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const (
	downloadRedirectLocationHeaderText     = "Location"
	downloadRedirectCacheControlHeaderText = "Cache-Control"
	downloadRedirectCacheControlNoStore    = "no-store"
	downloadRedirectSignedHeaderHost       = "host"
	downloadRedirectHeaderDiagnostic       = "download capability with signed headers cannot be spent by a redirect"
)

// WriteRedirect discloses this issued bearer only as the Location of one
// body-absent, no-store 303 See Other response, so a browser retrieves the
// exact object directly from the provider. The bearer never crosses to the
// caller as text. A bearer whose signature covers any request field other than
// the browser-owned Host is refused because a redirect cannot carry it.
func (p DownloadCapabilityProjection) WriteRedirect(call exchange.SocketServerCall) error {
	if err := p.Validate(); err != nil {
		return err
	}
	signed, err := providerSignedHeaderDeclaration(p.provider, p.target.URL.value.Query())
	if err != nil {
		return err
	}
	if len(p.target.Headers.values) != 0 || !strings.EqualFold(signed, downloadRedirectSignedHeaderHost) {
		return errors.Join(core.ErrObjectStoreContract, errors.New(downloadRedirectHeaderDiagnostic))
	}
	location, locationErr := core.ParseHTTPHeaderName(downloadRedirectLocationHeaderText)
	cacheControl, cacheControlErr := core.ParseHTTPHeaderName(downloadRedirectCacheControlHeaderText)
	target, targetErr := exchange.NewHeaderValue(p.target.URL.value.String())
	noStore, noStoreErr := exchange.NewHeaderValue(downloadRedirectCacheControlNoStore)
	if err := errors.Join(locationErr, cacheControlErr, targetErr, noStoreErr); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return exchange.WriteNoBody(exchange.NoBodyWriteCall{Call: call, Response: exchange.ServerNoBodyResponse{
		Status: core.HTTPStatusSeeOther(),
		Headers: exchange.ResponseHeaders{Values: []exchange.Header{
			{Name: location, Values: []exchange.HeaderValue{target}},
			{Name: cacheControl, Values: []exchange.HeaderValue{noStore}},
		}},
	}})
}
