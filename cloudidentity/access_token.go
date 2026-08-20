package cloudidentity

import (
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// AccessToken is one opaque provider-acquired OAuth access bearer with its
// positive provider-declared lifetime. It is intentionally distinct from an
// audience-bound identity Token so the compiler cannot interchange them.
type AccessToken struct {
	value    *string
	lifetime temporal.Duration
	provider Provider
}

func newGoogleAccessToken(value string, lifetime temporal.Duration) (AccessToken, error) {
	token := AccessToken{value: &value, lifetime: lifetime, provider: ProviderGoogleCloud}
	if err := token.Validate(); err != nil {
		return AccessToken{}, err
	}
	return token, nil
}

// Provider returns the authority that issued the access token.
func (t AccessToken) Provider() Provider { return t.provider }

// Lifetime returns the positive provider-declared remaining lifetime observed
// at acquisition. Cloudidentity does not turn it into a cache or refresh policy.
func (t AccessToken) Lifetime() temporal.Duration { return t.lifetime }

// Validate checks provenance, positive bounded lifetime, extent, and RFC 6750
// token68 syntax without parsing provider-specific claims.
func (t AccessToken) Validate() error {
	if t.provider != ProviderGoogleCloud {
		return core.ErrCloudIdentityContract
	}
	if t.value == nil || t.lifetime.IsZero() {
		return core.ErrCloudIdentityContract
	}
	if err := t.lifetime.Validate(); err != nil {
		return contractError(err)
	}
	return validateBearerTokenValue(*t.value)
}

// BearerValue explicitly crosses the authorization-header disclosure boundary.
func (t AccessToken) BearerValue() (string, error) {
	if err := t.Validate(); err != nil {
		return "", err
	}
	return bearerPrefix + *t.value, nil
}

// Format redacts the access token for every formatting verb.
func (AccessToken) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

var (
	_ core.Validatable = AccessToken{}
	_ fmt.Formatter    = AccessToken{}
)
