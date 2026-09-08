package exchange

import (
	"errors"
	"math"
	"net"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// This constructor admits typed configuration and delegates field projection
// to Go. It must perform no listen effect. Address text separately exercises
// the public literal boundary before crossing the configuration boundary.
func FuzzServerRuntimeConfigurationAdmission(f *testing.F) {
	seedAddress, err := ParseListenAddress("127.0.0.1:0")
	if err != nil {
		f.Fatalf("address seed = %v, want nil", err)
	}
	seedPolicy := runtimeAgreementPolicy(f)
	if err := seedPolicy.Validate(); err != nil {
		f.Fatalf("policy seed = %v, want nil", err)
	}
	seedMaximum, err := seedPolicy.MaximumHeaderBytes.Uint64()
	if err != nil {
		f.Fatalf("header seed projection = %v, want nil", err)
	}
	f.Add(seedAddress.String(), seedPolicy.ReadHeaderTimeout.Nanoseconds(), seedMaximum, uint8(0))
	for field := range uint8(4) {
		f.Add(seedAddress.String(), int64(0), seedMaximum, field)
		f.Add(seedAddress.String(), int64(1), seedMaximum, field)
		f.Add(seedAddress.String(), int64(math.MaxInt64), seedMaximum, field)
	}
	f.Add(seedAddress.String(), int64(1), uint64(0), uint8(0))
	f.Add(seedAddress.String(), int64(1), uint64(1), uint8(0))
	f.Add(seedAddress.String(), int64(1), uint64(core.HTTPServerHeaderMaximumBytes-1), uint8(0))
	f.Add(seedAddress.String(), int64(1), uint64(core.HTTPServerHeaderMaximumBytes), uint8(0))
	f.Add(seedAddress.String(), int64(1), uint64(core.HTTPServerHeaderMaximumBytes+1), uint8(0))
	f.Add(seedAddress.String(), int64(1), uint64(math.MaxInt), uint8(0))
	f.Add(seedAddress.String(), int64(1), uint64(math.MaxInt)+1, uint8(0))
	f.Add(seedAddress.String(), int64(1), uint64(math.MaxUint64), uint8(0))
	f.Add("[::%lo0]:0", int64(1), seedMaximum, uint8(0))
	f.Add("", int64(1), seedMaximum, uint8(0))
	f.Fuzz(func(t *testing.T, text string, nanoseconds int64, maximum uint64, field uint8) {
		if len(text) > 4096 || nanoseconds < 0 || field > 3 {
			return
		}
		standard, standardErr := netip.ParseAddrPort(text)
		wantAddress := standardErr == nil && !net.IP(standard.Addr().AsSlice()).IsUnspecified()
		address, addressErr := ParseListenAddress(text)
		if (addressErr == nil) != wantAddress {
			t.Fatalf("address admission = %v, want admitted %t", addressErr, wantAddress)
		}
		if !wantAddress && (address != (ListenAddress{}) || !errors.Is(addressErr, core.ErrExchangeContract)) {
			t.Fatalf("refused address = (%v,%v), want exact zero and typed refusal", address, addressErr)
		}
		policy := seedPolicy
		duration, err := temporal.DurationFromNanoseconds(nanoseconds)
		if err != nil {
			t.Fatalf("bounded duration fixture = %v, want nil", err)
		}
		wantTimeouts := [4]time.Duration{time.Duration(seedPolicy.ReadHeaderTimeout.Nanoseconds()), time.Duration(seedPolicy.ReadTimeout.Nanoseconds()), time.Duration(seedPolicy.WriteTimeout.Nanoseconds()), time.Duration(seedPolicy.IdleTimeout.Nanoseconds())}
		wantTimeouts[field] = time.Duration(nanoseconds)
		switch field {
		case 0:
			policy.ReadHeaderTimeout = duration
		case 1:
			policy.ReadTimeout = duration
		case 2:
			policy.WriteTimeout = duration
		case 3:
			policy.IdleTimeout = duration
		}
		// A refused Core count is passed as the explicit zero count. Its
		// earlier error is not claimed as a fact the runtime received.
		policy.MaximumHeaderBytes, _ = core.NewByteCount(maximum)
		configuration := ServerRuntimeConfiguration{Address: address, Policy: policy}
		wantAdmitted := wantAddress && nanoseconds > 0 && maximum > 0 && maximum <= uint64(core.HTTPServerHeaderMaximumBytes)
		var wantErr error
		if !wantAdmitted {
			wantErr = core.ErrExchangeContract
		}
		if gotErr := configuration.Validate(); !errors.Is(gotErr, wantErr) {
			t.Fatalf("configuration validation = %v, want %v before construction", gotErr, wantErr)
		}
		handler := http.NewServeMux()
		got, gotErr := NewServerRuntime(configuration, handler)
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("runtime admission = %v, want %v", gotErr, wantErr)
		}
		if !wantAdmitted {
			if got != nil {
				t.Fatalf("refused server=%v, want nil", got)
			}
			return
		}
		t.Cleanup(func() {
			if err := got.Close(); err != nil {
				t.Errorf("dormant close = %v, want nil", err)
			}
		})
		if err := got.Validate(); err != nil {
			t.Fatalf("runtime validation = %v, want nil", err)
		}
		gotTimeouts := [4]time.Duration{got.server.ReadHeaderTimeout, got.server.ReadTimeout, got.server.WriteTimeout, got.server.IdleTimeout}
		if gotTimeouts != wantTimeouts || got.server.MaxHeaderBytes != int(maximum) || got.server.Handler != handler || got.configuration != configuration {
			t.Fatalf("Go server projection = (%v,%d), want (%v,%d) and exact handler/configuration", gotTimeouts, got.server.MaxHeaderBytes, wantTimeouts, maximum)
		}
		if observed, err := got.Address(); observed != (ListenAddress{}) || !errors.Is(err, core.ErrExchangeContract) {
			t.Fatalf("dormant bound address = (%v,%v), want absent", observed, err)
		}
		select {
		case extra := <-got.Ready():
			t.Fatalf("dormant constructor published acquisition %v", extra)
		default:
		}
	})
}

func TestServerCapabilityZeroAdmissionTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		runtime  *ServerRuntime
		listener *ServerListener
	}{
		{name: "nil capabilities cannot execute or invent an address"},
		{name: "zero capabilities cannot execute or invent an address", runtime: &ServerRuntime{}, listener: &ServerListener{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, call := range []func() error{tc.runtime.Validate, tc.runtime.Close, tc.runtime.Serve, func() error { return tc.runtime.Shutdown(t.Context()) }, func() error { return tc.runtime.ServeListener(tc.listener) }, tc.listener.Validate, tc.listener.Close} {
				if err := call(); !errors.Is(err, core.ErrExchangeContract) {
					t.Fatalf("zero capability operation = %v, want contract refusal", err)
				}
			}
			for _, call := range []func() (ListenAddress, error){tc.runtime.Address, tc.listener.Address} {
				if address, err := call(); address != (ListenAddress{}) || !errors.Is(err, core.ErrExchangeContract) {
					t.Fatalf("zero capability address = (%v,%v), want absent", address, err)
				}
			}
			if tc.runtime.Ready() != nil {
				t.Fatalf("zero-runtime readiness=%v, want nil", tc.runtime.Ready())
			}
		})
	}
}
