package core

import (
	"errors"
	"testing"
)

func TestIPNetworkParserHostileCanonicalBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		wire string
		want string
	}{
		{name: "IPv4 unspecified exact address", wire: "0.0.0.0", want: "0.0.0.0"},
		{name: "IPv4 loopback exact address", wire: "127.0.0.1", want: "127.0.0.1"},
		{name: "IPv4 maximum exact address", wire: "255.255.255.255", want: "255.255.255.255"},
		{name: "IPv6 unspecified exact address", wire: "::", want: "::"},
		{name: "IPv6 loopback exact address", wire: "::1", want: "::1"},
		{name: "IPv6 maximum exact address", wire: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", want: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"},
		{name: "IPv4 whole address space", wire: "0.0.0.0/0", want: "0.0.0.0/0"},
		{name: "IPv4 interior masked prefix", wire: "10.20.0.0/16", want: "10.20.0.0/16"},
		{name: "IPv4 full width prefix projects as address", wire: "192.0.2.1/32", want: "192.0.2.1"},
		{name: "IPv6 whole address space", wire: "::/0", want: "::/0"},
		{name: "IPv6 interior masked prefix", wire: "2001:db8::/32", want: "2001:db8::/32"},
		{name: "IPv6 full width prefix projects as address", wire: "2001:db8::1/128", want: "2001:db8::1"},
		{name: "empty input", wire: ""},
		{name: "ASCII space prefix", wire: " 127.0.0.1"},
		{name: "ASCII space suffix", wire: "127.0.0.1 "},
		{name: "IPv4 leading zero alternate spelling", wire: "127.000.0.1"},
		{name: "IPv4 truncated address", wire: "127.0.0"},
		{name: "IPv4 octet above maximum", wire: "256.0.0.1"},
		{name: "IPv4 unmasked prefix", wire: "10.20.30.40/16"},
		{name: "IPv4 prefix one above maximum width", wire: "10.0.0.0/33"},
		{name: "IPv6 uppercase alternate spelling", wire: "2001:DB8::1"},
		{name: "IPv6 expanded alternate spelling", wire: "2001:0db8:0:0:0:0:0:1"},
		{name: "IPv6 mapped IPv4 exact address", wire: "::ffff:192.0.2.1"},
		{name: "IPv6 mapped IPv4 prefix", wire: "::ffff:192.0.2.0/120"},
		{name: "IPv6 zone identifier", wire: "fe80::1%en0"},
		{name: "IPv6 unmasked prefix", wire: "2001:db8::1/64"},
		{name: "IPv6 prefix one above maximum width", wire: "2001:db8::/129"},
		{name: "CIDR missing prefix width", wire: "192.0.2.0/"},
		{name: "CIDR duplicated separator", wire: "192.0.2.0//24"},
		{name: "NUL suffix", wire: "127.0.0.1\x00"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseIPNetwork(tc.wire)
			if tc.want == "" {
				if !errors.Is(gotErr, ErrPrimitiveContract) || got != (IPNetwork{}) {
					t.Fatalf("ParseIPNetwork(%q) = (%v, %v), want (zero, %v)", tc.wire, got, gotErr, ErrPrimitiveContract)
				}
				return
			}
			if gotErr != nil || got.String() != tc.want {
				t.Fatalf("ParseIPNetwork(%q) = (%q, %v), want (%q, nil)", tc.wire, got.String(), gotErr, tc.want)
			}
			if gotValidateErr := got.Validate(); gotValidateErr != nil {
				t.Fatalf("ParseIPNetwork(%q).Validate() error = %v, want nil", tc.wire, gotValidateErr)
			}
			prefix, gotPrefixErr := got.Prefix()
			if gotPrefixErr != nil || !prefix.IsValid() {
				t.Fatalf("ParseIPNetwork(%q).Prefix() = (%v, %v), want valid prefix", tc.wire, prefix, gotPrefixErr)
			}
		})
	}
}

func TestIPNetworkContainmentHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		network   string
		candidate string
		want      bool
		wantErr   bool
	}{
		{name: "IPv4 network lower edge", network: "192.0.2.0/24", candidate: "192.0.2.0", want: true},
		{name: "IPv4 network interior", network: "192.0.2.0/24", candidate: "192.0.2.127", want: true},
		{name: "IPv4 network upper edge", network: "192.0.2.0/24", candidate: "192.0.2.255", want: true},
		{name: "IPv4 one below network", network: "192.0.2.0/24", candidate: "192.0.1.255"},
		{name: "IPv4 one above network", network: "192.0.2.0/24", candidate: "192.0.3.0"},
		{name: "IPv4 exact identity matches itself", network: "192.0.2.1", candidate: "192.0.2.1", want: true},
		{name: "IPv4 exact identity rejects neighbor", network: "192.0.2.1", candidate: "192.0.2.2"},
		{name: "IPv6 network lower edge", network: "2001:db8::/64", candidate: "2001:db8::", want: true},
		{name: "IPv6 network interior", network: "2001:db8::/64", candidate: "2001:db8::1", want: true},
		{name: "IPv6 network upper edge", network: "2001:db8::/64", candidate: "2001:db8::ffff:ffff:ffff:ffff", want: true},
		{name: "IPv6 neighboring network", network: "2001:db8::/64", candidate: "2001:db8:0:1::"},
		{name: "IPv6 exact identity matches itself", network: "2001:db8::1", candidate: "2001:db8::1", want: true},
		{name: "empty candidate", network: "192.0.2.0/24", candidate: "", wantErr: true},
		{name: "candidate with leading space", network: "192.0.2.0/24", candidate: " 192.0.2.1", wantErr: true},
		{name: "candidate IPv4 leading zeros", network: "192.0.2.0/24", candidate: "192.000.2.1", wantErr: true},
		{name: "candidate is CIDR not address", network: "192.0.2.0/24", candidate: "192.0.2.0/24", wantErr: true},
		{name: "candidate mapped IPv6", network: "::/0", candidate: "::ffff:192.0.2.1", wantErr: true},
		{name: "candidate IPv6 uppercase alternate", network: "2001:db8::/64", candidate: "2001:DB8::1", wantErr: true},
		{name: "candidate IPv6 zone", network: "fe80::/64", candidate: "fe80::1%en0", wantErr: true},
		{name: "unset network rejects canonical candidate", candidate: "192.0.2.1", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var network IPNetwork
			if tc.network != "" {
				var parseErr error
				network, parseErr = ParseIPNetwork(tc.network)
				if parseErr != nil {
					t.Fatalf("ParseIPNetwork(%q) setup error = %v, want nil", tc.network, parseErr)
				}
			}
			got, gotErr := network.Contains(tc.candidate)
			if tc.wantErr {
				if !errors.Is(gotErr, ErrPrimitiveContract) || got {
					t.Fatalf("IPNetwork(%q).Contains(%q) = (%t, %v), want (false, %v)", tc.network, tc.candidate, got, gotErr, ErrPrimitiveContract)
				}
				return
			}
			if gotErr != nil || got != tc.want {
				t.Fatalf("IPNetwork(%q).Contains(%q) = (%t, %v), want (%t, nil)", tc.network, tc.candidate, got, gotErr, tc.want)
			}
		})
	}
}
