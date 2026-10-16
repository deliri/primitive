package exchange_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// publishedCloudflareEdgeText is the provider's published list as retained in
// evidence (api.cloudflare.com/client/v4/ips, etag
// 38f79d050aa027e3be3865e495dcc9bc). It is an independent text oracle for the
// compiled typed-octet set: the two representations must agree exactly.
const publishedCloudflareEdgeText = "173.245.48.0/20,103.21.244.0/22,103.22.200.0/22,103.31.4.0/22,141.101.64.0/18,108.162.192.0/18,190.93.240.0/20,188.114.96.0/20,197.234.240.0/22,198.41.128.0/17,162.158.0.0/15,104.16.0.0/13,104.24.0.0/14,172.64.0.0/13,131.0.72.0/22," +
	"2400:cb00::/32,2606:4700::/32,2803:f800::/32,2405:b500::/32,2405:8100::/32,2a06:98c0::/29,2c0f:f248::/32"

func TestCloudflareEdgePrefixesAgreeWithPublishedText(t *testing.T) {
	t.Parallel()

	got, err := exchange.CloudflareEdgePrefixes()
	if err != nil {
		t.Fatalf("CloudflareEdgePrefixes() error = %v, want nil", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("CloudflareEdgePrefixes().Validate() error = %v, want nil", err)
	}
	want, err := exchange.ParseTrustedProxyPrefixes(publishedCloudflareEdgeText)
	if err != nil {
		t.Fatalf("ParseTrustedProxyPrefixes(published) error = %v, want nil", err)
	}
	if got != want {
		t.Fatalf("CloudflareEdgePrefixes() = %q, want published %q", got.String(), want.String())
	}
	if gotCount, wantCount := got.Count(), uint8(exchange.CloudflareEdgeIPv4PrefixCount+exchange.CloudflareEdgeIPv6PrefixCount); gotCount != wantCount {
		t.Fatalf("CloudflareEdgePrefixes().Count() = %d, want %d", gotCount, wantCount)
	}
	if got.String() != publishedCloudflareEdgeText {
		t.Fatalf("CloudflareEdgePrefixes().String() = %q, want provider order %q", got.String(), publishedCloudflareEdgeText)
	}
	if !strings.Contains(exchange.CloudflareEdgeRevision, "38f79d050aa027e3be3865e495dcc9bc") {
		t.Fatalf("CloudflareEdgeRevision = %q, want the retained provider etag", exchange.CloudflareEdgeRevision)
	}
}

// Addresses used by the hostile table. Edge values sit at both inside edges
// of published prefixes; outside values sit one address past those edges.
const (
	cfEdgeIPv4Low         = "173.245.48.0"
	cfEdgeIPv4High        = "173.245.63.255"
	cfOutsideIPv4BelowLow = "173.245.47.255"
	cfOutsideIPv4AboveTop = "173.245.64.0"
	cfEdgeIPv6            = "2606:4700::6810:84e5"
	cfEdgeIPv6Wide29High  = "2a06:98c7:ffff:ffff:ffff:ffff:ffff:ffff"
	cfOutsideIPv6AboveTop = "2a06:98c8::"
	cfGoogleForwardRule   = "198.51.100.2"
	cfClientIPv4          = "203.0.113.9"
	cfClientIPv6          = "2001:db8:1:2:3:4:5:6"
)

type cloudflareCase struct {
	name       string
	remote     string
	forwarded  []string
	connecting []string
	authority  exchange.ClientAddressAuthority
	proxies    exchange.TrustedProxyPrefixes
	want       string
	wantErr    error
}

func TestCloudflareClientAddressHostileTable(t *testing.T) {
	t.Parallel()

	peer := exchange.ClientAddressAuthorityCloudflare
	google := exchange.ClientAddressAuthorityGoogleCloudCloudflare
	gfe := func(hop string) []string { return []string{"198.51.100.250, " + hop + ", " + cfGoogleForwardRule} }
	trusted := mustTrustedProxyPrefixes(t, "10.0.0.0/8")
	oversized := strings.Repeat("1", exchange.HeaderValueMaximumBytes+1)
	tests := []cloudflareCase{
		// Positive: the delivering hop is a published edge and the header is exact.
		{name: "positive direct edge IPv4 peer exposes IPv4 client", remote: cfEdgeIPv4Low + ":443", connecting: []string{cfClientIPv4}, authority: peer, want: cfClientIPv4},
		{name: "positive direct edge IPv4 peer exposes IPv6 client", remote: cfEdgeIPv4Low + ":443", connecting: []string{cfClientIPv6}, authority: peer, want: cfClientIPv6},
		{name: "positive direct edge IPv6 peer exposes IPv4 client", remote: "[" + cfEdgeIPv6 + "]:443", connecting: []string{cfClientIPv4}, authority: peer, want: cfClientIPv4},
		{name: "positive direct edge IPv6 peer exposes IPv6 client", remote: "[" + cfEdgeIPv6 + "]:443", connecting: []string{cfClientIPv6}, authority: peer, want: cfClientIPv6},
		{name: "positive Google packet source edge exposes client", remote: "127.0.0.1:8080", forwarded: gfe(cfEdgeIPv4Low), connecting: []string{cfClientIPv4}, authority: google, want: cfClientIPv4},
		{name: "positive Google IPv6 packet source edge exposes client", remote: "127.0.0.1:8080", forwarded: gfe(cfEdgeIPv6), connecting: []string{cfClientIPv6}, authority: google, want: cfClientIPv6},
		{name: "positive mapped IPv4 client canonicalizes to IPv4", remote: cfEdgeIPv4Low + ":443", connecting: []string{"::ffff:" + cfClientIPv4}, authority: peer, want: cfClientIPv4},
		{name: "positive mapped IPv4 edge peer still proves edge", remote: "[::ffff:" + cfEdgeIPv4Low + "]:443", connecting: []string{cfClientIPv4}, authority: peer, want: cfClientIPv4},
		{name: "positive raw edge peer without port", remote: cfEdgeIPv4Low, connecting: []string{cfClientIPv4}, authority: peer, want: cfClientIPv4},
		{name: "positive client inside the edge set is still the declared client", remote: cfEdgeIPv4Low + ":443", connecting: []string{cfEdgeIPv6}, authority: peer, want: cfEdgeIPv6},

		// Negative: an untrusted hop or contradictory policy never yields the header value.
		{name: "negative spoofed header from non-edge direct peer keeps peer", remote: "192.0.2.10:443", connecting: []string{cfClientIPv4}, authority: peer, want: "192.0.2.10"},
		{name: "negative spoofed header from non-edge Google packet source keeps source", remote: "127.0.0.1:8080", forwarded: gfe("192.0.2.11"), connecting: []string{cfClientIPv4}, authority: google, want: "192.0.2.11"},
		{name: "negative forged edge address in unverified XFF prefix is not the hop", remote: "127.0.0.1:8080", forwarded: []string{cfEdgeIPv4Low + ", 192.0.2.12, " + cfGoogleForwardRule}, connecting: []string{cfClientIPv4}, authority: google, want: "192.0.2.12"},
		{name: "negative direct-peer authority ignores edge in forwarded chain", remote: "192.0.2.13:443", forwarded: []string{cfEdgeIPv4Low + ", " + cfGoogleForwardRule}, connecting: []string{cfClientIPv4}, authority: peer, want: "192.0.2.13"},
		{name: "negative spoofed header from non-edge IPv6 direct peer keeps peer", remote: "[2001:db8::10]:443", connecting: []string{cfClientIPv4}, authority: peer, want: "2001:db8::10"},
		{name: "negative trusted prefixes contradict Cloudflare peer authority", remote: cfEdgeIPv4Low + ":443", connecting: []string{cfClientIPv4}, authority: peer, proxies: trusted, wantErr: core.ErrExchangeContract},
		{name: "negative trusted prefixes contradict Cloudflare Google authority", remote: "127.0.0.1:8080", forwarded: gfe(cfEdgeIPv4Low), connecting: []string{cfClientIPv4}, authority: google, proxies: trusted, wantErr: core.ErrExchangeContract},
		{name: "negative malformed direct peer is a request refusal", remote: "edge.example:443", connecting: []string{cfClientIPv4}, authority: peer, wantErr: core.ErrExchangeRequest},
		{name: "negative zoned direct peer is a request refusal", remote: "[fe80::1%en0]:443", connecting: []string{cfClientIPv4}, authority: peer, wantErr: core.ErrExchangeRequest},
		{name: "negative next unknown authority is refused", remote: cfEdgeIPv4Low + ":443", connecting: []string{cfClientIPv4}, authority: google + 1, wantErr: core.ErrExchangeContract},

		// Boundary: edge set extents, header cardinality, size and representation.
		{name: "boundary lowest address of edge prefix is edge", remote: cfEdgeIPv4Low + ":443", connecting: []string{cfClientIPv4}, authority: peer, want: cfClientIPv4},
		{name: "boundary highest address of edge prefix is edge", remote: cfEdgeIPv4High + ":443", connecting: []string{cfClientIPv4}, authority: peer, want: cfClientIPv4},
		{name: "boundary one below edge prefix is not edge", remote: cfOutsideIPv4BelowLow + ":443", connecting: []string{cfClientIPv4}, authority: peer, want: cfOutsideIPv4BelowLow},
		{name: "boundary one above edge prefix is not edge", remote: cfOutsideIPv4AboveTop + ":443", connecting: []string{cfClientIPv4}, authority: peer, want: cfOutsideIPv4AboveTop},
		{name: "boundary highest address of IPv6 slash 29 is edge", remote: "[" + cfEdgeIPv6Wide29High + "]:443", connecting: []string{cfClientIPv4}, authority: peer, want: cfClientIPv4},
		{name: "boundary one above IPv6 slash 29 is not edge", remote: "[" + cfOutsideIPv6AboveTop + "]:443", connecting: []string{cfClientIPv4}, authority: peer, want: cfOutsideIPv6AboveTop},
		{name: "boundary absent header keeps edge hop", remote: cfEdgeIPv4Low + ":443", authority: peer, want: cfEdgeIPv4Low},
		{name: "boundary empty header keeps edge hop", remote: cfEdgeIPv4Low + ":443", connecting: []string{""}, authority: peer, want: cfEdgeIPv4Low},
		{name: "boundary two header lines keep edge hop", remote: cfEdgeIPv4Low + ":443", connecting: []string{cfClientIPv4, "198.51.100.77"}, authority: peer, want: cfEdgeIPv4Low},
		{name: "boundary identical duplicate header lines keep edge hop", remote: cfEdgeIPv4Low + ":443", connecting: []string{cfClientIPv4, cfClientIPv4}, authority: peer, want: cfEdgeIPv4Low},
		{name: "boundary comma list in one line keeps edge hop", remote: cfEdgeIPv4Low + ":443", connecting: []string{cfClientIPv4 + ", 198.51.100.77"}, authority: peer, want: cfEdgeIPv4Low},
		{name: "boundary address with port keeps edge hop", remote: cfEdgeIPv4Low + ":443", connecting: []string{cfClientIPv4 + ":1234"}, authority: peer, want: cfEdgeIPv4Low},
		{name: "boundary surrounding whitespace keeps edge hop", remote: cfEdgeIPv4Low + ":443", connecting: []string{" " + cfClientIPv4}, authority: peer, want: cfEdgeIPv4Low},
		{name: "boundary bracketed IPv6 keeps edge hop", remote: cfEdgeIPv4Low + ":443", connecting: []string{"[" + cfClientIPv6 + "]"}, authority: peer, want: cfEdgeIPv4Low},
		{name: "boundary zoned IPv6 client keeps edge hop", remote: cfEdgeIPv4Low + ":443", connecting: []string{"fe80::1%en0"}, authority: peer, want: cfEdgeIPv4Low},
		{name: "boundary leading-zero IPv4 client keeps edge hop", remote: cfEdgeIPv4Low + ":443", connecting: []string{"203.000.113.9"}, authority: peer, want: cfEdgeIPv4Low},
		{name: "boundary hostname client keeps edge hop", remote: cfEdgeIPv4Low + ":443", connecting: []string{"client.example"}, authority: peer, want: cfEdgeIPv4Low},
		{name: "boundary oversized header keeps edge hop", remote: cfEdgeIPv4Low + ":443", connecting: []string{oversized}, authority: peer, want: cfEdgeIPv4Low},
		{name: "boundary minimum IPv4 client is admitted", remote: cfEdgeIPv4Low + ":443", connecting: []string{"0.0.0.0"}, authority: peer, want: "0.0.0.0"},
		{name: "boundary maximum IPv6 client is admitted", remote: cfEdgeIPv4Low + ":443", connecting: []string{"ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"}, authority: peer, want: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"},
		{name: "boundary malformed Google chain falls back to non-edge peer", remote: "127.0.0.1:8080", forwarded: []string{"malformed, " + cfGoogleForwardRule}, connecting: []string{cfClientIPv4}, authority: google, want: "127.0.0.1"},
		{name: "boundary Google authority absent chain uses the direct edge peer as hop", remote: cfEdgeIPv4Low + ":443", connecting: []string{cfClientIPv4}, authority: google, want: cfClientIPv4},
		{name: "boundary Google chain with only one member falls back to non-edge peer", remote: "127.0.0.1:8080", forwarded: []string{cfEdgeIPv4Low}, connecting: []string{cfClientIPv4}, authority: google, want: "127.0.0.1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
			request.RemoteAddr = tc.remote
			for _, value := range tc.forwarded {
				request.Header.Add(exchange.StandardHeaderForwardedFor.String(), value)
			}
			for _, value := range tc.connecting {
				request.Header.Add("cf-connecting-ip", value)
			}
			call := exchange.ClientAddressRequest{Call: socketServerCall(t, request), Authority: tc.authority, TrustedProxies: tc.proxies}
			got, gotErr := exchange.ResolveClientAddress(call)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (exchange.ClientAddress{}) {
					t.Fatalf("ResolveClientAddress() = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("ResolveClientAddress() error = %v, want nil", gotErr)
			}
			if want := mustAddress(t, tc.want); got.Address != want {
				t.Fatalf("ResolveClientAddress().Address = %v, want %v", got.Address, want)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("ClientAddress.Validate() error = %v, want nil", err)
			}
		})
	}
}

// FuzzCloudflareClientAddressSemanticClosure fuzzes the CF-Connecting-IP
// representation and edge-hop proof. The oracle is an independent standard
// library projection over the published text list, not the production set.
func FuzzCloudflareClientAddressSemanticClosure(f *testing.F) {
	edge := exchange.ClientAddress{Address: netip.MustParseAddr(cfEdgeIPv4Low)}
	client := exchange.ClientAddress{Address: netip.MustParseAddr(cfClientIPv4)}
	outside := exchange.ClientAddress{Address: netip.MustParseAddr(cfOutsideIPv4AboveTop)}
	for _, value := range []exchange.ClientAddress{edge, client, outside} {
		if err := value.Validate(); err != nil {
			f.Fatalf("typed address seed = %v, want nil", err)
		}
	}
	edgePeer := netip.AddrPortFrom(edge.Address, 443).String()
	f.Add(edgePeer, "", client.Address.String(), uint8(1), false)
	f.Add(netip.AddrPortFrom(outside.Address, 443).String(), "", client.Address.String(), uint8(1), false)
	f.Add("127.0.0.1:8080", edge.Address.String()+","+cfGoogleForwardRule, client.Address.String(), uint8(1), true)
	f.Add("127.0.0.1:8080", outside.Address.String()+","+cfGoogleForwardRule, client.Address.String(), uint8(1), true)
	f.Add(edgePeer, "", client.Address.String(), uint8(2), false)
	f.Add(edgePeer, "", "", uint8(0), false)
	f.Add(edgePeer, "", "::ffff:"+client.Address.String(), uint8(1), false)
	f.Add(edgePeer, "", client.Address.String()+":80", uint8(1), false)
	f.Add(edgePeer, "", strings.Repeat("1", exchange.HeaderValueMaximumBytes+1), uint8(1), false)
	f.Fuzz(func(t *testing.T, remote, forwarded, connecting string, connectingCount uint8, viaGoogle bool) {
		authority := exchange.ClientAddressAuthorityCloudflare
		if viaGoogle {
			authority = exchange.ClientAddressAuthorityGoogleCloudCloudflare
		}
		count := int(connectingCount % 4)
		request := httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
		request.RemoteAddr = remote
		request.Header.Add(exchange.StandardHeaderForwardedFor.String(), forwarded)
		for range count {
			request.Header.Add("Cf-Connecting-Ip", connecting)
		}
		call := exchange.ClientAddressRequest{Call: socketServerCall(t, request), Authority: authority}
		want, wantOK := independentCloudflareAddress(remote, forwarded, connecting, count, viaGoogle)
		got, gotErr := exchange.ResolveClientAddress(call)
		if (gotErr == nil) != wantOK {
			t.Fatalf("ResolveClientAddress admission = %v, want accepted=%t", gotErr, wantOK)
		}
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrExchangeRequest) || got != (exchange.ClientAddress{}) {
				t.Fatalf("ResolveClientAddress(rejected) = (%v, %v), want zero request refusal", got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil || got.Address != want {
			t.Fatalf("ResolveClientAddress(accepted) = (%v, %v), want independent projection %v", got.Address, err, want)
		}
		second, secondErr := exchange.ResolveClientAddress(call)
		if secondErr != nil || second != got {
			t.Fatalf("ResolveClientAddress second projection = (%v, %v), want (%v, nil)", second, secondErr, got)
		}
	})
}

func independentCloudflareAddress(remote, forwarded, connecting string, count int, viaGoogle bool) (netip.Addr, bool) {
	peer, ok := independentParsedAddress(remote)
	if !ok {
		return netip.Addr{}, false
	}
	hop := peer
	if viaGoogle {
		hop = independentGoogleCloudAddress(peer, forwarded)
	}
	if !independentCloudflareEdge(hop) || count != 1 || len(connecting) > exchange.HeaderValueMaximumBytes {
		return hop, true
	}
	client, err := netip.ParseAddr(connecting)
	if err != nil || client.Zone() != "" {
		return hop, true
	}
	return client.Unmap(), true
}

func independentCloudflareEdge(address netip.Addr) bool {
	return slices.ContainsFunc(strings.Split(publishedCloudflareEdgeText, ","), func(raw string) bool {
		return netip.MustParsePrefix(raw).Contains(address)
	})
}
