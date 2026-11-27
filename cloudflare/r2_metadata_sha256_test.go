package cloudflare

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestR2SignedSHA256MetadataPersistsThroughPutAndHead(t *testing.T) {
	t.Parallel()
	const header = "X-Amz-Meta-Sha256"
	const object = "/media-bucket/bundle/member.html"
	const payload = "<p>Frozen member</p>"
	digest := core.SHA256Of([]byte(payload))
	expectedHex, err := digest.Hex()
	if err != nil {
		t.Fatal(err)
	}
	server := testR2Server(t, R2JurisdictionDefault)
	intent := testR2Intent(t, exchange.MethodPut, "bundle/member.html")
	intent.MetadataSHA256 = digest
	intent.Conditions = R2WriteConditions{CreateOnly: true}
	grant, err := server.Presign(t.Context(), intent)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := grant.Endpoint()
	if err != nil {
		t.Fatal(err)
	}
	wire := R2GrantInput{Account: server.account, Jurisdiction: R2JurisdictionDefault,
		Method: exchange.MethodPut, Endpoint: endpoint, Conditions: intent.Conditions, MetadataSHA256: digest}
	parsed, err := ParseR2Grant(wire)
	if err != nil || parsed != grant {
		t.Fatalf("crossed grant failed: %v", err)
	}
	wire.MetadataSHA256 = core.SHA256Digest{}
	if _, err := ParseR2Grant(wire); !errors.Is(err, core.ErrCloudflareBinding) {
		t.Fatalf("grant omitting signed digest error = %v, want binding refusal", err)
	}
	var stored atomic.Pointer[string]
	var puts, heads atomic.Int32
	client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != object || !verifyR2TestSignature(r, []byte("r2-secret-key")) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.Method {
		case http.MethodPut:
			puts.Add(1)
			received := r.Header.Get(header)
			if received != expectedHex || r.Header.Get("If-None-Match") != "*" {
				t.Errorf("PUT metadata/condition = %q/%q, want exact lower hex and create-only", received, r.Header.Get("If-None-Match"))
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, int64(len(payload)+1)))
			if err != nil || string(body) != payload {
				t.Errorf("body = %q/%v, want exact payload", body, err)
			}
			stored.Store(&received)
		case http.MethodHead:
			heads.Add(1)
			received := stored.Load()
			if received == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set(header, *received)
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("Content-Length", "20")
			w.Header().Set("ETag", `"etag-opaque"`)
		default:
			t.Errorf("unexpected verb %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	provider, err := NewR2Client(client)
	if err != nil {
		t.Fatal(err)
	}
	length, err := core.NewByteLength(uint64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	put := func(g R2Grant) error {
		_, putErr := provider.Put(t.Context(), R2WriteRequest{Source: bytes.NewBufferString(payload), Grant: g, Bytes: length}, testPolicy())
		return putErr
	}
	if err := put(parsed); err != nil {
		t.Fatal(err)
	}
	// Manipulate the input before it crosses the SDK boundary, keeping its
	// original signature to check that the provider oracle rejects the change.
	wire.MetadataSHA256 = core.SHA256Of([]byte("different recorded value"))
	tampered, err := ParseR2Grant(wire)
	if err == nil {
		err = put(tampered)
	}
	if !errors.Is(err, core.ErrExchangeAuthentication) {
		t.Fatalf("tampered metadata error = %v, want provider signature refusal", err)
	}
	head, err := server.Presign(t.Context(), testR2Intent(t, exchange.MethodHead, "bundle/member.html"))
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := provider.HeadWithMetadataSHA256(t.Context(), head, testPolicy())
	if err != nil || metadata.MetadataSHA256 != digest || metadata.ETag != `"etag-opaque"` || metadata.Bytes != length {
		t.Fatalf("strict HEAD = %+v/%v, want exact independent metadata and extent", metadata, err)
	}
	if puts.Load() != 1 || heads.Load() != 1 {
		t.Fatalf("provider accepted PUT/HEAD = %d/%d, want one each", puts.Load(), heads.Load())
	}
}

func TestR2SHA256MetadataRejectsReadGrantsAndInvalidReadback(t *testing.T) {
	t.Parallel()
	server := testR2Server(t, R2JurisdictionDefault)
	for _, method := range []exchange.Method{exchange.MethodGet, exchange.MethodHead, exchange.MethodDelete} {
		intent := testR2Intent(t, method, "bundle/member")
		intent.MetadataSHA256 = core.SHA256Of([]byte("object"))
		if grant, err := server.Presign(t.Context(), intent); grant != (R2Grant{}) || !errors.Is(err, core.ErrCloudflareBinding) {
			t.Fatalf("%s grant error = %v, want refusal", method, err)
		}
	}
	for _, tc := range []struct {
		name   string
		values []string
	}{
		{"missing", nil},
		{"short", []string{"abcdef"}},
		{"uppercase", []string{strings.Repeat("A", 64)}},
		{"duplicate", []string{strings.Repeat("a", 64), strings.Repeat("b", 64)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "20")
				w.Header().Set("Content-Type", "text/html")
				w.Header().Set("ETag", `"etag-opaque"`)
				for _, value := range tc.values {
					w.Header().Add("X-Amz-Meta-Sha256", value)
				}
			})
			provider, err := NewR2Client(client)
			if err != nil {
				t.Fatal(err)
			}
			grant, err := server.Presign(t.Context(), testR2Intent(t, exchange.MethodHead, "bundle/member"))
			if err != nil {
				t.Fatal(err)
			}
			if got, err := provider.HeadWithMetadataSHA256(t.Context(), grant, testPolicy()); got != (R2ObjectMetadata{}) || !errors.Is(err, core.ErrCloudflareResponse) {
				t.Fatalf("strict HEAD = %+v/%v, want no result and response refusal", got, err)
			}
			if got, err := provider.Head(t.Context(), grant, testPolicy()); err != nil || got.ETag != `"etag-opaque"` || got.MetadataSHA256 != (core.SHA256Digest{}) {
				t.Fatalf("routine HEAD = %+v/%v, want original ETag and no optional metadata", got, err)
			}
		})
	}
}

func FuzzR2SHA256MetadataReadback(f *testing.F) {
	f.Add(strings.Repeat("a", 64))
	f.Add(strings.Repeat("A", 64))
	f.Add("missing")
	f.Fuzz(func(t *testing.T, raw string) {
		value, err := exchange.NewHeaderValue(raw)
		if err != nil {
			return // Exchange rejects invalid HTTP fields before capture.
		}
		headers := exchange.CapturedHeaders{Values: []exchange.Header{
			cloudflareRequestHeader("Content-Type", "text/html"),
			cloudflareRequestHeader("Content-Length", "20"),
			cloudflareRequestHeader("ETag", `"opaque"`),
			{Name: cloudflareHeader("X-Amz-Meta-Sha256"), Values: []exchange.HeaderValue{value}},
		}}
		got, err := r2Metadata(headers, true)
		decoded, decodeErr := hex.DecodeString(raw)
		canonical := decodeErr == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == raw
		if !canonical {
			if !errors.Is(err, core.ErrCloudflareResponse) || got != (R2ObjectMetadata{}) {
				t.Fatalf("noncanonical digest yielded %+v/%v", got, err)
			}
			return
		}
		rendered, hexErr := got.MetadataSHA256.Hex()
		if err != nil || hexErr != nil || rendered != raw {
			t.Fatalf("canonical digest readback = %q/%v/%v", rendered, hexErr, err)
		}
	})
}
