package tailnetconfig_test

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/tailnetconfig"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestConfigurationExactBoundaryTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		edit    func(*tailnetconfig.Configuration)
		wantErr error
	}{
		{name: "client identifier minimum", edit: func(c *tailnetconfig.Configuration) { c.ClientID = "a" }},
		{name: "client identifier below maximum", edit: func(c *tailnetconfig.Configuration) {
			c.ClientID = tailnetconfig.ClientID(strings.Repeat("a", tailnetconfig.ClientIDMaximumBytes-1))
		}},
		{name: "client identifier exact maximum", edit: func(c *tailnetconfig.Configuration) {
			c.ClientID = tailnetconfig.ClientID(strings.Repeat("a", tailnetconfig.ClientIDMaximumBytes))
		}},
		{name: "client identifier above maximum", edit: func(c *tailnetconfig.Configuration) {
			c.ClientID = tailnetconfig.ClientID(strings.Repeat("a", tailnetconfig.ClientIDMaximumBytes+1))
		}, wantErr: core.ErrTailnetContract},
		{name: "hostname minimum", edit: func(c *tailnetconfig.Configuration) { c.Hostname = "a" }},
		{name: "hostname below maximum", edit: func(c *tailnetconfig.Configuration) {
			c.Hostname = tailnetconfig.Hostname(strings.Repeat("a", tailnetconfig.HostnameMaximumBytes-1))
		}},
		{name: "hostname exact maximum", edit: func(c *tailnetconfig.Configuration) {
			c.Hostname = tailnetconfig.Hostname(strings.Repeat("a", tailnetconfig.HostnameMaximumBytes))
		}},
		{name: "hostname above maximum", edit: func(c *tailnetconfig.Configuration) {
			c.Hostname = tailnetconfig.Hostname(strings.Repeat("a", tailnetconfig.HostnameMaximumBytes+1))
		}, wantErr: core.ErrTailnetContract},
		{name: "tag minimum suffix", edit: func(c *tailnetconfig.Configuration) { c.Tag = tailnetconfig.Tag(tailnetconfig.TagPrefix + "a") }},
		{name: "tag below suffix maximum", edit: func(c *tailnetconfig.Configuration) {
			c.Tag = tailnetconfig.Tag(tailnetconfig.TagPrefix + strings.Repeat("a", tailnetconfig.HostnameMaximumBytes-1))
		}},
		{name: "tag exact suffix maximum", edit: func(c *tailnetconfig.Configuration) {
			c.Tag = tailnetconfig.Tag(tailnetconfig.TagPrefix + strings.Repeat("a", tailnetconfig.HostnameMaximumBytes))
		}},
		{name: "tag above suffix maximum", edit: func(c *tailnetconfig.Configuration) {
			c.Tag = tailnetconfig.Tag(tailnetconfig.TagPrefix + strings.Repeat("a", tailnetconfig.HostnameMaximumBytes+1))
		}, wantErr: core.ErrTailnetContract},
		{name: "IPv4 lower neighbor outside assigned prefix", edit: func(c *tailnetconfig.Configuration) { c.Destination = netip.MustParseAddrPort("100.63.255.255:1") }, wantErr: core.ErrTailnetContract},
		{name: "IPv4 exact lower assigned address and minimum port", edit: func(c *tailnetconfig.Configuration) { c.Destination = netip.MustParseAddrPort("100.64.0.0:1") }},
		{name: "IPv4 exact upper assigned address and maximum port", edit: func(c *tailnetconfig.Configuration) { c.Destination = netip.MustParseAddrPort("100.127.255.255:65535") }},
		{name: "IPv4 upper neighbor outside assigned prefix", edit: func(c *tailnetconfig.Configuration) { c.Destination = netip.MustParseAddrPort("100.128.0.0:65535") }, wantErr: core.ErrTailnetContract},
		{name: "IPv6 exact lower assigned address", edit: func(c *tailnetconfig.Configuration) { c.Destination = netip.MustParseAddrPort("[fd7a:115c:a1e0::]:1") }},
		{name: "IPv6 exact upper assigned address", edit: func(c *tailnetconfig.Configuration) {
			c.Destination = netip.MustParseAddrPort("[fd7a:115c:a1e0:ffff:ffff:ffff:ffff:ffff]:65535")
		}},
		{name: "IPv6 lower neighbor outside assigned prefix", edit: func(c *tailnetconfig.Configuration) {
			c.Destination = netip.MustParseAddrPort("[fd7a:115c:a1df:ffff:ffff:ffff:ffff:ffff]:1")
		}, wantErr: core.ErrTailnetContract},
		{name: "IPv6 upper neighbor outside assigned prefix", edit: func(c *tailnetconfig.Configuration) { c.Destination = netip.MustParseAddrPort("[fd7a:115c:a1e1::]:1") }, wantErr: core.ErrTailnetContract},
		{name: "IPv6 zone cannot bind ambient interface", edit: func(c *tailnetconfig.Configuration) {
			c.Destination = netip.MustParseAddrPort("[fd7a:115c:a1e0::1%en0]:1")
		}, wantErr: core.ErrTailnetContract},
		{name: "mapped IPv4 remains same tailnet address", edit: func(c *tailnetconfig.Configuration) { c.Destination = netip.MustParseAddrPort("[::ffff:100.64.0.1]:1") }},
		{name: "client identifier cannot inject federation options", edit: func(c *tailnetconfig.Configuration) { c.ClientID = "client?ephemeral=false" }, wantErr: core.ErrTailnetContract},
		{name: "hostname cannot traverse state paths", edit: func(c *tailnetconfig.Configuration) { c.Hostname = "../node" }, wantErr: core.ErrTailnetContract},
		{name: "hostname cannot acquire implicit normalization", edit: func(c *tailnetconfig.Configuration) { c.Hostname = "Node" }, wantErr: core.ErrTailnetContract},
		{name: "tag cannot combine authorities", edit: func(c *tailnetconfig.Configuration) { c.Tag = "tag:node,tag:admin" }, wantErr: core.ErrTailnetContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			configuration := fixtureConfiguration(t)
			tc.edit(&configuration)
			if got := configuration.Validate(); !errors.Is(got, tc.wantErr) {
				t.Fatalf("Validate() = %v, want %v", got, tc.wantErr)
			}
		})
	}
}

func TestStartupTimeoutBoundaryTable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		nanoseconds int64
		wantErr     error
	}{
		{"zero has no enrollment budget", 0, core.ErrTailnetContract},
		{"minimum one nanosecond is not rounded away", 1, nil},
		{"one below maximum", tailnetconfig.MaximumStartupNanoseconds - 1, nil},
		{"exact maximum", tailnetconfig.MaximumStartupNanoseconds, nil},
		{"one above maximum", tailnetconfig.MaximumStartupNanoseconds + 1, core.ErrTailnetContract},
		{"maximum representable duration cannot bypass ceiling", 1<<63 - 1, core.ErrTailnetContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			configuration := fixtureConfiguration(t)
			duration, err := temporal.DurationFromNanoseconds(tc.nanoseconds)
			if err != nil {
				t.Fatalf("DurationFromNanoseconds() = %v, want nil", err)
			}
			configuration.StartupTimeout = duration
			if got := configuration.Validate(); !errors.Is(got, tc.wantErr) {
				t.Fatalf("Validate() = %v, want %v", got, tc.wantErr)
			}
		})
	}
}
