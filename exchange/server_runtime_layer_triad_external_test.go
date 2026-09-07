package exchange_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/netip"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestServerRuntimeLayerTriad(t *testing.T) {
	t.Parallel()
	ipv4 := netip.AddrFrom4([4]byte{127, 0, 0, 1})
	cases := []struct {
		name           string
		address        string
		wantHost       netip.Addr
		preopen        bool
		configureBound bool
		forceClose     bool
		dormant        bool
		wantRequests   int64
		maximumHeader  uint64
	}{
		{name: "Go allocated listener serves exact binary bytes and drains", address: "127.0.0.1:0", wantHost: ipv4, wantRequests: 1},
		{name: "portable header ceiling leaves Go read allowance representable", address: "127.0.0.1:0", wantHost: ipv4, wantRequests: 1, maximumHeader: core.HTTPServerHeaderMaximumBytes},
		{name: "preopened socket retains exact acquisition through transfer", address: "127.0.0.1:0", wantHost: ipv4, preopen: true, wantRequests: 1},
		{name: "observed bound address satisfies an allocated listener agreement", address: "127.0.0.1:0", wantHost: ipv4, preopen: true, configureBound: true, wantRequests: 1},
		{name: "Go IPv4 mapping cannot lose the original transfer agreement", address: "[::ffff:127.0.0.1]:0", wantHost: ipv4, preopen: true, wantRequests: 1},
		{name: "IPv6 allocation retains the bound address family", address: "[::1]:0", wantHost: netip.IPv6Loopback(), preopen: true, wantRequests: 1},
		{name: "immediate Go close terminates the owned serving goroutine", address: "127.0.0.1:0", wantHost: ipv4, preopen: true, forceClose: true, wantRequests: 1},
		{name: "neutral dormant construction cannot acquire an occupied socket", address: "127.0.0.1:0", wantHost: ipv4, preopen: true, configureBound: true, dormant: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			configuration := serverRuntimeConfiguration(t, tc.address)
			if tc.maximumHeader != 0 {
				configuration.Policy.MaximumHeaderBytes = mustByteCount(t, tc.maximumHeader)
			}
			var listener *exchange.ServerListener
			var bound exchange.ListenAddress
			if tc.preopen {
				var err error
				listener, err = exchange.Listen(configuration.Address)
				if err != nil {
					t.Fatalf("listener acquisition = %v, want nil", err)
				}
				t.Cleanup(func() {
					if err := listener.Close(); err != nil {
						t.Errorf("listener cleanup = %v, want nil", err)
					}
				})
				bound, err = listener.Address()
				if err != nil {
					t.Fatalf("listener address = %v, want nil", err)
				}
				if tc.configureBound {
					configuration.Address = bound
				}
			}
			payload := []byte{0x00, 0xff}
			var requests atomic.Int64
			handled := make(chan error, 1)
			handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				requests.Add(1)
				call, err := exchange.NewSocketServerCall(writer, request)
				if err == nil {
					err = exchange.WriteBounded(exchange.BoundedWriteCall{Call: call, Response: exchange.ServerBoundedResponse{Body: payload, ContentType: core.HTTPMediaTypeOctetStream(), Status: core.HTTPStatusOK()}})
				}
				select {
				case handled <- err:
				default:
				}
			})
			runtime, err := exchange.NewServerRuntime(configuration, handler)
			if err != nil {
				t.Fatalf("runtime construction = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := runtime.Close(); err != nil {
					t.Errorf("runtime cleanup = %v, want nil", err)
				}
			})
			if got, err := runtime.Address(); !errors.Is(err, core.ErrExchangeContract) || got != (exchange.ListenAddress{}) {
				t.Fatalf("dormant address = (%v,%v), want zero and contract refusal", got, err)
			}
			select {
			case extra := <-runtime.Ready():
				t.Fatalf("dormant acquisition = %v, want none", extra)
			default:
			}
			if tc.dormant {
				if err := runtime.Shutdown(t.Context()); !errors.Is(err, core.ErrExchangeContract) {
					t.Fatalf("dormant graceful shutdown = %v, want contract refusal", err)
				}
				if err := runtime.Close(); err != nil {
					t.Fatalf("dormant Go close = %v, want nil", err)
				}
				if requests.Load() != tc.wantRequests {
					t.Fatalf("dormant requests = %d, want %d", requests.Load(), tc.wantRequests)
				}
				return
			}
			backstop := exchangeFixtureBackstop(t, 10*time.Second)
			done := make(chan struct{})
			var serveErr error
			t.Cleanup(func() {
				if err := runtime.Close(); err != nil {
					t.Errorf("serving owner close = %v, want nil", err)
				}
				if listener != nil {
					if err := listener.Close(); err != nil {
						t.Errorf("listener backstop close = %v, want nil", err)
					}
				}
				select {
				case <-done:
				case <-backstop:
					t.Error("owned serving goroutine did not exit after close")
				}
			})
			go func() {
				if tc.preopen {
					serveErr = runtime.ServeListener(listener)
				} else {
					serveErr = runtime.Serve()
				}
				close(done)
			}()
			select {
			case readyErr := <-runtime.Ready():
				if readyErr != nil {
					t.Fatalf("acquisition result = %v, want nil", readyErr)
				}
			case <-backstop:
				t.Fatal("listener acquisition did not finish")
			}
			observed, err := runtime.Address()
			if err != nil {
				t.Fatalf("runtime bound address = %v, want nil", err)
			}
			standard, err := netip.ParseAddrPort(observed.String())
			if err != nil || standard.Port() == 0 || standard.Addr() != tc.wantHost {
				t.Fatalf("bound address = (%v,%v), want exact loopback and allocated port", standard, err)
			}
			if tc.preopen && observed != bound {
				t.Fatalf("transferred address = %v, want acquired %v", observed, bound)
			}
			if err := runtime.Serve(); !errors.Is(err, core.ErrExchangeContract) {
				t.Fatalf("duplicate serve = %v, want contract refusal", err)
			}
			select {
			case extra := <-runtime.Ready():
				t.Fatalf("duplicate serve invented acquisition %v", extra)
			default:
			}
			transport := &http.Transport{Proxy: nil}
			t.Cleanup(transport.CloseIdleConnections)
			client := mustExchangeClient(t, &http.Client{Transport: transport})
			response, err := exchange.SendNoBodyBounded(exchange.NoBodyBoundedCall{
				Context: t.Context(), Client: client,
				Request: exchange.NoBodyBoundedRequest{Target: mustEndpoint(t, "http://"+observed.String()+"/"), Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK(), ExpectedResponseContentType: core.HTTPMediaTypeOctetStream()},
				Policy:  exchange.NoBodyBoundedPolicy{Operation: singleAttemptOperationPolicy(t), ResponseBodyLimit: mustByteCount(t, uint64(len(payload)))},
			})
			if err != nil || !bytes.Equal(response.Body, payload) || response.Metadata.Bytes.Uint64() != uint64(len(payload)) || response.Metadata.Attempts != 1 || response.Metadata.Status != core.HTTPStatusOK() {
				t.Fatalf("runtime exchange = (%x,%+v,%v), want exact binary body, status and one attempt", response.Body, response.Metadata, err)
			}
			select {
			case err := <-handled:
				if err != nil {
					t.Fatalf("server write = %v, want nil", err)
				}
			case <-backstop:
				t.Fatal("server handler did not publish its completed write")
			}
			if tc.forceClose {
				err = runtime.Close()
			} else {
				err = runtime.Shutdown(t.Context())
			}
			if err != nil {
				t.Fatalf("server stop = %v, want nil", err)
			}
			select {
			case <-done:
			case <-backstop:
				t.Fatal("server stop did not end its serving goroutine")
			}
			if serveErr != nil || requests.Load() != tc.wantRequests {
				t.Fatalf("serve result/requests = (%v,%d), want (nil,%d)", serveErr, requests.Load(), tc.wantRequests)
			}
			if retained, err := runtime.Address(); err != nil || retained != observed {
				t.Fatalf("closed runtime acquisition = (%v,%v), want (%v,nil)", retained, err, observed)
			}
		})
	}
}

func TestServerRuntimeAdmissionRefusalTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                          string
		missingHandler, missingPolicy bool
	}{
		{name: "absent handler cannot select Go global default mux", missingHandler: true},
		{name: "zero bounds cannot create a server", missingPolicy: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			configuration := serverRuntimeConfiguration(t, "127.0.0.1:0")
			var handler http.Handler = http.NotFoundHandler()
			if tc.missingHandler {
				handler = nil
			}
			if tc.missingPolicy {
				configuration.Policy = exchange.ServerRuntimePolicy{}
			}
			got, err := exchange.NewServerRuntime(configuration, handler)
			if !errors.Is(err, core.ErrExchangeContract) || got != nil {
				t.Fatalf("runtime admission = (%v,%v), want nil capability and contract refusal", got, err)
			}
		})
	}
}

func TestServerRuntimeInvalidTransferPreservesRetryAndAbsence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		inputs       []*exchange.ServerListener
		wantAttempts int
	}{
		{name: "absent and zero capabilities cannot acquire or consume a runtime", inputs: []*exchange.ServerListener{nil, {}}, wantAttempts: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runtime, err := exchange.NewServerRuntime(serverRuntimeConfiguration(t, "127.0.0.1:0"), http.NotFoundHandler())
			if err != nil {
				t.Fatalf("runtime fixture = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := runtime.Close(); err != nil {
					t.Errorf("runtime cleanup = %v, want nil", err)
				}
			})
			attempts := 0
			for _, input := range tc.inputs {
				attempts++
				if err := runtime.ServeListener(input); !errors.Is(err, core.ErrExchangeContract) {
					t.Fatalf("invalid transfer = %v, want contract refusal", err)
				}
				select {
				case readyErr := <-runtime.Ready():
					if !errors.Is(readyErr, core.ErrExchangeContract) {
						t.Fatalf("refused acquisition = %v, want contract refusal", readyErr)
					}
				default:
					t.Fatal("completed refusal omitted its acquisition result")
				}
				select {
				case extra := <-runtime.Ready():
					t.Fatalf("extra acquisition result = %v, want none", extra)
				default:
				}
				if got, err := runtime.Address(); !errors.Is(err, core.ErrExchangeContract) || got != (exchange.ListenAddress{}) {
					t.Fatalf("invalid transfer address = (%v,%v), want zero and contract refusal", got, err)
				}
			}
			if attempts != tc.wantAttempts {
				t.Fatalf("admission attempts = %d, want %d", attempts, tc.wantAttempts)
			}
		})
	}
}

func serverRuntimeConfiguration(t testing.TB, address string) exchange.ServerRuntimeConfiguration {
	t.Helper()
	parsed, addressErr := exchange.ParseListenAddress(address)
	readHeader, readHeaderErr := temporal.DurationFromSeconds(5)
	read, readErr := temporal.DurationFromSeconds(10)
	write, writeErr := temporal.DurationFromSeconds(10)
	idle, idleErr := temporal.DurationFromSeconds(30)
	headerBytes, headerErr := core.NewByteCount(32 * 1024)
	if err := errors.Join(addressErr, readHeaderErr, readErr, writeErr, idleErr, headerErr); err != nil {
		t.Fatalf("server runtime configuration error = %v, want nil", err)
	}
	configuration := exchange.ServerRuntimeConfiguration{
		Address: parsed,
		Policy: exchange.ServerRuntimePolicy{
			ReadHeaderTimeout:  readHeader,
			ReadTimeout:        read,
			WriteTimeout:       write,
			IdleTimeout:        idle,
			MaximumHeaderBytes: headerBytes,
		},
	}
	if err := configuration.Validate(); err != nil {
		t.Fatalf("ServerRuntimeConfiguration.Validate() error = %v, want nil", err)
	}
	return configuration
}

func TestServerRuntimeOccupiedAddressRecoversByOwnedTransfer(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                   string
		attempts, wantRefusals int
	}{
		{name: "repeated Go bind refusals preserve custody for later exact transfer", attempts: 2, wantRefusals: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			configuration := serverRuntimeConfiguration(t, "127.0.0.1:0")
			listener, err := exchange.Listen(configuration.Address)
			if err != nil {
				t.Fatalf("occupied listener fixture = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := listener.Close(); err != nil {
					t.Errorf("listener cleanup = %v, want nil", err)
				}
			})
			address, err := listener.Address()
			if err != nil {
				t.Fatalf("occupied address = %v, want nil", err)
			}
			configuration.Address = address
			runtime, err := exchange.NewServerRuntime(configuration, http.NotFoundHandler())
			if err != nil {
				t.Fatalf("runtime fixture = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := runtime.Close(); err != nil {
					t.Errorf("runtime cleanup = %v, want nil", err)
				}
			})
			refusals := 0
			for range tc.attempts {
				gotErr := runtime.Serve()
				if !errors.Is(gotErr, core.ErrExchangeTransport) || !errors.Is(gotErr, syscall.EADDRINUSE) {
					t.Fatalf("occupied bind = %v, want Exchange transport and Go address-in-use identity", gotErr)
				}
				select {
				case readyErr := <-runtime.Ready():
					if !errors.Is(readyErr, core.ErrExchangeTransport) || !errors.Is(readyErr, syscall.EADDRINUSE) {
						t.Fatalf("occupied readiness = %v, want typed bind refusal", readyErr)
					}
				default:
					t.Fatal("completed bind refusal omitted readiness")
				}
				if got, err := runtime.Address(); got != (exchange.ListenAddress{}) || !errors.Is(err, core.ErrExchangeContract) {
					t.Fatalf("failed bind address = (%v,%v), want absent", got, err)
				}
				refusals++
			}
			if refusals != tc.wantRefusals {
				t.Fatalf("bind refusals = %d, want %d", refusals, tc.wantRefusals)
			}
			backstop := exchangeFixtureBackstop(t, 10*time.Second)
			done := make(chan struct{})
			var serveErr error
			t.Cleanup(func() {
				if err := runtime.Close(); err != nil {
					t.Errorf("server owner close = %v, want nil", err)
				}
				if err := listener.Close(); err != nil {
					t.Errorf("listener backstop = %v, want nil", err)
				}
				select {
				case <-done:
				case <-backstop:
					t.Error("recovered serving goroutine did not exit")
				}
			})
			go func() { serveErr = runtime.ServeListener(listener); close(done) }()
			select {
			case readyErr := <-runtime.Ready():
				if readyErr != nil {
					t.Fatalf("recovered transfer = %v, want nil", readyErr)
				}
			case <-backstop:
				t.Fatal("recovered transfer omitted readiness")
			}
			if got, err := runtime.Address(); err != nil || got != address {
				t.Fatalf("recovered address = (%v,%v), want exact acquired %v", got, err, address)
			}
			if err := runtime.Shutdown(t.Context()); err != nil {
				t.Fatalf("recovered shutdown = %v, want nil", err)
			}
			select {
			case <-done:
			case <-backstop:
				t.Fatal("recovered serving goroutine did not exit")
			}
			if serveErr != nil {
				t.Fatalf("recovered serve = %v, want nil", serveErr)
			}
		})
	}
}
