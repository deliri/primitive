package cloudflare

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestR2CacheControlDurationBoundaries(t *testing.T) {
	t.Parallel()
	second := int64(temporal.NanosecondsPerSecond)
	maximum := int64(core.CloudflareR2CacheMaxAgeMaximumSeconds) * second
	for _, tc := range []struct {
		name    string
		nanos   int64
		wantErr error
	}{
		{name: "zero explicitly requests immediate staleness"},
		{name: "smallest fraction cannot silently round down", nanos: 1, wantErr: core.ErrCloudflareBinding},
		{name: "one nanosecond below second is refused", nanos: second - 1, wantErr: core.ErrCloudflareBinding},
		{name: "exact second is representable", nanos: second},
		{name: "one nanosecond above second is refused", nanos: second + 1, wantErr: core.ErrCloudflareBinding},
		{name: "one second below ceiling is representable", nanos: maximum - second},
		{name: "exact ceiling is representable", nanos: maximum},
		{name: "one second above ceiling refuses overflow ambiguity", nanos: maximum + second, wantErr: core.ErrCloudflareBinding},
		{name: "extreme duration is refused", nanos: math.MaxInt64, wantErr: core.ErrCloudflareBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			duration, err := temporal.DurationFromNanoseconds(tc.nanos)
			if err != nil {
				t.Fatal(err)
			}
			got, err := NewR2CacheControl(duration)
			if !errors.Is(err, tc.wantErr) || err != nil && got != (R2CacheControl{}) || err == nil && (got.mode != r2CacheMaxAge || got.maxAge != duration || got.Validate() != nil) {
				t.Fatalf("cache admission=(%+v,%v), want %v and exact duration or zero", got, err, tc.wantErr)
			}
		})
	}
}

func FuzzR2CacheControlSignedGrant(f *testing.F) {
	seed := testR2CacheControl(f, 30)
	f.Add(seed.maxAge.Nanoseconds())
	f.Add(int64(0))
	f.Add(int64(1))
	f.Add(int64(core.CloudflareR2CacheMaxAgeMaximumSeconds) * int64(temporal.NanosecondsPerSecond))
	f.Add(int64(math.MaxInt64))
	f.Fuzz(func(t *testing.T, nanos int64) {
		duration, err := temporal.DurationFromNanoseconds(nanos)
		if err != nil {
			if !errors.Is(err, core.ErrTemporalContract) || duration != (temporal.Duration{}) {
				t.Fatalf("duration=(%v,%v), want typed zero refusal", duration, err)
			}
			return
		}
		cache, err := NewR2CacheControl(duration)
		seconds := nanos / int64(temporal.NanosecondsPerSecond)
		wantValid := nanos == seconds*int64(temporal.NanosecondsPerSecond) && seconds <= core.CloudflareR2CacheMaxAgeMaximumSeconds
		if !wantValid {
			if !errors.Is(err, core.ErrCloudflareBinding) || cache != (R2CacheControl{}) {
				t.Fatalf("cache=(%+v,%v), want zero/binding refusal", cache, err)
			}
			return
		}
		if err != nil || cache.Validate() != nil {
			t.Fatalf("cache admission error=%v, want nil", err)
		}
		server := testR2Server(t, R2JurisdictionDefault)
		intent := testR2Intent(t, exchange.MethodPut, "cache/object")
		intent.CacheControl = cache
		grant, err := server.Presign(t.Context(), intent)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := ParseR2Grant(R2GrantInput{Account: server.account, Jurisdiction: server.jurisdiction, Method: intent.Method, Endpoint: grant.endpoint, CacheControl: cache})
		if err != nil || parsed != grant {
			t.Fatalf("signed cache grant round trip=(%v,%v), want original", parsed, err)
		}
		request, err := http.NewRequestWithContext(t.Context(), http.MethodPut, grant.endpoint.String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set(exchange.StandardHeaderCacheControl.String(), core.CloudflareR2CacheMaxAgePrefix+strconv.FormatInt(seconds, 10))
		if !verifyR2TestSignature(request, []byte("r2-secret-key")) {
			t.Fatal("exact cache lifetime signature=false, want true")
		}
		request.Header.Set(exchange.StandardHeaderCacheControl.String(), core.CloudflareR2CacheMaxAgePrefix+strconv.FormatInt(seconds+1, 10))
		if verifyR2TestSignature(request, []byte("r2-secret-key")) {
			t.Fatal("changed cache lifetime signature=true, want false")
		}
		input := R2GrantInput{Account: server.account, Jurisdiction: server.jurisdiction, Method: intent.Method, Endpoint: grant.endpoint}
		got, err := ParseR2Grant(input)
		if !errors.Is(err, core.ErrCloudflareBinding) || got != (R2Grant{}) {
			t.Fatalf("omitted signed cache=(%v,%v), want zero/binding refusal", got, err)
		}
	})
}

func FuzzR2CacheControlModeSemanticClosure(f *testing.F) {
	seed := R2NoStoreCacheControl()
	f.Add(uint8(seed.mode), int64(0))
	f.Add(uint8(r2CacheOmit), int64(0))
	f.Add(uint8(r2CacheMaxAge), int64(temporal.NanosecondsPerSecond))
	f.Add(uint8(r2CacheNoStore), int64(temporal.NanosecondsPerSecond))
	f.Add(uint8(255), int64(0))
	f.Fuzz(func(t *testing.T, raw uint8, nanos int64) {
		duration, err := temporal.DurationFromNanoseconds(nanos)
		if err != nil {
			if !errors.Is(err, core.ErrTemporalContract) {
				t.Fatalf("duration error=%v, want Temporal", err)
			}
			return
		}
		cache := R2CacheControl{mode: r2CacheMode(raw), maxAge: duration}
		valid := (raw == uint8(r2CacheOmit) || raw == uint8(r2CacheNoStore)) && nanos == 0 || raw == uint8(r2CacheMaxAge) && nanos%int64(temporal.NanosecondsPerSecond) == 0 && nanos/int64(temporal.NanosecondsPerSecond) <= core.CloudflareR2CacheMaxAgeMaximumSeconds
		server := testR2Server(t, R2JurisdictionDefault)
		request := testR2Intent(t, exchange.MethodPut, "private/cache-mode")
		request.CacheControl = cache
		grant, err := server.Presign(t.Context(), request)
		if !valid {
			if !errors.Is(err, core.ErrCloudflareBinding) || grant != (R2Grant{}) {
				t.Fatalf("invalid mode=%d nanos=%d result=(%v,%v),want zero/binding", raw, nanos, grant, err)
			}
			return
		}
		if err != nil || cache.Validate() != nil {
			t.Fatalf("admitted mode=%d nanos=%d error=%v,want nil", raw, nanos, err)
		}
		headers, err := grant.Headers()
		if err != nil {
			t.Fatal(err)
		}
		want := ""
		if raw == uint8(r2CacheNoStore) {
			want = core.CloudflareR2CacheNoStore
		}
		if raw == uint8(r2CacheMaxAge) {
			want = core.CloudflareR2CacheMaxAgePrefix + strconv.FormatInt(nanos/int64(temporal.NanosecondsPerSecond), 10)
		}
		count := 0
		if want != "" {
			count = 1
		}
		if len(headers.Values) != count {
			t.Fatalf("cache fields=%d,want %d", len(headers.Values), count)
		}
		native, err := http.NewRequestWithContext(t.Context(), http.MethodPut, grant.endpoint.String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if count == 1 {
			value, err := headers.Values[0].Values[0].Value()
			if err != nil || value != want {
				t.Fatalf("cache=%q,%v,want %q,nil", value, err, want)
			}
			native.Header.Set(exchange.StandardHeaderCacheControl.String(), value)
		}
		if !verifyR2TestSignature(native, []byte("r2-secret-key")) {
			t.Fatal("mode signature=false,want true")
		}
		if count == 1 {
			native.Header.Set(exchange.StandardHeaderCacheControl.String(), want+"x")
			if verifyR2TestSignature(native, []byte("r2-secret-key")) {
				t.Fatal("modified mode signature=true,want false")
			}
		}
		parsed, err := ParseR2Grant(R2GrantInput{Account: server.account, Jurisdiction: server.jurisdiction, Method: request.Method, Endpoint: grant.endpoint, CacheControl: cache})
		if err != nil || parsed != grant {
			t.Fatalf("mode grant roundtrip=(%v,%v),want exact,nil", parsed, err)
		}
	})
}
