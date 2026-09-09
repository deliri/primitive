package tailnet

import (
	"context"
	"errors"
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
		mutate  func(*Configuration)
		wantErr error
	}{
		{"positive pinned destination and explicit enrollment", func(*Configuration) {}, nil},
		{"neutral zero client identity cannot enroll", func(c *Configuration) { c.ClientID = "" }, ErrContract},
		{"missing host label cannot acquire ambient hostname", func(c *Configuration) { c.Hostname = "" }, ErrContract},
		{"missing tag cannot enroll as a user node", func(c *Configuration) { c.Tag = "" }, ErrContract},
		{"missing audience cannot receive a generic token", func(c *Configuration) { c.Audience = googleidentity.Audience{} }, ErrContract},
		{"missing state owner cannot use home directory", func(c *Configuration) { c.StateDirectory = core.AbsolutePath{} }, ErrContract},
		{"zero deadline cannot leave enrollment unbounded", func(c *Configuration) { c.StartupTimeout = temporal.Duration{} }, ErrContract},
		{"zero destination cannot select another host", func(c *Configuration) { c.Destination = netip.AddrPort{} }, ErrContract},
		{"public destination cannot escape capability", func(c *Configuration) { c.Destination = netip.MustParseAddrPort("1.1.1.1:443") }, ErrContract},
		{"private LAN destination requires a different capability", func(c *Configuration) { c.Destination = netip.MustParseAddrPort("192.168.1.81:8088") }, ErrContract},
		{"zero destination port cannot select an ephemeral port", func(c *Configuration) { c.Destination = netip.MustParseAddrPort("100.84.35.44:0") }, ErrContract},
		{"tailnet IPv6 destination is explicitly admitted", func(c *Configuration) { c.Destination = netip.MustParseAddrPort("[fd7a:115c:a1e0::1]:8088") }, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := configuration
			tc.mutate(&in)
			if got := in.Validate(); !errors.Is(got, tc.wantErr) {
				t.Fatalf("Configuration.Validate() = %v, want %v", got, tc.wantErr)
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
		{"client ID exact ceiling", strings.Repeat("a", 128), func(s string) error { return ClientID(s).Validate() }, nil},
		{"client ID above ceiling", strings.Repeat("a", 129), func(s string) error { return ClientID(s).Validate() }, ErrContract},
		{"client ID separators preserve provider alphabet", "Az_09-id", func(s string) error { return ClientID(s).Validate() }, nil},
		{"client ID control byte rejected", "id\n", func(s string) error { return ClientID(s).Validate() }, ErrContract},
		{"hostname exact ceiling", strings.Repeat("a", 63), func(s string) error { return Hostname(s).Validate() }, nil},
		{"hostname above ceiling", strings.Repeat("a", 64), func(s string) error { return Hostname(s).Validate() }, ErrContract},
		{"hostname minimum single byte", "a", func(s string) error { return Hostname(s).Validate() }, nil},
		{"hostname zero bytes", "", func(s string) error { return Hostname(s).Validate() }, ErrContract},
		{"hostname no leading hyphen", "-host", func(s string) error { return Hostname(s).Validate() }, ErrContract},
		{"hostname no trailing hyphen", "host-", func(s string) error { return Hostname(s).Validate() }, ErrContract},
		{"hostname canonical lower case", "Host", func(s string) error { return Hostname(s).Validate() }, ErrContract},
		{"hostname no underscore", "host_name", func(s string) error { return Hostname(s).Validate() }, ErrContract},
		{"hostname no suffix injection", "host.other", func(s string) error { return Hostname(s).Validate() }, ErrContract},
		{"hostname no unicode lookalike", "hοst", func(s string) error { return Hostname(s).Validate() }, ErrContract},
		{"tag requires typed prefix", "blink-api", func(s string) error { return Tag(s).Validate() }, ErrContract},
		{"tag empty suffix", "tag:", func(s string) error { return Tag(s).Validate() }, ErrContract},
		{"tag singular named authority", "tag:blink-api", func(s string) error { return Tag(s).Validate() }, nil},
		{"tag no second authority", "tag:blink-api,tag:admin", func(s string) error { return Tag(s).Validate() }, ErrContract},
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
			if got := IsAddress(netip.MustParseAddr(tc.in)); got != tc.want {
				t.Fatalf("IsAddress(%s) = %t, want %t", tc.in, got, tc.want)
			}
		})
	}
	if IsAddress(netip.Addr{}) {
		t.Fatal("IsAddress(zero) = true, want false")
	}
}

type refusingIdentity struct {
	calls int
	cause error
}

func (s *refusingIdentity) Identity(context.Context, googleidentity.Audience) (googleidentity.Token, error) {
	s.calls++
	return googleidentity.Token{}, s.cause
}

func TestClientEnrollmentLayerTriad(t *testing.T) {
	t.Parallel()
	t.Run("positive construction owns transport without enrollment", func(t *testing.T) {
		t.Parallel()
		identity := &refusingIdentity{cause: ErrContract}
		client, err := NewClient(fixtureConfiguration(t), identity)
		if err != nil {
			t.Fatalf("NewClient() = %v, want nil", err)
		}
		if _, err := client.Exchange(); err != nil {
			t.Fatalf("Exchange() = %v, want nil", err)
		}
		if identity.calls != 0 {
			t.Fatalf("enrollments = %d, want 0", identity.calls)
		}
		if err := client.Close(); err != nil {
			t.Fatalf("Close() = %v, want nil", err)
		}
		if err := client.Close(); err != nil {
			t.Fatalf("Close(replay) = %v, want nil", err)
		}
		if _, err := client.dial(t.Context(), "tcp", client.configuration.Destination.String()); !errors.Is(err, ErrClosed) {
			t.Fatalf("dial(closed) = %v, want %v", err, ErrClosed)
		}
	})
	t.Run("negative refusal retains cause and permits explicit next attempt", func(t *testing.T) {
		t.Parallel()
		cause := errors.New("identity unavailable")
		identity := &refusingIdentity{cause: cause}
		client, err := NewClient(fixtureConfiguration(t), identity)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("Close() = %v, want nil", err)
			}
		}()
		for attempt := 1; attempt <= 2; attempt++ {
			conn, err := client.dial(t.Context(), "tcp", client.configuration.Destination.String())
			if conn != nil || !errors.Is(err, ErrEnrollment) || !errors.Is(err, cause) || identity.calls != attempt {
				t.Fatalf("dial(attempt %d) = (%v,%v,%d calls), want nil, typed refusal, %d", attempt, conn, err, identity.calls, attempt)
			}
			if client.server != nil {
				t.Fatal("refused identity retained server, want none")
			}
		}
	})
	t.Run("neutral cancellation and foreign destinations have no enrollment", func(t *testing.T) {
		t.Parallel()
		identity := &refusingIdentity{cause: ErrContract}
		client, err := NewClient(fixtureConfiguration(t), identity)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("Close() = %v, want nil", err)
			}
		}()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := client.dial(ctx, "tcp", client.configuration.Destination.String()); !errors.Is(err, context.Canceled) {
			t.Fatalf("dial(cancelled) = %v, want cancelled", err)
		}
		if _, err := client.dial(t.Context(), "tcp", "100.84.35.45:8088"); !errors.Is(err, ErrDestination) {
			t.Fatalf("dial(other host) = %v, want %v", err, ErrDestination)
		}
		if _, err := client.dial(t.Context(), "udp", client.configuration.Destination.String()); !errors.Is(err, ErrDestination) {
			t.Fatalf("dial(other protocol) = %v, want %v", err, ErrDestination)
		}
		if identity.calls != 0 {
			t.Fatalf("enrollments = %d, want 0", identity.calls)
		}
	})
}

func fixtureConfiguration(t testing.TB) Configuration {
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
	return Configuration{ClientID: "fixture-client", Hostname: "blink-api", Tag: "tag:blink-api", Audience: audience, StateDirectory: path, Destination: netip.MustParseAddrPort("100.84.35.44:8088"), StartupTimeout: timeout}
}
