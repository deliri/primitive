package exchange

import (
	"errors"
	"net/netip"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAdmitResolvedAddressesNormalizesAndBoundsLayerTriad(t *testing.T) {
	t.Parallel()
	distinct := func(count int) []netip.Addr {
		addresses := make([]netip.Addr, 0, count)
		for index := range count {
			addresses = append(addresses, netip.AddrFrom4([4]byte{192, 0, 2, byte(index + 1)}))
		}
		return addresses
	}
	parse := func(values ...string) []netip.Addr {
		addresses := make([]netip.Addr, 0, len(values))
		for _, value := range values {
			addresses = append(addresses, netip.MustParseAddr(value))
		}
		return addresses
	}
	for _, row := range []struct {
		name    string
		input   []netip.Addr
		want    []netip.Addr
		wantErr error
	}{
		{name: "positive mapped IPv4 is unmapped", input: parse("::ffff:203.0.113.7"), want: parse("203.0.113.7")},
		{name: "positive zone is removed", input: parse("fe80::1%en0"), want: parse("fe80::1")},
		{name: "positive mapped duplicate and zoned duplicate collapse in resolver order", input: parse("2001:db8::1", "198.51.100.1", "::ffff:198.51.100.1", "2001:db8::1%en0"), want: parse("2001:db8::1", "198.51.100.1")},
		{name: "positive exactly sixteen distinct addresses", input: distinct(16), want: distinct(16)},
		{name: "positive seventeen answers with one duplicate admit sixteen", input: append(distinct(16), distinct(1)...), want: distinct(16)},
		{name: "negative seventeen distinct addresses are refused not clipped", input: distinct(17), wantErr: core.ErrExchangeResponse},
		{name: "negative invalid resolver address is refused", input: []netip.Addr{{}}, wantErr: core.ErrExchangeResponse},
		{name: "neutral empty answer is refused", wantErr: core.ErrExchangeResponse},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := admitResolvedAddresses(slices.Clone(row.input))
			if !errors.Is(err, row.wantErr) || !slices.Equal(got, row.want) || (row.wantErr != nil && got != nil) {
				t.Fatalf("admitResolvedAddresses(%v) = (%v,%v), want (%v,%v)", row.input, got, err, row.want, row.wantErr)
			}
		})
	}
}
