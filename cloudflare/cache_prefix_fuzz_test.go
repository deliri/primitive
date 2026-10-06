package cloudflare

import (
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzCachePurgePrefixClosure(f *testing.F) {
	endpoint, err := core.ParseHTTPEndpoint("https://media.example.test/owned/document.pdf")
	if err != nil {
		f.Fatal(err)
	}
	seed, err := NewCachePurgePrefix(endpoint)
	if err != nil || seed.Validate() != nil {
		f.Fatalf("prefix seed = %+v/%v, want valid", seed, err)
	}
	f.Add(seed.endpoint.String())
	f.Add("https://media.example.test/owned?query")
	f.Add("https://media.example.test" + strings.Repeat("/a", core.CloudflareCachePrefixMaximumSeparators+1))
	f.Fuzz(func(t *testing.T, source string) {
		endpoint, err := core.ParseHTTPEndpoint(source)
		if err != nil {
			return
		} // Core owns admission of HTTP URL representation.
		independent, err := url.Parse(endpoint.String())
		if err != nil {
			t.Fatal(err)
		}
		got, err := NewCachePurgePrefix(endpoint)
		wantValid := independent.User == nil && independent.RawQuery == "" && !independent.ForceQuery && independent.Fragment == "" && strings.Count(independent.Path, "/") <= core.CloudflareCachePrefixMaximumSeparators
		if !wantValid {
			if !errors.Is(err, core.ErrCloudflareBinding) || got != (CachePurgePrefix{}) {
				t.Fatalf("prefix = %+v/%v, want zero and binding refusal", got, err)
			}
			return
		}
		want := independent.Host + independent.EscapedPath()
		if err != nil || got.Validate() != nil || got.String() != want || got.endpoint != endpoint {
			t.Fatalf("prefix = %q/%v, want exact %q", got.String(), err, want)
		}
		again, err := NewCachePurgePrefix(got.endpoint)
		if err != nil || again != got {
			t.Fatalf("prefix reconstruction = %+v/%v, want %+v/nil", again, err, got)
		}
	})
}
