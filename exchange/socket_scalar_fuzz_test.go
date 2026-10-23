package exchange_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func FuzzSocketScalarSelectorNames(f *testing.F) {
	cookie, err := exchange.ParseCookieName("_oauth")
	if err != nil {
		f.Fatal(err)
	}
	query, err := exchange.ParseQueryParameterName("state")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(cookie.String())
	f.Add(query.String())
	f.Add("")
	f.Add("bad\x00name")
	f.Add("é")
	f.Add("a=b")
	f.Add("a b")
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 4096 {
			return
		}
		cookie, err := exchange.ParseCookieName(input)
		goErr := (&http.Cookie{Name: input}).Valid()
		if (err == nil) != (goErr == nil) {
			t.Fatalf("cookie selector=(%v,%v), native=%v", cookie, err, goErr)
		}
		if err != nil {
			if !errors.Is(err, core.ErrExchangeContract) || cookie != (exchange.CookieName{}) {
				t.Fatalf("cookie refusal=(%v,%v), want zero/contract", cookie, err)
			}
		} else if cookie.String() != input || cookie.Validate() != nil {
			t.Fatalf("cookie admitted=%v, want exact valid selector", cookie)
		}
		query, err := exchange.ParseQueryParameterName(input)
		if err != nil {
			if !errors.Is(err, core.ErrExchangeContract) || query != (exchange.QueryParameterName{}) {
				t.Fatalf("query refusal=(%v,%v), want zero/contract", query, err)
			}
			return
		}
		if query.String() != input || query.Validate() != nil {
			t.Fatalf("query admitted=%v, want exact valid selector", query)
		}
		canonical := url.QueryEscape(query.String())
		decoded, err := url.QueryUnescape(canonical)
		if err != nil || decoded != input {
			t.Fatalf("query selector roundtrip=(%q,%v), want %q", decoded, err, input)
		}
	})
}

func FuzzSocketUniqueQueryNativeScalar(f *testing.F) {
	name, err := exchange.ParseQueryParameterName("state")
	if err != nil {
		f.Fatal(err)
	}
	canonical := url.Values{}
	canonical.Add(name.String(), "opaque-value")
	f.Add(canonical.Encode(), uint16(12))
	f.Add("state=&state=", uint16(1))
	f.Add("state=%zz", uint16(8))
	f.Add("%73tate=a", uint16(1))
	f.Add("scope=%zz&state=a", uint16(1))
	f.Add("state=a%26b", uint16(3))
	f.Add("state=%61%62%63", uint16(2))
	f.Add("", uint16(1))
	f.Add("state=a", uint16(0))
	f.Fuzz(func(t *testing.T, raw string, ceiling uint16) {
		if len(raw) > 4096 {
			return
		}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		request.URL.RawQuery = raw
		call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
		if err != nil {
			t.Fatal(err)
		}
		var limit core.ByteCount
		if ceiling != 0 {
			limit = mustByteCount(t, uint64(ceiling))
		}
		got, err := call.UniqueQuery(name, limit)
		native, nativeErr := url.ParseQuery(raw)
		values := native[name.String()]
		wantAccepted := nativeErr == nil && ceiling != 0 && len(values) <= 1 && (len(values) == 0 || len(values[0]) <= int(ceiling))
		if wantAccepted && err != nil {
			t.Fatalf("native valid unique query refused: %v; values=%q ceiling=%d", err, values, ceiling)
		}
		if err != nil {
			if !errors.Is(err, core.ErrExchangeContract) || got != (exchange.ScalarValue{}) {
				t.Fatalf("query refusal=(%v,%v), want zero/contract", got, err)
			}
			return
		}
		if got.Present() != (len(values) == 1) || len(values) > 1 || got.Present() && (got.Value() != values[0] || len(got.Value()) > int(ceiling)) {
			t.Fatalf("query=(%t,%q), native=%q ceiling=%d", got.Present(), got.Value(), values, ceiling)
		}
		// Every accepted scalar has an independently encoded canonical form.
		if got.Present() {
			canonical := url.Values{}
			canonical.Add(name.String(), got.Value())
			request.URL.RawQuery = canonical.Encode()
			again, err := call.UniqueQuery(name, limit)
			if err != nil || again != got {
				t.Fatalf("query canonical=(%v,%v), want %v", again, err, got)
			}
		}
	})
}

func FuzzSocketUniqueCookieNativeScalar(f *testing.F) {
	name, err := exchange.ParseCookieName("_oauth")
	if err != nil {
		f.Fatal(err)
	}
	canonical := (&http.Cookie{Name: name.String(), Value: "opaque-value"}).String()
	f.Add(canonical, uint16(12))
	f.Add("_oauth=a;_oauth=a", uint16(8))
	f.Add("_oauth=\"a b\"", uint16(3))
	f.Add("_oauth", uint16(8))
	f.Add("_oauth=\"", uint16(8))
	f.Add("other=a;_oauth=b", uint16(1))
	f.Add("", uint16(1))
	f.Add("_oauth=a", uint16(0))
	f.Fuzz(func(t *testing.T, raw string, ceiling uint16) {
		if len(raw) > 4096 {
			return
		}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		request.Header.Add("Cookie", raw)
		call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
		if err != nil {
			t.Fatal(err)
		}
		var limit core.ByteCount
		if ceiling != 0 {
			limit = mustByteCount(t, uint64(ceiling))
		}
		got, err := call.UniqueCookie(name, limit)
		native, nativeErr := http.ParseCookie(raw)
		nativeCount := 0
		var nativeValue string
		for _, cookie := range native {
			if cookie.Name == name.String() {
				nativeCount++
				nativeValue = cookie.Value
			}
		}
		wantAccepted := nativeErr == nil && ceiling != 0 && nativeCount <= 1 && len(nativeValue) <= int(ceiling)
		if wantAccepted && err != nil {
			t.Fatalf("native valid unique cookie refused: %v; count=%d ceiling=%d", err, nativeCount, ceiling)
		}
		if err != nil {
			if !errors.Is(err, core.ErrExchangeContract) || got != (exchange.ScalarValue{}) {
				t.Fatalf("cookie refusal=(%v,%v), want zero/contract", got, err)
			}
			return
		}
		// Native Request.Cookies also ignores unrelated malformed pairs. The
		// selected pair itself is stricter: ParseCookie must admit it exactly.
		count := 0
		var value string
		for _, cookie := range request.Cookies() {
			if cookie.Name == name.String() {
				count++
				value = cookie.Value
			}
		}
		if got.Present() != (count == 1) || count > 1 || got.Present() && (got.Value() != value || len(value) > int(ceiling)) {
			t.Fatalf("cookie=(%t,%q), native count=%d value=%q ceiling=%d", got.Present(), got.Value(), count, value, ceiling)
		}
		if got.Present() {
			request.Header.Set("Cookie", (&http.Cookie{Name: name.String(), Value: got.Value()}).String())
			again, err := call.UniqueCookie(name, limit)
			if err != nil || again != got {
				t.Fatalf("cookie canonical=(%v,%v), want %v", again, err, got)
			}
		}
		if strings.Join(request.Header.Values("Cookie"), "") == "" && got.Present() {
			t.Fatalf("empty cookie projection=%v, want absent scalar", got)
		}
	})
}
