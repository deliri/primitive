package github

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestBrowserOAuthAuthorizationURLLayerTriad(t *testing.T) {
	t.Parallel()
	id, e1 := core.ParseOAuthClientID("github-browser-client")
	callback, e2 := core.ParseOAuthRedirectURI("https://api.app.example/v1/auth/oauth/github/callback")
	state, e3 := core.ParseOAuthState("owned-state&+value")
	verifier, e4 := core.ParseOAuthPKCEVerifier(strings.Repeat("g", 43))
	challenge, e5 := verifier.Challenge()
	if err := errors.Join(e1, e2, e3, e4, e5); err != nil {
		t.Fatal(err)
	}
	request := BrowserAuthorizationRequest{ClientID: id, RedirectURI: callback, State: state, Challenge: challenge}
	got, err := BeginBrowserAuthorization(request)
	if err != nil {
		t.Fatalf("GitHub begin = (%+v,%v), want admitted URL", got, err)
	}
	u := got.URL.HTTPURL()
	q := u.Query()
	if u.Scheme != "https" || u.Host != "github.com" || u.Path != "/login/oauth/authorize" || len(q) != 7 || q.Get("client_id") != id.String() || q.Get("redirect_uri") != callback.String() || q.Get("response_type") != "code" || q.Get("scope") != "user:email" || q.Get("state") != state.String() || q.Get("code_challenge") != challenge.String() || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("GitHub begin URL = %q, want exact code/S256/email bindings", got.URL.String())
	}
	for _, row := range []struct {
		name  string
		clear func(*BrowserAuthorizationRequest)
	}{
		{name: "neutral/unissued request refuses", clear: func(r *BrowserAuthorizationRequest) { *r = BrowserAuthorizationRequest{} }},
		{name: "negative/missing anti-forgery state", clear: func(r *BrowserAuthorizationRequest) { r.State = core.OAuthState{} }},
		{name: "negative/missing proof-key challenge", clear: func(r *BrowserAuthorizationRequest) { r.Challenge = core.OAuthPKCEChallenge{} }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			candidate := request
			row.clear(&candidate)
			got, err := BeginBrowserAuthorization(candidate)
			if !errors.Is(err, core.ErrGitHubContract) || got != (BrowserAuthorization{}) {
				t.Fatalf("refused GitHub begin = (%+v,%v), want zero and owning refusal", got, err)
			}
		})
	}
}
func TestBrowserOAuthMissingEmailScopeKeepsSubject(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name  string
		scope string
		email bool
		calls uint64
	}{
		{name: "neutral/no scope skips forbidden email read", scope: "", calls: 2},
		{name: "neutral/profile scope does not grant email", scope: "read:user", calls: 2},
		{name: "neutral/repository scope does not grant email", scope: "repo", calls: 2},
		{name: "positive/direct email grant", scope: "user:email", email: true, calls: 3},
		{name: "positive/parent user grant includes email", scope: "user", email: true, calls: 3},
		{name: "boundary/comma normalized grant", scope: "repo,user:email", email: true, calls: 3},
		{name: "boundary/space separated grant", scope: "repo user:email", email: true, calls: 3},
		{name: "negative/scope substring never grants email", scope: "notuser:email", calls: 2},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Uint64
			f := browserProviderFixture{pages: 1, userID: 42, scope: &row.scope}
			if !row.email {
				f.emailStatus = 403
			}
			client := browserOAuthClient(t, f, &calls)
			got, err := CompleteBrowserAuthorization(t.Context(), browserOAuthRequest(t, client))
			id, idErr := got.UserID.Uint64()
			if err != nil || idErr != nil || id != 42 || got.EmailVerified != row.email || calls.Load() != row.calls {
				t.Fatalf("GitHub granted-scope observation = (%+v,%v,%v,%dcalls), want subject42/email%t/%dcalls", got, err, idErr, calls.Load(), row.email, row.calls)
			}
		})
	}
}
func TestBrowserOAuthRequestFormattingRedactsProofCustody(t *testing.T) {
	t.Parallel()
	var calls atomic.Uint64
	request := browserOAuthRequest(t, browserOAuthClient(t, browserProviderFixture{pages: 1, userID: 42}, &calls))
	for _, verb := range []string{"%v", "%+v", "%#v"} {
		for _, text := range []string{fmt.Sprintf(verb, request), fmt.Sprintf(verb, request.Code), fmt.Sprintf(verb, request.Verifier)} {
			if strings.Contains(text, request.Code.String()) || strings.Contains(text, request.Verifier.String()) || strings.Contains(text, "public-github-test-client-secret") {
				t.Fatalf("GitHub request formatting with %s retained private proof material, want redaction", verb)
			}
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("formatting provider calls = %d, want0", calls.Load())
	}
}
