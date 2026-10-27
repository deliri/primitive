package cloudflare

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func testR2CacheControl(t testing.TB, seconds uint64) R2CacheControl {
	t.Helper()
	duration, err := temporal.DurationFromSeconds(seconds)
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewR2CacheControl(duration)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestR2CacheControlSignsAndTransfersPUT(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		seconds uint64
		present bool
		want    string
	}{
		{name: "absent metadata emits no cache field"},
		{name: "explicit zero cannot become absent", present: true, want: "max-age=0"},
		{name: "testing lifetime is signed", present: true, seconds: 30, want: "max-age=30"},
		{name: "production lifetime is signed", present: true, seconds: 31536000, want: "max-age=31536000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			intent := testR2Intent(t, exchange.MethodPut, "owned/file.pdf")
			intent.ContentType = core.HTTPMediaTypeOctetStream()
			intent.Conditions.CreateOnly = true
			if tc.present {
				intent.CacheControl = testR2CacheControl(t, tc.seconds)
			}
			server := testR2Server(t, R2JurisdictionDefault)
			grant, err := server.Presign(t.Context(), intent)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := ParseR2Grant(R2GrantInput{Account: server.account, Jurisdiction: server.jurisdiction, Endpoint: grant.endpoint, Method: intent.Method, ContentType: intent.ContentType, Conditions: intent.Conditions, CacheControl: intent.CacheControl})
			if err != nil || parsed != grant {
				t.Fatalf("grant round trip=(%v,%v), want same nominal grant", parsed, err)
			}
			headers, err := grant.Headers()
			if err != nil {
				t.Fatal(err)
			}
			wantCount := 1
			if tc.present {
				wantCount++
			}
			if len(headers.Values) != wantCount {
				t.Fatalf("signed fields=%d, want %d", len(headers.Values), wantCount)
			}
			var calls atomic.Int64
			client, err := NewR2Client(testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if got := r.Header.Get(exchange.StandardHeaderCacheControl.String()); got != tc.want {
					t.Errorf("cache field=%q, want %q", got, tc.want)
				}
				if !verifyR2TestSignature(r, []byte("r2-secret-key")) {
					t.Error("signature verified=false, want true")
				}
				if tc.present {
					r.Header.Set(exchange.StandardHeaderCacheControl.String(), "max-age=31")
					if verifyR2TestSignature(r, []byte("r2-secret-key")) {
						t.Error("changed cache lifetime verified=true, want false")
					}
				}
				w.WriteHeader(http.StatusOK)
			}))
			if err != nil {
				t.Fatal(err)
			}
			length, err := core.NewByteLength(1)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Put(t.Context(), R2WriteRequest{Grant: parsed, Source: strings.NewReader("x"), Bytes: length}, testPolicy())
			if err != nil || calls.Load() != 1 {
				t.Fatalf("PUT=(%v,%d calls), want nil and one call", err, calls.Load())
			}
		})
	}
}

func TestR2CacheControlBindsOnlyObjectCreation(t *testing.T) {
	t.Parallel()
	for _, method := range []exchange.Method{exchange.MethodGet, exchange.MethodHead, exchange.MethodDelete} {
		t.Run(method.String(), func(t *testing.T) {
			t.Parallel()
			intent := testR2Intent(t, method, "owned/file.pdf")
			intent.CacheControl = testR2CacheControl(t, 30)
			server := testR2Server(t, R2JurisdictionDefault)
			got, err := server.Presign(t.Context(), intent)
			if !errors.Is(err, core.ErrCloudflareBinding) || got != (R2Grant{}) {
				t.Fatalf("non-write cache metadata=(%v,%v), want zero/binding refusal", got, err)
			}
		})
	}
	for _, action := range []R2MultipartAction{R2MultipartPart, R2MultipartComplete, R2MultipartAbort} {
		intent := multipartIntent(t, action)
		intent.CacheControl = testR2CacheControl(t, 30)
		server := testR2Server(t, R2JurisdictionDefault)
		got, err := server.PresignMultipart(t.Context(), intent)
		if !errors.Is(err, core.ErrCloudflareBinding) || got != (R2MultipartGrant{}) {
			t.Fatalf("action %d cache metadata=(%v,%v), want zero/binding refusal", action, got, err)
		}
	}
}

func TestR2CacheControlMultipartMetadata(t *testing.T) {
	t.Parallel()
	intent := multipartIntent(t, R2MultipartCreate)
	intent.CacheControl = testR2CacheControl(t, 30)
	server := testR2Server(t, R2JurisdictionDefault)
	grant, err := server.PresignMultipart(t.Context(), intent)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int64
	client, err := NewR2Client(testExchange(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if got := r.Header.Get(exchange.StandardHeaderCacheControl.String()); got != "max-age=30" || !verifyR2TestSignature(r, []byte("r2-secret-key")) {
			t.Errorf("multipart cache=%q signature=%t, want signed max-age=30", got, verifyR2TestSignature(r, []byte("r2-secret-key")))
		}
		body := "<InitiateMultipartUploadResult><Bucket>" + intent.Bucket.String() + "</Bucket><Key>" + intent.Key.String() + "</Key><UploadId>owned-upload</UploadId></InitiateMultipartUploadResult>"
		w.Header().Set(core.HTTPHeaderContentType().String(), core.CloudflareR2XMLMediaType)
		if _, err := w.Write([]byte(body)); err != nil {
			t.Error(err)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.CreateMultipart(t.Context(), grant, testPolicy())
	if err != nil || got.Validate() != nil || got.Key != intent.Key || got.Bucket != intent.Bucket || calls.Load() != 1 {
		t.Fatalf("multipart=(%+v,%v,%d calls), want exact creation", got, err, calls.Load())
	}
}

// The fixture stores exactly the object metadata accepted at signed creation.
// It proves SDK signing/transport and provider protocol behavior, not a live R2 deployment.
func TestR2NoStoreMetadataSurvivesSignedCreationAndRead(t *testing.T) {
	t.Parallel()
	for _, multipart := range []bool{false, true} {
		name := "whole original"
		if multipart {
			name = "multipart original"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server := testR2Server(t, R2JurisdictionDefault)
			intent := testR2Intent(t, exchange.MethodPut, "private/original.pdf")
			intent.ContentType = core.HTTPMediaTypeOctetStream()
			intent.CacheControl = R2NoStoreCacheControl()
			var stored string
			var methods []string
			transport := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				methods = append(methods, r.Method)
				if !verifyR2TestSignature(r, []byte("r2-secret-key")) {
					t.Error("actual signature=false, want true")
					w.WriteHeader(http.StatusForbidden)
					return
				}
				if r.Method == http.MethodPut || r.Method == http.MethodPost {
					stored = r.Header.Get(exchange.StandardHeaderCacheControl.String())
					if stored != core.CloudflareR2CacheNoStore {
						t.Errorf("stored cache=%q, want no-store", stored)
					}
					r.Header.Set(exchange.StandardHeaderCacheControl.String(), "max-age=1")
					if verifyR2TestSignature(r, []byte("r2-secret-key")) {
						t.Error("tampered no-store signature=true, want false")
					}
					if multipart {
						body := "<InitiateMultipartUploadResult><Bucket>" + intent.Bucket.String() + "</Bucket><Key>" + intent.Key.String() + "</Key><UploadId>private-upload</UploadId></InitiateMultipartUploadResult>"
						w.Header().Set(core.HTTPHeaderContentType().String(), core.CloudflareR2XMLMediaType)
						if _, err := io.WriteString(w, body); err != nil {
							t.Error(err)
						}
					} else if _, err := io.Copy(io.Discard, r.Body); err != nil {
						t.Error(err)
					}
					return
				}
				if stored != core.CloudflareR2CacheNoStore {
					t.Errorf("read retained cache=%q, want no-store", stored)
				}
				if r.Header.Get(exchange.StandardHeaderCacheControl.String()) != "" {
					t.Error("GET/HEAD carries write metadata")
				}
				w.Header().Set(exchange.StandardHeaderCacheControl.String(), stored)
				w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
				w.Header().Set(core.HTTPHeaderContentLength().String(), "1")
				w.Header().Set(core.CloudflareR2ETagHeader, "\"private-version\"")
				if r.Method == http.MethodGet {
					if _, err := io.WriteString(w, "x"); err != nil {
						t.Error(err)
					}
				}
			})
			client, err := NewR2Client(transport)
			if err != nil {
				t.Fatal(err)
			}
			if multipart {
				request := multipartIntent(t, R2MultipartCreate)
				request.Bucket, request.Key, request.CacheControl = intent.Bucket, intent.Key, intent.CacheControl
				grant, err := server.PresignMultipart(t.Context(), request)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := client.CreateMultipart(t.Context(), grant, testPolicy()); err != nil {
					t.Fatal(err)
				}
			} else {
				grant, err := server.Presign(t.Context(), intent)
				if err != nil {
					t.Fatal(err)
				}
				parsed, err := ParseR2Grant(R2GrantInput{Account: server.account, Jurisdiction: server.jurisdiction, Endpoint: grant.endpoint, Method: intent.Method, ContentType: intent.ContentType, CacheControl: intent.CacheControl})
				if err != nil || parsed != grant {
					t.Fatalf("private grant parse=(%v,%v), want exact grant", parsed, err)
				}
				length, err := core.NewByteLength(1)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := client.Put(t.Context(), R2WriteRequest{Grant: parsed, Source: strings.NewReader("x"), Bytes: length}, testPolicy()); err != nil {
					t.Fatal(err)
				}
			}
			for _, method := range []exchange.Method{exchange.MethodHead, exchange.MethodGet} {
				read := intent
				read.Method, read.ContentType, read.CacheControl = method, core.HTTPMediaType{}, R2CacheControl{}
				grant, err := server.Presign(t.Context(), read)
				if err != nil {
					t.Fatal(err)
				}
				if method == exchange.MethodHead {
					if _, err := client.Head(t.Context(), grant, testPolicy()); err != nil {
						t.Fatal(err)
					}
					continue
				}
				var out bytes.Buffer
				if _, err := client.Read(t.Context(), grant, &out, testPolicy()); err != nil || out.String() != "x" {
					t.Fatalf("read=(%q,%v), want x,nil", out.String(), err)
				}
			}
			if len(methods) != 3 || stored != core.CloudflareR2CacheNoStore {
				t.Fatalf("methods=%v stored=%q, want creation/HEAD/GET and no-store", methods, stored)
			}
		})
	}
}

func TestR2NoStoreRefusesReadAndLaterMultipartMetadata(t *testing.T) {
	t.Parallel()
	server := testR2Server(t, R2JurisdictionDefault)
	for _, method := range []exchange.Method{exchange.MethodGet, exchange.MethodHead, exchange.MethodDelete} {
		t.Run(method.String(), func(t *testing.T) {
			t.Parallel()
			request := testR2Intent(t, method, "private/refused")
			request.CacheControl = R2NoStoreCacheControl()
			got, err := server.Presign(t.Context(), request)
			if !errors.Is(err, core.ErrCloudflareBinding) || got != (R2Grant{}) {
				t.Fatalf("read metadata=(%v,%v), want zero/binding", got, err)
			}
		})
	}
	for _, action := range []R2MultipartAction{R2MultipartPart, R2MultipartComplete, R2MultipartAbort} {
		request := multipartIntent(t, action)
		request.CacheControl = R2NoStoreCacheControl()
		got, err := server.PresignMultipart(t.Context(), request)
		if !errors.Is(err, core.ErrCloudflareBinding) || got != (R2MultipartGrant{}) {
			t.Fatalf("later action=%v result=(%v,%v), want zero/binding", action, got, err)
		}
	}
}

func TestR2CacheControlClosedModes(t *testing.T) {
	t.Parallel()
	duration, err := temporal.DurationFromSeconds(1)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		cache   R2CacheControl
		wantErr error
	}{
		{name: "zero omits", cache: R2CacheControl{}},
		{name: "no-store has no age", cache: R2NoStoreCacheControl()},
		{name: "age remains supported", cache: R2CacheControl{mode: r2CacheMaxAge, maxAge: duration}},
		{name: "omit cannot smuggle age", cache: R2CacheControl{maxAge: duration}, wantErr: core.ErrCloudflareBinding},
		{name: "no-store cannot carry age", cache: R2CacheControl{mode: r2CacheNoStore, maxAge: duration}, wantErr: core.ErrCloudflareBinding},
		{name: "next unknown mode refused", cache: R2CacheControl{mode: r2CacheNoStore + 1}, wantErr: core.ErrCloudflareBinding},
		{name: "extreme unknown mode refused", cache: R2CacheControl{mode: 255}, wantErr: core.ErrCloudflareBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.cache.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate()=%v,want %v", err, tc.wantErr)
			}
		})
	}
}
