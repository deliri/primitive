package exchange_test

import (
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestClientAddressAuthorityExhaustsCompleteByteDomain(t *testing.T) {
	t.Parallel()

	for value := range math.MaxUint8 + 1 {
		authority := exchange.ClientAddressAuthority(value)
		wantValid := authority == exchange.ClientAddressAuthorityPeer ||
			authority == exchange.ClientAddressAuthorityTrustedProxy ||
			authority == exchange.ClientAddressAuthorityGoogleCloud
		if got := authority.IsValid(); got != wantValid {
			t.Fatalf("ClientAddressAuthority(%d).IsValid() = %t, want %t", value, got, wantValid)
		}
		gotErr := authority.Validate()
		if !wantValid {
			if !errors.Is(gotErr, core.ErrExchangeContract) || authority.String() != "" {
				t.Fatalf("ClientAddressAuthority(%d) = (%q, %v), want empty and %v", value, authority.String(), gotErr, core.ErrExchangeContract)
			}
			continue
		}
		if gotErr != nil || authority.String() == "" {
			t.Fatalf("ClientAddressAuthority(%d) = (%q, %v), want nonempty and nil", value, authority.String(), gotErr)
		}
		var offWire core.OffWireEnum = authority
		offWire.OffWireEnum()
	}
}

func TestTrustedProxyPrefixesHostileTable(t *testing.T) {
	t.Parallel()

	exactMaximumCount := proxyPrefixText(exchange.TrustedProxyMaximumCount)
	oneBelowMaximumCount := proxyPrefixText(exchange.TrustedProxyMaximumCount - 1)
	oneAboveMaximumCount := proxyPrefixText(exchange.TrustedProxyMaximumCount + 1)
	tests := []struct {
		wantErr   error
		name      string
		raw       string
		wantCount uint8
	}{
		{name: "positive empty text owns a trust-nothing set"},
		{name: "positive one canonical IPv4 prefix", raw: "192.0.2.0/24", wantCount: 1},
		{name: "positive one canonical IPv6 prefix", raw: "2001:db8::/32", wantCount: 1},
		{name: "positive surrounding whitespace is external syntax", raw: " 192.0.2.0/24 ", wantCount: 1},
		{name: "positive member whitespace is external syntax", raw: "192.0.2.0/24, 2001:db8::/32", wantCount: 2},
		{name: "positive host bits are masked", raw: "192.0.2.255/24", wantCount: 1},
		{name: "positive IPv4 and IPv6 coexist", raw: "10.0.0.0/8,2001:db8:1::/48", wantCount: 2},
		{name: "positive IPv4 single host", raw: "203.0.113.9/32", wantCount: 1},
		{name: "positive IPv6 single host", raw: "2001:db8::9/128", wantCount: 1},
		{name: "positive newline surrounding complete value", raw: "\n192.0.2.0/24\n", wantCount: 1},

		{name: "negative leading empty member", raw: ",192.0.2.0/24", wantErr: core.ErrExchangeContract},
		{name: "negative trailing empty member", raw: "192.0.2.0/24,", wantErr: core.ErrExchangeContract},
		{name: "negative middle empty member", raw: "192.0.2.0/24,,2001:db8::/32", wantErr: core.ErrExchangeContract},
		{name: "negative bare IPv4 is not a prefix", raw: "192.0.2.1", wantErr: core.ErrExchangeContract},
		{name: "negative bare IPv6 is not a prefix", raw: "2001:db8::1", wantErr: core.ErrExchangeContract},
		{name: "negative hostname is not a prefix", raw: "proxy.example", wantErr: core.ErrExchangeContract},
		{name: "negative IPv4 prefix above width", raw: "192.0.2.0/33", wantErr: core.ErrExchangeContract},
		{name: "negative IPv6 prefix above width", raw: "2001:db8::/129", wantErr: core.ErrExchangeContract},
		{name: "negative duplicate canonical prefix", raw: "192.0.2.0/24,192.0.2.0/24", wantErr: core.ErrExchangeContract},
		{name: "negative duplicate after masking", raw: "192.0.2.1/24,192.0.2.255/24", wantErr: core.ErrExchangeContract},

		{name: "boundary one prefix below maximum count", raw: oneBelowMaximumCount, wantCount: exchange.TrustedProxyMaximumCount - 1},
		{name: "boundary exact maximum prefix count", raw: exactMaximumCount, wantCount: exchange.TrustedProxyMaximumCount},
		{name: "boundary one prefix above maximum count", raw: oneAboveMaximumCount, wantErr: core.ErrExchangeContract},
		{name: "boundary empty text one byte", raw: " ", wantCount: 0},
		{name: "boundary exact maximum blank bytes", raw: strings.Repeat(" ", exchange.HeaderValueMaximumBytes), wantCount: 0},
		{name: "boundary one above maximum bytes", raw: strings.Repeat(" ", exchange.HeaderValueMaximumBytes+1), wantErr: core.ErrExchangeContract},
		{name: "boundary IPv4 minimum address and width", raw: "0.0.0.0/0", wantCount: 1},
		{name: "boundary IPv4 maximum address and width", raw: "255.255.255.255/32", wantCount: 1},
		{name: "boundary IPv6 minimum address and width", raw: "::/0", wantCount: 1},
		{name: "boundary IPv6 maximum address and width", raw: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff/128", wantCount: 1},
		{name: "boundary mapped IPv4 prefix canonicalizes", raw: "::ffff:192.0.2.1/128", wantCount: 1},
		{name: "boundary IPv6 zone is refused", raw: "fe80::1%en0/128", wantErr: core.ErrExchangeContract},
		{name: "boundary missing prefix width is refused", raw: "192.0.2.0/", wantErr: core.ErrExchangeContract},
		{name: "boundary negative prefix width is refused", raw: "192.0.2.0/-1", wantErr: core.ErrExchangeContract},
		{name: "boundary leading zero IPv4 is refused", raw: "192.0.002.0/24", wantErr: core.ErrExchangeContract},
		{name: "boundary NUL suffix is refused", raw: "192.0.2.0/24\x00", wantErr: core.ErrExchangeContract},
		{name: "boundary tab around complete value", raw: "\t192.0.2.0/24\t", wantCount: 1},
		{name: "boundary carriage return around complete value", raw: "\r192.0.2.0/24\r", wantCount: 1},
		{name: "boundary two disjoint adjacent prefixes", raw: "192.0.2.0/25,192.0.2.128/25", wantCount: 2},
		{name: "boundary repeated family with distinct prefixes", raw: "192.0.2.0/24,198.51.100.0/24,203.0.113.0/24", wantCount: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := exchange.ParseTrustedProxyPrefixes(tc.raw)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (exchange.TrustedProxyPrefixes{}) {
					t.Fatalf("ParseTrustedProxyPrefixes() = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("ParseTrustedProxyPrefixes() error = %v, want nil", gotErr)
			}
			if gotErr = got.Validate(); gotErr != nil {
				t.Fatalf("TrustedProxyPrefixes.Validate() error = %v, want nil", gotErr)
			}
			if gotCount := got.Count(); gotCount != tc.wantCount {
				t.Fatalf("TrustedProxyPrefixes.Count() = %d, want %d", gotCount, tc.wantCount)
			}
		})
	}
}

func TestClientAddressResolutionLayerTriad(t *testing.T) {
	t.Parallel()

	trusted := mustTrustedProxyPrefixes(t, "10.0.0.0/8,2001:db8:ffff::/48")
	exactValueCount := repeatedForwardedValues(exchange.HeaderValueMaximumCount-1, "203.0.113.9, 198.51.100.2")
	aboveValueCount := repeatedForwardedValues(exchange.HeaderValueMaximumCount, "203.0.113.9, 198.51.100.2")
	finalPair := "203.0.113.9, 198.51.100.2"
	exactByteValue := strings.Repeat("x", exchange.HeaderValueMaximumBytes-len(finalPair)-1) + "," + finalPair
	tests := []struct {
		want          netip.Addr
		wantErr       error
		name          string
		remote        string
		proxies       exchange.TrustedProxyPrefixes
		forwarded     []string
		authority     exchange.ClientAddressAuthority
		absentRequest bool
	}{
		{name: "positive peer IPv4 with port", remote: "192.0.2.10:443", authority: exchange.ClientAddressAuthorityPeer, want: mustAddress(t, "192.0.2.10")},
		{name: "positive peer bracketed IPv6 with port", remote: "[2001:db8::10]:443", authority: exchange.ClientAddressAuthorityPeer, want: mustAddress(t, "2001:db8::10")},
		{name: "positive trusted proxy exposes IPv4 client", remote: "10.1.2.3:80", forwarded: []string{"203.0.113.10, 10.9.8.7"}, proxies: trusted, authority: exchange.ClientAddressAuthorityTrustedProxy, want: mustAddress(t, "203.0.113.10")},
		{name: "positive trusted proxy exposes IPv6 client", remote: "[2001:db8:ffff::1]:80", forwarded: []string{"2001:db8::10, 2001:db8:ffff::2"}, proxies: trusted, authority: exchange.ClientAddressAuthorityTrustedProxy, want: mustAddress(t, "2001:db8::10")},
		{name: "positive trusted proxy walks consecutive trusted hops", remote: "10.1.2.3:80", forwarded: []string{"198.51.100.10, 10.8.7.6, 10.9.8.7"}, proxies: trusted, authority: exchange.ClientAddressAuthorityTrustedProxy, want: mustAddress(t, "198.51.100.10")},
		{name: "positive Google Cloud pair exposes packet source", remote: "127.0.0.1:8080", forwarded: []string{"142.126.228.37, 198.51.100.2"}, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "142.126.228.37")},
		{name: "positive Google Cloud ignores supplied spoof prefix", remote: "127.0.0.1:8080", forwarded: []string{"203.0.113.250, 142.126.228.37, 198.51.100.2"}, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "142.126.228.37")},
		{name: "positive Google Cloud IPv6 pair", remote: "127.0.0.1:8080", forwarded: []string{"2001:db8::20, 2001:db8:ffff::20"}, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "2001:db8::20")},
		{name: "positive Google Cloud final pair crosses header lines", remote: "127.0.0.1:8080", forwarded: []string{"unverified", "203.0.113.20", "198.51.100.2"}, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "203.0.113.20")},
		{name: "positive mapped Google Cloud client canonicalizes", remote: "127.0.0.1:8080", forwarded: []string{"::ffff:203.0.113.20, 198.51.100.2"}, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "203.0.113.20")},

		{name: "positive peer ignores one forged member", remote: "192.0.2.40:80", forwarded: []string{"203.0.113.40"}, authority: exchange.ClientAddressAuthorityPeer, want: mustAddress(t, "192.0.2.40")},
		{name: "positive peer ignores forged Google Cloud pair", remote: "192.0.2.41:80", forwarded: []string{"203.0.113.41, 198.51.100.2"}, authority: exchange.ClientAddressAuthorityPeer, want: mustAddress(t, "192.0.2.41")},
		{name: "negative trusted authority requires prefixes", remote: "192.0.2.42:80", authority: exchange.ClientAddressAuthorityTrustedProxy, wantErr: core.ErrExchangeContract},
		{name: "negative peer authority refuses unrelated prefixes", remote: "192.0.2.43:80", proxies: trusted, authority: exchange.ClientAddressAuthorityPeer, wantErr: core.ErrExchangeContract},
		{name: "negative Google Cloud authority refuses unrelated prefixes", remote: "127.0.0.1:8080", proxies: trusted, authority: exchange.ClientAddressAuthorityGoogleCloud, wantErr: core.ErrExchangeContract},
		{name: "negative unknown authority is rejected", remote: "192.0.2.44:80", authority: exchange.ClientAddressAuthorityGoogleCloud + 1, wantErr: core.ErrExchangeContract},
		{name: "negative absent request is rejected", remote: "192.0.2.45:80", authority: exchange.ClientAddressAuthorityPeer, wantErr: core.ErrExchangeRequest, absentRequest: true},
		{name: "negative malformed direct peer is rejected", remote: "not-an-address", authority: exchange.ClientAddressAuthorityPeer, wantErr: core.ErrExchangeRequest},
		{name: "negative direct peer with zone is rejected", remote: "fe80::1%en0", authority: exchange.ClientAddressAuthorityPeer, wantErr: core.ErrExchangeRequest},
		{name: "negative empty direct peer is rejected", authority: exchange.ClientAddressAuthorityPeer, wantErr: core.ErrExchangeRequest},
		{name: "negative direct peer with nonnumeric port is rejected", remote: "192.0.2.46:not-a-port", authority: exchange.ClientAddressAuthorityPeer, wantErr: core.ErrExchangeRequest},
		{name: "negative bracketed direct peer without port is rejected", remote: "[2001:db8::46]", authority: exchange.ClientAddressAuthorityPeer, wantErr: core.ErrExchangeRequest},

		{name: "boundary untrusted direct peer ignores trusted-mode chain", remote: "192.0.2.50:80", forwarded: []string{"203.0.113.50, 10.9.8.7"}, proxies: trusted, authority: exchange.ClientAddressAuthorityTrustedProxy, want: mustAddress(t, "192.0.2.50")},
		{name: "boundary trusted mode absent header falls back to peer", remote: "10.1.2.3:80", proxies: trusted, authority: exchange.ClientAddressAuthorityTrustedProxy, want: mustAddress(t, "10.1.2.3")},
		{name: "boundary trusted mode malformed rightmost member falls back to peer", remote: "10.1.2.3:80", forwarded: []string{"203.0.113.50, malformed"}, proxies: trusted, authority: exchange.ClientAddressAuthorityTrustedProxy, want: mustAddress(t, "10.1.2.3")},
		{name: "boundary trusted mode all hops trusted falls back to peer", remote: "10.1.2.3:80", forwarded: []string{"10.7.6.5, 10.9.8.7"}, proxies: trusted, authority: exchange.ClientAddressAuthorityTrustedProxy, want: mustAddress(t, "10.1.2.3")},
		{name: "boundary Google Cloud absent header falls back to peer", remote: "127.0.0.1:8080", authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "127.0.0.1")},
		{name: "boundary Google Cloud one member falls back to peer", remote: "127.0.0.1:8080", forwarded: []string{"203.0.113.60"}, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "127.0.0.1")},
		{name: "boundary Google Cloud malformed final member falls back to peer", remote: "127.0.0.1:8080", forwarded: []string{"203.0.113.61, malformed"}, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "127.0.0.1")},
		{name: "boundary Google Cloud malformed penultimate member falls back to peer", remote: "127.0.0.1:8080", forwarded: []string{"malformed, 198.51.100.2"}, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "127.0.0.1")},
		{name: "boundary Google Cloud trims final pair whitespace", remote: "127.0.0.1:8080", forwarded: []string{" 203.0.113.62 , 198.51.100.2 "}, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "203.0.113.62")},
		{name: "boundary raw direct IPv4 without port", remote: "203.0.113.63", authority: exchange.ClientAddressAuthorityPeer, want: mustAddress(t, "203.0.113.63")},
		{name: "boundary raw direct IPv6 without port", remote: "2001:db8::63", authority: exchange.ClientAddressAuthorityPeer, want: mustAddress(t, "2001:db8::63")},
		{name: "boundary mapped direct IPv4 canonicalizes", remote: "[::ffff:203.0.113.64]:80", authority: exchange.ClientAddressAuthorityPeer, want: mustAddress(t, "203.0.113.64")},
		{name: "boundary Google Cloud minimum IPv4 client", remote: "127.0.0.1:8080", forwarded: []string{"0.0.0.0, 198.51.100.2"}, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "0.0.0.0")},
		{name: "boundary Google Cloud maximum IPv4 client", remote: "127.0.0.1:8080", forwarded: []string{"255.255.255.255, 198.51.100.2"}, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "255.255.255.255")},
		{name: "boundary exact maximum header bytes accepts final pair", remote: "127.0.0.1:8080", forwarded: []string{exactByteValue}, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "203.0.113.9")},
		{name: "boundary one above maximum header bytes falls back", remote: "127.0.0.1:8080", forwarded: []string{exactByteValue + "x"}, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "127.0.0.1")},
		{name: "boundary exact maximum header value count accepts final pair", remote: "127.0.0.1:8080", forwarded: exactValueCount, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "203.0.113.9")},
		{name: "boundary one above maximum header value count falls back", remote: "127.0.0.1:8080", forwarded: aboveValueCount, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "127.0.0.1")},
		{name: "boundary Google Cloud final client zone falls back", remote: "127.0.0.1:8080", forwarded: []string{"fe80::1%en0, 198.51.100.2"}, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "127.0.0.1")},
		{name: "boundary Google Cloud forwarding rule zone falls back", remote: "127.0.0.1:8080", forwarded: []string{"203.0.113.70, fe80::1%en0"}, authority: exchange.ClientAddressAuthorityGoogleCloud, want: mustAddress(t, "127.0.0.1")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			call := clientAddressCall(tc.remote, tc.forwarded, tc.authority, tc.proxies)
			if tc.absentRequest {
				call.Request = nil
			}
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
			if gotErr = got.Validate(); gotErr != nil {
				t.Fatalf("ClientAddress.Validate() error = %v, want nil", gotErr)
			}
			if got.Address != tc.want {
				t.Fatalf("ResolveClientAddress().Address = %v, want %v", got.Address, tc.want)
			}
		})
	}
}

func FuzzTrustedProxyPrefixesSemanticClosure(f *testing.F) {
	for _, raw := range []string{"", "192.0.2.0/24", "2001:db8::/32", "192.0.2.0/24,2001:db8::/32"} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		got, gotErr := exchange.ParseTrustedProxyPrefixes(raw)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrExchangeContract) || got != (exchange.TrustedProxyPrefixes{}) {
				t.Fatalf("ParseTrustedProxyPrefixes(rejected) = (%v, %v), want zero and %v", got, gotErr, core.ErrExchangeContract)
			}
			return
		}
		if gotErr = got.Validate(); gotErr != nil || got.Count() > exchange.TrustedProxyMaximumCount {
			t.Fatalf("ParseTrustedProxyPrefixes(accepted) = (%v, %v), want valid count <= %d", got, gotErr, exchange.TrustedProxyMaximumCount)
		}
		canonical := got.String()
		roundTrip, roundTripErr := exchange.ParseTrustedProxyPrefixes(canonical)
		if roundTripErr != nil || roundTrip != got {
			t.Fatalf("TrustedProxyPrefixes canonical round trip = (%v, %v), want (%v, nil)", roundTrip, roundTripErr, got)
		}
		if second := roundTrip.String(); second != canonical {
			t.Fatalf("TrustedProxyPrefixes second canonical projection = %q, want %q", second, canonical)
		}
	})
}

func FuzzClientAddressResolutionSemanticClosure(f *testing.F) {
	f.Add("127.0.0.1:8080", "203.0.113.9, 198.51.100.2", uint8(exchange.ClientAddressAuthorityGoogleCloud), false)
	f.Add("192.0.2.1:80", "203.0.113.9", uint8(exchange.ClientAddressAuthorityPeer), false)
	f.Add("10.0.0.1:80", "203.0.113.9", uint8(exchange.ClientAddressAuthorityTrustedProxy), true)
	f.Fuzz(func(t *testing.T, remote, forwarded string, rawAuthority uint8, useTrustedPrefixes bool) {
		authority := exchange.ClientAddressAuthority(rawAuthority)
		var proxies exchange.TrustedProxyPrefixes
		if useTrustedPrefixes {
			proxies = mustTrustedProxyPrefixes(t, "10.0.0.0/8,2001:db8:ffff::/48")
		}
		call := clientAddressCall(remote, []string{forwarded}, authority, proxies)
		got, gotErr := exchange.ResolveClientAddress(call)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrExchangeContract) && !errors.Is(gotErr, core.ErrExchangeRequest) {
				t.Fatalf("ResolveClientAddress(rejected) error = %v, want Exchange identity", gotErr)
			}
			if got != (exchange.ClientAddress{}) {
				t.Fatalf("ResolveClientAddress(rejected) result = %v, want zero", got)
			}
			return
		}
		if gotErr = got.Validate(); gotErr != nil || !got.Address.IsValid() || got.Address.Zone() != "" || got.Address != got.Address.Unmap() {
			t.Fatalf("ResolveClientAddress(accepted) = (%v, %v), want valid canonical address", got, gotErr)
		}
		want, wantOK := independentClientAddress(remote, forwarded, authority)
		if !wantOK || got.Address != want {
			t.Fatalf("ResolveClientAddress(accepted).Address = %v, want independent standard-library projection %v (valid=%t)", got.Address, want, wantOK)
		}
		second, secondErr := exchange.ResolveClientAddress(call)
		if secondErr != nil || second != got {
			t.Fatalf("ResolveClientAddress second projection = (%v, %v), want (%v, nil)", second, secondErr, got)
		}
	})
}

func independentClientAddress(remote, forwarded string, authority exchange.ClientAddressAuthority) (netip.Addr, bool) {
	peer, ok := independentParsedAddress(remote)
	if !ok {
		return netip.Addr{}, false
	}
	switch authority {
	case exchange.ClientAddressAuthorityPeer:
		return peer, true
	case exchange.ClientAddressAuthorityGoogleCloud:
		return independentGoogleCloudAddress(peer, forwarded), true
	case exchange.ClientAddressAuthorityTrustedProxy:
		return independentTrustedProxyAddress(peer, forwarded), true
	default:
		return netip.Addr{}, false
	}
}

func independentGoogleCloudAddress(peer netip.Addr, forwarded string) netip.Addr {
	if len(forwarded) > exchange.HeaderValueMaximumBytes {
		return peer
	}
	members := strings.Split(forwarded, ",")
	if len(members) < 2 {
		return peer
	}
	if _, ok := independentParsedAddress(strings.TrimSpace(members[len(members)-1])); !ok {
		return peer
	}
	client, ok := independentParsedAddress(strings.TrimSpace(members[len(members)-2]))
	if !ok {
		return peer
	}
	return client
}

func independentTrustedProxyAddress(peer netip.Addr, forwarded string) netip.Addr {
	if len(forwarded) > exchange.HeaderValueMaximumBytes || !independentTrustedProxyContains(peer) {
		return peer
	}
	members := strings.Split(forwarded, ",")
	for index := len(members) - 1; index >= 0; index-- {
		candidate, ok := independentParsedAddress(strings.TrimSpace(members[index]))
		if !ok {
			return peer
		}
		if !independentTrustedProxyContains(candidate) {
			return candidate
		}
	}
	return peer
}

func independentTrustedProxyContains(address netip.Addr) bool {
	return netip.MustParsePrefix("10.0.0.0/8").Contains(address) ||
		netip.MustParsePrefix("2001:db8:ffff::/48").Contains(address)
}

func independentParsedAddress(raw string) (netip.Addr, bool) {
	if addressPort, err := netip.ParseAddrPort(raw); err == nil {
		return independentCanonicalAddress(addressPort.Addr())
	}
	address, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Addr{}, false
	}
	return independentCanonicalAddress(address)
}

func independentCanonicalAddress(address netip.Addr) (netip.Addr, bool) {
	if !address.IsValid() || address.Zone() != "" {
		return netip.Addr{}, false
	}
	return address.Unmap(), true
}

func clientAddressCall(remote string, forwarded []string, authority exchange.ClientAddressAuthority, proxies exchange.TrustedProxyPrefixes) exchange.ClientAddressRequest {
	request := httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
	request.RemoteAddr = remote
	request.Header.Del(exchange.StandardHeaderForwardedFor.String())
	for _, value := range forwarded {
		request.Header.Add(exchange.StandardHeaderForwardedFor.String(), value)
	}
	return exchange.ClientAddressRequest{Request: request, Authority: authority, TrustedProxies: proxies}
}

func mustTrustedProxyPrefixes(t *testing.T, raw string) exchange.TrustedProxyPrefixes {
	t.Helper()
	got, err := exchange.ParseTrustedProxyPrefixes(raw)
	if err != nil {
		t.Fatalf("ParseTrustedProxyPrefixes(%q) error = %v, want nil", raw, err)
	}
	return got
}

func mustAddress(t *testing.T, raw string) netip.Addr {
	t.Helper()
	got, err := netip.ParseAddr(raw)
	if err != nil {
		t.Fatalf("netip.ParseAddr(%q) error = %v, want nil", raw, err)
	}
	return got.Unmap()
}

func proxyPrefixText(count uint8) string {
	parts := make([]string, 0, count)
	for index := uint8(0); index < count; index++ {
		parts = append(parts, "10."+strconv.Itoa(int(index))+".0.0/16")
	}
	return strings.Join(parts, ",")
}

func repeatedForwardedValues(prefixCount int, final string) []string {
	values := make([]string, 0, prefixCount+1)
	for range prefixCount {
		values = append(values, "unverified")
	}
	return append(values, final)
}

func TestClientAddressResultRejectsImpossibleValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		got  exchange.ClientAddress
		name string
	}{
		{name: "zero address", got: exchange.ClientAddress{}},
		{name: "zoned address", got: exchange.ClientAddress{Address: netip.MustParseAddr("fe80::1%en0")}},
		{name: "mapped address is not canonical", got: exchange.ClientAddress{Address: netip.MustParseAddr("::ffff:192.0.2.1")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if gotErr := tc.got.Validate(); !errors.Is(gotErr, core.ErrExchangeContract) {
				t.Fatalf("ClientAddress.Validate() error = %v, want %v", gotErr, core.ErrExchangeContract)
			}
		})
	}
}
