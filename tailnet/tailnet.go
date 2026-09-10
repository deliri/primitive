// Package tailnet owns an outbound userspace Tailscale connection. Product
// policy chooses one destination; the capability cannot dial other hosts.
package tailnet

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/tailnetconfig"
	"net"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/temporal"
	"tailscale.com/ipn"
	"tailscale.com/tsnet"
)

const (
	maximumDestinationConnections = 2
	responseHeaderMaximumBytes    = 64 << 10
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
	enrollment    exchange.Client
	closed        atomic.Bool
	closeOnce     sync.Once
	closeErr      error
}

func NewClient(configuration tailnetconfig.Configuration, identity IdentitySource) (*Client, error) {
	if err := configuration.Validate(); err != nil {
		return nil, err
	}
	if nilIdentitySource(identity) {
		return nil, core.ErrTailnetContract
	}
	result := &Client{configuration: configuration, identity: identity, gate: make(chan struct{}, 1)}
	timeout, err := configuration.StartupTimeout.Stdlib()
	if err != nil {
		return nil, errors.Join(core.ErrTailnetContract, err)
	}
	result.transport = &http.Transport{TLSHandshakeTimeout: timeout, ResponseHeaderTimeout: timeout, ExpectContinueTimeout: timeout, DialContext: result.dial, MaxConnsPerHost: maximumDestinationConnections, MaxIdleConns: maximumDestinationConnections, MaxIdleConnsPerHost: maximumDestinationConnections, IdleConnTimeout: 0, MaxResponseHeaderBytes: responseHeaderMaximumBytes}
	client, err := exchange.NewClient(&http.Client{Transport: result.transport})
	if err != nil {
		return nil, errors.Join(core.ErrTailnetContract, err)
	}
	result.exchange = client
	result.enrollment, err = exchange.NewStandardClient()
	if err != nil {
		return nil, errors.Join(core.ErrTailnetContract, err)
	}
	return result, nil
}

func (c *Client) Exchange() (exchange.Client, error) {
	if c == nil || c.gate == nil || c.transport == nil {
		return exchange.Client{}, core.ErrTailnetContract
	}
	if c.closed.Load() {
		return exchange.Client{}, core.ErrTailnetClosed
	}
	return c.exchange, c.exchange.Validate()
}

func (c *Client) dial(ctx context.Context, network, address string) (net.Conn, error) {
	if c == nil || c.gate == nil || c.transport == nil || ctx == nil {
		return nil, core.ErrTailnetContract
	}
	if network != "tcp" || address != c.configuration.Destination.String() {
		return nil, core.ErrTailnetDestination
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
	if c.closed.Load() {
		return nil, core.ErrTailnetClosed
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
		return nil, errors.Join(core.ErrTailnetEnrollment, err)
	}
	authKey, err := acquireAuthKey(owned, c.enrollment, c.configuration, token)
	if err != nil {
		return nil, err
	}
	server := newServer(c.configuration, authKey)
	// Start owns rollback of partial initialization. Close requires initialized
	// SDK state and may panic after an early filesystem failure in v1.102.3.
	if err := server.Start(); err != nil {
		return nil, errors.Join(core.ErrTailnetEnrollment, err)
	}
	if _, err := server.Up(owned); err != nil {
		return nil, errors.Join(core.ErrTailnetEnrollment, err, server.Close())
	}
	return server, nil
}

func (c *Client) Close() error {
	if c == nil || c.gate == nil || c.transport == nil {
		return core.ErrTailnetContract
	}
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		c.gate <- struct{}{}
		defer func() { <-c.gate }()
		c.transport.CloseIdleConnections()
		if c.server != nil {
			c.closeErr = c.server.Close()
		}
	})
	return c.closeErr
}

var _ IdentitySource = GoogleIdentity{}

func nilIdentitySource(identity IdentitySource) bool {
	if identity == nil {
		return true
	}
	value := reflect.ValueOf(identity)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// newServer projects the validated agreement directly into the provider SDK.
func newServer(configuration tailnetconfig.Configuration, authKey string) *tsnet.Server {
	// witness:waiver doctrine/http/server_timeouts -- This literal is tailscale.com/tsnet.Server, not net/http.Server; the provider type has no HTTP server timeout fields.
	return &tsnet.Server{
		Dir: configuration.StateDirectory.String(), Hostname: configuration.Hostname.String(),
		Ephemeral: true, AuthKey: authKey, ControlURL: ipn.DefaultControlURL,
		AdvertiseTags: []string{configuration.Tag.String()},
	}
}
