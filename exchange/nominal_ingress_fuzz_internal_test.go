package exchange

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzMethodJSONSemanticClosure(f *testing.F) {
	// The independent oracle binds each published nominal arm to Go's token,
	// rather than asking Method.String or parseMethod to grade themselves.
	methods := []struct {
		value Method
		token string
	}{
		{MethodGet, http.MethodGet}, {MethodHead, http.MethodHead},
		{MethodPost, http.MethodPost}, {MethodPut, http.MethodPut},
		{MethodPatch, http.MethodPatch}, {MethodDelete, http.MethodDelete},
		{MethodOptions, http.MethodOptions},
	}
	for _, method := range methods {
		wire, err := method.value.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(wire)
	}
	minimum, err := MethodGet.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	for _, size := range []int{core.JSONDocumentMaximumBytes - 1, core.JSONDocumentMaximumBytes, core.JSONDocumentMaximumBytes + 1} {
		f.Add(append(bytes.Repeat([]byte{' '}, size-len(minimum)), minimum...))
	}
	for _, wire := range [][]byte{nil, []byte("null"), []byte(`"get"`), []byte(`"TRACE"`), []byte(`"GET" 0`), []byte(`"\ud800"`), {'"', 0xff, '"'}} {
		f.Add(wire)
	}
	f.Fuzz(func(t *testing.T, wire []byte) {
		if len(wire) > core.JSONDocumentMaximumBytes+1 {
			return
		}
		var token *string
		want := MethodUnknown
		if len(wire) <= core.JSONDocumentMaximumBytes && json.Unmarshal(wire, &token) == nil && token != nil {
			for _, method := range methods {
				if *token == method.token {
					want = method.value
				}
			}
		}
		got := MethodPatch
		err := got.UnmarshalJSON(wire)
		if want == MethodUnknown {
			if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrExchangeContract) || got != MethodPatch {
				t.Fatalf("method refusal = (%v, %v), want retained %v and both typed identities", got, err, MethodPatch)
			}
			return
		}
		if err != nil || got != want || got.String() != *token || got.Validate() != nil {
			t.Fatalf("method admission = (%v, %v), want exact %v", got, err, want)
		}
		first, err := got.MarshalJSON()
		var again Method
		if err != nil || again.UnmarshalJSON(first) != nil || again != want {
			t.Fatalf("method canonical round trip = (%v, %v), want %v", again, err, want)
		}
		second, err := again.MarshalJSON()
		if err != nil || !bytes.Equal(first, second) {
			t.Fatalf("method canonical stability = (%q, %v), want %q", second, err, first)
		}
	})
}

func FuzzParseBasicAuthorizationIdentitySemanticClosure(f *testing.F) {
	for _, identity := range []BasicAuthorizationIdentity{"i", "é界🔌", BasicAuthorizationIdentity(strings.Repeat("i", BasicAuthorizationIdentityMaximumBytes-1)), BasicAuthorizationIdentity(strings.Repeat("i", BasicAuthorizationIdentityMaximumBytes))} {
		if err := identity.Validate(); err != nil {
			f.Fatal(err)
		}
		f.Add(identity.String())
	}
	for _, raw := range []string{"", "a:b", "a\x00", "a\u0085", "\xff", strings.Repeat("i", BasicAuthorizationIdentityMaximumBytes+1)} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		wantAccepted := len(raw) > 0 && len(raw) <= BasicAuthorizationIdentityMaximumBytes && utf8.ValidString(raw) &&
			!strings.ContainsRune(raw, ':') && strings.IndexFunc(raw, unicode.IsControl) < 0
		got, err := ParseBasicAuthorizationIdentity(raw)
		if !wantAccepted {
			if !errors.Is(err, core.ErrExchangeContract) || got != "" {
				t.Fatalf("identity refusal = (%q, %v), want zero and contract refusal", got, err)
			}
			return
		}
		if err != nil || got.String() != raw || got.Validate() != nil {
			t.Fatalf("identity admission = (%q, %v), want exact %q", got, err, raw)
		}
		wire, err := got.MarshalJSON()
		var again BasicAuthorizationIdentity
		if err != nil || again.UnmarshalJSON(wire) != nil || again != got {
			t.Fatalf("identity JSON projection = (%q, %v), want %q", again, err, got)
		}
	})
}

func FuzzParseIdempotencyKeySemanticClosure(f *testing.F) {
	for _, key := range []IdempotencyKey{{value: "!"}, {value: "identity-~"}, {value: strings.Repeat("~", IdempotencyKeyMaximumBytes-1)}, {value: strings.Repeat("~", IdempotencyKeyMaximumBytes)}} {
		if err := key.Validate(); err != nil {
			f.Fatal(err)
		}
		f.Add(key.String())
	}
	for _, raw := range []string{"", "key space", "key\x7f", "\xff", strings.Repeat("!", IdempotencyKeyMaximumBytes+1)} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		wantAccepted := len(raw) > 0 && len(raw) <= IdempotencyKeyMaximumBytes &&
			strings.IndexFunc(raw, func(r rune) bool { return r < '!' || r > '~' }) < 0
		got, err := ParseIdempotencyKey(raw)
		if !wantAccepted {
			if !errors.Is(err, core.ErrExchangeContract) || got != (IdempotencyKey{}) || !got.IsZero() {
				t.Fatalf("key refusal = (%v, %v), want zero and contract refusal", got, err)
			}
			return
		}
		if err != nil || got.String() != raw || got.IsZero() || got.Validate() != nil {
			t.Fatalf("key admission = (%q, %v), want exact %q", got.String(), err, raw)
		}
		again, err := ParseIdempotencyKey(got.String())
		if err != nil || again != got {
			t.Fatalf("key canonical round trip = (%v, %v), want %v", again, err, got)
		}
	})
}

func FuzzParseSocketRoutePathSemanticClosure(f *testing.F) {
	for _, route := range []SocketRoutePath{{value: "/"}, {value: "/a/b"}, {value: "/" + strings.Repeat("a", SocketRoutePathMaximumBytes-2)}, {value: "/" + strings.Repeat("a", SocketRoutePathMaximumBytes-1)}} {
		if err := route.Validate(); err != nil {
			f.Fatal(err)
		}
		f.Add(route.String())
	}
	for _, raw := range []string{"", "/a/../b", "//host", "/a%2fb", "/?", "/#fragment", "/\n", "/" + strings.Repeat("a", SocketRoutePathMaximumBytes)} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > SocketRoutePathMaximumBytes+1 {
			return
		}
		// Go owns URI parsing and path normalization. The typed socket requires
		// the parsed path alone to reproduce the entire supplied target.
		parsed, parseErr := url.ParseRequestURI(raw)
		wantAccepted := parseErr == nil && len(raw) > 0 && len(raw) <= SocketRoutePathMaximumBytes &&
			strings.HasPrefix(raw, "/") && path.Clean(raw) == raw && parsed.Path == raw &&
			parsed.RawPath == "" && parsed.RawQuery == "" && !parsed.ForceQuery && parsed.Fragment == "" && parsed.Host == "" && parsed.Scheme == ""
		got, err := ParseSocketRoutePath(raw)
		if !wantAccepted {
			if !errors.Is(err, core.ErrExchangeContract) || got != (SocketRoutePath{}) {
				t.Fatalf("route refusal = (%v, %v), want zero and contract refusal", got, err)
			}
			return
		}
		if err != nil || got.String() != raw || got.Validate() != nil {
			t.Fatalf("route admission = (%q, %v), want exact %q", got.String(), err, raw)
		}
		again, err := ParseSocketRoutePath(got.String())
		if err != nil || again != got {
			t.Fatalf("route canonical round trip = (%v, %v), want %v", again, err, got)
		}
	})
}
