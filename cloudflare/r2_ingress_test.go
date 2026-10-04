package cloudflare

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestR2BucketAlphabetAndExtentExhaustion(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		value := "a" + string([]byte{byte(raw)}) + "z"
		got, err := ParseR2Bucket(value)
		want := raw >= 'a' && raw <= 'z' || raw >= '0' && raw <= '9' || raw == '-'
		if (err == nil) != want {
			t.Fatalf("bucket byte %d error=%v, want admitted=%t", raw, err, want)
		}
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareBinding) || got != (R2Bucket{}) {
				t.Fatalf("bucket refusal=(%v,%v), want typed zero", got, err)
			}
		} else if got.String() != value {
			t.Fatalf("bucket=%q, want exact %q", got.String(), value)
		}
	}
	for _, size := range []int{0, 2, 3, 4, 62, 63, 64, 1024} {
		value := strings.Repeat("a", size)
		got, err := ParseR2Bucket(value)
		want := size >= core.CloudflareR2BucketMinimumBytes && size <= core.CloudflareR2BucketMaximumBytes
		if (err == nil) != want {
			t.Fatalf("bucket size %d=(%v,%v), want admitted=%t", size, got, err, want)
		}
	}
	for _, value := range []string{"-ab", "ab-", "a.b", "192.168.0.1"} {
		got, err := ParseR2Bucket(value)
		if !errors.Is(err, core.ErrCloudflareBinding) || got != (R2Bucket{}) {
			t.Fatalf("R2 bucket %q=(%v,%v), want refusal even when another provider admits it", value, got, err)
		}
	}
}

func TestR2JurisdictionExhaustsUnderlyingDomain(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		jurisdiction := R2Jurisdiction(raw)
		want := raw >= int(R2JurisdictionDefault) && raw <= int(R2JurisdictionFedRAMP)
		err := jurisdiction.Validate()
		if (err == nil) != want {
			t.Fatalf("jurisdiction %d error=%v, want admitted=%t", raw, err, want)
		}
		if err != nil && !errors.Is(err, core.ErrCloudflareBinding) {
			t.Fatalf("jurisdiction error=%v, want binding refusal", err)
		}
	}
}

func FuzzR2ObjectRepresentationClosure(f *testing.F) {
	bucket, err := ParseR2Bucket("media-bucket")
	if err != nil {
		f.Fatal(err)
	}
	key, err := ParseR2Key("folder/café")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(bucket.String(), key.String())
	f.Add("", "/")
	f.Add("a.b", string([]byte{0xff}))
	f.Fuzz(func(t *testing.T, bucket, key string) {
		gotBucket, bucketErr := ParseR2Bucket(bucket)
		if bucketErr != nil {
			if !errors.Is(bucketErr, core.ErrCloudflareBinding) || gotBucket != (R2Bucket{}) {
				t.Fatalf("bucket refusal=(%v,%v), want typed zero", gotBucket, bucketErr)
			}
		} else if gotBucket.Validate() != nil || gotBucket.String() != bucket || strings.Trim(bucket, "abcdefghijklmnopqrstuvwxyz0123456789-") != "" || strings.HasPrefix(bucket, "-") || strings.HasSuffix(bucket, "-") {
			t.Fatalf("admitted bucket=%v, want bounded exact R2 name", gotBucket)
		}
		gotKey, keyErr := ParseR2Key(key)
		if keyErr != nil {
			if !errors.Is(keyErr, core.ErrCloudflareBinding) || gotKey != (R2Key{}) {
				t.Fatalf("key refusal=(%v,%v), want typed zero", gotKey, keyErr)
			}
		} else if gotKey.Validate() != nil || gotKey.String() != key || key == "" || !utf8.ValidString(key) || len(key) > core.CloudflareR2ObjectKeyMaximumBytes || strings.ContainsRune(key, 0) {
			t.Fatalf("admitted key=%v, want bounded unmodified key", gotKey)
		}
	})
}

func FuzzR2GrantRepresentationAndProviderVerification(f *testing.F) {
	// Produce the canonical seed through the real typed server signer.
	credentials, err := ParseR2Credentials([]byte("r2-access-key"), []byte("r2-secret-key"))
	if err != nil {
		f.Fatal(err)
	}
	account, err := ParseAccountID(strings.Repeat("a", core.CloudflareIdentityCharacters))
	if err != nil {
		f.Fatal(err)
	}
	server, err := NewR2Server(credentials, account, R2JurisdictionDefault)
	if err != nil {
		f.Fatal(err)
	}
	if err := credentials.Close(); err != nil {
		f.Fatal(err)
	}
	bucket, err := ParseR2Bucket("media-bucket")
	if err != nil {
		f.Fatal(err)
	}
	key, err := ParseR2Key("object")
	if err != nil {
		f.Fatal(err)
	}
	stamp, err := temporal.InstantFromUnixSeconds(webhookTestSeconds)
	if err != nil {
		f.Fatal(err)
	}
	expires, err := temporal.DurationFromSeconds(60)
	if err != nil {
		f.Fatal(err)
	}
	grant, err := server.Presign(f.Context(), R2PresignRequest{Bucket: bucket, Key: key, SignedAt: stamp, Expires: expires, Method: exchange.MethodGet})
	if err != nil {
		f.Fatal(err)
	}
	if err := server.Close(); err != nil {
		f.Fatal(err)
	}
	f.Add(grant.endpoint.String())
	f.Add("https://foreign.example/object")
	f.Add("")
	f.Fuzz(func(t *testing.T, text string) {
		endpoint, err := core.ParseHTTPEndpoint(text)
		if err != nil {
			return
		}
		got, gotErr := ParseR2Grant(R2GrantInput{Endpoint: endpoint, Account: account, Jurisdiction: R2JurisdictionDefault, Method: exchange.MethodGet})
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrCloudflareContract) || got != (R2Grant{}) {
				t.Fatalf("grant refusal=(%v,%v), want zero typed refusal", got, gotErr)
			}
			return
		}
		if got.Validate() != nil || got.endpoint != endpoint || got.method != exchange.MethodGet || endpoint.HTTPURL().Host != account.value+core.CloudflareR2HostSuffix {
			t.Fatalf("grant validation = %v, method = %v, endpoint matches = %v, want nil, GET and exact input", got.Validate(), got.method, got.endpoint == endpoint)
		}
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, endpoint.String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if verifyR2TestSignature(request, []byte("r2-secret-key")) {
			signed := grant.endpoint.HTTPURL()
			observed := endpoint.HTTPURL()
			if signed.Host != observed.Host || signed.EscapedPath() != observed.EscapedPath() || signed.Query().Encode() != observed.Query().Encode() {
				t.Fatalf("verified request host/path = (%q,%q), want (%q,%q) and identical signed query", observed.Host, observed.EscapedPath(), signed.Host, signed.EscapedPath())
			}
		}
	})
}

// This is a structural grant-boundary test; mutated URLs are not claimed to
// carry an authentic signature. R2 remains the authentication authority.
func TestR2GrantSigningIdentityCannotExceedCredentialCustody(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr error
		name    string
		size    int
	}{
		{name: "one below credential custody remains representable", size: core.CloudflareSecretCustodyMaximumBytes - 1},
		{name: "exact credential custody remains representable", size: core.CloudflareSecretCustodyMaximumBytes},
		{name: "one above credential custody refuses impossible issuer state", size: core.CloudflareSecretCustodyMaximumBytes + 1, wantErr: core.ErrCloudflareBinding},
		{name: "above representation budget refuses query before decoding", size: core.CloudflareR2QueryMaximumBytes + 1, wantErr: core.ErrCloudflareBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := testR2Server(t, R2JurisdictionDefault)
			seed, err := server.Presign(t.Context(), testR2Intent(t, exchange.MethodGet, "bounded"))
			if err != nil {
				t.Fatal(err)
			}
			u := seed.endpoint.HTTPURL()
			query := u.Query()
			identity, scope, ok := strings.Cut(query.Get(core.CloudflareR2QuerySigningIdentity), "/")
			if !ok || len(identity) == tc.size {
				t.Fatalf("seed identity extent=%d, scope present=%v, want distinct mutation and scope", len(identity), ok)
			}
			query.Set(core.CloudflareR2QuerySigningIdentity, strings.Repeat("a", tc.size)+"/"+scope)
			u.RawQuery = query.Encode()
			endpoint, err := core.ParseHTTPEndpoint(u.String())
			if err != nil {
				t.Fatal(err)
			}
			got, gotErr := ParseR2Grant(R2GrantInput{Endpoint: endpoint, Account: server.account, Jurisdiction: R2JurisdictionDefault, Method: exchange.MethodGet})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ParseR2Grant(%d-byte signing identity) error=%v, want %v", tc.size, gotErr, tc.wantErr)
			}
			if gotErr != nil && got != (R2Grant{}) {
				t.Fatalf("refused grant=%v, want zero authority", got)
			}
		})
	}
}
