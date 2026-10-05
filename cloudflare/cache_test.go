package cloudflare

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func cacheFixture(t *testing.T, handler http.HandlerFunc) (CacheServer, CacheFilePurgeRequest) {
	t.Helper()
	options := testOptions(t)
	zone, err := ParseZoneID(strings.Repeat("b", core.CloudflareIdentityCharacters))
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewCacheServer(testExchange(t, handler), CacheServerOptions{Zone: zone, Token: options.Token, ResponseLimits: options.ResponseLimits})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	target, err := core.ParseHTTPEndpoint("https://media.example.test/owned/document.pdf")
	if err != nil {
		t.Fatal(err)
	}
	return server, CacheFilePurgeRequest{URL: target}
}

func TestCachePurgeProviderLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		body    func() ([]byte, error)
		wantErr error
		wantID  CachePurgeOperationID
	}{
		{name: "accepted exact URL retains provider operation", wantID: CachePurgeOperationID("purge-receipt"), body: func() ([]byte, error) {
			return core.MarshalCanonicalJSONDocument(apiEnvelope[cachePurgeWire]{Success: new(true), Result: cachePurgeWire{ID: CachePurgeOperationID("purge-receipt")}})
		}},
		{name: "provider refusal yields zero receipt", wantErr: core.ErrCloudflareResponse, body: func() ([]byte, error) {
			return core.MarshalCanonicalJSONDocument(apiEnvelope[cachePurgeWire]{Success: new(false), Errors: []APIIssue{{Code: 1134, Message: "rate limited"}}})
		}},
		{name: "missing optional operation remains absent after acceptance", body: func() ([]byte, error) {
			return core.MarshalCanonicalJSONDocument(apiEnvelope[cachePurgeWire]{Success: new(true)})
		}},
		{name: "absent success cannot manufacture acceptance", wantErr: core.ErrCloudflareResponse, body: func() ([]byte, error) { return core.MarshalCanonicalJSONDocument(apiEnvelope[cachePurgeWire]{}) }},
		{name: "success and errors conflict", wantErr: core.ErrCloudflareResponse, body: func() ([]byte, error) {
			return core.MarshalCanonicalJSONDocument(apiEnvelope[cachePurgeWire]{Success: new(true), Errors: []APIIssue{{Code: 1000}}})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body, err := tc.body()
			if err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int64
			server, request := cacheFixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if got := r.Header.Get(exchange.StandardHeaderAuthorization.String()); got != "Bearer cloudflare-test-token" {
					t.Errorf("authorization matches owned token = %t, want true", got == "Bearer cloudflare-test-token")
				}
				wantPath := core.CloudflareAPIZonesPath + strings.Repeat("b", core.CloudflareIdentityCharacters) + core.CloudflareCachePurgePath
				if r.Method != http.MethodPost || r.URL.Path != wantPath || r.Host != core.CloudflareAPIHost {
					t.Errorf("request=(%s,%s,%s), want zone-bound POST %s", r.Method, r.Host, r.URL.Path, wantPath)
				}
				got, err := core.DecodeStrictJSON[cacheFilePurgeWire](r.Body, core.DefaultStrictJSONLimits())
				if err != nil || got.Files != [1]string{"https://media.example.test/owned/document.pdf"} {
					t.Errorf("request body=(%+v,%v), want exactly the requested URL", got, err)
				}
				w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				if _, err := w.Write(body); err != nil {
					t.Error(err)
				}
			})
			got, err := server.PurgeFile(t.Context(), request, testPolicy())
			if !errors.Is(err, tc.wantErr) || calls.Load() != 1 {
				t.Fatalf("PurgeFile=(%+v,%v,%d calls), want %v and one call", got, err, calls.Load(), tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (CachePurgeReceipt{}) {
					t.Fatalf("refused receipt=%+v, want zero", got)
				}
				return
			}
			if got.Validate() != nil || got.Zone != server.zone || got.URL != request.URL || got.OperationID != tc.wantID {
				t.Fatalf("receipt=%+v, want exact zone/URL and operation %q", got, tc.wantID)
			}
		})
	}
}

func TestCachePurgeRefusesUnownedAndCancelledWork(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		canceled bool
		wantErr  error
	}{
		{name: "empty target never reaches provider", wantErr: core.ErrCloudflareContract},
		{name: "cancelled operation retains cancellation", canceled: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			server, request := cacheFixture(t, func(http.ResponseWriter, *http.Request) { calls.Add(1) })
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			} else {
				request = CacheFilePurgeRequest{}
			}
			got, err := server.PurgeFile(ctx, request, testPolicy())
			if !errors.Is(err, tc.wantErr) || got != (CachePurgeReceipt{}) || calls.Load() != 0 {
				t.Fatalf("refusal=(%+v,%v,%d calls), want zero, %v, no calls", got, err, calls.Load(), tc.wantErr)
			}
		})
	}
}
