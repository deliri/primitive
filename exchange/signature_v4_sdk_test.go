package exchange

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"math"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The official SDK is retained as a test-only independent canonicalization
// oracle. Production signing owns no SDK cache, map or header/query inventory.
func v4OfficialSDKOracle(t testing.TB, intent V4PresignRequest) string {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), intent.Method.String(), intent.Target.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	applyRequestHeaders(request, intent.Headers)
	if !intent.ContentType.IsZero() {
		request.Header.Set(core.HTTPHeaderContentType().String(), intent.ContentType.String())
	}
	stamp, err := intent.SignedAt.Time()
	if err != nil {
		t.Fatal(err)
	}
	signer := v4.NewSigner(func(o *v4.SignerOptions) {
		o.DisableHeaderHoisting = true
		o.DisableURIPathEscaping = intent.DisableURIPathEscaping
	})
	result, _, err := signer.PresignHTTP(t.Context(), aws.Credentials{AccessKeyID: string(intent.AccessKey), SecretAccessKey: string(intent.SecretKey)}, request, intent.PayloadHash, intent.Service, intent.Region, stamp)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestV4PresignGoCryptoMatchesOfficialRequestScope(t *testing.T) {
	t.Parallel()
	for pathIndex, path := range []string{"", "/", "/object", "/a b/é", "/literal%25/+", "/a/../b", "//double///", "/東京/☃"} {
		for mode := range 8 {
			t.Run(fmt.Sprintf("path%q/mode%d", path, mode), func(t *testing.T) {
				t.Parallel()
				intent := signingFixture(t)
				target := url.URL{Scheme: "https", Host: "signing.example", Path: path, RawQuery: "z=last&a=second&a=first&a=first&empty&space=a+b&encoded=%25%2B%2F&X-Amz-Expires=60"}
				switch mode {
				case 0:
					intent.DisableURIPathEscaping = false
				case 1:
					target.Host = "signing.example:443"
				case 2:
					target.Host = "[::1]:443"
				case 3:
					intent.Method = MethodPut
					intent.ContentType = core.HTTPMediaTypeJSON()
				case 4:
					target.RawQuery += "&X-Amz-Algorithm=replace&X-Amz-Date=replace&X-Amz-Credential=replace&X-Amz-SignedHeaders=replace"
				case 5:
					name, err := core.ParseHTTPHeaderName("X-Custom-Proof")
					if err != nil {
						t.Fatal(err)
					}
					first, err := NewHeaderValue("  first   middle  ")
					if err != nil {
						t.Fatal(err)
					}
					second, err := NewHeaderValue("second")
					if err != nil {
						t.Fatal(err)
					}
					intent.Headers = Headers{Values: []Header{{Name: name, Values: []HeaderValue{first, second}}}}
				case 6:
					name, err := core.ParseHTTPHeaderName("User-Agent")
					if err != nil {
						t.Fatal(err)
					}
					value, err := NewHeaderValue("unsigned user agent")
					if err != nil {
						t.Fatal(err)
					}
					intent.Headers = Headers{Values: []Header{{Name: name, Values: []HeaderValue{value}}}}
				case 7:
					intent.SignedAt = temporal.InstantFromNanoseconds([]int64{math.MinInt64, -1000000001, -1, 0, 1, 1000000001, 1791000000000000127, math.MaxInt64}[pathIndex])
				}
				var err error
				intent.Target, err = core.ParseHTTPEndpoint(target.String())
				if err != nil {
					t.Fatal(err)
				}
				got, err := PresignV4(t.Context(), intent)
				if err != nil {
					t.Fatal(err)
				}
				if want := v4OfficialSDKOracle(t, intent); got.String() != want {
					t.Fatalf("Go signature=%s SDK=%s", got.String(), want)
				}

			})
		}
	}
}

func TestV4PresignRefusesPartialOrAlreadySignedCanonicalInput(t *testing.T) {
	t.Parallel()
	for _, query := range []string{"bad=%GG", "%GG=bad", "a=1;b=2", "X-Amz-Signature=old", "X%2DAmz%2DSignature=old"} {
		t.Run(query, func(t *testing.T) {
			t.Parallel()
			intent := signingFixture(t)
			target := intent.Target.HTTPURL()
			target.RawQuery = query
			var err error
			intent.Target, err = core.ParseHTTPEndpoint(target.String())
			if err != nil {
				return
			}
			got, err := PresignV4(t.Context(), intent)
			if !errors.Is(err, core.ErrExchangeRequest) || got != (core.HTTPEndpoint{}) {
				t.Fatalf("partial canonical input admitted: endpoint=%v error=%v", got, err)
			}
		})
	}
	intent := signingFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := PresignV4(ctx, intent)
	if !errors.Is(err, context.Canceled) || got != (core.HTTPEndpoint{}) {
		t.Fatal("cancel admitted signature")
	}
}

func FuzzV4PresignGoCryptoMatchesOfficialRequest(f *testing.F) {
	f.Add("/path", "a+b é", uint8(MethodGet), true)
	f.Add("/literal%/../☃", "% + /", uint8(MethodPut), false)
	f.Fuzz(func(t *testing.T, path, value string, method uint8, disableEscaping bool) {
		intent := signingFixture(t)
		target := url.URL{Scheme: "https", Host: "signing.example", Path: path, RawQuery: "X-Amz-Expires=60&input=" + url.QueryEscape(value)}
		endpoint, err := core.ParseHTTPEndpoint(target.String())
		if err != nil {
			return
		}
		intent.Target = endpoint
		intent.Method = Method(method)
		intent.DisableURIPathEscaping = disableEscaping
		got, err := PresignV4(t.Context(), intent)
		if intent.Validate() != nil {
			if !errors.Is(err, core.ErrExchangeRequest) || got != (core.HTTPEndpoint{}) {
				t.Fatal("invalid request has no typed refusal")
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if want := v4OfficialSDKOracle(t, intent); got.String() != want {
			t.Fatalf("request agreement changed: Go=%s SDK=%s", got.String(), want)
		}
		// The whole mutated value is signed. No prefix-only oracle or fuzz quota.
		if !strings.Contains(got.HTTPURL().RawQuery, "X-Amz-Signature=") {
			t.Fatal("signature absent")
		}
	})
}

// AWS specifies sorting after URI encoding. These exact grammar facts cover
// the non-ASCII/reserved ordering the former decoded-key SDK map did not seal.
func TestV4PresignCanonicalQuerySortsEncodedNamesAndValues(t *testing.T) {
	t.Parallel()
	facts := v4SigningFacts{stamp: "stamp", credential: "id/scope", signedHeaders: "host"}
	for _, query := range []string{"Z=last&é=first", "é=first&Z=last", "a=Z&a=é&a=Z", "a=é&a=Z&a=Z"} {
		t.Run(query, func(t *testing.T) {
			got, err := v4CanonicalQuery(t.Context(), url.URL{RawQuery: query}, facts)
			if err != nil {
				t.Fatal(err)
			}
			want := "X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=id%2Fscope&X-Amz-Date=stamp&X-Amz-SignedHeaders=host&a=%C3%A9&a=Z&a=Z"
			if strings.Contains(query, "last") {
				want = "%C3%A9=first&X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=id%2Fscope&X-Amz-Date=stamp&X-Amz-SignedHeaders=host&Z=last"
			}
			if got != want {
				t.Fatalf("canonical pair order=%q want=%q", got, want)
			}
		})
	}
}

func BenchmarkV4PresignGoCryptoBorrowedQuery64(b *testing.B) {
	intent := signingFixture(b)
	target := intent.Target.HTTPURL()
	var query strings.Builder
	for i := 63; i >= 0; i-- {
		fmt.Fprintf(&query, "key%02d=value%02d&", i, i)
	}
	query.WriteString("X-Amz-Expires=60")
	target.RawQuery = query.String()
	var err error
	intent.Target, err = core.ParseHTTPEndpoint(target.String())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := PresignV4(b.Context(), intent); err != nil {
			b.Fatal(err)
		}
	}
}

func TestV4PresignLongBorrowedSecretsMatchNativeHMACKeyNormalization(t *testing.T) {
	t.Parallel()
	for _, size := range []int{1, 59, 60, 61, 64, 65, 4096, 1048577} {
		for mode := range 8 {
			t.Run(fmt.Sprintf("secret%d/mode%d", size, mode), func(t *testing.T) {
				t.Parallel()
				intent := signingFixture(t)
				intent.SecretKey = make([]byte, size)
				for i := range intent.SecretKey {
					intent.SecretKey[i] = byte(i*17 + mode)
				}
				got, err := PresignV4(t.Context(), intent)
				if err != nil {
					t.Fatal(err)
				}
				if want := v4OfficialSDKOracle(t, intent); got.String() != want {
					t.Fatalf("native HMAC key normalization disagrees for %d bytes", size)
				}
				for i, b := range intent.SecretKey {
					if b != byte(i*17+mode) {
						t.Fatal("signing mutated borrowed secret")
					}
				}
			})
		}
	}
}
