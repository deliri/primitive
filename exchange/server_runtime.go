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

// Validate rejects zero, unspecified, or portless listener addresses.
func (a ListenAddress) Validate() error {
	if !a.value.IsValid() || a.value.Addr().IsUnspecified() || a.value.Port() == 0 {
		return core.ErrExchangeContract
	}
	return nil
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
	configuration ServerRuntimeConfiguration
	server        *http.Server
	ready         chan struct{}
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
		ready:         make(chan struct{}),
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

// Ready closes after the real listener is open and immediately before serving.
func (r *ServerRuntime) Ready() <-chan struct{} {
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
	listener, err := net.Listen("tcp", r.configuration.Address.String())
	if err != nil {
		return transportError(err)
	}
	close(r.ready)
	err = r.server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return transportError(err)
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
	_ core.Validatable = ServerRuntimePolicy{}
	_ core.Validatable = ServerRuntimeConfiguration{}
	_ core.Validatable = (*ServerRuntime)(nil)
)
