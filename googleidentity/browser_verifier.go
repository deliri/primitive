package googleidentity

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// BrowserVerifierConfiguration fixes the relying-party audience and caller's
// Exchange capability. Products construct this once and share it across logins.
type BrowserVerifierConfiguration struct {
	Client   exchange.Client
	ClientID core.OAuthClientID
}

func (c BrowserVerifierConfiguration) Validate() error {
	if err := errors.Join(c.Client.Validate(), c.ClientID.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// BrowserVerifier shares only the official SDK validator and its native JWKS
// cache. Google owns certificate freshness; Primitive adds no cache or worker.
// Browser claim policy remains distinct from service-account Verify policy.
type BrowserVerifier struct{ native GoogleCloudVerifier }

func NewBrowserVerifier(ctx context.Context, c BrowserVerifierConfiguration) (BrowserVerifier, error) {
	if err := c.Validate(); err != nil {
		return BrowserVerifier{}, err
	}
	audience, err := ParseAudience(c.ClientID.String())
	if err != nil {
		return BrowserVerifier{}, err
	}
	native, err := NewGoogleCloudVerifier(ctx, GoogleCloudVerifierConfiguration{Client: c.Client, Audience: audience})
	if err != nil {
		return BrowserVerifier{}, err
	}
	return BrowserVerifier{native: native}, nil
}
func (v BrowserVerifier) Validate() error                        { return v.native.Validate() }
func (BrowserVerifierConfiguration) googleIdentityProtocolFact() {}
func (BrowserVerifier) googleIdentityCapabilityWrapper()         {}
