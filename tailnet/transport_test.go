package tailnet

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type observingEnrollmentTransport struct {
	context context.Context
	calls   int
	matched bool
}

func (p *observingEnrollmentTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	p.calls++
	p.matched = request.Context() == p.context
	return nil, core.ErrTailnetEnrollment
}

type trackedRequestBody struct {
	*strings.Reader
	closes int
}

func (b *trackedRequestBody) Close() error { b.closes++; return nil }

func TestEnrollmentTransportContextAndRequestTable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		path          string
		host          string
		scheme        string
		authorization string
		cancel        bool
		wantCalls     int
		wantCause     error
	}{
		{name: "SDK background token request inherits exact owning context", path: tokenExchangePath, wantCalls: 1},
		{name: "authenticated key request inherits exact owning context", path: createKeyPath, authorization: exchange.BearerAuthorizationScheme + " " + fixtureAccessToken, wantCalls: 1},
		{name: "cancelled operation closes unsent SDK body", path: tokenExchangePath, cancel: true, wantCause: context.Canceled},
		{name: "empty SDK credential prevents key request", path: createKeyPath, authorization: exchange.BearerAuthorizationScheme + " "},
		{name: "wrong authorization scheme prevents key request", path: createKeyPath, authorization: "Basic abc"},
		{name: "credential whitespace cannot become multiple fields", path: createKeyPath, authorization: exchange.BearerAuthorizationScheme + " a b"},
		{name: "foreign provider cannot receive credentials", path: tokenExchangePath, host: "foreign.invalid"},
		{name: "plaintext provider cannot receive credentials", path: tokenExchangePath, scheme: "http"},
		{name: "unknown operation cannot broaden SDK capability", path: "/api/v2/devices"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			host := tc.host
			if host == "" {
				host = enrollmentAPIHost
			}
			scheme := tc.scheme
			if scheme == "" {
				scheme = "https"
			}
			body := &trackedRequestBody{Reader: strings.NewReader("fixture")}
			request, err := http.NewRequest(http.MethodPost, scheme+"://"+host+tc.path, body)
			if err != nil {
				t.Fatalf("request construction = %v, want nil", err)
			}
			request.Header.Set("Authorization", tc.authorization)
			if tc.cancel {
				cancel()
			}
			observed := &observingEnrollmentTransport{context: ctx}
			transport := enrollmentTransport{context: ctx, base: observed}
			got, err := transport.RoundTrip(request)
			wantCause := tc.wantCause
			if wantCause == nil {
				wantCause = core.ErrTailnetEnrollment
			}
			if got != nil || !errors.Is(err, wantCause) || observed.calls != tc.wantCalls {
				t.Fatalf("RoundTrip() = (%v,%v,%d calls), want nil,%v,%d", got, err, observed.calls, wantCause, tc.wantCalls)
			}
			if tc.wantCalls == 1 && !observed.matched {
				t.Fatalf("request context inheritance = %t, want true", observed.matched)
			}
			if tc.wantCalls == 0 && body.closes != 1 {
				t.Fatalf("unsent body closes = %d, want 1", body.closes)
			}
			// The observing transport is a unit seam; it owns bodies on delegated paths.
			if tc.wantCalls == 1 {
				if err := body.Close(); err != nil {
					t.Errorf("fixture body close = %v, want nil", err)
				}
			}
		})
	}
}
