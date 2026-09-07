package exchange_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// These rows pin forwarding and canonicalization through Go's literal parser.
// Exchange's own wildcard/port policy is covered separately; there is no
// duplicated address parser or artificial 10/10/20 counter in this fixture.
func TestListenAddressGoLiteralGrammarTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		inputs   []string
		wantText string
		wantErr  error
	}{
		{name: "IPv4 decimal port canonicalization stays with Go", inputs: []string{"127.0.0.1:08080"}, wantText: "127.0.0.1:8080"},
		{name: "expanded IPv6 case and compression canonicalize together", inputs: []string{"[2001:0DB8:0:0:0:0:0:1]:443"}, wantText: "[2001:db8::1]:443"},
		{name: "zone identity preserves case without interpretation", inputs: []string{"[fe80::1%aBC0]:8080"}, wantText: "[fe80::1%aBC0]:8080"},
		{name: "highest representable IPv6 address and port stay exact", inputs: []string{"[ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff]:65535"}, wantText: "[ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff]:65535"},
		{name: "empty input cannot manufacture a listener address", inputs: []string{""}, wantErr: core.ErrExchangeContract},
		{name: "hostnames cannot replace a literal or trigger resolution", inputs: []string{"localhost:8080"}, wantErr: core.ErrExchangeContract},
		{name: "missing port is never inferred", inputs: []string{"127.0.0.1"}, wantErr: core.ErrExchangeContract},
		{name: "unbracketed IPv6 cannot impersonate host and port", inputs: []string{"::1:8080"}, wantErr: core.ErrExchangeContract},
		{name: "signed port spellings cannot enter the unsigned domain", inputs: []string{"127.0.0.1:-1", "127.0.0.1:+8080"}, wantErr: core.ErrExchangeContract},
		{name: "port immediately above uint16 cannot wrap", inputs: []string{"127.0.0.1:65536"}, wantErr: core.ErrExchangeContract},
		{name: "service names cannot hide a port lookup", inputs: []string{"127.0.0.1:http"}, wantErr: core.ErrExchangeContract},
		{name: "truncated IPv6 bracket is refused intact", inputs: []string{"[::1:8080"}, wantErr: core.ErrExchangeContract},
		{name: "whitespace is not silently trimmed from address input", inputs: []string{" 127.0.0.1:8080", "127.0.0.1:8080 "}, wantErr: core.ErrExchangeContract},
		{name: "URL path cannot be discarded from a listen address", inputs: []string{"127.0.0.1:8080/path"}, wantErr: core.ErrExchangeContract},
		{name: "IPv4 octet overflow cannot wrap into another interface", inputs: []string{"256.0.0.1:8080"}, wantErr: core.ErrExchangeContract},
		{name: "leading zero IPv4 octet is not guessed as another radix", inputs: []string{"127.000.0.1:8080"}, wantErr: core.ErrExchangeContract},
		{name: "IPv6 group overflow cannot truncate to an interface", inputs: []string{"[10000::1]:8080"}, wantErr: core.ErrExchangeContract},
		{name: "multiple IPv6 compressions cannot merge unknown groups", inputs: []string{"[2001::db8::1]:8080"}, wantErr: core.ErrExchangeContract},
		{name: "empty zone cannot become a default zone", inputs: []string{"[fe80::1%]:8080"}, wantErr: core.ErrExchangeContract},
		{name: "zone after the port cannot be reinterpreted as host scope", inputs: []string{"[::1]:8080%lo0"}, wantErr: core.ErrExchangeContract},
		{name: "Unicode digits cannot masquerade as a decimal port", inputs: []string{"127.0.0.1:\uff18\uff10\uff18\uff10"}, wantErr: core.ErrExchangeContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, input := range tc.inputs {
				got, gotErr := exchange.ParseListenAddress(input)
				if !errors.Is(gotErr, tc.wantErr) || got.String() != tc.wantText {
					t.Fatalf("ParseListenAddress(%q) = (%q, %v), want (%q, %v)", input, got.String(), gotErr, tc.wantText, tc.wantErr)
				}
				if tc.wantErr != nil {
					if got != (exchange.ListenAddress{}) {
						t.Fatalf("refused address = %v, want zero", got)
					}
					continue
				}
				if err := got.Validate(); err != nil {
					t.Fatalf("admitted address validation = %v, want nil", err)
				}
			}
		})
	}
}
