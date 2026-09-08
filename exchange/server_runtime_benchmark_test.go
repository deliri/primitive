package exchange

import (
	"net/http"
	"net/netip"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Construction measures a dormant real Go server. Listener acquisition measures
// the actual Go/OS bind and close, including zero-port allocation on loopback;
// it never connects clients and therefore creates no client TIME_WAIT workload.
func BenchmarkServerRuntimeBoundary(b *testing.B) {
	b.ReportAllocs()
	cases := []struct {
		name   string
		listen bool
	}{
		{name: "dormant_construction"},
		{name: "os_allocated_listener_acquire_close", listen: true},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			duration, err := temporal.DurationFromSeconds(30)
			if err != nil {
				b.Fatalf("duration fixture = %v, want nil", err)
			}
			maximum, err := core.NewByteCount(32 * 1024)
			if err != nil {
				b.Fatalf("header fixture = %v, want nil", err)
			}
			address, err := ParseListenAddress("127.0.0.1:8080")
			if err != nil {
				b.Fatalf("address fixture = %v, want nil", err)
			}
			configuration := ServerRuntimeConfiguration{Address: address, Policy: ServerRuntimePolicy{
				ReadHeaderTimeout: duration, ReadTimeout: duration, WriteTimeout: duration, IdleTimeout: duration, MaximumHeaderBytes: maximum}}
			handler := http.NewServeMux()
			b.ReportAllocs()
			for b.Loop() {
				if tc.listen {
					requested, err := ParseListenAddress("127.0.0.1:0")
					if err != nil {
						b.Fatalf("Go zero-port intent = %v, want nil", err)
					}
					listener, err := Listen(requested)
					if err != nil {
						b.Fatalf("listener acquisition = %v, want nil", err)
					}
					observed, parseErr := netip.ParseAddrPort(listener.listener.Addr().String())
					validationErr := listener.Validate()
					closeErr := listener.Close()
					if parseErr != nil || validationErr != nil || closeErr != nil || observed.Port() == 0 || observed.Addr() != netip.AddrFrom4([4]byte{127, 0, 0, 1}) {
						b.Fatalf("Go listener = (%v, %v, %v, %v), want exact loopback, allocated port and successful close", observed, parseErr, validationErr, closeErr)
					}
					continue
				}
				runtime, err := NewServerRuntime(configuration, handler)
				if err != nil {
					b.Fatalf("dormant server = %v, want nil", err)
				}
				if err := runtime.Validate(); err != nil || runtime.configuration != configuration || runtime.server.Handler != handler || len(runtime.Ready()) != 0 {
					b.Fatalf("dormant server projection = %v, want exact configuration, handler and no acquisition fact", err)
				}
			}
		})
	}
}
