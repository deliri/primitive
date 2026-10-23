package github

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"golang.org/x/oauth2"
)

type browserTokenFixture struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}
type browserUserFixture struct {
	ID uint64 `json:"id"`
}
type browserEmailFixture struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}
type browserProviderFixture struct {
	scope         *string
	pages         uint64
	userID        uint64
	link          string
	tokenStatus   int
	userStatus    int
	emailStatus   int
	unverified    bool
	empty         bool
	nonprimary    bool
	conflicting   bool
	duplicate     bool
	truncated     bool
	malformedUser bool
}

func browserOAuthRequest(t testing.TB, client exchange.Client) BrowserAuthorizationCompleteRequest {
	t.Helper()
	id, e1 := core.ParseOAuthClientID("github-browser-client")
	secret, e2 := core.NewOAuthClientSecret([]byte("public-github-test-client-secret"))
	callback, e3 := core.ParseOAuthRedirectURI("https://api.app.example/v1/auth/oauth/github/callback")
	code, e4 := core.ParseOAuthAuthorizationCode("public-github-test-code")
	verifier, e5 := core.ParseOAuthPKCEVerifier(strings.Repeat("g", 43))
	agent, e6 := ParseUserAgent("typed-oauth-fixture")
	policy, e7 := boundedPolicy()
	if err := errors.Join(e1, e2, e3, e4, e5, e6, e7); err != nil {
		t.Fatalf("GitHub browser fixture contract = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := secret.Close(); err != nil {
			t.Errorf("GitHub client secret Close = %v, want nil", err)
		}
	})
	return BrowserAuthorizationCompleteRequest{Client: client, ClientID: id, ClientSecret: secret, RedirectURI: callback, Code: code, Verifier: verifier, UserAgent: agent, Policy: policy.Operation}
}
func browserOAuthClient(t testing.TB, f browserProviderFixture, calls *atomic.Uint64) exchange.Client {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		var body []byte
		var err error
		switch r.URL.Path {
		case "/login/oauth/access_token":
			if err := r.ParseForm(); err != nil {
				t.Errorf("GitHub token ParseForm = %v, want nil", err)
			}
			if r.Host != "github.com" || r.Method != "POST" || r.PostForm.Get("code") != "public-github-test-code" || r.PostForm.Get("client_id") != "github-browser-client" || r.PostForm.Get("client_secret") != "public-github-test-client-secret" || r.PostForm.Get("redirect_uri") != "https://api.app.example/v1/auth/oauth/github/callback" || r.PostForm.Get("code_verifier") != strings.Repeat("g", 43) || r.PostForm.Get("grant_type") != "authorization_code" {
				t.Errorf("GitHub OAuth exchange = %s/%s, want exact code/client/callback/PKCE", r.Method, r.URL.Path)
			}
			if f.tokenStatus != 0 {
				w.WriteHeader(f.tokenStatus)
			}
			scope := "user:email"
			if f.scope != nil {
				scope = *f.scope
			}
			body, err = core.MarshalCanonicalJSONDocument(browserTokenFixture{AccessToken: "public-user-bearer", TokenType: "bearer", Scope: scope})
		case "/user":
			if r.Host != "api.github.com" || r.Method != "GET" || r.Header.Get("Authorization") != "Bearer public-user-bearer" || r.Header.Get(headerUserAgent) != "typed-oauth-fixture" || r.Header.Get(headerAPIVersion) != core.GitHubAPIVersion {
				t.Errorf("GitHub user binding = %s/%s, want exact authenticated REST request", r.Method, r.URL.Path)
			}
			if f.userStatus != 0 {
				w.WriteHeader(f.userStatus)
			}
			if f.malformedUser {
				body = []byte(`{"id":"not-a-native-numeric-id"}`)
			} else {
				body, err = core.MarshalCanonicalJSONDocument(browserUserFixture{ID: f.userID})
			}
		case githubBrowserEmailPath:
			if r.Host != "api.github.com" || r.Header.Get("Authorization") != "Bearer public-user-bearer" || r.URL.Query().Get("per_page") != "100" {
				t.Errorf("GitHub email binding = %s/%s, want exact authenticated page", r.Method, r.URL.Path)
			}
			page, parseErr := strconv.ParseUint(r.URL.Query().Get("page"), 10, 64)
			if parseErr != nil || page == 0 {
				t.Errorf("GitHub email page = %s/%v, want positive native ordinal", r.URL.Query().Get("page"), parseErr)
			}
			if f.emailStatus != 0 {
				w.WriteHeader(f.emailStatus)
			}
			if f.link != "" {
				w.Header().Set(headerLink, f.link)
			} else if page < f.pages {
				w.Header().Set(headerLink, "<https://api.github.com/user/emails?per_page=100&page="+strconv.FormatUint(page+1, 10)+`>; rel="next"`)
			}
			rows := []browserEmailFixture{}
			if !f.empty {
				primary := !f.nonprimary && page == f.pages
				if f.duplicate || f.conflicting {
					primary = true
				}
				email := "athlete@example.test"
				if f.conflicting && page > 1 {
					email = "different@example.test"
				}
				rows = append(rows, browserEmailFixture{Email: email, Primary: primary, Verified: !f.unverified})
				if f.conflicting && f.pages == 1 {
					rows = append(rows, browserEmailFixture{Email: "different@example.test", Primary: true, Verified: true})
				}
			}
			body, err = core.MarshalCanonicalJSONDocument(rows)
			if f.truncated && len(body) > 0 {
				body = body[:len(body)-1]
			}
		default:
			t.Errorf("GitHub browser request path = %q, want only token/user/emails", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err != nil {
			t.Errorf("GitHub browser fixture encode = %v, want nil", err)
			return
		}
		if _, err := w.Write(body); err != nil {
			t.Errorf("GitHub browser fixture write = %v, want nil", err)
		}
	}))
	t.Cleanup(server.Close)
	transport, ok := server.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("GitHub local transport = %T, want native transport", server.Client().Transport)
	}
	owned := transport.Clone()
	owned.Proxy = nil
	owned.TLSClientConfig.ServerName = server.Certificate().DNSNames[0]
	owned.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}
	t.Cleanup(owned.CloseIdleConnections)
	client, err := exchange.NewClient(&http.Client{Transport: owned})
	if err != nil {
		t.Fatalf("GitHub browser Exchange client = %v, want nil", err)
	}
	return client
}
func TestBrowserOAuthNativeProviderAndPageBoundaries(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name         string
		fixture      browserProviderFixture
		wantErr      error
		cancel       bool
		wantEmail    bool
		wantVerified bool
		wantCalls    uint64
	}{
		{name: "positive/stable subject with verified primary", wantEmail: true, wantVerified: true, wantCalls: 3},
		{name: "neutral/no emails retains authenticated subject", fixture: browserProviderFixture{empty: true}, wantCalls: 3},
		{name: "neutral/no primary retains authenticated subject", fixture: browserProviderFixture{nonprimary: true}, wantCalls: 3},
		{name: "neutral/unverified primary is only an observation", fixture: browserProviderFixture{unverified: true}, wantEmail: true, wantCalls: 3},
		{name: "positive/primary arrives on second page", fixture: browserProviderFixture{pages: 2}, wantEmail: true, wantVerified: true, wantCalls: 4},
		{name: "positive/identical primary replay is idempotent", fixture: browserProviderFixture{pages: 2, duplicate: true}, wantEmail: true, wantVerified: true, wantCalls: 4},
		{name: "negative/conflicting primary within page", fixture: browserProviderFixture{conflicting: true}, wantErr: core.ErrGitHubResponse},
		{name: "negative/conflicting primary across pages", fixture: browserProviderFixture{pages: 2, conflicting: true}, wantErr: core.ErrGitHubResponse},
		{name: "negative/foreign continuation origin", fixture: browserProviderFixture{link: `<https://foreign.example/user/emails?per_page=100&page=2>; rel="next"`}, wantErr: core.ErrGitHubBinding},
		{name: "negative/foreign continuation path", fixture: browserProviderFixture{link: `<https://api.github.com/repos/private?per_page=100&page=2>; rel="next"`}, wantErr: core.ErrGitHubBinding},
		{name: "negative/replayed page cannot cycle", fixture: browserProviderFixture{link: `<https://api.github.com/user/emails?per_page=100&page=1>; rel="next"`}, wantErr: core.ErrGitHubBinding},
		{name: "negative/skipped page cannot hide earlier primary", fixture: browserProviderFixture{link: `<https://api.github.com/user/emails?per_page=100&page=3>; rel="next"`}, wantErr: core.ErrGitHubBinding},
		{name: "negative/duplicate page query", fixture: browserProviderFixture{link: `<https://api.github.com/user/emails?per_page=100&page=2&page=2>; rel="next"`}, wantErr: core.ErrGitHubBinding},
		{name: "negative/credentialed continuation", fixture: browserProviderFixture{link: `<https://untrusted@api.github.com/user/emails?per_page=100&page=2>; rel="next"`}, wantErr: core.ErrGitHubBinding},
		{name: "negative/fragment continuation", fixture: browserProviderFixture{link: `<https://api.github.com/user/emails?per_page=100&page=2#tail>; rel="next"`}, wantErr: core.ErrGitHubBinding},
		{name: "negative/duplicate next links", fixture: browserProviderFixture{link: `<https://api.github.com/user/emails?per_page=100&page=2>; rel="next", <https://api.github.com/user/emails?per_page=100&page=2>; rel="next"`}, wantErr: core.ErrGitHubResponse},
		{name: "negative/string user ID cannot enter numeric principal", fixture: browserProviderFixture{malformedUser: true}, wantErr: core.ErrGitHubResponse},
		{name: "negative/provider refuses code", fixture: browserProviderFixture{tokenStatus: 401}, wantErr: core.ErrGitHubAuthentication},
		{name: "negative/provider refuses user capability", fixture: browserProviderFixture{userStatus: 401}, wantErr: core.ErrGitHubResponse},
		{name: "negative/provider email outage yields no partial identity", fixture: browserProviderFixture{emailStatus: 503}, wantErr: core.ErrGitHubResponse},
		{name: "negative/truncated email prefix yields no partial identity", fixture: browserProviderFixture{truncated: true}, wantErr: core.ErrGitHubResponse},
		{name: "neutral/canceled caller performs no exchange", cancel: true, wantErr: context.Canceled},
		{name: "boundary/129 forward pages retain one primary", fixture: browserProviderFixture{pages: 129}, wantEmail: true, wantVerified: true, wantCalls: 131},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			f := row.fixture
			if f.pages == 0 {
				f.pages = 1
			}
			if f.userID == 0 {
				f.userID = 42
			}
			var calls atomic.Uint64
			client := browserOAuthClient(t, f, &calls)
			request := browserOAuthRequest(t, client)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if row.cancel {
				cancel()
			}
			got, err := CompleteBrowserAuthorization(ctx, request)
			if !errors.Is(err, row.wantErr) {
				t.Fatalf("GitHub browser identity = (%+v,%v), want error %v", got, err, row.wantErr)
			}
			if row.wantErr != nil {
				if got != (BrowserVerifiedIdentity{}) {
					t.Fatalf("refused GitHub identity = %+v, want zero", got)
				}
				if f.link != "" && calls.Load() != 3 {
					t.Fatalf("refused continuation requests = %d, want token/user/first-email only", calls.Load())
				}
				if row.cancel && calls.Load() != 0 {
					t.Fatalf("canceled GitHub requests = %d, want0", calls.Load())
				}
				return
			}
			id, err := got.UserID.Uint64()
			if err != nil || id != 42 || got.EmailVerified != row.wantVerified || (got.Email != (core.OAuthEmail{})) != row.wantEmail || calls.Load() != row.wantCalls {
				t.Fatalf("GitHub identity = %+v/%v calls=%d, want stable42/verified%t/email%t/calls%d", got, err, calls.Load(), row.wantVerified, row.wantEmail, row.wantCalls)
			}
		})
	}
}

func TestBrowserOAuthTokenRefusalCannotReplayClientSecrets(t *testing.T) {
	t.Parallel()
	var calls atomic.Uint64
	client := browserOAuthClient(t, browserProviderFixture{pages: 1, userID: 42, tokenStatus: 401}, &calls)
	got, err := CompleteBrowserAuthorization(t.Context(), browserOAuthRequest(t, client))
	var sdk *oauth2.RetrieveError
	if !errors.As(err, &sdk) || !errors.Is(err, core.ErrGitHubAuthentication) || got != (BrowserVerifiedIdentity{}) {
		t.Fatalf("GitHub OAuth SDK refusal = (%+v,%v), want zero and retained native OAuth2 cause", got, err)
	}
	if sdk.Response == nil || sdk.Response.StatusCode != 401 || sdk.Response.Request != nil || len(sdk.Body) != 0 || sdk.ErrorDescription != "" || sdk.ErrorURI != "" {
		t.Fatalf("GitHub SDK refusal retains sensitive request/body: request=%t bodybytes=%d, want only safe status/class", sdk.Response != nil && sdk.Response.Request != nil, len(sdk.Body))
	}
}
