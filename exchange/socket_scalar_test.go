package exchange_test

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"unsafe"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestSocketUniqueQuerySelectedScalar(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, query, want        string
		limit                    uint64
		present, refused, escape bool
	}{
		{name: "absent query is neutral", limit: 8},
		{name: "unrelated provider keys stay neutral", query: "scope=openid&authuser=0&prompt=consent", limit: 8},
		{name: "ordinary selected value", query: "state=abc", limit: 8, present: true, want: "abc"},
		{name: "present empty differs from absent", query: "state=", limit: 8, present: true},
		{name: "bare name is present empty", query: "state", limit: 8, present: true},
		{name: "plus uses native space decoding", query: "state=a+b", limit: 8, present: true, want: "a b"},
		{name: "escaped delimiter remains one value", query: "state=a%26b%3Dc", limit: 8, present: true, want: "a&b=c"},
		{name: "encoded name selects same identity", query: "%73tate=abc", limit: 8, present: true, want: "abc"},
		{name: "uppercase is a different name", query: "STATE=abc", limit: 8},
		{name: "unrelated malformed value is not decoded", query: "foreign=%zz&state=abc", limit: 8, present: true, want: "abc"},
		{name: "literal below decoded ceiling", query: "state=ab", limit: 3, present: true, want: "ab"},
		{name: "literal at decoded ceiling", query: "state=abc", limit: 3, present: true, want: "abc"},
		{name: "literal above decoded ceiling", query: "state=abcd", limit: 3, refused: true},
		{name: "escaped below decoded ceiling", query: "state=%61%62", limit: 3, present: true, want: "ab"},
		{name: "escaped at decoded ceiling", query: "state=%61%62%63", limit: 3, present: true, want: "abc"},
		{name: "escaped above decoded ceiling", query: "state=%61%62%63%64", limit: 3, refused: true},
		{name: "mixed wire width at decoded ceiling", query: "state=a%62c", limit: 3, present: true, want: "abc"},
		{name: "many equals belong to scalar", query: "state=a=b=c", limit: 5, present: true, want: "a=b=c"},
		{name: "identical duplicate is refused", query: "state=a&state=a", limit: 8, refused: true},
		{name: "conflicting duplicate is refused", query: "state=a&state=b", limit: 8, refused: true},
		{name: "encoded duplicate cannot hide", query: "state=a&%73tate=a", limit: 8, refused: true},
		{name: "empty duplicate is still duplicate", query: "state=&state=", limit: 8, refused: true},
		{name: "malformed selected escape keeps native cause", query: "state=%zz", limit: 8, refused: true, escape: true},
		{name: "truncated selected escape keeps native cause", query: "state=%", limit: 8, refused: true, escape: true},
		{name: "malformed candidate name keeps native cause", query: "%7z=abc", limit: 8, refused: true, escape: true},
		{name: "raw semicolon is not query framing", query: "state=a;b", limit: 8, refused: true},
		{name: "escaped semicolon remains data", query: "state=a%3Bb", limit: 8, present: true, want: "a;b"},
		{name: "zero budget is refused", query: "state=a", refused: true},
		{name: "signed-overflow budget is refused", query: "state=a", limit: math.MaxUint64, refused: true},
		{name: "largest signed budget cannot overflow bound", query: "state=a", limit: math.MaxInt64, present: true, want: "a"},
		{name: "minimum positive budget accepts one byte", query: "state=a", limit: 1, present: true, want: "a"},
		{name: "minimum positive budget rejects two bytes", query: "state=ab", limit: 1, refused: true},
		{name: "empty separators do not invent selections", query: "&&state=a&&", limit: 8, present: true, want: "a"},
		{name: "first key is retained through unrelated tail", query: "state=a&scope=openid&authuser=0", limit: 8, present: true, want: "a"},
		{name: "last key is found after unrelated prefix", query: "scope=openid&authuser=0&state=a", limit: 8, present: true, want: "a"},
		{name: "encoded NUL remains caller-owned data", query: "state=%00", limit: 1, present: true, want: "\x00"},
		{name: "UTF8 byte ceiling is exact", query: "state=%C3%A9", limit: 2, present: true, want: "é"},
		{name: "UTF8 above byte ceiling is refused", query: "state=%C3%A9", limit: 1, refused: true},
		{name: "large unrelated value is not copied", query: "foreign=" + strings.Repeat("x", 65536) + "&state=a", limit: 1, present: true, want: "a"},
		{name: "large unrelated name is not decoded", query: strings.Repeat("x", 65536) + "=%zz&state=a", limit: 1, present: true, want: "a"},
		{name: "large selected extent refuses before decode", query: "state=" + strings.Repeat("%61", 65536), limit: 8, refused: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			request.URL.RawQuery = tc.query
			call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
			if err != nil {
				t.Fatal(err)
			}
			name, err := exchange.ParseQueryParameterName("state")
			if err != nil {
				t.Fatal(err)
			}
			var maximum core.ByteCount
			if tc.limit != 0 {
				maximum = mustByteCount(t, tc.limit)
			}
			got, err := call.UniqueQuery(name, maximum)
			if tc.refused {
				if !errors.Is(err, core.ErrExchangeContract) || got != (exchange.ScalarValue{}) {
					t.Fatalf("query=(%v,%v), want zero and contract refusal", got, err)
				}
			} else if err != nil || got.Present() != tc.present || got.Value() != tc.want {
				t.Fatalf("query=(present %t,%q,%v), want (%t,%q,nil)", got.Present(), got.Value(), err, tc.present, tc.want)
			}
			var native url.EscapeError
			if errors.As(err, &native) != tc.escape {
				t.Fatalf("native escape cause=%t, want %t: %v", errors.As(err, &native), tc.escape, err)
			}
			if request.URL.RawQuery != tc.query {
				t.Fatalf("request query changed to %q, want %q", request.URL.RawQuery, tc.query)
			}
		})
	}
}

func TestSocketUniqueCookieSelectedScalar(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		lines            []string
		limit            uint64
		want             string
		present, refused bool
	}{
		{name: "absent cookie headers are neutral", limit: 8},
		{name: "unrelated cookie remains neutral", lines: []string{"other=a"}, limit: 8},
		{name: "ordinary selected cookie", lines: []string{"_oauth=abc"}, limit: 8, present: true, want: "abc"},
		{name: "empty selected cookie is present", lines: []string{"_oauth="}, limit: 8, present: true},
		{name: "native quoted scalar loses quotes", lines: []string{"_oauth=\"abc\""}, limit: 3, present: true, want: "abc"},
		{name: "empty quoted scalar is present", lines: []string{"_oauth=\"\""}, limit: 1, present: true},
		{name: "whitespace around pair uses native trimming", lines: []string{" \t_oauth=abc\t "}, limit: 3, present: true, want: "abc"},
		{name: "case-distinct cookie is unrelated", lines: []string{"_OAUTH=abc"}, limit: 3},
		{name: "percent text is not URL decoded", lines: []string{"_oauth=%61"}, limit: 3, present: true, want: "%61"},
		{name: "plus is not converted to space", lines: []string{"_oauth=a+b"}, limit: 3, present: true, want: "a+b"},
		{name: "selected below ceiling", lines: []string{"_oauth=ab"}, limit: 3, present: true, want: "ab"},
		{name: "selected at ceiling", lines: []string{"_oauth=abc"}, limit: 3, present: true, want: "abc"},
		{name: "selected above ceiling", lines: []string{"_oauth=abcd"}, limit: 3, refused: true},
		{name: "quoted below ceiling", lines: []string{"_oauth=\"ab\""}, limit: 3, present: true, want: "ab"},
		{name: "quoted above ceiling", lines: []string{"_oauth=\"abcd\""}, limit: 3, refused: true},
		{name: "same-value duplicate within header refuses", lines: []string{"_oauth=a; _oauth=a"}, limit: 8, refused: true},
		{name: "conflicting duplicate within header refuses", lines: []string{"_oauth=a; _oauth=b"}, limit: 8, refused: true},
		{name: "duplicate across header lines refuses", lines: []string{"_oauth=a", "_oauth=a"}, limit: 8, refused: true},
		{name: "empty duplicate refuses", lines: []string{"_oauth=; _oauth="}, limit: 8, refused: true},
		{name: "quoted duplicate cannot hide", lines: []string{"_oauth=a; _oauth=\"a\""}, limit: 8, refused: true},
		{name: "bare selected name refuses native framing", lines: []string{"_oauth"}, limit: 8, refused: true},
		{name: "unterminated selected quote refuses", lines: []string{"_oauth=\"abc"}, limit: 8, refused: true},
		{name: "selected control byte refuses", lines: []string{"_oauth=a\x00b"}, limit: 8, refused: true},
		{name: "selected nonASCII byte refuses", lines: []string{"_oauth=é"}, limit: 8, refused: true},
		{name: "selected backslash refuses", lines: []string{"_oauth=a\\b"}, limit: 8, refused: true},
		{name: "selected interior quote refuses", lines: []string{"_oauth=a\"b"}, limit: 8, refused: true},
		{name: "native quoted spaces remain data", lines: []string{"_oauth=\"a b\""}, limit: 3, present: true, want: "a b"},
		{name: "equals remains cookie data", lines: []string{"_oauth=a=b"}, limit: 3, present: true, want: "a=b"},
		{name: "native comma remains cookie data", lines: []string{"_oauth=a,b"}, limit: 3, present: true, want: "a,b"},
		{name: "zero cookie budget refuses", lines: []string{"_oauth=a"}, refused: true},
		{name: "overflow cookie budget refuses", lines: []string{"_oauth=a"}, limit: math.MaxUint64, refused: true},
		{name: "largest signed cookie budget accepts", lines: []string{"_oauth=a"}, limit: math.MaxInt64, present: true, want: "a"},
		{name: "minimum budget one byte accepts", lines: []string{"_oauth=a"}, limit: 1, present: true, want: "a"},
		{name: "minimum budget two bytes refuses", lines: []string{"_oauth=ab"}, limit: 1, refused: true},
		{name: "selected last after unrelated pairs", lines: []string{"one=a;two=b;_oauth=c"}, limit: 1, present: true, want: "c"},
		{name: "selected first before unrelated pairs", lines: []string{"_oauth=c;one=a;two=b"}, limit: 1, present: true, want: "c"},
		{name: "unrelated malformed cookie is not admitted", lines: []string{"broken; _oauth=a"}, limit: 1, present: true, want: "a"},
		{name: "large unrelated cookie is not materialized", lines: []string{"other=" + strings.Repeat("x", 65536) + ";_oauth=a"}, limit: 1, present: true, want: "a"},
		{name: "large selected cookie refuses", lines: []string{"_oauth=" + strings.Repeat("x", 65536)}, limit: 8, refused: true},
		{name: "empty unrelated separators stay neutral", lines: []string{";;"}, limit: 1},
		{name: "different headers may be unrelated", lines: []string{"one=a", "two=b", "_oauth=c"}, limit: 1, present: true, want: "c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			for _, line := range tc.lines {
				request.Header.Add("Cookie", line)
			}
			call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
			if err != nil {
				t.Fatal(err)
			}
			name, err := exchange.ParseCookieName("_oauth")
			if err != nil {
				t.Fatal(err)
			}
			var maximum core.ByteCount
			if tc.limit != 0 {
				maximum = mustByteCount(t, tc.limit)
			}
			got, err := call.UniqueCookie(name, maximum)
			if tc.refused {
				if !errors.Is(err, core.ErrExchangeContract) || got != (exchange.ScalarValue{}) {
					t.Fatalf("cookie=(%v,%v), want zero and contract refusal", got, err)
				}
			} else if err != nil || got.Present() != tc.present || got.Value() != tc.want {
				t.Fatalf("cookie=(present %t,%q,%v), want (%t,%q,nil)", got.Present(), got.Value(), err, tc.present, tc.want)
			}
		})
	}
}

func TestSocketScalarCancellationAndInvalidCapabilities(t *testing.T) {
	t.Parallel()
	for _, cookie := range []bool{false, true} {
		t.Run(map[bool]string{false: "query", true: "cookie"}[cookie], func(t *testing.T) {
			t.Parallel()
			name, _ := exchange.ParseQueryParameterName("state")
			cookieName, _ := exchange.ParseCookieName("_oauth")
			limit := mustByteCount(t, 8)
			invoke := func(call exchange.SocketServerCall) (exchange.ScalarValue, error) {
				if cookie {
					return call.UniqueCookie(cookieName, limit)
				}
				return call.UniqueQuery(name, limit)
			}
			got, err := invoke(exchange.SocketServerCall{})
			if !errors.Is(err, core.ErrExchangeContract) || got != (exchange.ScalarValue{}) {
				t.Fatalf("unset call=(%v,%v), want zero/contract", got, err)
			}
			ctx, cancel := context.WithCancelCause(t.Context())
			cause := errors.New("owned cancellation")
			cancel(cause)
			call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil))
			if err != nil {
				t.Fatal(err)
			}
			got, err = invoke(call)
			if !errors.Is(err, context.Canceled) || !errors.Is(err, cause) || got != (exchange.ScalarValue{}) {
				t.Fatalf("cancelled call=(%v,%v), want zero and both native causes", got, err)
			}
		})
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.URL = nil
	call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
	if err != nil {
		t.Fatal(err)
	}
	name, _ := exchange.ParseQueryParameterName("state")
	got, err := call.UniqueQuery(name, mustByteCount(t, 8))
	if !errors.Is(err, core.ErrExchangeContract) || got != (exchange.ScalarValue{}) {
		t.Fatalf("nil URL=(%v,%v), want zero/contract", got, err)
	}
}

func TestSocketScalarNativeCausesAndUnsetSelectors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, cookie string }{
		{name: "native missing equals cause", cookie: "_oauth"},
		{name: "native malformed quoted value cause", cookie: "_oauth=\"unfinished"},
		{name: "native invalid cookie byte cause", cookie: "_oauth=a\x00b"},
		{name: "native request name whitespace cannot become absence", cookie: "_oauth =0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			request.Header.Add("Cookie", tc.cookie)
			call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
			if err != nil {
				t.Fatal(err)
			}
			name, err := exchange.ParseCookieName("_oauth")
			if err != nil {
				t.Fatal(err)
			}
			_, nativeErr := http.ParseCookie(tc.cookie)
			if nativeErr == nil {
				t.Fatalf("native fixture refusal=%v, want native parser refusal", nativeErr)
			}
			got, err := call.UniqueCookie(name, mustByteCount(t, 64))
			if !errors.Is(err, core.ErrExchangeContract) || !errors.Is(err, nativeErr) || got != (exchange.ScalarValue{}) {
				t.Fatalf("native refusal=(%v,%v), want zero, contract and %v", got, err, nativeErr)
			}
		})
	}
	for _, cookie := range []bool{false, true} {
		name := "query unset selector"
		if cookie {
			name = "cookie unset selector"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
			if err != nil {
				t.Fatal(err)
			}
			var got exchange.ScalarValue
			if cookie {
				got, err = call.UniqueCookie(exchange.CookieName{}, mustByteCount(t, 1))
			} else {
				got, err = call.UniqueQuery(exchange.QueryParameterName{}, mustByteCount(t, 1))
			}
			if !errors.Is(err, core.ErrExchangeContract) || got != (exchange.ScalarValue{}) {
				t.Fatalf("unset selector=(%v,%v), want zero/contract", got, err)
			}
		})
	}
}

func TestSocketScalarDoesNotPinUnrelatedRequestStorage(t *testing.T) {
	t.Parallel()
	for _, cookie := range []bool{false, true} {
		name := "query bounded result owns bytes"
		if cookie {
			name = "cookie bounded result owns bytes"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			prefix := "foreign=" + strings.Repeat("x", 65536)
			raw := prefix + "&state=abc"
			if cookie {
				raw = prefix + ";_oauth=abc"
			}
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			request.URL.RawQuery = raw
			if cookie {
				request.Header.Set("Cookie", raw)
			}
			call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
			if err != nil {
				t.Fatal(err)
			}
			var got exchange.ScalarValue
			if cookie {
				selector, _ := exchange.ParseCookieName("_oauth")
				got, err = call.UniqueCookie(selector, mustByteCount(t, 3))
			} else {
				selector, _ := exchange.ParseQueryParameterName("state")
				got, err = call.UniqueQuery(selector, mustByteCount(t, 3))
			}
			if err != nil || !got.Present() || got.Value() != "abc" {
				t.Fatalf("selected scalar=(%v,%v), want present abc", got, err)
			}
			// The unescaped native result would alias this exact suffix, keeping
			// the whole request target/header live after its owner is released.
			if unsafe.StringData(got.Value()) == unsafe.StringData(raw[len(raw)-3:]) {
				t.Fatalf("selected scalar storage aliases request=%t, want independent bounded allocation", true)
			}
		})
	}
}
