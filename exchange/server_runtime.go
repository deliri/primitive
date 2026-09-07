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

// ListenAddress is one concrete TCP host and port. In a listen request, port
// zero asks Go and the operating system to allocate the port. An acquired
// ServerListener reports the actual nonzero bound address separately.
type ListenAddress struct{ value netip.AddrPort }

// ParseListenAddress admits one concrete host and Go TCP port intent.
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

// Validate rejects absent, portless, and unspecified listener addresses. Match
// Go's TCP wildcard interpretation: a zone or IPv4 mapping cannot make an
// unspecified host concrete. Keep the caller's admitted address unchanged.
func (a ListenAddress) Validate() error {
	if !a.value.IsValid() || a.value.Addr().WithZone("").Unmap().IsUnspecified() {
		return core.ErrExchangeContract
	}
	return nil
}

// ServerListener is one already-open TCP listener. It keeps the standard
// listener private to Exchange while allowing a product to prove port
// acquisition before it constructs the rest of its boot graph.
type ServerListener struct {
	listener  net.Listener
	requested ListenAddress
	address   ListenAddress
	claimed   atomic.Bool
}

// Listen opens one exact TCP listener. The caller owns the returned capability
// until it transfers the capability to ServerRuntime or closes it.
func Listen(address ListenAddress) (owned *ServerListener, resultErr error) {
	if err := address.Validate(); err != nil {
		return nil, err
	}
	listener, err := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(address.value))
	if err != nil {
		return nil, transportError(err)
	}
	defer func() {
		if owned != nil {
			return
		}
		if closeErr := listener.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, transportError(closeErr))
		}
	}()
	bound, err := ParseListenAddress(listener.Addr().String())
	if err != nil {
		return nil, err
	}
	candidate := &ServerListener{requested: address, address: bound, listener: listener}
	if err := candidate.Validate(); err != nil {
		return nil, err
	}
	return candidate, nil
}

// Validate rejects a zero or partially constructed listener capability.
func (l *ServerListener) Validate() error {
	if l == nil || l.listener == nil {
		return core.ErrExchangeContract
	}
	if err := errors.Join(l.requested.Validate(), l.address.Validate()); err != nil {
		return err
	}
	if l.address.value.Port() == 0 {
		return core.ErrExchangeContract
	}
	return nil
}

// Address returns the exact address observed from Go's acquired listener,
// including the port allocated for a zero-port request. Closing the listener
// does not erase this acquisition fact.
func (l *ServerListener) Address() (ListenAddress, error) {
	if err := l.Validate(); err != nil {
		return ListenAddress{}, err
	}
	return l.address, nil
}

// Close closes the owned listener and normalizes an already-closed socket.
func (l *ServerListener) Close() error {
	if err := l.Validate(); err != nil {
		return err
	}
	l.claimed.Store(true)
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
	_, headerErr := serverHeaderBytes(p.MaximumHeaderBytes)
	if err := errors.Join(
		p.ReadHeaderTimeout.Validate(),
		p.ReadTimeout.Validate(),
		p.WriteTimeout.Validate(),
		p.IdleTimeout.Validate(),
		headerErr,
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
	bound         atomic.Pointer[ServerListener]
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
// until Shutdown completes or the listener fails. Configuration must name
// either its exact original listen intent or its actual bound address.
func (r *ServerRuntime) ServeListener(listener *ServerListener) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if !r.started.CompareAndSwap(false, true) {
		return core.ErrExchangeContract
	}
	if err := listener.admitConfiguration(r.configuration.Address); err != nil {
		r.started.Store(false)
		r.publishReady(err)
		return err
	}
	return r.serveListener(listener)
}

func (l *ServerListener) admitConfiguration(address ListenAddress) error {
	if err := l.Validate(); err != nil {
		return err
	}
	if address != l.requested && address != l.address {
		return core.ErrExchangeContract
	}
	return nil
}

func (r *ServerRuntime) serveListener(listener *ServerListener) error {
	owned, err := listener.take()
	if err != nil {
		r.started.Store(false)
		r.publishReady(err)
		return err
	}
	r.bound.Store(listener)
	r.publishReady(nil)
	err = r.server.Serve(owned)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return transportError(err)
}

// Address returns the exact listener acquisition fact after Ready reports nil.
// It refuses before acquisition; shutdown preserves the observed address.
func (r *ServerRuntime) Address() (ListenAddress, error) {
	if err := r.Validate(); err != nil {
		return ListenAddress{}, err
	}
	return r.bound.Load().Address()
}

// Close delegates immediate listener and connection shutdown to Go. It is
// also valid for a dormant server. Call Shutdown for graceful draining first
// when that is the caller's policy. Hijacked connections remain caller-owned,
// exactly as documented by net/http.Server.Close.
func (r *ServerRuntime) Close() error {
	if err := r.Validate(); err != nil {
		return err
	}
	if err := r.server.Close(); err != nil {
		return transportError(err)
	}
	return nil
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
	if err != nil || value > uint64(core.HTTPServerHeaderMaximumBytes) {
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
