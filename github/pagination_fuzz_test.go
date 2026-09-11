package github

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"net/url"
	"testing"
)

func FuzzGitHubPaginationQuerySemanticClosure(f *testing.F) {
	seed := url.Values{"page": {"2"}, "per_page": {"100"}}
	f.Add(seed.Encode())
	f.Add("page=2&page=3&per_page=100")
	f.Add("page=0&per_page=100")
	f.Add("")
	f.Fuzz(func(t *testing.T, rawQuery string) {
		if len(rawQuery) > 65536 {
			t.Skip("query exceeds bounded header oracle custody")
		}
		client := clientFixture(t, "https://api.github.com")
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("Close()=%v, want nil", err)
			}
		}()
		endpoint := url.URL{Scheme: "https", Host: "api.github.com", Path: "/repos/owner/repository/tags", RawQuery: rawQuery}
		candidate := "<" + endpoint.String() + `>; rel="next"`
		value, valueErr := exchange.NewHeaderValue(candidate)
		if valueErr != nil {
			if !errors.Is(valueErr, core.ErrExchangeContract) {
				t.Fatalf("header admission=%v, want typed Exchange refusal", valueErr)
			}
			return
		} // Exchange owns HTTP field admission; GitHub receives its typed value.
		name, err := core.ParseHTTPHeaderName(headerLink)
		if err != nil {
			t.Fatal(err)
		}
		headers := exchange.CapturedHeaders{Values: []exchange.Header{{Name: name, Values: []exchange.HeaderValue{value}}}}
		got, err := client.nextTagPage(headers, TagPageRequest{Repository: parsedRepository(t, "owner/repository"), Page: 1})
		query, queryErr := url.ParseQuery(rawQuery)
		wantAccepted := queryErr == nil && len(query) == 2 && len(query["page"]) == 1 && query.Get("page") == "2" && len(query["per_page"]) == 1 && query.Get("per_page") == "100"
		if wantAccepted {
			if err != nil || got != 2 {
				t.Fatalf("continuation=%d/%v, want page 2 for exact query %q", got, err, rawQuery)
			}
		} else if !errors.Is(err, core.ErrGitHubBinding) || got != 0 {
			t.Fatalf("continuation=%d/%v, want zero typed binding refusal for %q", got, err, rawQuery)
		}
	})
}
