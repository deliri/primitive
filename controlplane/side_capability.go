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

// ServerConfiguration names the authority keys a control service trusts when
// it authenticates credentials and requests presented by installed tools.
type ServerConfiguration struct {
	TrustedAuthorityKeys attest.TrustedKeys
}

// Server is the authority half of the shared control-plane agreement. It owns
// no account, persistence, product, or transport policy.
type Server struct {
	configuration ServerConfiguration
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
func (c ServerConfiguration) Validate() error {
	if err := c.TrustedAuthorityKeys.Validate(); err != nil {
		return capabilityError(err)
	}
	return nil
}

// NewServer constructs one immutable authority capability.
func NewServer(configuration ServerConfiguration) (Server, error) {
	if err := configuration.Validate(); err != nil {
		return Server{}, err // witness:waiver doctrine/http/server_timeouts -- Controlplane Server is a typed authority capability, not net/http.Server, and owns no HTTP runtime.
	}
	server := Server{configuration: configuration} // witness:waiver doctrine/http/server_timeouts -- Controlplane Server is a typed authority capability, not net/http.Server, and owns no HTTP runtime.
	return server, server.Validate()
}

// Validate proves the server still owns a complete trust configuration.
func (s Server) Validate() error {
	return s.configuration.Validate()
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
	_ core.Validatable = ServerConfiguration{}
	_ core.Validatable = Server{} // witness:waiver doctrine/http/server_timeouts -- Controlplane Server is a typed authority capability, not net/http.Server, and owns no HTTP runtime.
)
