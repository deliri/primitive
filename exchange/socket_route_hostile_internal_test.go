package exchange

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"net/url"
	"path"
	"strings"
	"testing"
)

func TestSocketRoutePathHostileGrammarTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, input  string
		wantAccepted bool
	}{
		{name: "literal case is not silently folded", input: "/Inventory", wantAccepted: true},
		{name: "segment separation remains exact", input: "/a/b", wantAccepted: true},
		{name: "extension dot is not a traversal segment", input: "/a.b", wantAccepted: true},
		{name: "dot-prefixed name is not current-directory syntax", input: "/.well-known", wantAccepted: true},
		{name: "interior double dot is not parent-directory syntax", input: "/a..b", wantAccepted: true},
		{name: "colon inside absolute path is not a URI scheme", input: "/a:b", wantAccepted: true},
		{name: "semicolon remains path material", input: "/a;b", wantAccepted: true},
		{name: "at sign remains path material not authority", input: "/a@b", wantAccepted: true},
		{name: "plus remains literal instead of query-decoded space", input: "/a+b", wantAccepted: true},
		{name: "comma remains one path segment", input: "/a,b", wantAccepted: true},
		{name: "relative name cannot admit a route", input: "a"},
		{name: "absolute URI cannot import authority", input: "https://foreign.invalid/a"},
		{name: "empty query marker still changes request target", input: "/a?"},
		{name: "query payload cannot become route agreement", input: "/a?b=c"},
		{name: "empty fragment marker is not bare path", input: "/a#"},
		{name: "fragment payload cannot become route agreement", input: "/a#b"},
		{name: "NUL cannot cross Go request URI parser", input: "/a\x00b"},
		{name: "CRLF cannot split request line", input: "/a\r\nb"},
		{name: "invalid UTF8 cannot retain canonical escaped path", input: "/a\xff"},
		{name: "literal space requires a different escaped representation", input: "/a b"},
		{name: "one below minimum path cannot admit zero value", input: ""},
		{name: "root at minimum does not require a product segment", input: "/", wantAccepted: true},
		{name: "one above minimum retains exact segment", input: "/a", wantAccepted: true},
		{name: "one below byte ceiling remains admitted", input: "/" + strings.Repeat("a", SocketRoutePathMaximumBytes-2), wantAccepted: true},
		{name: "exact byte ceiling remains admitted", input: "/" + strings.Repeat("a", SocketRoutePathMaximumBytes-1), wantAccepted: true},
		{name: "one above byte ceiling refuses instead of truncating", input: "/" + strings.Repeat("a", SocketRoutePathMaximumBytes)},
		{name: "duplicate leading separator cannot introduce authority ambiguity", input: "//a"},
		{name: "duplicate interior separator cannot normalize unnoticed", input: "/a//b"},
		{name: "trailing separator cannot change exact route identity", input: "/a/"},
		{name: "leading current-directory segment cannot normalize", input: "/./a"},
		{name: "interior current-directory segment cannot normalize", input: "/a/./b"},
		{name: "trailing current-directory segment cannot normalize", input: "/a/."},
		{name: "leading parent traversal cannot normalize outside root", input: "/../a"},
		{name: "interior parent traversal cannot remove owned segment", input: "/a/../b"},
		{name: "terminal parent traversal cannot erase route", input: "/a/.."},
		{name: "empty percent escape remains a syntax refusal", input: "/a%"},
		{name: "one digit percent escape cannot truncate silently", input: "/a%2"},
		{name: "upper-case escaped slash cannot hide segment boundary", input: "/a%2Fb"},
		{name: "lower-case escaped slash cannot bypass exact spelling", input: "/a%2fb"},
		{name: "escaped ordinary byte cannot introduce a second canonical identity", input: "/%41"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseSocketRoutePath(tc.input)
			if !tc.wantAccepted {
				if !errors.Is(err, core.ErrExchangeContract) || got != (SocketRoutePath{}) {
					t.Fatalf("route refusal=(%q,%v),want zero and typed contract", got.String(), err)
				}
				if !errors.Is((SocketRoutePath{value: tc.input}).Validate(), core.ErrExchangeContract) {
					t.Fatalf("nominal validation=%v, want %v", (SocketRoutePath{value: tc.input}).Validate(), core.ErrExchangeContract)
				}
				return
			}
			parsed, parseErr := url.ParseRequestURI(tc.input)
			if parseErr != nil || path.Clean(tc.input) != tc.input || parsed.Path != tc.input || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Host != "" || parsed.Scheme != "" {
				t.Fatalf("valid fixture is not exact Go path: %+v,%v", parsed, parseErr)
			}
			if err != nil || got.String() != tc.input || got.Validate() != nil {
				t.Fatalf("route admission=(%q,%v),want exact %q", got.String(), err, tc.input)
			}
			again, againErr := ParseSocketRoutePath(got.String())
			if againErr != nil || again != got {
				t.Fatalf("route roundtrip=(%q,%v),want unchanged nominal", again.String(), againErr)
			}
		})
	}
}
