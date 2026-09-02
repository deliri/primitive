package exchange_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestReceiveBearerAuthorizationTreatsAuthenticationSchemeCaseInsensitively(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "canonical Bearer scheme is admitted", value: "Bearer token-123"},
		{name: "lowercase bearer scheme is admitted", value: "bearer token-123"},
		{name: "uppercase bearer scheme is admitted", value: "BEARER token-123"},
		{name: "mixed-case bearer scheme is admitted", value: "bEaReR token-123"},
		{name: "Basic sibling scheme is refused", value: "Basic token-123", wantErr: core.ErrExchangeRequest},
		{name: "missing scheme separator is refused", value: "Bearertoken-123", wantErr: core.ErrExchangeRequest},
		{name: "empty token is refused", value: "Bearer ", wantErr: core.ErrExchangeRequest},
		{name: "single padding byte cannot stand in for token material", value: "Bearer =", wantErr: core.ErrExchangeRequest},
		{name: "multiple padding bytes cannot stand in for token material", value: "Bearer ==", wantErr: core.ErrExchangeRequest},
		{name: "additional separator is not token material", value: "Bearer  token-123", wantErr: core.ErrExchangeRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.test", nil)
			if err != nil {
				t.Fatalf("http.NewRequestWithContext() error = %v, want nil", err)
			}
			request.Header.Set("Authorization", tc.value)
			call, callErr := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
			if callErr != nil {
				t.Fatalf("NewSocketServerCall() setup error = %v, want nil", callErr)
			}
			got, gotErr := exchange.ReceiveBearerAuthorization(call)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ReceiveBearerAuthorization(%q) error = %v, want %v", tc.value, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got.Validate() == nil {
					t.Fatalf("ReceiveBearerAuthorization(%q) = valid value, want zero", tc.value)
				}
				return
			}
			if got.Validate() != nil || string(got.Token) != "token-123" {
				t.Fatalf("ReceiveBearerAuthorization(%q) token = %q, want exact token", tc.value, got.Token)
			}
			clear(got.Token)
		})
	}
}

func TestBearerAuthorizationMatchesExactValidatedTokenBytes(t *testing.T) {
	t.Parallel()

	baseline := exchange.BearerAuthorization{Token: []byte("token-123")}
	equal, equalErr := exchange.BearerAuthorizationMatches(baseline, exchange.BearerAuthorization{Token: []byte("token-123")})
	if equalErr != nil || !equal {
		t.Fatalf("BearerAuthorizationMatches(equal) = (%t, %v), want (true, nil)", equal, equalErr)
	}
	different, differentErr := exchange.BearerAuthorizationMatches(baseline, exchange.BearerAuthorization{Token: []byte("token-124")})
	if differentErr != nil || different {
		t.Fatalf("BearerAuthorizationMatches(different) = (%t, %v), want (false, nil)", different, differentErr)
	}
	invalid, invalidErr := exchange.BearerAuthorizationMatches(baseline, exchange.BearerAuthorization{})
	if !errors.Is(invalidErr, core.ErrExchangeContract) || invalid {
		t.Fatalf("BearerAuthorizationMatches(invalid) = (%t, %v), want (false, errors.Is(..., %v))", invalid, invalidErr, core.ErrExchangeContract)
	}
}
