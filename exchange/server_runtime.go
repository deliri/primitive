package exchange

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"sync/atomic"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// ListenAddress is one exact TCP address owned by a server listener.
type ListenAddress struct{ value netip.AddrPort }

// ParseListenAddress admits one concrete address with a nonzero port.
func ParseListenAddress(value string) (ListenAddress, error) {
	parsed, err := netip.ParseAddrPort(value)
	if err != nil {
		return ListenAddress{}, errors.Join(core.ErrExchangeContract, err)
	}
	address := ListenAddress{value: parsed}
	if err := address.Validate(); err != nil {
		return ListenAddress{}, err
	}
	return address, nil
}

// Validate rejects zero, portless, and unspecified listener addresses. An
// unspecified host binds every local address and must never be inferred from
// an omitted host.
func (a ListenAddress) Validate() error {
	if !a.value.IsValid() || a.value.Addr().IsUnspecified() || a.value.Port() == 0 {
		return core.ErrExchangeContract
	}
	return nil
}

// ServerListener is one already-open TCP listener. It keeps the standard
// listener private to Exchange while allowing a product to prove port
// acquisition before it constructs the rest of its boot graph.
type ServerListener struct {
	listener net.Listener
	address  ListenAddress
	claimed  atomic.Bool
}

// Listen opens one exact TCP listener. The caller owns the returned capability
// until it transfers the capability to ServerRuntime or closes it.
func Listen(address ListenAddress) (*ServerListener, error) {
	if err := address.Validate(); err != nil {
		return nil, err
	}
	// witness:waiver doctrine/code_form/defer_after_acquire -- the returned ServerListener transfers exact listener ownership to its caller.
	listener, err := net.Listen("tcp", address.String())
	if err != nil {
		return nil, transportError(err)
	}
	return &ServerListener{address: address, listener: listener}, nil
}

// Validate rejects a zero or partially constructed listener capability.
func (l *ServerListener) Validate() error {
	if l == nil || l.listener == nil {
		return core.ErrExchangeContract
	}
	return l.address.Validate()
}

// Close closes the owned listener and normalizes an already-closed socket.
func (l *ServerListener) Close() error {
	if err := l.Validate(); err != nil {
		return err
	}
	if err := l.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return transportError(err)
	}
	return nil
}

func (l *ServerListener) take() (net.Listener, error) {
	if err := l.Validate(); err != nil {
		return nil, err
	}
	if !l.claimed.CompareAndSwap(false, true) {
		return nil, core.ErrExchangeContract
	}
	return l.listener, nil
}

// String returns the admitted address or an empty string for an invalid value.
func (a ListenAddress) String() string {
	if a.Validate() != nil {
		return ""
	}
	return a.value.String()
}

// ServerRuntimePolicy bounds the HTTP server.
type ServerRuntimePolicy struct {
	ReadHeaderTimeout  temporal.Duration
	ReadTimeout        temporal.Duration
	WriteTimeout       temporal.Duration
	IdleTimeout        temporal.Duration
	MaximumHeaderBytes core.ByteCount
}

// Validate rejects unset time and size bounds.
func (p ServerRuntimePolicy) Validate() error {
	if err := errors.Join(
		p.ReadHeaderTimeout.Validate(),
		p.ReadTimeout.Validate(),
		p.WriteTimeout.Validate(),
		p.IdleTimeout.Validate(),
		p.MaximumHeaderBytes.Validate(),
	); err != nil {
		return errors.Join(core.ErrExchangeContract, err)
	}
	if p.ReadHeaderTimeout.IsZero() || p.ReadTimeout.IsZero() || p.WriteTimeout.IsZero() || p.IdleTimeout.IsZero() {
		return core.ErrExchangeContract
	}
	return nil
}

// ServerRuntimeConfiguration is the complete effect intent for one HTTP
// listener. Products own network-reachability and peer-authentication policy.
type ServerRuntimeConfiguration struct {
	Address ListenAddress
	Policy  ServerRuntimePolicy
}

// Validate closes the listener configuration before any real-world effect.
func (c ServerRuntimeConfiguration) Validate() error {
	if err := errors.Join(c.Address.Validate(), c.Policy.Validate()); err != nil {
		return errors.Join(core.ErrExchangeContract, err)
	}
	return nil
}

// ServerRuntime owns one standard-library HTTP server and listener.
type ServerRuntime struct {
	server        *http.Server
	ready         chan error
	configuration ServerRuntimeConfiguration
	started       atomic.Bool
}

// NewServerRuntime constructs a dormant runtime without opening files or a
// network listener.
func NewServerRuntime(configuration ServerRuntimeConfiguration, handler http.Handler) (*ServerRuntime, error) {
	if handler == nil {
		return nil, core.ErrExchangeContract
	}
	if err := configuration.Validate(); err != nil {
		return nil, err
	}
	server, err := newHTTPServer(configuration.Policy, handler)
	if err != nil {
		return nil, err
	}
	runtime := &ServerRuntime{
		configuration: configuration,
		server:        server,
		ready:         make(chan error, 1),
	}
	return runtime, runtime.Validate()
}

// Validate rejects a zero or partially constructed runtime.
func (r *ServerRuntime) Validate() error {
	if r == nil || r.server == nil || r.ready == nil {
		return core.ErrExchangeContract
	}
	return r.configuration.Validate()
}

// Ready reports the result of each real listener acquisition attempt. A nil
// result means the listener is open; a non-nil result is the same typed
// transport failure Serve returns. A caller consumes one result per Serve
// attempt before retrying a failed runtime.
func (r *ServerRuntime) Ready() <-chan error {
	if r == nil {
		return nil
	}
	return r.ready
}

// Serve opens the listener and serves until Shutdown completes or the listener
// fails.
func (r *ServerRuntime) Serve() error {
	if err := r.Validate(); err != nil {
		return err
	}
	if !r.started.CompareAndSwap(false, true) {
		return core.ErrExchangeContract
	}
	listener, err := Listen(r.configuration.Address)
	if err != nil {
		r.started.Store(false)
		r.publishReady(err)
		return err
	}
	return r.serveListener(listener)
}

// ServeListener transfers one pre-opened listener into the runtime and serves
// until Shutdown completes or the listener fails.
func (r *ServerRuntime) ServeListener(listener *ServerListener) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if !r.started.CompareAndSwap(false, true) {
		return core.ErrExchangeContract
	}
	if err := listener.Validate(); err != nil {
		r.started.Store(false)
		r.publishReady(err)
		return err
	}
	return r.serveListener(listener)
}

func (r *ServerRuntime) serveListener(listener *ServerListener) error {
	owned, err := listener.take()
	if err != nil {
		r.started.Store(false)
		r.publishReady(err)
		return err
	}
	r.publishReady(nil)
	err = r.server.Serve(owned)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return transportError(err)
}

// publishReady replaces an unconsumed result instead of allowing a listener
// retry to block behind stale readiness evidence. Serve is single-owner under
// started, so at most one producer can update this one-result slot.
func (r *ServerRuntime) publishReady(result error) {
	select {
	case r.ready <- result:
		return
	default:
	}
	select {
	case <-r.ready:
	default:
	}
	select {
	case r.ready <- result:
	default:
	}
}

// Shutdown gracefully stops the owned HTTP server.
func (r *ServerRuntime) Shutdown(ctx context.Context) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if ctx == nil {
		return core.ErrExchangeContract
	}
	if !r.started.Load() {
		return core.ErrExchangeContract
	}
	if err := r.server.Shutdown(ctx); err != nil {
		return transportError(err)
	}
	return nil
}

func newHTTPServer(policy ServerRuntimePolicy, handler http.Handler) (*http.Server, error) {
	readHeaderTimeout, readHeaderErr := policy.ReadHeaderTimeout.Stdlib()
	readTimeout, readErr := policy.ReadTimeout.Stdlib()
	writeTimeout, writeErr := policy.WriteTimeout.Stdlib()
	idleTimeout, idleErr := policy.IdleTimeout.Stdlib()
	headerBytes, headerErr := serverHeaderBytes(policy.MaximumHeaderBytes)
	if err := errors.Join(readHeaderErr, readErr, writeErr, idleErr, headerErr); err != nil {
		return nil, errors.Join(core.ErrExchangeContract, err)
	}
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    headerBytes,
	}, nil
}

func serverHeaderBytes(count core.ByteCount) (int, error) {
	value, err := count.Uint64()
	if err != nil || value > uint64(^uint(0)>>1) {
		return 0, errors.Join(core.ErrExchangeContract, err)
	}
	return int(value), nil
}

var (
	_ core.Validatable = ListenAddress{}
	_ core.Validatable = (*ServerListener)(nil)
	_ core.Validatable = ServerRuntimePolicy{}
	_ core.Validatable = ServerRuntimeConfiguration{}
	_ core.Validatable = (*ServerRuntime)(nil)
)
