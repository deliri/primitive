package exchange_test

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/deliri/primitive/v2026/exchange"
)

// Measures only address resolution and its typed validation. Request creation,
// trusted-prefix parsing and Go HTTP sealing are setup, outside the timed loop.
// Header inputs and exact expected addresses stay fixed throughout each leaf.
func BenchmarkResolveClientAddress(b *testing.B) {
	b.ReportAllocs()
	cases := []struct {
		name, peer, forwarded, proxies, want string
		authority                            exchange.ClientAddressAuthority
	}{
		{name: "direct_peer_ignores_header", peer: "192.0.2.8:443", forwarded: "203.0.113.9", want: "192.0.2.8", authority: exchange.ClientAddressAuthorityPeer},
		{name: "trusted_proxy_chain", peer: "192.0.2.8:443", forwarded: "203.0.113.9, 192.0.2.4", proxies: "192.0.2.0/24", want: "203.0.113.9", authority: exchange.ClientAddressAuthorityTrustedProxy},
		{name: "untrusted_peer_ignores_chain", peer: "198.51.100.8:443", forwarded: "203.0.113.9, 192.0.2.4", proxies: "192.0.2.0/24", want: "198.51.100.8", authority: exchange.ClientAddressAuthorityTrustedProxy},
		{name: "google_cloud_final_pair", peer: "127.0.0.1:443", forwarded: "198.51.100.200, 203.0.113.9, 192.0.2.4", want: "203.0.113.9", authority: exchange.ClientAddressAuthorityGoogleCloud},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			request := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/", nil)
			request.RemoteAddr = tc.peer
			field, err := exchange.StandardHeaderForwardedFor.Name()
			if err != nil {
				b.Fatal(err)
			}
			request.Header.Set(field.String(), tc.forwarded)
			prefixes, err := exchange.ParseTrustedProxyPrefixes(tc.proxies)
			if err != nil {
				b.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			call := exchange.ClientAddressRequest{Call: socketServerCallFrom(b, recorder, request), Authority: tc.authority, TrustedProxies: prefixes}
			if err := call.Validate(); err != nil {
				b.Fatal(err)
			}
			want, err := netip.ParseAddr(tc.want)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				got, err := exchange.ResolveClientAddress(call)
				if err != nil || got.Address != want {
					b.Fatalf("address resolution=(%v,%v),want exact %v", got.Address, err, want)
				}
			}
			if request.RemoteAddr != tc.peer || request.Header.Get(field.String()) != tc.forwarded || recorder.Body.Len() != 0 || len(recorder.Header()) != 0 {
				b.Fatalf("peer/forwarded/body/header=%q/%q/%d/%d, want %q/%q/0/0", request.RemoteAddr, request.Header.Get(field.String()), recorder.Body.Len(), len(recorder.Header()), tc.peer, tc.forwarded)
			}
		})
	}
}
