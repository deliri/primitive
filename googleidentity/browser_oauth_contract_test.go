package googleidentity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestBrowserOAuthAuthorizationURLLayerTriad(t *testing.T) {
	t.Parallel()
	id, e1 := core.ParseOAuthClientID("browser-client")
	callback, e2 := core.ParseOAuthRedirectURI("https://api.app.example/v1/auth/oauth/google/callback")
	state, e3 := core.ParseOAuthState("owned-state&+value")
	verifier, e4 := core.ParseOAuthPKCEVerifier(strings.Repeat("v", 43))
	nonce, e5 := core.ParseOAuthNonce("owned-nonce")
	challenge, e6 := verifier.Challenge()
	if err := errors.Join(e1, e2, e3, e4, e5, e6); err != nil {
		t.Fatalf("Google begin source values = %v, want nil", err)
	}
	request := BrowserAuthorizationRequest{ClientID: id, RedirectURI: callback, State: state, Challenge: challenge, Nonce: nonce}
	got, err := BeginBrowserAuthorization(request)
	if err != nil {
		t.Fatalf("Google begin = (%+v,%v), want admitted URL", got, err)
	}
	u := got.URL.HTTPURL()
	q := u.Query()
	if u.Scheme != "https" || u.Host != "accounts.google.com" || u.Path != "/o/oauth2/v2/auth" || len(q) != 8 || q.Get("client_id") != id.String() || q.Get("redirect_uri") != callback.String() || q.Get("response_type") != "code" || q.Get("scope") != "openid email profile" || q.Get("state") != state.String() || q.Get("nonce") != nonce.String() || q.Get("code_challenge") != challenge.String() || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("Google begin URL = %q, want exact server OAuth/S256 bindings", got.URL.String())
	}
	for _, row := range []struct {
		name  string
		clear func(*BrowserAuthorizationRequest)
	}{
		{name: "neutral/unissued request refuses", clear: func(r *BrowserAuthorizationRequest) { *r = BrowserAuthorizationRequest{} }},
		{name: "negative/missing anti-forgery state", clear: func(r *BrowserAuthorizationRequest) { r.State = core.OAuthState{} }},
		{name: "negative/missing proof-key challenge", clear: func(r *BrowserAuthorizationRequest) { r.Challenge = core.OAuthPKCEChallenge{} }},
		{name: "negative/missing OpenID nonce", clear: func(r *BrowserAuthorizationRequest) { r.Nonce = core.OAuthNonce{} }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			candidate := request
			row.clear(&candidate)
			got, err := BeginBrowserAuthorization(candidate)
			if !errors.Is(err, core.ErrGoogleIdentityContract) || got != (BrowserAuthorization{}) {
				t.Fatalf("refused Google begin = (%+v,%v), want zero URL and owning refusal", got, err)
			}
		})
	}
}

func TestBrowserOAuthReusableVerifierAndClockSkew(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name      string
		issued    int64
		skew      int64
		wantErr   error
		wantCalls uint32
	}{
		{name: "positive/RSA guard admits authenticated boundary", issued: 946684800, wantCalls: 2},
		{name: "boundary/subsecond tolerance below issued instant refuses", issued: 946684801, skew: 999999999, wantErr: core.ErrGoogleIdentityContract, wantCalls: 2},
		{name: "boundary/exact one second tolerance admits", issued: 946684801, skew: 1000000000, wantCalls: 2},
		{name: "boundary/exact60second tolerance admits", issued: 946684860, skew: 60000000000, wantCalls: 2},
		{name: "boundary/one second beyond60refuses", issued: 946684861, skew: 60000000000, wantErr: core.ErrGoogleIdentityContract, wantCalls: 2},
		{name: "negative/skew beyond configured safety ceiling refuses before provider", issued: 946684800, skew: 60000000001, wantErr: core.ErrGoogleIdentityContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				claims := browserClaimsSeed()
				claims.IssuedAt = row.issued
				var calls atomic.Uint32
				client := browserLocalClient(t, signBrowserFixture(t, claims, false), 200, &calls)
				request := browserRequestFixture(t, client)
				skew, err := temporal.DurationFromNanoseconds(row.skew)
				if err != nil {
					t.Fatal(err)
				}
				request.ClockSkew = skew
				got, err := CompleteBrowserAuthorization(t.Context(), request)
				if !errors.Is(err, row.wantErr) || calls.Load() != row.wantCalls {
					t.Fatalf("Google skew result = (%+v,%v,%dcalls), want %v/%dcalls", got, err, calls.Load(), row.wantErr, row.wantCalls)
				}
				if row.wantErr != nil && got != (BrowserVerifiedIdentity{}) {
					t.Fatalf("refused skew identity = %+v, want zero", got)
				}
				if row.wantErr == nil && got.Subject.String() != claims.Subject {
					t.Fatalf("admitted subject = %q, want %q", got.Subject.String(), claims.Subject)
				}
			})
		})
	}
	t.Run("positive/official JWKS cache survives separate code exchanges", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			var calls atomic.Uint32
			client := browserLocalClient(t, signBrowserFixture(t, browserClaimsSeed(), false), 200, &calls)
			request := browserRequestFixture(t, client)
			first, e1 := CompleteBrowserAuthorization(t.Context(), request)
			second, e2 := CompleteBrowserAuthorization(t.Context(), request)
			if e1 != nil || e2 != nil || first != second || calls.Load() != 3 {
				t.Fatalf("reused official verifier = (%+v,%+v,%v,%v,%dcalls), want same exact identity and2token+1JWKS requests", first, second, e1, e2, calls.Load())
			}
		})
	})
	t.Run("negative/foreign configured audience refuses before network", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			var calls atomic.Uint32
			client := browserLocalClient(t, "", 200, &calls)
			request := browserRequestFixture(t, client)
			other, err := core.ParseOAuthClientID("foreign-client")
			if err != nil {
				t.Fatal(err)
			}
			request.IdentityVerifier, err = NewBrowserVerifier(t.Context(), BrowserVerifierConfiguration{Client: client, ClientID: other})
			if err != nil {
				t.Fatal(err)
			}
			got, err := CompleteBrowserAuthorization(t.Context(), request)
			if !errors.Is(err, core.ErrGoogleIdentityContract) || got != (BrowserVerifiedIdentity{}) || calls.Load() != 0 {
				t.Fatalf("foreign verifier audience = (%+v,%v,%dcalls), want zero and no provider request", got, err, calls.Load())
			}
		})
	})
}
func TestBrowserOAuthRequestFormattingRedactsProofCustody(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Uint32
		request := browserRequestFixture(t, browserLocalClient(t, "", 200, &calls))
		for _, verb := range []string{"%v", "%+v", "%#v"} {
			for _, text := range []string{fmt.Sprintf(verb, request), fmt.Sprintf(verb, request.Code), fmt.Sprintf(verb, request.Verifier)} {
				if strings.Contains(text, request.Code.String()) || strings.Contains(text, request.Verifier.String()) || strings.Contains(text, "public-test-only-client-secret") {
					t.Fatalf("request formatting with %s retained private proof material, want redaction", verb)
				}
			}
		}
		if calls.Load() != 0 {
			t.Fatalf("formatting provider calls = %d, want0", calls.Load())
		}
	})
}
func TestBrowserOAuthVerifierConstructorRefusesCanceledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	id, err := core.ParseOAuthClientID("browser-client")
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Uint32
	client := browserLocalClient(t, "", 200, &calls)
	got, err := NewBrowserVerifier(ctx, BrowserVerifierConfiguration{Client: client, ClientID: id})
	if !errors.Is(err, context.Canceled) || got != (BrowserVerifier{}) || calls.Load() != 0 {
		t.Fatalf("canceled verifier constructor = (%+v,%v,%dcalls), want zero and preserved cancellation", got, err, calls.Load())
	}
}
