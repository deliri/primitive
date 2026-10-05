package cloudflare

import (
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestCachePurgeOperationBounds(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		id      CachePurgeOperationID
		wantErr error
	}{
		{name: "absent operation is optional metadata"},
		{name: "one character operation survives", id: "x"},
		{name: "below provider character ceiling", id: CachePurgeOperationID(strings.Repeat("x", core.CloudflareCachePurgeIDMaximumCharacters-1))},
		{name: "at provider ceiling counts Unicode characters", id: CachePurgeOperationID(strings.Repeat("é", core.CloudflareCachePurgeIDMaximumCharacters))},
		{name: "above provider ceiling refuses receipt", id: CachePurgeOperationID(strings.Repeat("x", core.CloudflareCachePurgeIDMaximumCharacters+1)), wantErr: core.ErrCloudflareResponse},
		{name: "invalid UTF8 never becomes metadata", id: CachePurgeOperationID(string([]byte{0xff})), wantErr: core.ErrCloudflareResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.id.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("operation Validate()=%v, want %v", err, tc.wantErr)
			}
		})
	}
}

// The generic envelope truth-domain test covers success/error precedence.
// These rows attack the additional purge response and HTTP boundaries.
func TestCachePurgeHTTPRefusalLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		malformed string
		status    int
		media     core.HTTPMediaType
		wantErr   error
	}{
		{name: "HTTP success retains acceptance", status: http.StatusOK, media: core.HTTPMediaTypeJSON()},
		{name: "empty body cannot imply acceptance", status: http.StatusOK, media: core.HTTPMediaTypeJSON(), malformed: " ", wantErr: core.ErrCloudflareResponse},
		{name: "unknown envelope member is refused", status: http.StatusOK, media: core.HTTPMediaTypeJSON(), malformed: `{"success":true,"unexpected":1}`, wantErr: core.ErrCloudflareResponse},
		{name: "duplicate success is refused", status: http.StatusOK, media: core.HTTPMediaTypeJSON(), malformed: `{"success":true,"success":true}`, wantErr: core.ErrCloudflareResponse},
		{name: "string success is not boolean authority", status: http.StatusOK, media: core.HTTPMediaTypeJSON(), malformed: `{"success":"true"}`, wantErr: core.ErrCloudflareResponse},
		{name: "truncated document cannot imply acceptance", status: http.StatusOK, media: core.HTTPMediaTypeJSON(), malformed: `{"success":true`, wantErr: core.ErrCloudflareResponse},
		{name: "trailing document is refused", status: http.StatusOK, media: core.HTTPMediaTypeJSON(), malformed: `{"success":true}{}`, wantErr: core.ErrCloudflareResponse},
		{name: "wrong content type refuses JSON-looking body", status: http.StatusOK, media: core.HTTPMediaTypeOctetStream(), wantErr: core.ErrExchangeContentType},
		{name: "rate limit never becomes accepted receipt", status: http.StatusTooManyRequests, media: core.HTTPMediaTypeJSON(), wantErr: core.ErrExchangeResponse},
		{name: "server failure never becomes accepted receipt", status: http.StatusServiceUnavailable, media: core.HTTPMediaTypeJSON(), wantErr: core.ErrExchangeResponse},
		{name: "redirect cannot forward bearer authority", status: http.StatusTemporaryRedirect, media: core.HTTPMediaTypeJSON(), wantErr: core.ErrExchangeResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body, err := core.MarshalCanonicalJSONDocument(apiEnvelope[cachePurgeWire]{Success: new(true)})
			if err != nil {
				t.Fatal(err)
			}
			if tc.malformed != "" {
				body = []byte(tc.malformed)
			}
			var calls atomic.Int64
			server, request := cacheFixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set(core.HTTPHeaderContentType().String(), tc.media.String())
				if tc.status == http.StatusTemporaryRedirect {
					w.Header().Set("Location", "https://foreign.example.test/receive")
				}
				w.WriteHeader(tc.status)
				if _, err := w.Write(body); err != nil {
					t.Error(err)
				}
			})
			got, err := server.PurgeFile(t.Context(), request, testPolicy())
			if !errors.Is(err, tc.wantErr) || calls.Load() != 1 {
				t.Fatalf("purge=(%+v,%v,%d calls), want %v and one call", got, err, calls.Load(), tc.wantErr)
			}
			if err != nil && got != (CachePurgeReceipt{}) || err == nil && (got.Validate() != nil || got.URL != request.URL) {
				t.Fatalf("receipt=%+v, want exact acceptance or zero on refusal", got)
			}
		})
	}
}

func TestCachePurgeResponseByteCeiling(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		spare   int
		wantErr error
	}{
		{name: "one byte over limit yields no receipt", spare: -1, wantErr: core.ErrCloudflareResponse},
		{name: "exact limit retains acceptance"},
		{name: "one byte below limit retains acceptance", spare: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body, err := core.MarshalCanonicalJSONDocument(apiEnvelope[cachePurgeWire]{Success: new(true)})
			if err != nil {
				t.Fatal(err)
			}
			server, request := cacheFixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				if _, err := w.Write(body); err != nil {
					t.Error(err)
				}
			})
			server.api.limits.DocumentMaximumBytes, err = core.NewByteCount(uint64(len(body) + tc.spare))
			if err != nil {
				t.Fatal(err)
			}
			got, err := server.PurgeFile(t.Context(), request, testPolicy())
			if !errors.Is(err, tc.wantErr) || err != nil && got != (CachePurgeReceipt{}) || err == nil && got.Validate() != nil {
				t.Fatalf("bounded purge=(%+v,%v), want %v with exact receipt or zero", got, err, tc.wantErr)
			}
		})
	}
}

func TestCacheServerOwnsCredentialLifetime(t *testing.T) {
	t.Parallel()
	options := testOptions(t)
	zone, err := ParseZoneID(strings.Repeat("b", core.CloudflareIdentityCharacters))
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int64
	client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if got := r.Header.Get(exchange.StandardHeaderAuthorization.String()); got != "Bearer cloudflare-test-token" {
			t.Errorf("owned bearer matches source=%t, want true", got == "Bearer cloudflare-test-token")
		}
		body, err := core.MarshalCanonicalJSONDocument(apiEnvelope[cachePurgeWire]{Success: new(true)})
		if err != nil {
			t.Error(err)
			return
		}
		w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
		if _, err := w.Write(body); err != nil {
			t.Error(err)
		}
	})
	server, err := NewCacheServer(client, CacheServerOptions{Zone: zone, Token: options.Token, ResponseLimits: options.ResponseLimits})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := options.Token.Close(); err != nil {
		t.Fatal(err)
	}
	target, err := core.ParseHTTPEndpoint("https://media.example.test/owned/document.pdf")
	if err != nil {
		t.Fatal(err)
	}
	request := CacheFilePurgeRequest{URL: target}
	got, err := server.PurgeFile(t.Context(), request, testPolicy())
	if err != nil || got.Validate() != nil || calls.Load() != 1 {
		t.Fatalf("caller closed token=(%+v,%v,%d calls), want independent custody", got, err, calls.Load())
	}
	copy := server
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	got, err = copy.PurgeFile(t.Context(), request, testPolicy())
	if !errors.Is(err, core.ErrCloudflareAuthentication) || got != (CachePurgeReceipt{}) || calls.Load() != 1 {
		t.Fatalf("closed owner=(%+v,%v,%d calls), want zero/authentication and no new call", got, err, calls.Load())
	}
}
