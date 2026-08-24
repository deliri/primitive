package exchange_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestBasicAuthorizationHeaderBoundaryAndRedactionMatrix(t *testing.T) {
	t.Parallel()

	type mutation func(*exchange.BasicAuthorizationRequest)
	cases := []struct {
		wantErr error
		mutate  mutation
		name    string
	}{
		{name: "ordinary ASCII credentials are admitted"},
		{name: "Unicode credentials are admitted", mutate: func(r *exchange.BasicAuthorizationRequest) {
			r.Identity = exchange.BasicAuthorizationIdentity("équipe")
			r.Secret = []byte("密碼")
		}},
		{name: "identity exact byte ceiling is admitted", mutate: func(r *exchange.BasicAuthorizationRequest) {
			r.Identity = exchange.BasicAuthorizationIdentity(strings.Repeat("i", exchange.BasicAuthorizationIdentityMaximumBytes))
		}},
		{name: "secret exact byte ceiling is admitted", mutate: func(r *exchange.BasicAuthorizationRequest) {
			r.Secret = []byte(strings.Repeat("s", exchange.BasicAuthorizationSecretMaximumBytes))
		}},
		{name: "empty identity is rejected", mutate: func(r *exchange.BasicAuthorizationRequest) { r.Identity = "" }, wantErr: core.ErrExchangeContract},
		{name: "empty secret is rejected", mutate: func(r *exchange.BasicAuthorizationRequest) { r.Secret = nil }, wantErr: core.ErrExchangeContract},
		{name: "identity one above byte ceiling is rejected", mutate: func(r *exchange.BasicAuthorizationRequest) {
			r.Identity = exchange.BasicAuthorizationIdentity(strings.Repeat("i", exchange.BasicAuthorizationIdentityMaximumBytes+1))
		}, wantErr: core.ErrExchangeContract},
		{name: "secret one above byte ceiling is rejected", mutate: func(r *exchange.BasicAuthorizationRequest) {
			r.Secret = []byte(strings.Repeat("s", exchange.BasicAuthorizationSecretMaximumBytes+1))
		}, wantErr: core.ErrExchangeContract},
		{name: "identity delimiter is rejected", mutate: func(r *exchange.BasicAuthorizationRequest) { r.Identity = "operator:name" }, wantErr: core.ErrExchangeContract},
		{name: "identity newline is rejected", mutate: func(r *exchange.BasicAuthorizationRequest) { r.Identity = "operator\n" }, wantErr: core.ErrExchangeContract},
		{name: "secret newline is rejected", mutate: func(r *exchange.BasicAuthorizationRequest) { r.Secret = []byte("secret\n") }, wantErr: core.ErrExchangeContract},
		{name: "invalid identity UTF-8 is rejected", mutate: func(r *exchange.BasicAuthorizationRequest) {
			r.Identity = exchange.BasicAuthorizationIdentity(string([]byte{0xff}))
		}, wantErr: core.ErrExchangeContract},
		{name: "invalid secret UTF-8 is rejected", mutate: func(r *exchange.BasicAuthorizationRequest) { r.Secret = []byte{0xff} }, wantErr: core.ErrExchangeContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := exchange.BasicAuthorizationRequest{Identity: exchange.BasicAuthorizationIdentity("operator"), Secret: []byte("secret")}
			if tc.mutate != nil {
				tc.mutate(&request)
			}
			got, gotErr := exchange.NewBasicAuthorizationHeader(request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("NewBasicAuthorizationHeader() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got.Name.String() != "" || len(got.Values) != 0 {
					t.Fatalf("NewBasicAuthorizationHeader(rejected) = %+v, want zero", got)
				}
				return
			}
			value, valueErr := got.Values[0].Value()
			if valueErr != nil {
				t.Fatalf("HeaderValue.Value() error = %v, want nil", valueErr)
			}
			probe, probeErr := http.NewRequest(http.MethodGet, "https://example.invalid", nil)
			if probeErr != nil {
				t.Fatalf("http.NewRequest() error = %v, want nil", probeErr)
			}
			probe.Header.Set(got.Name.String(), value)
			identity, secret, okay := probe.BasicAuth()
			if !okay || identity != request.Identity.String() || secret != string(request.Secret) {
				t.Fatalf("BasicAuth() = (%q, %q, %t), want exact credentials and true", identity, secret, okay)
			}
			if formatted := fmt.Sprintf("%v %+v", request, got.Values[0]); formatted != core.RedactedValueText+" "+core.RedactedValueText {
				t.Fatalf("generic formatting = %q, want two redactions", formatted)
			}
		})
	}
}

func TestReceiveBasicAuthorizationHostileBoundaryMatrix(t *testing.T) {
	t.Parallel()

	type setup func(*testing.T, core.HTTPHeaderName) *http.Request
	valid := func(identity string, secret string) setup {
		return func(t *testing.T, headerName core.HTTPHeaderName) *http.Request {
			t.Helper()
			credentials := exchange.BasicAuthorizationRequest{
				Identity: exchange.BasicAuthorizationIdentity(identity),
				Secret:   []byte(secret),
			}
			header, err := exchange.NewBasicAuthorizationHeader(credentials)
			if err != nil {
				t.Fatalf("NewBasicAuthorizationHeader() error = %v, want nil", err)
			}
			value, err := header.Values[0].Value()
			if err != nil {
				t.Fatalf("HeaderValue.Value() error = %v, want nil", err)
			}
			request := newBasicAuthorizationRequest(t)
			request.Header.Set(headerName.String(), value)
			return request
		}
	}
	raw := func(values ...string) setup {
		return func(t *testing.T, headerName core.HTTPHeaderName) *http.Request {
			t.Helper()
			request := newBasicAuthorizationRequest(t)
			for _, value := range values {
				request.Header.Add(headerName.String(), value)
			}
			return request
		}
	}
	encoded := func(rawCredentials []byte) string {
		return "Basic " + base64.StdEncoding.EncodeToString(rawCredentials)
	}
	identityMaximum := strings.Repeat("i", exchange.BasicAuthorizationIdentityMaximumBytes)
	secretMaximum := strings.Repeat("s", exchange.BasicAuthorizationSecretMaximumBytes)
	cases := []struct {
		setup        setup
		wantErr      error
		wantIdentity exchange.BasicAuthorizationIdentity
		wantSecret   string
		name         string
	}{
		{name: "ordinary ASCII credentials cross the real request boundary", setup: valid("operator", "secret"), wantIdentity: "operator", wantSecret: "secret"},
		{name: "one-byte identity is admitted", setup: valid("i", "secret"), wantIdentity: "i", wantSecret: "secret"},
		{name: "one-byte secret is admitted", setup: valid("operator", "s"), wantIdentity: "operator", wantSecret: "s"},
		{name: "Unicode identity is admitted", setup: valid("équipe", "secret"), wantIdentity: "équipe", wantSecret: "secret"},
		{name: "Unicode secret is admitted", setup: valid("operator", "密碼"), wantIdentity: "operator", wantSecret: "密碼"},
		{name: "identity one below byte ceiling is admitted", setup: valid(strings.Repeat("i", exchange.BasicAuthorizationIdentityMaximumBytes-1), "secret"), wantIdentity: exchange.BasicAuthorizationIdentity(strings.Repeat("i", exchange.BasicAuthorizationIdentityMaximumBytes-1)), wantSecret: "secret"},
		{name: "identity exact byte ceiling is admitted", setup: valid(identityMaximum, "secret"), wantIdentity: exchange.BasicAuthorizationIdentity(identityMaximum), wantSecret: "secret"},
		{name: "secret one below byte ceiling is admitted", setup: valid("operator", strings.Repeat("s", exchange.BasicAuthorizationSecretMaximumBytes-1)), wantIdentity: "operator", wantSecret: strings.Repeat("s", exchange.BasicAuthorizationSecretMaximumBytes-1)},
		{name: "secret exact byte ceiling is admitted", setup: valid("operator", secretMaximum), wantIdentity: "operator", wantSecret: secretMaximum},
		{name: "identity and secret exact byte ceilings are admitted", setup: valid(identityMaximum, secretMaximum), wantIdentity: exchange.BasicAuthorizationIdentity(identityMaximum), wantSecret: secretMaximum},
		{name: "identity punctuation is admitted", setup: valid("agent.operator+1@example.invalid", "secret"), wantIdentity: "agent.operator+1@example.invalid", wantSecret: "secret"},
		{name: "secret punctuation and delimiter are admitted", setup: valid("operator", "a:b-c_d+e/f="), wantIdentity: "operator", wantSecret: "a:b-c_d+e/f="},
		{name: "nil request is rejected", setup: func(*testing.T, core.HTTPHeaderName) *http.Request { return nil }, wantErr: core.ErrExchangeRequest},
		{name: "absent authorization header is rejected", setup: raw(), wantErr: core.ErrExchangeRequest},
		{name: "empty authorization header is rejected", setup: raw(""), wantErr: core.ErrExchangeRequest},
		{name: "Basic scheme without credentials is rejected", setup: raw("Basic "), wantErr: core.ErrExchangeRequest},
		{name: "case-insensitive Basic scheme follows the standard library", setup: raw("basic dXNlcjpwYXNz"), wantIdentity: "user", wantSecret: "pass"},
		{name: "Bearer scheme is rejected", setup: raw("Bearer token"), wantErr: core.ErrExchangeRequest},
		{name: "leading whitespace before scheme is rejected", setup: raw(" Basic dXNlcjpwYXNz"), wantErr: core.ErrExchangeRequest},
		{name: "trailing whitespace after payload is rejected", setup: raw("Basic dXNlcjpwYXNz "), wantErr: core.ErrExchangeRequest},
		{name: "invalid base64 alphabet is rejected", setup: raw("Basic !!!!"), wantErr: core.ErrExchangeRequest},
		{name: "truncated base64 quantum is rejected", setup: raw("Basic dXNlcjpwYXN"), wantErr: core.ErrExchangeRequest},
		{name: "decoded payload without delimiter is rejected", setup: raw(encoded([]byte("operator"))), wantErr: core.ErrExchangeRequest},
		{name: "decoded empty identity is rejected", setup: raw(encoded([]byte(":secret"))), wantErr: core.ErrExchangeRequest},
		{name: "decoded empty secret is rejected", setup: raw(encoded([]byte("operator:"))), wantErr: core.ErrExchangeRequest},
		{name: "identity one above byte ceiling is rejected", setup: raw(encoded([]byte(strings.Repeat("i", exchange.BasicAuthorizationIdentityMaximumBytes+1) + ":secret"))), wantErr: core.ErrExchangeRequest},
		{name: "secret one above byte ceiling is rejected", setup: raw(encoded([]byte("operator:" + strings.Repeat("s", exchange.BasicAuthorizationSecretMaximumBytes+1)))), wantErr: core.ErrExchangeRequest},
		{name: "header one above compiler ceiling is rejected before decode", setup: raw(strings.Repeat("A", exchange.BasicAuthorizationHeaderMaximumBytes+1)), wantErr: core.ErrExchangeRequest},
		{name: "duplicate identical headers are rejected", setup: raw(encoded([]byte("operator:secret")), encoded([]byte("operator:secret"))), wantErr: core.ErrExchangeRequest},
		{name: "duplicate conflicting headers are rejected", setup: raw(encoded([]byte("operator:secret")), encoded([]byte("other:secret"))), wantErr: core.ErrExchangeRequest},
		{name: "invalid UTF-8 identity is rejected", setup: raw(encoded([]byte{0xff, ':', 's'})), wantErr: core.ErrExchangeRequest},
		{name: "invalid UTF-8 secret is rejected", setup: raw(encoded([]byte{'i', ':', 0xff})), wantErr: core.ErrExchangeRequest},
		{name: "identity newline is rejected", setup: raw(encoded([]byte("operator\n:secret"))), wantErr: core.ErrExchangeRequest},
		{name: "identity carriage return is rejected", setup: raw(encoded([]byte("operator\r:secret"))), wantErr: core.ErrExchangeRequest},
		{name: "identity NUL is rejected", setup: raw(encoded([]byte("operator\x00:secret"))), wantErr: core.ErrExchangeRequest},
		{name: "secret newline is rejected", setup: raw(encoded([]byte("operator:secret\n"))), wantErr: core.ErrExchangeRequest},
		{name: "secret carriage return is rejected", setup: raw(encoded([]byte("operator:secret\r"))), wantErr: core.ErrExchangeRequest},
		{name: "secret NUL is rejected", setup: raw(encoded([]byte("operator:secret\x00"))), wantErr: core.ErrExchangeRequest},
		{name: "secret delete control is rejected", setup: raw(encoded([]byte("operator:secret\x7f"))), wantErr: core.ErrExchangeRequest},
		{name: "base64 payload with extra padding is rejected", setup: raw("Basic dXNlcjpwYXNz=="), wantErr: core.ErrExchangeRequest},
	}

	headerName, err := exchange.StandardHeaderAuthorization.Name()
	if err != nil {
		t.Fatalf("StandardHeaderAuthorization.Name() error = %v, want nil", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := tc.setup(t, headerName)
			var before []string
			if request != nil {
				before = slices.Clone(request.Header.Values(headerName.String()))
			}
			got, gotErr := exchange.ReceiveBasicAuthorization(exchange.BasicAuthorizationReceiveCall{Request: request})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ReceiveBasicAuthorization() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !errors.Is(gotErr, core.ErrExchangeContract) {
					t.Fatalf("ReceiveBasicAuthorization(rejected) error = %v, want %v", gotErr, core.ErrExchangeContract)
				}
				if got.Identity != "" || got.Secret != nil {
					t.Fatalf("ReceiveBasicAuthorization(rejected) = %v, want zero", got)
				}
			} else if got.Identity != tc.wantIdentity || !bytes.Equal(got.Secret, []byte(tc.wantSecret)) {
				t.Fatalf("ReceiveBasicAuthorization() = %v, want identity %q and exact secret", got, tc.wantIdentity)
			}
			if request != nil {
				after := request.Header.Values(headerName.String())
				if !slices.Equal(after, before) {
					t.Fatalf("ReceiveBasicAuthorization() headers = %q, want unchanged %q", after, before)
				}
			}
		})
	}
}

func newBasicAuthorizationRequest(t *testing.T) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, "https://example.invalid", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v, want nil", err)
	}
	return request
}
