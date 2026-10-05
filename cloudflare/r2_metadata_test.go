package cloudflare

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestR2SignedCreateOnlyUploadLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		conditions R2WriteConditions
		wantErr    error
	}{
		{name: "content checksum and create-only survive server client handoff", conditions: R2WriteConditions{ContentMD5: "XUFAKrxLKna5cZ2REBfFkg==", CreateOnly: true}},
		{name: "checksum alone is signed", conditions: R2WriteConditions{ContentMD5: "XUFAKrxLKna5cZ2REBfFkg=="}},
		{name: "create-only alone is signed", conditions: R2WriteConditions{CreateOnly: true}},
		{name: "absent optional conditions add no headers"},
		{name: "truncated digest cannot produce a grant", conditions: R2WriteConditions{ContentMD5: "XUFAKrxLKna5cZ2REBfFkg="}, wantErr: core.ErrCloudflareBinding},
		{name: "24 non-padding bytes cannot overflow decoder", conditions: R2WriteConditions{ContentMD5: strings.Repeat("A", 24)}, wantErr: core.ErrCloudflareBinding},
		{name: "noncanonical padding bits rejected", conditions: R2WriteConditions{ContentMD5: "XUFAKrxLKna5cZ2REBfFkh=="}, wantErr: core.ErrCloudflareBinding},
		{name: "header injection refused before signing", conditions: R2WriteConditions{ContentMD5: "\r\n" + strings.Repeat("A", 22)}, wantErr: core.ErrCloudflareBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			intent := testR2Intent(t, exchange.MethodPut, "session/original.pdf")
			intent.ContentType = core.HTTPMediaTypeOctetStream()
			intent.Conditions = tc.conditions
			server := testR2Server(t, R2JurisdictionDefault)
			grant, gotErr := server.Presign(t.Context(), intent)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Presign error=%v, want %v", gotErr, tc.wantErr)
			}
			if gotErr != nil {
				if grant != (R2Grant{}) {
					t.Fatal("rejected grant is populated, want zero")
				}
				return
			}
			endpoint, err := grant.Endpoint()
			if err != nil {
				t.Fatal(err)
			}
			input := R2GrantInput{Account: server.account, Jurisdiction: R2JurisdictionDefault, Endpoint: endpoint, ContentType: intent.ContentType, Method: intent.Method, Conditions: tc.conditions}
			crossed, err := ParseR2Grant(input)
			if err != nil {
				t.Fatal(err)
			}
			var calls int
			client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if !verifyR2TestSignature(r, []byte("r2-secret-key")) {
					t.Error("provider signature invalid, want exact signed conditions")
				}
				if got := r.Header.Get("Content-MD5"); got != tc.conditions.ContentMD5 {
					t.Errorf("checksum=%q, want %q", got, tc.conditions.ContentMD5)
				}
				if got := r.Header.Get("If-None-Match"); (got == "*") != tc.conditions.CreateOnly {
					t.Errorf("condition=%q, want create-only %t", got, tc.conditions.CreateOnly)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil || string(body) != "hello" {
					t.Errorf("body=%q error=%v, want hello and nil", body, err)
				}
			})
			provider, err := NewR2Client(client)
			if err != nil {
				t.Fatal(err)
			}
			length, err := core.NewByteLength(5)
			if err != nil {
				t.Fatal(err)
			}
			_, err = provider.Put(t.Context(), R2WriteRequest{Grant: crossed, Bytes: length, Source: bytes.NewReader([]byte("hello"))}, testPolicy())
			if err != nil || calls != 1 {
				t.Fatalf("Put=(%v,%d calls), want nil and one", err, calls)
			}
			if tc.conditions != (R2WriteConditions{}) {
				input.Conditions = R2WriteConditions{}
				if _, err := ParseR2Grant(input); !errors.Is(err, core.ErrCloudflareBinding) {
					t.Fatalf("stripped conditions error=%v, want binding refusal", err)
				}
			}
		})
	}
}

func TestR2HeadMetadataLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, length, media, etag string
		wantBytes                 uint64
		wantErr                   error
	}{
		{name: "PDF metadata without media-body delivery", length: "5", media: "application/pdf", etag: "\"abc\"", wantBytes: 5},
		{name: "empty object remains exact zero", length: "0", media: "application/octet-stream", etag: "\"empty\""},
		{name: "missing content type is not guessed", length: "5", etag: "\"abc\"", wantErr: core.ErrCloudflareResponse},
		{name: "missing entity tag cannot identify upload", length: "5", media: "application/pdf", wantErr: core.ErrCloudflareResponse},
		{name: "multipart etag stays opaque", length: "5", media: "video/mp4", etag: "\"abc-2\"", wantBytes: 5},
		{name: "unquoted etag is refused", length: "5", media: "video/mp4", etag: "abc", wantErr: core.ErrCloudflareResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodHead {
					t.Errorf("method=%s, want HEAD", r.Method)
				}
				if tc.length != "" {
					w.Header().Set("Content-Length", tc.length)
				}
				if tc.media != "" {
					w.Header().Set("Content-Type", tc.media)
				}
				if tc.etag != "" {
					w.Header().Set("ETag", tc.etag)
				}
			})
			server := testR2Server(t, R2JurisdictionDefault)
			grant, err := server.Presign(t.Context(), testR2Intent(t, exchange.MethodHead, "media/proof"))
			if err != nil {
				t.Fatal(err)
			}
			provider, err := NewR2Client(client)
			if err != nil {
				t.Fatal(err)
			}
			got, err := provider.Head(t.Context(), grant, testPolicy())
			if !errors.Is(err, tc.wantErr) || calls != 1 {
				t.Fatalf("Head=(%v,%d calls), want %v and one", err, calls, tc.wantErr)
			}
			if err == nil && (got.Bytes.Uint64() != tc.wantBytes || got.ETag != tc.etag || got.ContentType.String() != tc.media) {
				t.Fatalf("metadata=(%d,%q,%s), want (%d,%q,%s)", got.Bytes.Uint64(), got.ETag, got.ContentType.String(), tc.wantBytes, tc.etag, tc.media)
			}
			if err != nil && got != (R2ObjectMetadata{}) {
				t.Fatal("rejected metadata populated, want zero")
			}
		})
	}
}

func FuzzR2WriteConditionsCanonicalClosure(f *testing.F) {
	f.Add("XUFAKrxLKna5cZ2REBfFkg==", true)
	f.Add(strings.Repeat("A", 24), false)
	f.Fuzz(func(t *testing.T, value string, only bool) {
		conditions := R2WriteConditions{ContentMD5: value, CreateOnly: only}
		err := conditions.Validate()
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareBinding) {
				t.Fatalf("refusal=%v, want binding", err)
			}
			return
		}
		if err := conditions.headers().Validate(); err != nil {
			t.Fatalf("accepted conditions headers error=%v, want nil", err)
		}
		intent := testR2Intent(t, exchange.MethodPut, "condition/fuzz")
		intent.Conditions = conditions
		server := testR2Server(t, R2JurisdictionDefault)
		grant, err := server.Presign(t.Context(), intent)
		if err != nil {
			t.Fatal(err)
		}
		endpoint, err := grant.Endpoint()
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := ParseR2Grant(R2GrantInput{Account: server.account, Jurisdiction: R2JurisdictionDefault, Endpoint: endpoint, Method: exchange.MethodPut, Conditions: conditions})
		if err != nil || parsed != grant {
			t.Fatalf("round trip equal=%t error=%v, want true,nil", parsed == grant, err)
		}
	})
}
