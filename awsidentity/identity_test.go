package awsidentity

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	awsTestHost    = "sts.us-east-2.amazonaws.com"
	awsTestRegion  = "us-east-2"
	awsTestAccess  = "AKIATEST"
	awsTestDate    = "20260729"
	awsTestInstant = awsTestDate + "T120000Z"
	awsTestBearer  = "a.b.c"
)

type awsURLMutation uint8

const (
	awsURLUnchanged awsURLMutation = iota
	awsURLScheme
	awsURLHost
	awsURLPath
	awsURLDelete
	awsURLAppend
	awsURLSet
	awsURLUnknownField
)

type awsRewriteTransport struct {
	transport http.RoundTripper
	base      url.URL
}

func (r awsRewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.URL.Scheme, copy.URL.Host, copy.Host = r.base.Scheme, r.base.Host, ""
	return r.transport.RoundTrip(copy)
}

func mustAWSPolicy(tb testing.TB) Policy {
	tb.Helper()
	got, err := DefaultPolicy()
	if err != nil {
		tb.Fatalf("DefaultPolicy() error = %v, want nil", err)
	}
	return got
}

func mustAWSAudience(tb testing.TB) Audience {
	tb.Helper()
	got, err := ParseAudience("https://api.example.com/release")
	if err != nil {
		tb.Fatalf("ParseAudience() error = %v, want nil", err)
	}
	return got
}

func awsSignedURL(audience Audience, host, region string) string {
	// url.Values is used only at Go's wire-encoding boundary. The fixture's
	// domain is the production closed query enum; these are shape fixtures,
	// not authenticated SigV4 signatures.
	query := make(url.Values)
	for _, pair := range []struct {
		field amazonQueryField
		value string
	}{
		{amazonQueryFieldAction, amazonActionValue}, {amazonQueryFieldVersion, amazonVersionValue},
		{amazonQueryFieldAudience, audience.String()}, {amazonQueryFieldSigningAlgorithm, amazonSigningAlgorithmValue},
		{amazonQueryFieldDuration, amazonDurationValue}, {amazonQueryFieldSignatureAlgorithm, amazonSigAlgorithmValue},
		{amazonQueryFieldCredential, awsTestAccess + "/" + awsTestDate + "/" + region + "/" + amazonCredentialService + "/" + amazonCredentialTerminal},
		{amazonQueryFieldDate, awsTestInstant}, {amazonQueryFieldExpires, "60"},
		{amazonQueryFieldSignedHeaders, amazonSignedHeadersValue}, {amazonQueryFieldSignature, strings.Repeat("a", hex.EncodedLen(core.SHA256DigestBytes))},
	} {
		query.Set(pair.field.name(), pair.value)
	}

	return (&url.URL{Scheme: core.SchemeHTTPS, Host: host, Path: "/", RawQuery: query.Encode()}).String()
}

func mutateAWSURL(t *testing.T, raw string, kind awsURLMutation, field amazonQueryField, value string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v, want nil", raw, err)
	}
	switch kind {
	case awsURLScheme:
		parsed.Scheme = value
	case awsURLHost:
		parsed.Host = value
	case awsURLPath:
		parsed.Path = value
	case awsURLDelete:
		query := parsed.Query()
		query.Del(field.name())
		parsed.RawQuery = query.Encode()
	case awsURLAppend:
		query := parsed.Query()
		query.Add(field.name(), "duplicate")
		parsed.RawQuery = query.Encode()
	case awsURLSet, awsURLUnknownField:
		query := parsed.Query()
		name := field.name()
		if kind == awsURLUnknownField {
			name = "FutureParameter"
		}
		query.Set(name, value)
		parsed.RawQuery = query.Encode()
	default:
		t.Fatalf("URL mutation = %d, want a declared edit", kind)
	}
	return parsed.String()
}

func TestAWSRequestInputHostileBoundaryTable(t *testing.T) {
	t.Parallel()
	audience := mustAWSAudience(t)
	base := awsSignedURL(audience, awsTestHost, awsTestRegion)
	type testCase struct {
		wantErr error
		name    string
		kind    awsURLMutation
		field   amazonQueryField
		value   string
	}
	cases := []testCase{
		{name: "commercial regional endpoint is admitted"},
		{name: "commercial dual stack endpoint is admitted", kind: awsURLHost, value: "sts.us-east-2.api.aws"},
		{name: "commercial FIPS endpoint is admitted", kind: awsURLHost, value: "sts-fips.us-east-2.amazonaws.com"},
		{name: "commercial FIPS dual stack endpoint is admitted", kind: awsURLHost, value: "sts-fips.us-east-2.api.aws"},
		{name: "one second expiry is admitted", kind: awsURLSet, field: amazonQueryFieldExpires, value: "1"},
		{name: "exact maximum expiry is admitted", kind: awsURLSet, field: amazonQueryFieldExpires, value: strconv.Itoa(amazonSignedURLMaximumSecs)},
		{name: "session credential token is admitted", kind: awsURLSet, field: amazonQueryFieldSecurityToken, value: "session-token"},
		{name: "lowercase signature is admitted", kind: awsURLSet, field: amazonQueryFieldSignature, value: strings.Repeat("0", hex.EncodedLen(core.SHA256DigestBytes))},
		{name: "maximum hexadecimal signature is admitted", kind: awsURLSet, field: amazonQueryFieldSignature, value: strings.Repeat("f", hex.EncodedLen(core.SHA256DigestBytes))},
		{name: "root path omission is admitted", kind: awsURLPath, value: ""},
		{name: "global endpoint is refused", kind: awsURLHost, value: "sts.amazonaws.com", wantErr: core.ErrAWSIdentityContract},
		{name: "plaintext endpoint is refused", kind: awsURLScheme, value: "http", wantErr: core.ErrAWSIdentityContract},
		{name: "non root path is refused", kind: awsURLPath, value: "/identity", wantErr: core.ErrAWSIdentityContract},
		{name: "custom port is refused", kind: awsURLHost, value: "sts.us-east-2.amazonaws.com:8443", wantErr: core.ErrAWSIdentityContract},
		{name: "foreign host is refused", kind: awsURLHost, value: "identity.example.com", wantErr: core.ErrAWSIdentityContract},
		{name: "empty region is refused", kind: awsURLHost, value: "sts..amazonaws.com", wantErr: core.ErrAWSIdentityContract},
		{name: "dotted region is refused", kind: awsURLHost, value: "sts.us.east-2.amazonaws.com", wantErr: core.ErrAWSIdentityContract},
		{name: "wrong action is refused", kind: awsURLSet, field: amazonQueryFieldAction, value: "GetCallerIdentity", wantErr: core.ErrAWSIdentityContract},
		{name: "wrong version is refused", kind: awsURLSet, field: amazonQueryFieldVersion, value: "2026-01-01", wantErr: core.ErrAWSIdentityContract},
		{name: "contradictory audience is refused", kind: awsURLSet, field: amazonQueryFieldAudience, value: "other", wantErr: core.ErrAWSIdentityContract},
		{name: "wrong signing algorithm is refused", kind: awsURLSet, field: amazonQueryFieldSigningAlgorithm, value: "ES384", wantErr: core.ErrAWSIdentityContract},
		{name: "wrong duration is refused", kind: awsURLSet, field: amazonQueryFieldDuration, value: "3600", wantErr: core.ErrAWSIdentityContract},
		{name: "zero expiry is refused", kind: awsURLSet, field: amazonQueryFieldExpires, value: "0", wantErr: core.ErrAWSIdentityContract},
		{name: "noncanonical expiry is refused", kind: awsURLSet, field: amazonQueryFieldExpires, value: "060", wantErr: core.ErrAWSIdentityContract},
		{name: "expiry above maximum is refused", kind: awsURLSet, field: amazonQueryFieldExpires, value: strconv.Itoa(amazonSignedURLMaximumSecs + 1), wantErr: core.ErrAWSIdentityContract},
		{name: "short signature is refused", kind: awsURLSet, field: amazonQueryFieldSignature, value: strings.Repeat("a", hex.EncodedLen(core.SHA256DigestBytes)-1), wantErr: core.ErrAWSIdentityContract},
		{name: "long signature is refused", kind: awsURLSet, field: amazonQueryFieldSignature, value: strings.Repeat("a", hex.EncodedLen(core.SHA256DigestBytes)+1), wantErr: core.ErrAWSIdentityContract},
		{name: "uppercase noncanonical signature is refused", kind: awsURLSet, field: amazonQueryFieldSignature, value: strings.Repeat("A", hex.EncodedLen(core.SHA256DigestBytes)), wantErr: core.ErrAWSIdentityContract},
		{name: "nonhex signature is refused", kind: awsURLSet, field: amazonQueryFieldSignature, value: strings.Repeat("z", hex.EncodedLen(core.SHA256DigestBytes)), wantErr: core.ErrAWSIdentityContract},
		{name: "unknown query field is refused", kind: awsURLUnknownField, value: "value", wantErr: core.ErrAWSIdentityContract},
		{name: "missing action is refused", kind: awsURLDelete, field: amazonQueryFieldAction, wantErr: core.ErrAWSIdentityContract},
		{name: "missing version is refused", kind: awsURLDelete, field: amazonQueryFieldVersion, wantErr: core.ErrAWSIdentityContract},
		{name: "missing audience is refused", kind: awsURLDelete, field: amazonQueryFieldAudience, wantErr: core.ErrAWSIdentityContract},
		{name: "missing credential is refused", kind: awsURLDelete, field: amazonQueryFieldCredential, wantErr: core.ErrAWSIdentityContract},
		{name: "missing signed date is refused", kind: awsURLDelete, field: amazonQueryFieldDate, wantErr: core.ErrAWSIdentityContract},
		{name: "missing signature is refused", kind: awsURLDelete, field: amazonQueryFieldSignature, wantErr: core.ErrAWSIdentityContract},
		{name: "duplicate action is refused", kind: awsURLAppend, field: amazonQueryFieldAction, wantErr: core.ErrAWSIdentityContract},
		{name: "duplicate audience is refused", kind: awsURLAppend, field: amazonQueryFieldAudience, wantErr: core.ErrAWSIdentityContract},
		{name: "duplicate expiry is refused", kind: awsURLAppend, field: amazonQueryFieldExpires, wantErr: core.ErrAWSIdentityContract},
		{name: "duplicate signature is refused", kind: awsURLAppend, field: amazonQueryFieldSignature, wantErr: core.ErrAWSIdentityContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw := base
			if tc.kind != awsURLUnchanged {
				raw = mutateAWSURL(t, raw, tc.kind, tc.field, tc.value)
			}
			input := RequestInput{SignedURL: raw, Audience: audience, Policy: mustAWSPolicy(t)}
			got, gotErr := NewRequest(input)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (Request{}) {
					t.Fatalf("NewRequest() = (%v, %v), want zero and %v", got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || input.Validate() != nil || got.Validate() != nil {
				t.Fatalf("NewRequest accepted case error = %v, want nil", gotErr)
			}
			if got.endpoint.String() != raw || got.audience != audience || got.policy != input.Policy {
				t.Fatalf("NewRequest() = (%v, %v), want validated request", got, gotErr)
			}
		})
	}
}

func TestLayerTriadAWSAcquireRealTLS(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr   error
		name      string
		body      string
		status    int
		wantCalls uint64
		cancel    bool
		redirect  bool
	}{
		{name: "provider receipt produces redacted bearer", body: string(awsProviderBytes(t, awsProviderDocument(awsTestBearer))), status: http.StatusOK, wantCalls: 1},
		{name: "malformed provider receipt is refused", body: "<truncated", status: http.StatusOK, wantErr: core.ErrAWSIdentityContract, wantCalls: 1},
		{name: "cancelled intent performs no effect", body: string(awsProviderBytes(t, awsProviderDocument(awsTestBearer))), status: http.StatusOK, cancel: true, wantErr: context.Canceled},
		{name: "unauthorized status cannot disclose token", body: string(awsProviderBytes(t, awsProviderDocument(awsTestBearer))), status: http.StatusUnauthorized, wantCalls: 1, wantErr: core.ErrExchangeResponse},
		{name: "retryable server failure stays one attempt", body: string(awsProviderBytes(t, awsProviderDocument(awsTestBearer))), status: http.StatusServiceUnavailable, wantCalls: 1, wantErr: core.ErrExchangeResponse},
		{name: "permanent redirect refused", status: http.StatusMovedPermanently, redirect: true, wantCalls: 1, wantErr: core.ErrExchangeRedirect},
		{name: "found redirect refused", status: http.StatusFound, redirect: true, wantCalls: 1, wantErr: core.ErrExchangeRedirect},
		{name: "see other redirect refused", status: http.StatusSeeOther, redirect: true, wantCalls: 1, wantErr: core.ErrExchangeRedirect},
		{name: "temporary redirect refused", status: http.StatusTemporaryRedirect, redirect: true, wantCalls: 1, wantErr: core.ErrExchangeRedirect},
		{name: "method preserving permanent redirect refused", status: http.StatusPermanentRedirect, redirect: true, wantCalls: 1, wantErr: core.ErrExchangeRedirect},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Uint64
			observations := make(chan awsHTTPRequestObservation, 1)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var probe [1]byte
				n, readErr := r.Body.Read(probe[:])
				observed := awsHTTPRequestObservation{method: r.Method, path: r.URL.EscapedPath(), query: r.URL.RawQuery, bodyBytes: n, bodyErr: readErr, authorization: r.Header.Get(exchange.StandardHeaderAuthorization.String())}
				if tc.redirect {
					http.Redirect(w, r, "/redirected", tc.status)
				} else {
					w.WriteHeader(tc.status)
					_, observed.writeErr = io.WriteString(w, tc.body)
				}
				select {
				case observations <- observed:
				default:
				}
			}))
			defer server.Close()
			base, _ := url.Parse(server.URL)
			httpClient := &http.Client{Transport: awsRewriteTransport{base: *base, transport: server.Client().Transport}}
			exchangeClient, err := exchange.NewClient(httpClient)
			if err != nil {
				t.Fatalf("exchange.NewClient() error = %v, want nil", err)
			}
			client, err := NewClient(exchangeClient)
			if err != nil {
				t.Fatalf("NewClient() error = %v, want nil", err)
			}
			audience := mustAWSAudience(t)
			request, err := NewRequest(RequestInput{SignedURL: awsSignedURL(audience, awsTestHost, awsTestRegion), Audience: audience, Policy: mustAWSPolicy(t)})
			if err != nil {
				t.Fatalf("NewRequest() error = %v, want nil", err)
			}
			ctx := t.Context()
			if tc.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			got, gotErr := Acquire(ctx, client, request)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (Token{}) {
					t.Fatalf("Acquire() = (%v, %v), want zero and %v", got, gotErr, tc.wantErr)
				}
			} else {
				bearer, bearerErr := got.BearerValue()
				if gotErr != nil || bearerErr != nil || bearer != bearerPrefix+awsTestBearer || fmt.Sprint(got) != core.RedactedValueText {
					t.Fatalf("Acquire() bearer = (%q, %v, %v), want redacted valid bearer", bearer, gotErr, bearerErr)
				}
			}
			if tc.wantCalls > 0 {
				duration, durationErr := temporal.DurationFromSeconds(DefaultTimeoutSeconds)
				if durationErr != nil {
					t.Fatalf("observation backstop error = %v, want nil", durationErr)
				}
				waitContext, cancelWait, waitErr := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: duration})
				if waitErr != nil {
					t.Fatalf("observation context error = %v, want nil", waitErr)
				}
				defer cancelWait()
				select {
				case observed := <-observations:
					target := request.endpoint.HTTPURL()
					if observed.method != http.MethodGet || observed.path != target.EscapedPath() || observed.query != target.RawQuery || observed.bodyBytes != 0 || !errors.Is(observed.bodyErr, io.EOF) || observed.authorization != "" || observed.writeErr != nil {
						t.Fatalf("TLS observation = %+v, want exact signed GET, no body/header bearer, complete write", observed)
					}
				case <-waitContext.Done():
					t.Fatalf("TLS observation error = %v, want one observed request", waitContext.Err())
				}
			}
			if calls.Load() != tc.wantCalls {
				t.Fatalf("provider calls = %d, want %d", calls.Load(), tc.wantCalls)
			}
		})
	}
}

type awsHTTPRequestObservation struct {
	method, path, query, authorization string
	bodyBytes                          int
	bodyErr, writeErr                  error
}
