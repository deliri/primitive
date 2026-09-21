package tailnetconfig_test

import (
	"errors"
	"github.com/deliri/primitive/v2026/tailnetconfig"
	"net/netip"
	"path/filepath"
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
		wantErr error
		mutate  func(*tailnetconfig.Configuration)
		name    string
	}{
		{name: "positive pinned destination and explicit enrollment", mutate: func(*tailnetconfig.Configuration) {}, wantErr: nil},
		{name: "neutral zero client identity cannot enroll", mutate: func(c *tailnetconfig.Configuration) { c.ClientID = "" }, wantErr: core.ErrTailnetContract},
		{name: "missing host label cannot acquire ambient hostname", mutate: func(c *tailnetconfig.Configuration) { c.Hostname = "" }, wantErr: core.ErrTailnetContract},
		{name: "missing tag cannot enroll as a user node", mutate: func(c *tailnetconfig.Configuration) { c.Tag = "" }, wantErr: core.ErrTailnetContract},
		{name: "missing audience cannot receive a generic token", mutate: func(c *tailnetconfig.Configuration) { c.Audience = googleidentity.Audience{} }, wantErr: core.ErrTailnetContract},
		{name: "missing state owner cannot use home directory", mutate: func(c *tailnetconfig.Configuration) { c.StateDirectory = core.AbsolutePath{} }, wantErr: core.ErrTailnetContract},
		{name: "zero deadline cannot leave enrollment unbounded", mutate: func(c *tailnetconfig.Configuration) { c.StartupTimeout = temporal.Duration{} }, wantErr: core.ErrTailnetContract},
		{name: "zero destination cannot select another host", mutate: func(c *tailnetconfig.Configuration) { c.Destination = netip.AddrPort{} }, wantErr: core.ErrTailnetContract},
		{name: "public destination cannot escape capability", mutate: func(c *tailnetconfig.Configuration) { c.Destination = netip.MustParseAddrPort("1.1.1.1:443") }, wantErr: core.ErrTailnetContract},
		{name: "private LAN destination requires a different capability", mutate: func(c *tailnetconfig.Configuration) { c.Destination = netip.MustParseAddrPort("192.168.1.81:8088") }, wantErr: core.ErrTailnetContract},
		{name: "zero destination port cannot select an ephemeral port", mutate: func(c *tailnetconfig.Configuration) { c.Destination = netip.MustParseAddrPort("100.84.35.44:0") }, wantErr: core.ErrTailnetContract},
		{name: "tailnet IPv6 destination is explicitly admitted", mutate: func(c *tailnetconfig.Configuration) {
			c.Destination = netip.MustParseAddrPort("[fd7a:115c:a1e0::1]:8088")
		}, wantErr: nil},
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
		wantErr  error
		validate func(string) error
		name     string
		in       string
	}{
		{name: "client ID exact ceiling", in: strings.Repeat("a", 128), validate: func(s string) error { return tailnetconfig.ClientID(s).Validate() }, wantErr: nil},
		{name: "client ID above ceiling", in: strings.Repeat("a", 129), validate: func(s string) error { return tailnetconfig.ClientID(s).Validate() }, wantErr: core.ErrTailnetContract},
		{name: "client ID separators preserve provider alphabet", in: "Az_09-id", validate: func(s string) error { return tailnetconfig.ClientID(s).Validate() }, wantErr: nil},
		{name: "client ID control byte rejected", in: "id\n", validate: func(s string) error { return tailnetconfig.ClientID(s).Validate() }, wantErr: core.ErrTailnetContract},
		{name: "hostname exact ceiling", in: strings.Repeat("a", 63), validate: func(s string) error { return tailnetconfig.Hostname(s).Validate() }, wantErr: nil},
		{name: "hostname above ceiling", in: strings.Repeat("a", 64), validate: func(s string) error { return tailnetconfig.Hostname(s).Validate() }, wantErr: core.ErrTailnetContract},
		{name: "hostname minimum single byte", in: "a", validate: func(s string) error { return tailnetconfig.Hostname(s).Validate() }, wantErr: nil},
		{name: "hostname zero bytes", in: "", validate: func(s string) error { return tailnetconfig.Hostname(s).Validate() }, wantErr: core.ErrTailnetContract},
		{name: "hostname no leading hyphen", in: "-host", validate: func(s string) error { return tailnetconfig.Hostname(s).Validate() }, wantErr: core.ErrTailnetContract},
		{name: "hostname no trailing hyphen", in: "host-", validate: func(s string) error { return tailnetconfig.Hostname(s).Validate() }, wantErr: core.ErrTailnetContract},
		{name: "hostname canonical lower case", in: "Host", validate: func(s string) error { return tailnetconfig.Hostname(s).Validate() }, wantErr: core.ErrTailnetContract},
		{name: "hostname no underscore", in: "host_name", validate: func(s string) error { return tailnetconfig.Hostname(s).Validate() }, wantErr: core.ErrTailnetContract},
		{name: "hostname no suffix injection", in: "host.other", validate: func(s string) error { return tailnetconfig.Hostname(s).Validate() }, wantErr: core.ErrTailnetContract},
		{name: "hostname no unicode lookalike", in: "hοst", validate: func(s string) error { return tailnetconfig.Hostname(s).Validate() }, wantErr: core.ErrTailnetContract},
		{name: "tag requires typed prefix", in: "blink-api", validate: func(s string) error { return tailnetconfig.Tag(s).Validate() }, wantErr: core.ErrTailnetContract},
		{name: "tag empty suffix", in: "tag:", validate: func(s string) error { return tailnetconfig.Tag(s).Validate() }, wantErr: core.ErrTailnetContract},
		{name: "tag singular named authority", in: "tag:blink-api", validate: func(s string) error { return tailnetconfig.Tag(s).Validate() }, wantErr: nil},
		{name: "tag no second authority", in: "tag:blink-api,tag:admin", validate: func(s string) error { return tailnetconfig.Tag(s).Validate() }, wantErr: core.ErrTailnetContract},
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
		{in: "100.63.255.255", want: false}, {in: "100.64.0.0", want: true}, {in: "100.64.0.1", want: true},
		{in: "100.127.255.255", want: true}, {in: "100.128.0.0", want: false}, {in: "255.255.255.255", want: false},
		{in: "0.0.0.0", want: false}, {in: "127.0.0.1", want: false}, {in: "192.168.1.81", want: false},
		{in: "fd7a:115c:a1df:ffff:ffff:ffff:ffff:ffff", want: false}, {in: "fd7a:115c:a1e0::", want: true},
		{in: "fd7a:115c:a1e0::1", want: true}, {in: "fd7a:115c:a1e0:ffff:ffff:ffff:ffff:ffff", want: true},
		{in: "fd7a:115c:a1e1::", want: false}, {in: "fd7a:115c:a1e0::1%eth0", want: false},
		{in: "::ffff:100.84.35.44", want: true}, {in: "::ffff:1.1.1.1", want: false}, {in: "::", want: false},
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
	// This location is only validated lexically; enrollment fixtures replace it
	// with their own t.TempDir path before any filesystem effect.
	location := "/tmp/primitive-tailnet-test-unused"
	if filepath.Separator == '\\' {
		location = `C:\primitive-tailnet-test-unused`
	}
	path, err := core.ParseAbsolutePath(location)
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
