package exchange_test

import (
	"errors"
	"net"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// Go owns address parsing. This table pressures Exchange's small additional
// policy: explicit host, no wildcard representation, and Go-owned zero-port allocation.
// Equivalent textual spellings form one row, not separate quota members.
func TestListenAddressHostInterpretationMatchesGo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		inputs       []string
		wantWildcard bool
		wantErr      error
		wantText     string
	}{
		{name: "absent host cannot infer all interfaces", inputs: []string{":8080"}, wantWildcard: true, wantErr: core.ErrExchangeContract},
		{name: "IPv4 unspecified host cannot bind all interfaces", inputs: []string{"0.0.0.0:8080"}, wantWildcard: true, wantErr: core.ErrExchangeContract},
		{name: "IPv6 unspecified spellings cannot bind all interfaces", inputs: []string{"[::]:8080", "[0:0:0:0:0:0:0:0]:8080", "[::0.0.0.0]:8080"}, wantWildcard: true, wantErr: core.ErrExchangeContract},
		{name: "IPv4-mapped unspecified host is still a wildcard to Go", inputs: []string{"[::ffff:0.0.0.0]:8080", "[0:0:0:0:0:ffff:0:0]:8080"}, wantWildcard: true, wantErr: core.ErrExchangeContract},
		{name: "a zone cannot make unspecified IPv6 concrete", inputs: []string{"[::%lo0]:8080"}, wantWildcard: true, wantErr: core.ErrExchangeContract},
		{name: "a zone cannot hide an IPv4-mapped wildcard", inputs: []string{"[::ffff:0.0.0.0%lo0]:8080"}, wantWildcard: true, wantErr: core.ErrExchangeContract},
		{name: "zero port delegates allocation to Go without widening the host", inputs: []string{"127.0.0.1:0"}, wantText: "127.0.0.1:0"},
		{name: "zero port cannot turn a wildcard into a concrete host", inputs: []string{"[::%lo0]:0"}, wantWildcard: true, wantErr: core.ErrExchangeContract},
		{name: "one-bit IPv4 neighbor is not a wildcard", inputs: []string{"0.0.0.1:1"}, wantText: "0.0.0.1:1"},
		{name: "one-bit IPv6 neighbor keeps its canonical host", inputs: []string{"[::1]:1", "[::0.0.0.1]:1"}, wantText: "[::1]:1"},
		{name: "mapped one-bit IPv4 neighbor remains admissible", inputs: []string{"[::ffff:0.0.0.1]:65535"}, wantText: "[::ffff:0.0.0.1]:65535"},
		{name: "a concrete IPv6 zone stays part of the admitted address", inputs: []string{"[fe80::1%lo0]:8080"}, wantText: "[fe80::1%lo0]:8080"},
		{name: "a concrete mapped address is not rejected merely for a zone", inputs: []string{"[::ffff:127.0.0.1%lo0]:8080"}, wantText: "[::ffff:127.0.0.1%lo0]:8080"},
		{name: "zero low bits in a non-mapped IPv6 address do not imply wildcard", inputs: []string{"[::ffff:0:0:0]:8080"}, wantText: "[::ffff:0:0:0]:8080"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, input := range tc.inputs {
				// Every fixture is a numeric literal or an absent host. Go
				// resolves these without DNS or any listener/network effect.
				standard, err := net.ResolveTCPAddr("tcp", input)
				if err != nil {
					t.Fatalf("Go TCP fixture resolution(%q) = %v, want nil", input, err)
				}
				gotWildcard := len(standard.IP) == 0 || standard.IP.IsUnspecified()
				if gotWildcard != tc.wantWildcard {
					t.Fatalf("Go TCP wildcard interpretation(%q) = %t, want %t", input, gotWildcard, tc.wantWildcard)
				}
				got, gotErr := exchange.ParseListenAddress(input)
				if !errors.Is(gotErr, tc.wantErr) || got.String() != tc.wantText {
					t.Fatalf("ParseListenAddress(%q) = (%q, %v), want (%q, %v)", input, got.String(), gotErr, tc.wantText, tc.wantErr)
				}
				if tc.wantErr != nil {
					if got != (exchange.ListenAddress{}) {
						t.Fatalf("refused address(%q) = %v, want zero", input, got)
					}
					continue
				}
				if err := got.Validate(); err != nil {
					t.Fatalf("admitted address(%q).Validate() = %v, want nil", input, err)
				}
			}
		})
	}
}
