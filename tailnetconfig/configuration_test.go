package tailnetconfig_test

import (
	"errors"
	"github.com/deliri/primitive/v2026/tailnetconfig"
	"net/netip"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestConfigurationLayerTriad(t *testing.T) {
	t.Parallel()
	configuration := fixtureConfiguration(t)
	cases := []struct {
		name    string
		mutate  func(*tailnetconfig.Configuration)
		wantErr error
	}{
		{"positive pinned destination and explicit enrollment", func(*tailnetconfig.Configuration) {}, nil},
		{"neutral zero client identity cannot enroll", func(c *tailnetconfig.Configuration) { c.ClientID = "" }, tailnetconfig.ErrContract},
		{"missing host label cannot acquire ambient hostname", func(c *tailnetconfig.Configuration) { c.Hostname = "" }, tailnetconfig.ErrContract},
		{"missing tag cannot enroll as a user node", func(c *tailnetconfig.Configuration) { c.Tag = "" }, tailnetconfig.ErrContract},
		{"missing audience cannot receive a generic token", func(c *tailnetconfig.Configuration) { c.Audience = googleidentity.Audience{} }, tailnetconfig.ErrContract},
		{"missing state owner cannot use home directory", func(c *tailnetconfig.Configuration) { c.StateDirectory = core.AbsolutePath{} }, tailnetconfig.ErrContract},
		{"zero deadline cannot leave enrollment unbounded", func(c *tailnetconfig.Configuration) { c.StartupTimeout = temporal.Duration{} }, tailnetconfig.ErrContract},
		{"zero destination cannot select another host", func(c *tailnetconfig.Configuration) { c.Destination = netip.AddrPort{} }, tailnetconfig.ErrContract},
		{"public destination cannot escape capability", func(c *tailnetconfig.Configuration) { c.Destination = netip.MustParseAddrPort("1.1.1.1:443") }, tailnetconfig.ErrContract},
		{"private LAN destination requires a different capability", func(c *tailnetconfig.Configuration) { c.Destination = netip.MustParseAddrPort("192.168.1.81:8088") }, tailnetconfig.ErrContract},
		{"zero destination port cannot select an ephemeral port", func(c *tailnetconfig.Configuration) { c.Destination = netip.MustParseAddrPort("100.84.35.44:0") }, tailnetconfig.ErrContract},
		{"tailnet IPv6 destination is explicitly admitted", func(c *tailnetconfig.Configuration) {
			c.Destination = netip.MustParseAddrPort("[fd7a:115c:a1e0::1]:8088")
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := configuration
			tc.mutate(&in)
			if got := in.Validate(); !errors.Is(got, tc.wantErr) {
				t.Fatalf("tailnetconfig.Configuration.Validate() = %v, want %v", got, tc.wantErr)
			}
		})
	}
}

func TestNamesBoundaryContracts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		in       string
		validate func(string) error
		wantErr  error
	}{
		{"client ID exact ceiling", strings.Repeat("a", 128), func(s string) error { return tailnetconfig.ClientID(s).Validate() }, nil},
		{"client ID above ceiling", strings.Repeat("a", 129), func(s string) error { return tailnetconfig.ClientID(s).Validate() }, tailnetconfig.ErrContract},
		{"client ID separators preserve provider alphabet", "Az_09-id", func(s string) error { return tailnetconfig.ClientID(s).Validate() }, nil},
		{"client ID control byte rejected", "id\n", func(s string) error { return tailnetconfig.ClientID(s).Validate() }, tailnetconfig.ErrContract},
		{"hostname exact ceiling", strings.Repeat("a", 63), func(s string) error { return tailnetconfig.Hostname(s).Validate() }, nil},
		{"hostname above ceiling", strings.Repeat("a", 64), func(s string) error { return tailnetconfig.Hostname(s).Validate() }, tailnetconfig.ErrContract},
		{"hostname minimum single byte", "a", func(s string) error { return tailnetconfig.Hostname(s).Validate() }, nil},
		{"hostname zero bytes", "", func(s string) error { return tailnetconfig.Hostname(s).Validate() }, tailnetconfig.ErrContract},
		{"hostname no leading hyphen", "-host", func(s string) error { return tailnetconfig.Hostname(s).Validate() }, tailnetconfig.ErrContract},
		{"hostname no trailing hyphen", "host-", func(s string) error { return tailnetconfig.Hostname(s).Validate() }, tailnetconfig.ErrContract},
		{"hostname canonical lower case", "Host", func(s string) error { return tailnetconfig.Hostname(s).Validate() }, tailnetconfig.ErrContract},
		{"hostname no underscore", "host_name", func(s string) error { return tailnetconfig.Hostname(s).Validate() }, tailnetconfig.ErrContract},
		{"hostname no suffix injection", "host.other", func(s string) error { return tailnetconfig.Hostname(s).Validate() }, tailnetconfig.ErrContract},
		{"hostname no unicode lookalike", "hοst", func(s string) error { return tailnetconfig.Hostname(s).Validate() }, tailnetconfig.ErrContract},
		{"tag requires typed prefix", "blink-api", func(s string) error { return tailnetconfig.Tag(s).Validate() }, tailnetconfig.ErrContract},
		{"tag empty suffix", "tag:", func(s string) error { return tailnetconfig.Tag(s).Validate() }, tailnetconfig.ErrContract},
		{"tag singular named authority", "tag:blink-api", func(s string) error { return tailnetconfig.Tag(s).Validate() }, nil},
		{"tag no second authority", "tag:blink-api,tag:admin", func(s string) error { return tailnetconfig.Tag(s).Validate() }, tailnetconfig.ErrContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.validate(tc.in); !errors.Is(got, tc.wantErr) {
				t.Fatalf("validate(%q) = %v, want %v", tc.in, got, tc.wantErr)
			}
		})
	}
}

func TestTailnetAddressBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		{"100.63.255.255", false}, {"100.64.0.0", true}, {"100.64.0.1", true},
		{"100.127.255.255", true}, {"100.128.0.0", false}, {"255.255.255.255", false},
		{"0.0.0.0", false}, {"127.0.0.1", false}, {"192.168.1.81", false},
		{"fd7a:115c:a1df:ffff:ffff:ffff:ffff:ffff", false}, {"fd7a:115c:a1e0::", true},
		{"fd7a:115c:a1e0::1", true}, {"fd7a:115c:a1e0:ffff:ffff:ffff:ffff:ffff", true},
		{"fd7a:115c:a1e1::", false}, {"fd7a:115c:a1e0::1%eth0", false},
		{"::ffff:100.84.35.44", true}, {"::ffff:1.1.1.1", false}, {"::", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := tailnetconfig.IsAddress(netip.MustParseAddr(tc.in)); got != tc.want {
				t.Fatalf("tailnetconfig.IsAddress(%s) = %t, want %t", tc.in, got, tc.want)
			}
		})
	}
	if tailnetconfig.IsAddress(netip.Addr{}) {
		t.Fatal("tailnetconfig.IsAddress(zero) = true, want false")
	}
}

func fixtureConfiguration(t testing.TB) tailnetconfig.Configuration {
	t.Helper()
	path, err := core.ParseAbsolutePath("/tmp/primitive-tailnet-test-unused")
	if err != nil {
		t.Fatal(err)
	}
	audience, err := googleidentity.ParseAudience("api.tailscale.com/fixture-client")
	if err != nil {
		t.Fatal(err)
	}
	timeout, err := temporal.DurationFromSeconds(30)
	if err != nil {
		t.Fatal(err)
	}
	return tailnetconfig.Configuration{ClientID: "fixture-client", Hostname: "blink-api", Tag: "tag:blink-api", Audience: audience, StateDirectory: path, Destination: netip.MustParseAddrPort("100.84.35.44:8088"), StartupTimeout: timeout}
}
