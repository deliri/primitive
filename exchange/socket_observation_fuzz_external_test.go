package exchange_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func FuzzSocketHeaderQueryAndPathObservations(f *testing.F) {
	route, err := exchange.ParseSocketRoutePath("/scope")
	if err != nil {
		f.Fatal(err)
	}
	header, err := exchange.NewHeaderValue("opaque-value")
	if err != nil {
		f.Fatal(err)
	}
	value, err := header.Value()
	if err != nil {
		f.Fatal(err)
	}
	for _, count := range []uint8{0, 1, 2} {
		f.Add(value, count, uint16(len(value)), "", route.String(), "", false, false)
	}
	f.Add(value, uint8(1), uint16(len(value)-1), "", route.String(), "", false, false)
	f.Add(value, uint8(1), uint16(len(value)+1), "", route.String(), "", false, false)
	f.Add("", uint8(1), uint16(1), "", route.String(), "", false, false)
	f.Add("injected\r\n", uint8(1), uint16(64), "", route.String(), "", false, false)
	f.Add(value, uint8(1), uint16(0), "", route.String(), "", false, false)
	for _, size := range []int{exchange.SocketRequestTargetMaximumBytes - 1, exchange.SocketRequestTargetMaximumBytes, exchange.SocketRequestTargetMaximumBytes + 1} {
		f.Add(value, uint8(1), uint16(len(value)), strings.Repeat("q", size), route.String(), "", false, false)
	}
	f.Add(value, uint8(1), uint16(len(value)), "", route.String(), "", true, false)
	f.Add(value, uint8(1), uint16(len(value)), "", route.String(), "", false, true)
	f.Add(value, uint8(1), uint16(len(value)), "", route.String(), "/%73cope", false, false)
	f.Add(value, uint8(1), uint16(len(value)), "", "/foreign", "", false, false)
	endpoint := mustEndpoint(f, "https://example.invalid"+route.String())
	name := core.HTTPHeaderIdempotencyKey()
	f.Fuzz(func(t *testing.T, value string, count uint8, ceiling uint16, query, path, rawPath string, absentURL, forceQuery bool) {
		if count > 2 || len(value) > exchange.HeaderValueMaximumBytes+1 || len(query) > exchange.SocketRequestTargetMaximumBytes+1 || len(path) > exchange.SocketRoutePathMaximumBytes+1 || len(rawPath) > exchange.SocketRoutePathMaximumBytes+1 {
			return
		}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
		request.URL.Path, request.URL.RawPath, request.URL.RawQuery, request.URL.ForceQuery = path, rawPath, query, forceQuery
		if absentURL {
			request.URL = nil
		}
		for range count {
			request.Header.Add(name.String(), value)
		}
		before := slices.Clone(request.Header.Values(name.String()))
		writer := httptest.NewRecorder()
		call, err := exchange.NewSocketServerCall(writer, request)
		if err != nil {
			t.Fatal(err)
		}
		var limit core.ByteCount
		if ceiling != 0 {
			limit = mustByteCount(t, uint64(ceiling))
		}
		gotHeader, headerErr := call.UniqueHeader(name, limit)
		wantHeader := count == 1 && len(value) > 0 && len(value) <= int(ceiling) && len(value) <= exchange.HeaderValueMaximumBytes && goHeaderGrammarAccepts(t, value)
		if wantHeader {
			projected, err := gotHeader.Value()
			if headerErr != nil || err != nil || projected != value {
				t.Fatalf("unique field = (%q,%v,%v), want exact %q", projected, headerErr, err, value)
			}
		} else if !errors.Is(headerErr, core.ErrExchangeContract) || gotHeader != (exchange.HeaderValue{}) {
			t.Fatalf("field refusal = (%v,%v), want zero and typed refusal", gotHeader, headerErr)
		}
		gotQuery, queryErr := call.RawQuery()
		wantQuery := !absentURL && len(query) <= exchange.SocketRequestTargetMaximumBytes
		if wantQuery {
			if queryErr != nil || gotQuery != query {
				t.Fatalf("query observation = (%q,%v), want exact %q", gotQuery, queryErr, query)
			}
		} else if !errors.Is(queryErr, core.ErrExchangeContract) || gotQuery != "" {
			t.Fatalf("query refusal = (%q,%v), want absent and typed refusal", gotQuery, queryErr)
		}
		gotMatch, matchErr := call.MatchesEndpointPath(endpoint)
		wantMatch := !absentURL && path == route.String() && rawPath == ""
		if matchErr != nil || gotMatch != wantMatch {
			t.Fatalf("endpoint match = (%t,%v), want (%t,nil)", gotMatch, matchErr, wantMatch)
		}
		gotPathErr := exchange.ValidateSocketCallPath(call, route)
		wantPath := wantMatch && query == "" && !forceQuery
		if wantPath {
			if gotPathErr != nil {
				t.Fatalf("exact route error = %v, want nil", gotPathErr)
			}
		} else if !errors.Is(gotPathErr, core.ErrExchangeContract) {
			t.Fatalf("decorated/foreign route error = %v, want contract refusal", gotPathErr)
		}
		if !slices.Equal(before, request.Header.Values(name.String())) || writer.Body.Len() != 0 || len(writer.Header()) != 0 || writer.Flushed {
			t.Fatalf("request fields preserved/body/headers/flush=%t/%d/%d/%t, want true/0/0/false", slices.Equal(before, request.Header.Values(name.String())), writer.Body.Len(), len(writer.Header()), writer.Flushed)
		}
	})
}
