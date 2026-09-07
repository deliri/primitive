package exchange

import (
	"errors"
	"net"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzTrustedProxyPrefixesSemanticClosure(f *testing.F) {
	// Construct the actual nominal carrier, validate it, and use its real
	// projection. A broken Parse cannot prevent a valid seed reaching the callback.
	for _, count := range []uint8{0, 1, TrustedProxyMaximumCount - 1, TrustedProxyMaximumCount} {
		var seed TrustedProxyPrefixes
		seed.count = count
		for i := range count {
			seed.prefixes[i] = netip.PrefixFrom(netip.AddrFrom4([4]byte{10, i, 0, 0}), 16)
		}
		if err := seed.Validate(); err != nil {
			f.Fatalf("typed prefix seed = %v, want nil", err)
		}
		f.Add(seed.String())
	}
	for _, raw := range []string{",", "192.0.2.1/24,192.0.2.255/24", "::ffff:10.1.1.1/96", "::ffff:10.1.1.1/97", "::ffff:10.1.1.1/128", "::/0", "fe80::1%lo0/128", strings.Repeat(" ", HeaderValueMaximumBytes-1), strings.Repeat(" ", HeaderValueMaximumBytes), strings.Repeat(" ", HeaderValueMaximumBytes+1)} {
		f.Add(raw)
	}
	var maximum TrustedProxyPrefixes
	maximum.count = TrustedProxyMaximumCount
	for i := range maximum.count {
		maximum.prefixes[i] = netip.PrefixFrom(netip.AddrFrom4([4]byte{10, i, 0, 0}), 16)
	}
	f.Add(maximum.String() + ",192.0.2.0/24")
	f.Fuzz(func(t *testing.T, raw string) {
		want, accepted := oracleTrustedPrefixes(raw)
		got, err := ParseTrustedProxyPrefixes(raw)
		if (err == nil) != accepted {
			t.Fatalf("prefix admission = %v, want accepted=%t", err, accepted)
		}
		if !accepted {
			if !errors.Is(err, core.ErrExchangeContract) || got != (TrustedProxyPrefixes{}) {
				t.Fatalf("prefix refusal = (%v,%v), want zero and contract refusal", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("admitted prefix validation = %v, want nil", err)
		}
		if int(got.Count()) != len(want) {
			t.Fatalf("prefix count = %d, want %d", got.Count(), len(want))
		}
		parts := make([]string, len(want))
		for i, prefix := range want {
			if got.prefixes[i] != prefix {
				t.Fatalf("prefix[%d] = %v, want exact %v", i, got.prefixes[i], prefix)
			}
			parts[i] = prefix.String()
		}
		canonical := strings.Join(parts, ",")
		if got.String() != canonical {
			t.Fatalf("prefix projection = %q, want %q", got.String(), canonical)
		}
		again, err := ParseTrustedProxyPrefixes(canonical)
		if err != nil || again != got || again.String() != canonical {
			t.Fatalf("canonical closure = (%v,%v), want exact admitted value", again, err)
		}
	})
}

// A bounded slice model uses Go's prefix grammar and masking, independent of
// the production fixed carrier, incremental cursor and duplicate checks.
func oracleTrustedPrefixes(raw string) ([]netip.Prefix, bool) {
	if len(raw) > HeaderValueMaximumBytes {
		return nil, false
	}
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	members := strings.Split(strings.TrimSpace(raw), ",")
	if len(members) > TrustedProxyMaximumCount {
		return nil, false
	}
	want := make([]netip.Prefix, 0, len(members))
	for _, member := range members {
		p, err := netip.ParsePrefix(strings.TrimSpace(member))
		if err != nil || p.Addr().Zone() != "" {
			return nil, false
		}
		if p.Addr().Is4In6() {
			bits := p.Bits() - (net.IPv6len-net.IPv4len)*8
			if bits <= 0 {
				return nil, false
			}
			p = netip.PrefixFrom(p.Addr().Unmap(), bits)
		}
		p = p.Masked()
		if slices.Contains(want, p) {
			return nil, false
		}
		want = append(want, p)
	}
	return want, true
}
