package github

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestTagPaginationBindingLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		page     uint32
		links    func(string) []string
		wantNext uint32
		wantErr  error
	}{
		{name: "absent continuation", page: 1},
		{name: "exact named repository continuation", page: 1, links: func(a string) []string { return []string{a + `/repos/owner/repository/tags?page=2&per_page=100`} }, wantNext: 2},
		{name: "numeric provider repository continuation", page: 1, links: func(a string) []string { return []string{a + `/repositories/123/tags?page=2&per_page=100`} }, wantNext: 2},
		{name: "identical continuation replay is idempotent", page: 1, links: func(a string) []string {
			return []string{a + `/repos/owner/repository/tags?page=2&per_page=100`, a + `/repos/owner/repository/tags?page=2&per_page=100`}
		}, wantNext: 2},
		{name: "conflicting later page cannot hide behind first match", page: 1, links: func(a string) []string {
			return []string{a + `/repos/owner/repository/tags?page=2&per_page=100`, a + `/repos/owner/repository/tags?page=3&per_page=100`}
		}, wantErr: core.ErrGitHubBinding},
		{name: "conflict is invariant under link order", page: 1, links: func(a string) []string {
			return []string{a + `/repos/owner/repository/tags?page=3&per_page=100`, a + `/repos/owner/repository/tags?page=2&per_page=100`}
		}, wantErr: core.ErrGitHubBinding},
		{name: "foreign authority cannot nominate continuation", page: 1, links: func(string) []string {
			return []string{`https://foreign.invalid/repos/owner/repository/tags?page=2&per_page=100`}
		}, wantErr: core.ErrGitHubBinding},
		{name: "foreign named repository cannot nominate continuation", page: 1, links: func(a string) []string { return []string{a + `/repos/other/repository/tags?page=2&per_page=100`} }, wantErr: core.ErrGitHubBinding},
		{name: "zero numeric provider identity is rejected", page: 1, links: func(a string) []string { return []string{a + `/repositories/0/tags?page=2&per_page=100`} }, wantErr: core.ErrGitHubBinding},
		{name: "duplicate page query is rejected", page: 1, links: func(a string) []string {
			return []string{a + `/repos/owner/repository/tags?page=2&page=2&per_page=100`}
		}, wantErr: core.ErrGitHubBinding},
		{name: "foreign user info is rejected", page: 1, links: func(a string) []string {
			return []string{strings.Replace(a, "http://", "http://user@", 1) + `/repos/owner/repository/tags?page=2&per_page=100`}
		}, wantErr: core.ErrGitHubBinding},
		{name: "fragment cannot alter continuation", page: 1, links: func(a string) []string { return []string{a + `/repos/owner/repository/tags?page=2&per_page=100#other`} }, wantErr: core.ErrGitHubBinding},
		{name: "maximum page can finish without a next relation", page: math.MaxUint32},
		{name: "maximum page cannot wrap to zero", page: math.MaxUint32, links: func(a string) []string { return []string{a + `/repos/owner/repository/tags?page=0&per_page=100`} }, wantErr: core.ErrGitHubResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/repos/owner/repository/tags" || r.URL.Query().Get("page") != fmt.Sprint(tc.page) {
					t.Errorf("request=%s, want original repository and page %d", r.URL, tc.page)
				}
				if tc.links != nil {
					var values []string
					for _, link := range tc.links(server.URL) {
						values = append(values, "<"+link+`>; rel="next"`)
					}
					w.Header().Set(headerLink, strings.Join(values, ", "))
				} else {
					w.Header().Set(headerLink, "<"+server.URL+`/repos/owner/repository/tags?page=1&per_page=100>; rel="first"`)
				}
				writeJSON(t, w, []tagWire{}, http.StatusOK)
			}))
			defer server.Close()
			client := clientFixture(t, server.URL)
			got, err := client.ReadTagPage(t.Context(), TagPageRequest{Repository: parsedRepository(t, "owner/repository"), Page: tc.page})
			if !errors.Is(err, tc.wantErr) || got.NextPage != tc.wantNext || len(got.Tags) != 0 {
				t.Fatalf("ReadTagPage()=%+v/%v, want next=%d, no tags and %v", got, err, tc.wantNext, tc.wantErr)
			}
			if tc.wantErr != nil && (got.Repository != (Repository{}) || got.Page != 0) {
				t.Fatalf("failed page=%+v, want zero projection", got)
			}
		})
	}
}
