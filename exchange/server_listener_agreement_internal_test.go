package exchange

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestServerListenerConfigurationAgreementTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		configured  func(netip.AddrPort) netip.AddrPort
		wantErr     error
		wantClaimed bool
		closeBefore bool
	}{
		{name: "exact acquired socket transfers custody", configured: func(a netip.AddrPort) netip.AddrPort { return a }, wantClaimed: true},
		{name: "closed socket cannot publish successful transfer", configured: func(a netip.AddrPort) netip.AddrPort { return a }, closeBefore: true, wantErr: core.ErrExchangeContract, wantClaimed: true},
		{name: "another port cannot satisfy the configured socket", configured: func(a netip.AddrPort) netip.AddrPort {
			port := a.Port() + 1
			if port == 0 {
				port = 1
			}
			return netip.AddrPortFrom(a.Addr(), port)
		}, wantErr: core.ErrExchangeContract},
		{name: "another local interface cannot satisfy the configured socket", configured: func(a netip.AddrPort) netip.AddrPort {
			return netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 2}), a.Port())
		}, wantErr: core.ErrExchangeContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// The direct handoff fixture keeps the real Go listener open.
			// It does not reserve, release, and then try to reclaim a port.
			native, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("Go listener fixture = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := native.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
					t.Errorf("fixture close = %v, want nil or closed", err)
				}
			})
			observed, err := netip.ParseAddrPort(native.Addr().String())
			if err != nil || observed.Port() == 0 {
				t.Fatalf("Go bound address = (%v,%v), want allocated port", observed, err)
			}
			address, err := ParseListenAddress(observed.String())
			if err != nil {
				t.Fatalf("bound address fixture = %v, want nil", err)
			}
			listener := &ServerListener{listener: native, requested: address, address: address}
			if tc.closeBefore {
				if err := listener.Close(); err != nil {
					t.Fatalf("close-before-transfer fixture = %v, want nil", err)
				}
			}
			configured, err := ParseListenAddress(tc.configured(observed).String())
			if err != nil {
				t.Fatalf("configured address fixture = %v, want nil", err)
			}
			if tc.wantErr != nil && !tc.closeBefore && configured == address {
				t.Fatal("address mutation did not change the socket agreement")
			}
			runtime, err := NewServerRuntime(ServerRuntimeConfiguration{Address: configured, Policy: runtimeAgreementPolicy(t)}, http.NotFoundHandler())
			if err != nil {
				t.Fatalf("runtime fixture = %v, want nil", err)
			}
			backstop := runtimeAgreementContext(t)
			done := make(chan struct{})
			var serveErr error
			t.Cleanup(func() {
				// Direct Go close is the cancellation backstop for this private
				// handoff test; the public lifetime is tested separately.
				if err := runtime.server.Close(); err != nil {
					t.Errorf("Go server close = %v, want nil", err)
				}
				select {
				case <-done:
				case <-backstop.Done():
					t.Error("serving goroutine did not exit after its owner closed Go's server")
				}
			})
			go func() { serveErr = runtime.ServeListener(listener); close(done) }()
			select {
			case readyErr := <-runtime.Ready():
				if !errors.Is(readyErr, tc.wantErr) {
					t.Fatalf("listener admission = %v, want %v", readyErr, tc.wantErr)
				}
			case <-backstop.Done():
				t.Fatal("listener admission did not publish its result")
			}
			if listener.claimed.Load() != tc.wantClaimed {
				t.Fatalf("listener custody claimed = %t, want %t", listener.claimed.Load(), tc.wantClaimed)
			}
			if err := runtime.server.Close(); err != nil {
				t.Fatalf("Go close = %v, want nil", err)
			}
			select {
			case <-done:
			case <-backstop.Done():
				t.Fatal("serving goroutine did not exit")
			}
			if !errors.Is(serveErr, tc.wantErr) {
				t.Fatalf("serve result = %v, want %v", serveErr, tc.wantErr)
			}
		})
	}
}

func runtimeAgreementPolicy(t testing.TB) ServerRuntimePolicy {
	t.Helper()
	duration, err := temporal.DurationFromSeconds(10)
	if err != nil {
		t.Fatalf("timeout fixture = %v, want nil", err)
	}
	maximum, err := core.NewByteCount(32 * 1024)
	if err != nil {
		t.Fatalf("header limit fixture = %v, want nil", err)
	}
	return ServerRuntimePolicy{ReadHeaderTimeout: duration, ReadTimeout: duration, WriteTimeout: duration, IdleTimeout: duration, MaximumHeaderBytes: maximum}
}

func runtimeAgreementContext(t testing.TB) context.Context {
	t.Helper()
	duration, err := temporal.DurationFromSeconds(10)
	if err != nil {
		t.Fatalf("backstop duration = %v, want nil", err)
	}
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: context.WithoutCancel(t.Context()), Duration: duration})
	if err != nil {
		t.Fatalf("backstop context = %v, want nil", err)
	}
	t.Cleanup(cancel)
	return ctx
}
