package controlplane

import (
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

// ClientConfiguration names the authority keys an installed tool trusts when
// it authenticates control-plane documents.
type ClientConfiguration struct {
	TrustedAuthorityKeys attest.TrustedKeys
}

// Client is the installed-tool half of the shared control-plane agreement.
// Its configuration is private so callers cannot manufacture a configured
// capability or replace its trust roots after construction.
type Client struct {
	configuration ClientConfiguration
}

// AuthorityConfiguration names the authority keys a control service trusts when
// it authenticates credentials and requests presented by installed tools.
type AuthorityConfiguration struct {
	TrustedAuthorityKeys attest.TrustedKeys
}

// Authority is the authority half of the shared control-plane agreement. It owns
// no account, persistence, product, or transport policy.
type Authority struct {
	configuration AuthorityConfiguration
}

// Validate closes the installed tool's trust configuration.
func (c ClientConfiguration) Validate() error {
	if err := c.TrustedAuthorityKeys.Validate(); err != nil {
		return capabilityError(err)
	}
	return nil
}

// NewClient constructs one immutable installed-tool capability.
func NewClient(configuration ClientConfiguration) (Client, error) {
	if err := configuration.Validate(); err != nil {
		return Client{}, err
	}
	client := Client{configuration: configuration}
	return client, client.Validate()
}

// Validate proves the client still owns a complete trust configuration.
func (c Client) Validate() error {
	return c.configuration.Validate()
}

// Validate closes the authority's trust configuration.
func (c AuthorityConfiguration) Validate() error {
	if err := c.TrustedAuthorityKeys.Validate(); err != nil {
		return capabilityError(err)
	}
	return nil
}

// NewAuthority constructs one immutable authority capability.
func NewAuthority(configuration AuthorityConfiguration) (Authority, error) {
	if err := configuration.Validate(); err != nil {
		return Authority{}, err
	}
	authority := Authority{configuration: configuration}
	return authority, authority.Validate()
}

// Validate proves the authority still owns a complete trust configuration.
func (a Authority) Validate() error {
	return a.configuration.Validate()
}

func capabilityError(causes ...error) error {
	joined := make([]error, 0, len(causes)+1)
	joined = append(joined, core.ErrControlPlaneContract)
	for _, cause := range causes {
		if cause != nil {
			joined = append(joined, cause)
		}
	}
	return errors.Join(joined...)
}

var (
	_ core.Validatable = ClientConfiguration{}
	_ core.Validatable = Client{}
	_ core.Validatable = AuthorityConfiguration{}
	_ core.Validatable = Authority{}
)
