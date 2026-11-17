package exchange_test

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestResolveIPRequestHostLayerTriad(t *testing.T) {
	t.Parallel()
	label63 := strings.Repeat("a", 63)
	host253 := strings.Join([]string{label63, label63, label63, strings.Repeat("b", 61)}, ".")
	for _, row := range []struct {
		name, host string
		wantErr    error
	}{
		{name: "positive single label", host: "localhost"},
		{name: "positive mixed case hyphen and digits", host: "Shop-2.Example.COM"},
		{name: "positive numeric inner label", host: "123.example.com"},
		{name: "positive 63 byte label", host: label63 + ".com"},
		{name: "positive 253 byte host", host: host253},
		{name: "negative 254 byte host", host: host253 + "b", wantErr: core.ErrExchangeContract},
		{name: "negative 64 byte label", host: label63 + "a.com", wantErr: core.ErrExchangeContract},
		{name: "negative IPv4 literal", host: "127.0.0.1", wantErr: core.ErrExchangeContract},
		{name: "negative legacy numeric IPv4 form", host: "127.1", wantErr: core.ErrExchangeContract},
		{name: "negative hexadecimal IPv4 form", host: "0x7f.0.0.1", wantErr: core.ErrExchangeContract},
		{name: "negative IPv6 literal", host: "::1", wantErr: core.ErrExchangeContract},
		{name: "negative bracketed IPv6 literal", host: "[::1]", wantErr: core.ErrExchangeContract},
		{name: "negative host with port", host: "example.com:443", wantErr: core.ErrExchangeContract},
		{name: "negative URL", host: "https://example.com/", wantErr: core.ErrExchangeContract},
		{name: "negative trailing dot", host: "example.com.", wantErr: core.ErrExchangeContract},
		{name: "negative empty label", host: "example..com", wantErr: core.ErrExchangeContract},
		{name: "negative leading dot", host: ".example.com", wantErr: core.ErrExchangeContract},
		{name: "negative leading hyphen", host: "-example.com", wantErr: core.ErrExchangeContract},
		{name: "negative trailing hyphen", host: "example-.com", wantErr: core.ErrExchangeContract},
		{name: "negative underscore", host: "_dmarc.example.com", wantErr: core.ErrExchangeContract},
		{name: "negative space", host: "exa mple.com", wantErr: core.ErrExchangeContract},
		{name: "negative non-ASCII", host: "exämple.com", wantErr: core.ErrExchangeContract},
		{name: "neutral empty host", wantErr: core.ErrExchangeContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if err := (exchange.ResolveIPRequest{Host: row.host}).Validate(); !errors.Is(err, row.wantErr) {
				t.Fatalf("Validate(%q) = %v, want %v", row.host, err, row.wantErr)
			}
		})
	}
}

// The native resolver answers localhost from the host database, so this proof
// observes Go's real lookup without depending on external DNS.
func TestResolveIPNativeLookupLayerTriad(t *testing.T) {
	t.Parallel()
	t.Run("positive localhost resolves to distinct unmapped zone-free loopback", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		addresses, err := exchange.ResolveIP(ctx, exchange.ResolveIPRequest{Host: "localhost"})
		if err != nil || len(addresses) == 0 || len(addresses) > exchange.ResolveIPMaximumAddresses {
			t.Fatalf("ResolveIP(localhost) = (%v,%v), want 1..%d addresses", addresses, err, exchange.ResolveIPMaximumAddresses)
		}
		seen := map[netip.Addr]bool{}
		for _, address := range addresses {
			if !address.IsValid() || !address.IsLoopback() || address.Is4In6() || address.Zone() != "" || seen[address] {
				t.Fatalf("ResolveIP(localhost) = %v, want distinct unmapped zone-free loopback addresses", addresses)
			}
			seen[address] = true
		}
	})
	t.Run("negative unresolvable reserved name preserves native DNS error", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		addresses, err := exchange.ResolveIP(ctx, exchange.ResolveIPRequest{Host: "primitive-resolve-ip-absent.invalid"})
		var native *net.DNSError
		if addresses != nil || !errors.Is(err, core.ErrExchangeTransport) || !errors.As(err, &native) {
			t.Fatalf("ResolveIP(.invalid) = (%v,%v), want nil with transport and native *net.DNSError", addresses, err)
		}
	})
	t.Run("neutral IP literal is refused before lookup", func(t *testing.T) {
		t.Parallel()
		addresses, err := exchange.ResolveIP(t.Context(), exchange.ResolveIPRequest{Host: "127.0.0.1"})
		if addresses != nil || !errors.Is(err, core.ErrExchangeContract) || errors.Is(err, core.ErrExchangeTransport) {
			t.Fatalf("ResolveIP(literal) = (%v,%v), want nil and only %v", addresses, err, core.ErrExchangeContract)
		}
	})
}

func TestResolveIPStoppedContextReturnsNoAddress(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		context func(context.Context) (context.Context, context.CancelFunc)
		wantErr error
	}{
		{name: "nil context is a typed refusal", context: func(context.Context) (context.Context, context.CancelFunc) { return nil, func() {} }, wantErr: core.ErrNilContext},
		{name: "canceled context cannot resolve", context: func(parent context.Context) (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(parent)
			cancel()
			return ctx, cancel
		}, wantErr: context.Canceled},
		{name: "expired context cannot resolve", context: func(parent context.Context) (context.Context, context.CancelFunc) {
			return context.WithDeadline(parent, time.Time{})
		}, wantErr: context.DeadlineExceeded},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := row.context(t.Context())
			defer cancel()
			addresses, err := exchange.ResolveIP(ctx, exchange.ResolveIPRequest{Host: "localhost"})
			if addresses != nil || !errors.Is(err, row.wantErr) {
				t.Fatalf("ResolveIP stopped context = (%v,%v), want nil and %v", addresses, err, row.wantErr)
			}
		})
	}
}

var resolveHostOracle = regexp.MustCompile(`^([A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)*[A-Za-z0-9-]*[A-Za-z-][A-Za-z0-9-]*$`)

// An independent grammar oracle decides admission. Refused hosts must also be
// refused by ResolveIP before any native lookup; admitted hosts are not looked
// up here, so fuzzing never depends on external DNS.
func FuzzResolveIPRequestHostSemanticClosure(f *testing.F) {
	for _, seed := range []string{"localhost", "example.com", "127.0.0.1", "127.1", "::1", "example.com:443", "a-.b", "-a.b", "a..b", "a.", strings.Repeat("a", 64), "xn--bcher-kva.example", "1a.2b"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, host string) {
		err := (exchange.ResolveIPRequest{Host: host}).Validate()
		lastLabel := host[strings.LastIndexByte(host, '.')+1:]
		admit := len(host) <= 253 && resolveHostOracle.MatchString(host) && len(lastLabel) <= 63 && lastLabel[0] != '-' && lastLabel[len(lastLabel)-1] != '-'
		if admit {
			if _, literal := netip.ParseAddr(host); literal == nil || err != nil {
				t.Fatalf("Validate(%q) = %v, literal parse %v; oracle admits a non-literal DNS name", host, err, literal)
			}
			return
		}
		if !errors.Is(err, core.ErrExchangeContract) {
			t.Fatalf("Validate(%q) = %v, oracle refuses with %v", host, err, core.ErrExchangeContract)
		}
		addresses, resolveErr := exchange.ResolveIP(t.Context(), exchange.ResolveIPRequest{Host: host})
		if addresses != nil || !errors.Is(resolveErr, core.ErrExchangeContract) || errors.Is(resolveErr, core.ErrExchangeTransport) {
			t.Fatalf("ResolveIP(%q) = (%v,%v), want refusal before lookup", host, addresses, resolveErr)
		}
	})
}
