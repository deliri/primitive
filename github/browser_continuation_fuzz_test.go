package github

import (
	"errors"
	"math"
	"net/url"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func FuzzBrowserOAuthContinuationSemanticClosure(f *testing.F) {
	seed := url.URL{Scheme: "https", Host: core.GitHubAPIHost, Path: githubBrowserEmailPath}
	query := url.Values{}
	query.Set("page", "2")
	query.Set("per_page", strconv.Itoa(core.GitHubTagPageMaximumEntries))
	seed.RawQuery = query.Encode()
	f.Add(seed.String(), uint64(1))
	f.Add(seed.String(), uint64(0))
	f.Add(seed.String(), uint64(math.MaxUint64))
	f.Add("https://foreign.example/user/emails?page=2&per_page=100", uint64(1))
	f.Fuzz(func(t *testing.T, target string, page uint64) {
		client := clientFixture(t, apiAuthority)
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("client.Close() = %v, want nil", err)
			}
		}()
		name, err := core.ParseHTTPHeaderName(headerLink)
		if err != nil {
			t.Fatal(err)
		}
		value, err := exchange.NewHeaderValue("<" + target + `>; rel="next"`)
		if err != nil {
			if !errors.Is(err, core.ErrExchangeContract) {
				t.Fatalf("header admission = %v, want Exchange identity", err)
			}
			return
		}
		headers := exchange.CapturedHeaders{Values: []exchange.Header{{Name: name, Values: []exchange.HeaderValue{value}}}}
		got, err := client.nextBrowserEmailPage(headers, page)
		parsed, parseErr := url.Parse(target)
		admitted := parseErr == nil && page != math.MaxUint64
		if admitted {
			query, queryErr := url.ParseQuery(parsed.RawQuery)
			admitted = queryErr == nil && parsed.Scheme == "https" && parsed.Host == core.GitHubAPIHost && parsed.User == nil && parsed.Fragment == "" && parsed.Path == githubBrowserEmailPath && len(query) == 2 && len(query["page"]) == 1 && query.Get("page") == strconv.FormatUint(page+1, 10) && len(query["per_page"]) == 1 && query.Get("per_page") == strconv.Itoa(core.GitHubTagPageMaximumEntries)
		}
		if admitted {
			if err != nil || got != page+1 {
				t.Fatalf("continuation = (%d,%v), want immediate successor %d", got, err, page+1)
			}
		} else if got != 0 || (!errors.Is(err, core.ErrGitHubBinding) && !errors.Is(err, core.ErrGitHubResponse)) {
			t.Fatalf("continuation refusal = (%d,%v), want zero and typed provider refusal", got, err)
		}
	})
}
