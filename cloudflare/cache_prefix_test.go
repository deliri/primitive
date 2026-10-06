package cloudflare

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestCachePrefixRepresentationBoundaries(t *testing.T) {
	t.Parallel()
	if got, err := NewCachePurgePrefix(core.HTTPEndpoint{}); !errors.Is(err, core.ErrCloudflareBinding) || got != (CachePurgePrefix{}) {
		t.Fatalf("zero endpoint = %+v/%v, want zero prefix and binding refusal", got, err)
	}
	for _, tc := range []struct {
		name, source, want string
		wantErr            error
	}{
		{name: "root scope is explicit", source: "https://media.example.test", want: "media.example.test"},
		{name: "full object path retains filename", source: "https://media.example.test/owned/document.pdf", want: "media.example.test/owned/document.pdf"},
		{name: "trailing separator retains directory scope", source: "https://media.example.test/owned/", want: "media.example.test/owned/"},
		{name: "scheme is absent in provider prefix", source: "http://media.example.test/owned", want: "media.example.test/owned"},
		{name: "encoded filename preserves escaped path", source: "https://media.example.test/owned/a%20b.pdf", want: "media.example.test/owned/a%20b.pdf"},
		{name: "query cannot silently widen scope", source: "https://media.example.test/owned?a=b", wantErr: core.ErrCloudflareBinding},
		{name: "empty query is still refused", source: "https://media.example.test/owned?", wantErr: core.ErrCloudflareBinding},
		{name: "below provider separator ceiling", source: "https://media.example.test" + strings.Repeat("/a", core.CloudflareCachePrefixMaximumSeparators-1), want: "media.example.test" + strings.Repeat("/a", core.CloudflareCachePrefixMaximumSeparators-1)},
		{name: "at provider separator ceiling", source: "https://media.example.test" + strings.Repeat("/a", core.CloudflareCachePrefixMaximumSeparators), want: "media.example.test" + strings.Repeat("/a", core.CloudflareCachePrefixMaximumSeparators)},
		{name: "above provider separator ceiling", source: "https://media.example.test" + strings.Repeat("/a", core.CloudflareCachePrefixMaximumSeparators+1), wantErr: core.ErrCloudflareBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			target, err := core.ParseHTTPEndpoint(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			got, err := NewCachePurgePrefix(target)
			if !errors.Is(err, tc.wantErr) || got.String() != tc.want {
				t.Fatalf("prefix = %q/%v, want %q/%v", got.String(), err, tc.want, tc.wantErr)
			}
			if tc.wantErr == nil && got.Validate() != nil {
				t.Fatalf("accepted prefix validation = %v, want nil", got.Validate())
			}
			if tc.wantErr != nil && got != (CachePurgePrefix{}) {
				t.Fatalf("refused prefix = %+v, want zero", got)
			}
		})
	}
}

func TestCachePrefixPurgeProviderLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                 string
		cancel, zero, refuse bool
		wantCalls            int64
		wantErr              error
	}{
		{name: "full object prefix binds all cache variants", wantCalls: 1},
		{name: "zero prefix performs no effect", zero: true, wantErr: core.ErrCloudflareContract},
		{name: "cancelled purge performs no effect", cancel: true, wantErr: context.Canceled},
		{name: "provider refusal cannot become accepted", refuse: true, wantCalls: 1, wantErr: core.ErrCloudflareResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			server, file := cacheFixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				wantPath := core.CloudflareAPIZonesPath + strings.Repeat("b", core.CloudflareIdentityCharacters) + core.CloudflareCachePurgePath
				if r.Method != http.MethodPost || r.Host != core.CloudflareAPIHost || r.URL.Path != wantPath || r.Header.Get(exchange.StandardHeaderAuthorization.String()) != "Bearer cloudflare-test-token" {
					t.Errorf("prefix request = %s %s %s, want exact authenticated zone POST", r.Method, r.Host, r.URL.Path)
				}
				var got cachePrefixPurgeWire
				if err := json.UnmarshalRead(http.MaxBytesReader(w, r.Body, 4096), &got, json.RejectUnknownMembers(true)); err != nil || got.Prefixes != [1]string{"media.example.test/owned/document.pdf"} {
					t.Errorf("purge body = %+v/%v, want one full object prefix", got, err)
				}
				w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				body, err := core.MarshalCanonicalJSONDocument(apiEnvelope[cachePurgeWire]{Success: new(!tc.refuse)})
				if err != nil {
					t.Error(err)
					return
				}
				if _, err := w.Write(body); err != nil {
					t.Error(err)
				}
			})
			prefix, err := NewCachePurgePrefix(file.URL)
			if err != nil {
				t.Fatal(err)
			}
			if tc.zero {
				prefix = CachePurgePrefix{}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			got, err := server.PurgePrefix(ctx, CachePrefixPurgeRequest{Prefix: prefix}, testPolicy())
			if !errors.Is(err, tc.wantErr) || calls.Load() != tc.wantCalls {
				t.Fatalf("purge error/calls = %v/%d, want %v/%d", err, calls.Load(), tc.wantErr, tc.wantCalls)
			}
			if tc.wantErr != nil {
				if got != (CachePrefixPurgeReceipt{}) {
					t.Fatalf("refused receipt = %+v, want zero", got)
				}
				return
			}
			if got.Prefix != prefix || got.Zone != server.zone || got.Validate() != nil {
				t.Fatalf("receipt = %+v, want exact validated prefix and zone", got)
			}
		})
	}
}
