package exchange

import (
	"net/http"
	"net/netip"
)

// CloudflareEdgeRevision identifies the Cloudflare edge address set compiled
// into this release. It is the provider-published list observed on 2026-10-16
// at https://api.cloudflare.com/client/v4/ips (etag
// 38f79d050aa027e3be3865e495dcc9bc), byte-identical to
// https://www.cloudflare.com/ips-v4 and https://www.cloudflare.com/ips-v6.
// The set is never fetched at runtime; a provider change is a new release.
const CloudflareEdgeRevision = "2026-10-16/38f79d050aa027e3be3865e495dcc9bc"

const (
	// CloudflareEdgeIPv4PrefixCount is the published IPv4 prefix count.
	CloudflareEdgeIPv4PrefixCount = 15
	// CloudflareEdgeIPv6PrefixCount is the published IPv6 prefix count.
	CloudflareEdgeIPv6PrefixCount = 7

	cloudflareConnectingIPHeaderText = "Cf-Connecting-Ip"
)

// cloudflareEdgeIPv4 is constructed from typed octets so no text parser can
// fail or drift at process start. The fixed array length is the count fact.
func cloudflareEdgeIPv4() [CloudflareEdgeIPv4PrefixCount]netip.Prefix {
	return [...]netip.Prefix{
		netip.PrefixFrom(netip.AddrFrom4([4]byte{173, 245, 48, 0}), 20),
		netip.PrefixFrom(netip.AddrFrom4([4]byte{103, 21, 244, 0}), 22),
		netip.PrefixFrom(netip.AddrFrom4([4]byte{103, 22, 200, 0}), 22),
		netip.PrefixFrom(netip.AddrFrom4([4]byte{103, 31, 4, 0}), 22),
		netip.PrefixFrom(netip.AddrFrom4([4]byte{141, 101, 64, 0}), 18),
		netip.PrefixFrom(netip.AddrFrom4([4]byte{108, 162, 192, 0}), 18),
		netip.PrefixFrom(netip.AddrFrom4([4]byte{190, 93, 240, 0}), 20),
		netip.PrefixFrom(netip.AddrFrom4([4]byte{188, 114, 96, 0}), 20),
		netip.PrefixFrom(netip.AddrFrom4([4]byte{197, 234, 240, 0}), 22),
		netip.PrefixFrom(netip.AddrFrom4([4]byte{198, 41, 128, 0}), 17),
		netip.PrefixFrom(netip.AddrFrom4([4]byte{162, 158, 0, 0}), 15),
		netip.PrefixFrom(netip.AddrFrom4([4]byte{104, 16, 0, 0}), 13),
		netip.PrefixFrom(netip.AddrFrom4([4]byte{104, 24, 0, 0}), 14),
		netip.PrefixFrom(netip.AddrFrom4([4]byte{172, 64, 0, 0}), 13),
		netip.PrefixFrom(netip.AddrFrom4([4]byte{131, 0, 72, 0}), 22),
	}
}

func cloudflareEdgeIPv6() [CloudflareEdgeIPv6PrefixCount]netip.Prefix {
	return [...]netip.Prefix{
		netip.PrefixFrom(netip.AddrFrom16([16]byte{0x24, 0x00, 0xcb, 0x00}), 32),
		netip.PrefixFrom(netip.AddrFrom16([16]byte{0x26, 0x06, 0x47, 0x00}), 32),
		netip.PrefixFrom(netip.AddrFrom16([16]byte{0x28, 0x03, 0xf8, 0x00}), 32),
		netip.PrefixFrom(netip.AddrFrom16([16]byte{0x24, 0x05, 0xb5, 0x00}), 32),
		netip.PrefixFrom(netip.AddrFrom16([16]byte{0x24, 0x05, 0x81, 0x00}), 32),
		netip.PrefixFrom(netip.AddrFrom16([16]byte{0x2a, 0x06, 0x98, 0xc0}), 29),
		netip.PrefixFrom(netip.AddrFrom16([16]byte{0x2c, 0x0f, 0xf2, 0x48}), 32),
	}
}

// CloudflareEdgePrefixes projects the compiled edge set into the canonical
// fixed-capacity prefix carrier so callers can render or compare it (for
// example, an origin firewall allow-list) without a second copy of the facts.
func CloudflareEdgePrefixes() (TrustedProxyPrefixes, error) {
	var prefixes TrustedProxyPrefixes
	for _, prefix := range cloudflareEdgeIPv4() {
		prefixes.prefixes[prefixes.count] = prefix
		prefixes.count++
	}
	for _, prefix := range cloudflareEdgeIPv6() {
		prefixes.prefixes[prefixes.count] = prefix
		prefixes.count++
	}
	if err := prefixes.Validate(); err != nil {
		return TrustedProxyPrefixes{}, err
	}
	return prefixes, nil
}

func cloudflareEdgeContains(address netip.Addr) bool {
	for _, prefix := range cloudflareEdgeIPv4() {
		if prefix.Contains(address) {
			return true
		}
	}
	for _, prefix := range cloudflareEdgeIPv6() {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

// resolveCloudflareAddress admits the edge-declared client only when the hop
// that delivered the request is itself a Cloudflare edge address. The header
// must occur exactly once and hold exactly one bare address; ports, lists,
// whitespace, zones and oversized values leave the proven hop in place.
func resolveCloudflareAddress(request *http.Request, hop netip.Addr) netip.Addr {
	if !cloudflareEdgeContains(hop) {
		return hop
	}
	values := request.Header.Values(cloudflareConnectingIPHeaderText)
	if len(values) != 1 || len(values[0]) > HeaderValueMaximumBytes {
		return hop
	}
	address, err := netip.ParseAddr(values[0])
	if err != nil {
		return hop
	}
	canonical, err := canonicalClientAddress(address)
	if err != nil {
		return hop
	}
	return canonical
}
