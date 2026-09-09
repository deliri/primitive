// Package tailnet owns an outbound userspace Tailscale connection. Product
// policy chooses one destination; the capability cannot dial other hosts.
package tailnet

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/tailnetconfig"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/temporal"
	_ "tailscale.com/feature/identityfederation"
	"tailscale.com/tsnet"
)

var (
	ErrClosed      = errors.New("tailnet closed")
	ErrEnrollment  = errors.New("tailnet enrollment")
	ErrDestination = errors.New("tailnet destination")
)

// IdentitySource acquires a fresh token only when the node needs enrollment.
type IdentitySource interface {
	Identity(context.Context, googleidentity.Audience) (googleidentity.Token, error)
}

type GoogleIdentity struct{ Client googleidentity.Client }

func (s GoogleIdentity) Identity(ctx context.Context, audience googleidentity.Audience) (googleidentity.Token, error) {
	policy, err := googleidentity.DefaultPolicy()
	if err != nil {
		return googleidentity.Token{}, err
	}
	return googleidentity.AcquireGoogleCloud(ctx, s.Client, googleidentity.IdentityTokenRequest{Audience: audience, Policy: policy})
}

// Client owns SDK state, connections, enrollment, and the shared HTTP transport.
// Creation is effect-free; enrollment is bounded by the first request's context.
type Client struct {
	configuration tailnetconfig.Configuration
	identity      IdentitySource
	gate          chan struct{}
	server        *tsnet.Server
	transport     *http.Transport
	exchange      exchange.Client
	closed        bool
	closeOnce     sync.Once
	closeErr      error
}

func NewClient(configuration tailnetconfig.Configuration, identity IdentitySource) (*Client, error) {
	if err := configuration.Validate(); err != nil {
		return nil, err
	}
	if identity == nil {
		return nil, tailnetconfig.ErrContract
	}
	result := &Client{configuration: configuration, identity: identity, gate: make(chan struct{}, 1)}
	result.transport = &http.Transport{DialContext: result.dial, MaxConnsPerHost: 2, MaxIdleConns: 2, MaxIdleConnsPerHost: 2, IdleConnTimeout: 0, MaxResponseHeaderBytes: 64 * 1024}
	client, err := exchange.NewClient(&http.Client{Transport: result.transport})
	if err != nil {
		return nil, errors.Join(tailnetconfig.ErrContract, err)
	}
	result.exchange = client
	return result, nil
}

func (c *Client) Exchange() (exchange.Client, error) {
	if c == nil {
		return exchange.Client{}, tailnetconfig.ErrContract
	}
	return c.exchange, c.exchange.Validate()
}

func (c *Client) dial(ctx context.Context, network, address string) (net.Conn, error) {
	if c == nil || ctx == nil {
		return nil, tailnetconfig.ErrContract
	}
	if network != "tcp" || address != c.configuration.Destination.String() {
		return nil, ErrDestination
	}
	server, err := c.acquire(ctx)
	if err != nil {
		return nil, err
	}
	return server.Dial(ctx, network, address)
}

func (c *Client) acquire(ctx context.Context) (*tsnet.Server, error) {
	select {
	case c.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-c.gate }()
	if c.closed {
		return nil, ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.server != nil {
		return c.server, nil
	}
	server, err := c.enroll(ctx)
	if err != nil {
		return nil, err
	}
	c.server = server
	return server, nil
}

func (c *Client) enroll(ctx context.Context) (*tsnet.Server, error) {
	owned, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: c.configuration.StartupTimeout})
	if err != nil {
		return nil, err
	}
	defer cancel()
	token, err := c.identity.Identity(owned, c.configuration.Audience)
	if err != nil {
		return nil, errors.Join(ErrEnrollment, err)
	}
	bearer, err := token.BearerValue()
	if err != nil {
		return nil, errors.Join(ErrEnrollment, err)
	}
	server := &tsnet.Server{Dir: c.configuration.StateDirectory.String(), Hostname: string(c.configuration.Hostname), Ephemeral: true, ClientID: string(c.configuration.ClientID), IDToken: strings.TrimPrefix(bearer, exchange.BearerAuthorizationScheme+" "), AdvertiseTags: []string{string(c.configuration.Tag)}}
	if _, err := server.Up(owned); err != nil {
		return nil, errors.Join(ErrEnrollment, err, server.Close())
	}
	return server, nil
}

func (c *Client) Close() error {
	if c == nil {
		return tailnetconfig.ErrContract
	}
	c.closeOnce.Do(func() {
		c.gate <- struct{}{}
		defer func() { <-c.gate }()
		c.closed = true
		c.transport.CloseIdleConnections()
		if c.server != nil {
			c.closeErr = c.server.Close()
		}
	})
	return c.closeErr
}

var _ IdentitySource = GoogleIdentity{}
