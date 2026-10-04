package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func signingFixture(t testing.TB) V4PresignRequest {
	t.Helper()
	target, err := core.ParseHTTPEndpoint("https://signing.example/bucket/object?X-Amz-Expires=60")
	if err != nil {
		t.Fatal(err)
	}
	stamp, err := temporal.InstantFromUnixSeconds(1791000000)
	if err != nil {
		t.Fatal(err)
	}
	return V4PresignRequest{AccessKey: []byte("test-access"), SecretKey: []byte("test-secret"), Region: "test-region", Service: "test-service", PayloadHash: "UNSIGNED-PAYLOAD", Target: target, SignedAt: stamp, Method: MethodGet, DisableURIPathEscaping: true}
}

// This verifier derives the canonical request independently of the official
// signer. It pins the complete input scope rather than trusting the signed URL.
func signingProofMatches(intent V4PresignRequest, endpoint core.HTTPEndpoint) bool {
	u := endpoint.HTTPURL()
	q := u.Query()
	signature := q.Get("X-Amz-Signature")
	q.Del("X-Amz-Signature")
	stamp, err := intent.SignedAt.CompactUTC()
	if err != nil {
		return false
	}
	scope := stamp[:8] + "/" + intent.Region + "/" + intent.Service + "/aws4_request"
	if q.Get("X-Amz-Credential") != string(intent.AccessKey)+"/"+scope || q.Get("X-Amz-Date") != stamp {
		return false
	}
	headers := "host:" + intent.Target.HTTPURL().Host + "\n"
	names := "host"
	if !intent.ContentType.IsZero() {
		headers = "content-type:" + intent.ContentType.String() + "\n" + headers
		names = "content-type;host"
	}
	if q.Get("X-Amz-SignedHeaders") != names {
		return false
	}
	signedPath := u.EscapedPath()
	if signedPath == "" {
		signedPath = "/"
	}
	canonical := intent.Method.String() + "\n" + signedPath + "\n" + strings.ReplaceAll(q.Encode(), "+", "%20") + "\n" + headers + "\n" + names + "\n" + intent.PayloadHash
	digest := sha256.Sum256([]byte(canonical))
	message := "AWS4-HMAC-SHA256\n" + stamp + "\n" + scope + "\n" + hex.EncodeToString(digest[:])
	key := append([]byte("AWS4"), intent.SecretKey...)
	for _, text := range []string{stamp[:8], intent.Region, intent.Service, "aws4_request", message} {
		mac := hmac.New(sha256.New, key)
		_, _ = mac.Write([]byte(text))
		key = mac.Sum(nil)
	}
	return hmac.Equal([]byte(signature), []byte(hex.EncodeToString(key)))
}

func TestV4PresignLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		wantErr error
		mutate  func(*V4PresignRequest)
		name    string
		cancel  bool
	}{
		{name: "exact input is signed without a network client"},
		{name: "empty headers preserve only host authority", mutate: func(r *V4PresignRequest) { r.Headers = Headers{} }},
		{name: "media type is authenticated", mutate: func(r *V4PresignRequest) { r.ContentType = core.HTTPMediaTypeJSON() }},
		{name: "absent access key produces no capability", mutate: func(r *V4PresignRequest) { r.AccessKey = nil }, wantErr: core.ErrExchangeRequest},
		{name: "absent secret produces no capability", mutate: func(r *V4PresignRequest) { r.SecretKey = nil }, wantErr: core.ErrExchangeRequest},
		{name: "unset service cannot invent a signing scope", mutate: func(r *V4PresignRequest) { r.Service = "" }, wantErr: core.ErrExchangeRequest},
		{name: "unset region cannot invent a signing scope", mutate: func(r *V4PresignRequest) { r.Region = "" }, wantErr: core.ErrExchangeRequest},
		{name: "scope delimiter is rejected", mutate: func(r *V4PresignRequest) { r.Region = "a/b" }, wantErr: core.ErrExchangeRequest},
		{name: "missing payload agreement is rejected", mutate: func(r *V4PresignRequest) { r.PayloadHash = "" }, wantErr: core.ErrExchangeRequest},
		{name: "unknown method is rejected", mutate: func(r *V4PresignRequest) { r.Method = Method(255) }, wantErr: core.ErrExchangeRequest},
		{name: "unset timestamp cannot observe the wall clock", mutate: func(r *V4PresignRequest) { r.SignedAt = temporal.Instant{} }, wantErr: core.ErrExchangeRequest},
		{name: "cancellation remains typed without producing a URL", cancel: true, wantErr: context.Canceled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			intent := signingFixture(t)
			if tc.mutate != nil {
				tc.mutate(&intent)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			got, gotErr := PresignV4(ctx, intent)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("PresignV4 error=%v, want %v", gotErr, tc.wantErr)
			}
			if gotErr != nil {
				if got != (core.HTTPEndpoint{}) {
					t.Fatalf("refused endpoint=%v, want zero", got)
				}
				return
			}
			if !signingProofMatches(intent, got) {
				t.Fatalf("signature agreement=false, want true")
			}
			mutated := intent
			mutated.Method = MethodPut
			if signingProofMatches(mutated, got) {
				t.Fatalf("changed method retained signature validity, want refusal")
			}
			if text := fmt.Sprintf("%+v", intent); strings.Contains(text, "test-secret") || strings.Contains(text, "test-access") {
				t.Fatalf("formatted credentials leaked, want redaction")
			}
		})
	}
}

func FuzzV4PresignExactRequestAgreement(f *testing.F) {
	seed := signingFixture(f)
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	f.Add(seed.Target.HTTPURL().Path, uint8(seed.Method))
	f.Add("/literal%25/../space +/café", uint8(MethodPut))
	f.Add("", uint8(255))
	f.Fuzz(func(t *testing.T, path string, method uint8) {
		// Bound secondary oracle work; admitted endpoint has its own independent ceiling.
		if len(path) > 4096 {
			return
		}
		intent := signingFixture(t)
		u := url.URL{Scheme: "https", Host: "signing.example", Path: path, RawQuery: "X-Amz-Expires=60"}
		endpoint, err := core.ParseHTTPEndpoint(u.String())
		if err != nil {
			return
		}
		intent.Target = endpoint
		intent.Method = Method(method)
		got, gotErr := PresignV4(t.Context(), intent)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrExchangeRequest) || got != (core.HTTPEndpoint{}) {
				t.Fatalf("refusal=(%v,%v), want zero typed request refusal", got, gotErr)
			}
			return
		}
		if got.Validate() != nil || !signingProofMatches(intent, got) {
			t.Fatalf("accepted signature has wrong request agreement, want exact source facts")
		}
		if got.HTTPURL().Path != endpoint.HTTPURL().Path {
			t.Fatalf("signed path=%q, want %q", got.HTTPURL().Path, endpoint.HTTPURL().Path)
		}
		second, err := PresignV4(t.Context(), intent)
		if err != nil || second != got {
			t.Fatalf("deterministic signing=(%v,%v), want same endpoint and nil", second, err)
		}
	})
}
