package exchange_test

import (
	"errors"
	"fmt"
	"net/http"
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
