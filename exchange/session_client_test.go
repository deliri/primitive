package exchange

import (
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	sessionCookieName       = "session"
	sessionCookieValue      = "opaque-proof"
	sessionOtherCookieValue = "replacement-proof"
	sessionFixtureSuffix    = "private.invalid"
	sessionFixtureParent    = "tenant." + sessionFixtureSuffix
	sessionFixtureHost      = "api." + sessionFixtureParent
	sessionFixtureChild     = "child." + sessionFixtureHost
	sessionFixtureSibling   = "other." + sessionFixtureParent
	sessionFixtureForeign   = "outsider." + sessionFixtureSuffix
	sessionCookiePath       = "/scope"
	sessionEstablishPath    = "/establish"
	sessionObservePath      = "/observe"
)

// This is a fixture authority for reserved .invalid domains, not a public
// suffix database. It deliberately makes a multi-label suffix authoritative,
// so substituting a nil list changes an observable cookie-isolation result.
type sessionFixtureSuffixes []string

func (s sessionFixtureSuffixes) PublicSuffix(domain string) string {
	for _, suffix := range s {
		if domain == suffix || strings.HasSuffix(domain, "."+suffix) {
			return suffix
		}
	}
	return domain
}
func (sessionFixtureSuffixes) String() string { return "reserved session-test domains" }

type sessionUncalledSuffixList struct{}

func (*sessionUncalledSuffixList) PublicSuffix(string) string { panic(core.ErrExchangeContract) }
func (*sessionUncalledSuffixList) String() string             { panic(core.ErrExchangeContract) }

// This constructor's admission rule has three equivalence classes: absent
// interface, absent concrete capability, and present capability. Slice cases
// additionally distinguish nil from a present empty implementation. Domain
// semantics are delegated to Go and exercised separately below; these rows do
// not claim to exhaust the caller's arbitrary PublicSuffix implementation.
func TestSessionClientAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		list    cookiejar.PublicSuffixList
		wantErr error
	}{
		{name: "negative absent authority cannot create an insecure jar", wantErr: core.ErrExchangeContract},
		{name: "negative typed nil pointer cannot escape admission", list: (*sessionUncalledSuffixList)(nil), wantErr: core.ErrExchangeContract},
		{name: "negative typed nil slice cannot escape admission", list: sessionFixtureSuffixes(nil), wantErr: core.ErrExchangeContract},
		{name: "positive present empty slice is not confused with nil", list: sessionFixtureSuffixes{}},
		{name: "neutral constructor does not invoke domain policy or diagnostics", list: &sessionUncalledSuffixList{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := SessionClientRequest{PublicSuffixList: tc.list}
			gotValidationErr := request.Validate()
			if !errors.Is(gotValidationErr, tc.wantErr) {
				t.Fatalf("Validate() = %v, want %v", gotValidationErr, tc.wantErr)
			}
			got, gotErr := NewSessionClient(request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("NewSessionClient() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (Client{}) {
					t.Fatalf("refused client = %v, want exact zero", got)
				}
				return
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("client.Validate() = %v, want nil", err)
			}
			if got.http.Transport != nil || got.http.Timeout != 0 || got.http.CheckRedirect != nil {
				t.Fatalf("client transport/timeout/redirect = (%T, %v, present %t), want Go defaults", got.http.Transport, got.http.Timeout, got.http.CheckRedirect != nil)
			}
			firstJar, ok := got.http.Jar.(*cookiejar.Jar)
			if !ok || firstJar == nil {
				t.Fatalf("client jar = %T, want nonnil *cookiejar.Jar", got.http.Jar)
			}
			second, err := NewSessionClient(request)
			if err != nil || second.http == nil {
				t.Fatalf("second client = (%v, %v), want valid independent client", second, err)
			}
			if second.http == got.http || second.http.Jar == firstJar {
				t.Fatalf("client/jar shared=%t/%t, want false/false", second.http == got.http, second.http.Jar == firstJar)
			}
		})
	}
}

// A delegation ratchet: the wrapper must hand the exact authority to Go's
// concrete jar. Explicit cookie facts independently pin the consequences of
// losing that authority, replacing the jar, or sharing state. This is not a
// replacement implementation or an exhaustive test of Go's cookie parser.
func TestSessionCookieScopeLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		sourceHost   string
		targetHost   string
		sourcePath   string
		targetPath   string
		targetScheme string
		cookie       http.Cookie
		setCookie    bool
		wantCookies  int
	}{
		{name: "positive host cookie returns to exact host", sourceHost: sessionFixtureHost, targetHost: sessionFixtureHost, setCookie: true, wantCookies: 1},
		{name: "negative host cookie cannot reach child", sourceHost: sessionFixtureHost, targetHost: sessionFixtureChild, setCookie: true},
		{name: "negative host cookie cannot reach sibling", sourceHost: sessionFixtureHost, targetHost: sessionFixtureSibling, setCookie: true},
		{name: "positive parent domain cookie reaches sibling", sourceHost: sessionFixtureHost, targetHost: sessionFixtureSibling, cookie: http.Cookie{Domain: sessionFixtureParent}, setCookie: true, wantCookies: 1},
		{name: "positive dotted parent domain uses Go canonicalization", sourceHost: sessionFixtureHost, targetHost: sessionFixtureSibling, cookie: http.Cookie{Domain: "." + sessionFixtureParent}, setCookie: true, wantCookies: 1},
		{name: "negative public suffix cannot poison another tenant", sourceHost: sessionFixtureHost, targetHost: sessionFixtureForeign, cookie: http.Cookie{Domain: sessionFixtureSuffix}, setCookie: true},
		{name: "negative suffix cookie is refused even for issuing subdomain", sourceHost: sessionFixtureHost, targetHost: sessionFixtureHost, cookie: http.Cookie{Domain: sessionFixtureSuffix}, setCookie: true},
		{name: "negative unrelated domain cannot be planted", sourceHost: sessionFixtureHost, targetHost: sessionFixtureForeign, cookie: http.Cookie{Domain: sessionFixtureForeign}, setCookie: true},
		{name: "boundary suffix host may set its own host cookie", sourceHost: sessionFixtureSuffix, targetHost: sessionFixtureSuffix, cookie: http.Cookie{Domain: sessionFixtureSuffix}, setCookie: true, wantCookies: 1},
		{name: "boundary suffix host exception cannot escape to child", sourceHost: sessionFixtureSuffix, targetHost: sessionFixtureHost, cookie: http.Cookie{Domain: sessionFixtureSuffix}, setCookie: true},
		{name: "boundary explicit path matches exactly", cookie: http.Cookie{Path: sessionCookiePath}, targetPath: sessionCookiePath, setCookie: true, wantCookies: 1},
		{name: "boundary explicit path matches slash child", cookie: http.Cookie{Path: sessionCookiePath}, targetPath: sessionCookiePath + "/child", setCookie: true, wantCookies: 1},
		{name: "boundary path prefix without slash cannot leak", cookie: http.Cookie{Path: sessionCookiePath}, targetPath: sessionCookiePath + "extra", setCookie: true},
		{name: "boundary parent path cannot receive child cookie", cookie: http.Cookie{Path: sessionCookiePath}, targetPath: "/", setCookie: true},
		{name: "boundary absent path derives directory in Go", sourcePath: sessionCookiePath + "/establish", targetPath: sessionCookiePath + "/observe", setCookie: true, wantCookies: 1},
		{name: "negative derived directory cannot leak to root", sourcePath: sessionCookiePath + "/establish", targetPath: "/", setCookie: true},
		{name: "positive secure cookie crosses HTTPS", cookie: http.Cookie{Secure: true}, targetScheme: "https", setCookie: true, wantCookies: 1},
		{name: "negative secure cookie cannot cross plain remote HTTP", cookie: http.Cookie{Secure: true}, targetScheme: "http", setCookie: true},
		{name: "negative deletion cannot become session state", cookie: http.Cookie{MaxAge: -1}, setCookie: true},
		{name: "neutral absent cookie leaves jar empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, err := NewSessionClient(SessionClientRequest{PublicSuffixList: sessionFixtureSuffixes{sessionFixtureSuffix}})
			if err != nil {
				t.Fatalf("NewSessionClient() = %v, want nil", err)
			}
			source := &url.URL{Scheme: "https", Host: sessionFixtureHost, Path: "/"}
			target := *source
			if tc.sourceHost != "" {
				source.Host = tc.sourceHost
			}
			if tc.targetHost != "" {
				target.Host = tc.targetHost
			}
			if tc.sourcePath != "" {
				source.Path = tc.sourcePath
			}
			if tc.targetPath != "" {
				target.Path = tc.targetPath
			}
			if tc.targetScheme != "" {
				target.Scheme = tc.targetScheme
			}
			cookie := tc.cookie
			cookie.Name, cookie.Value = sessionCookieName, sessionCookieValue
			if tc.setCookie {
				client.http.Jar.SetCookies(source, []*http.Cookie{&cookie})
			}
			got := client.http.Jar.Cookies(&target)
			if len(got) != tc.wantCookies {
				t.Fatalf("cookies = %v, want count %d", got, tc.wantCookies)
			}
			for _, cookie := range got {
				if cookie.Name != sessionCookieName || cookie.Value != sessionCookieValue {
					t.Fatalf("cookie = (%q, %q), want (%q, %q)", cookie.Name, cookie.Value, sessionCookieName, sessionCookieValue)
				}
			}
			isolated, err := NewSessionClient(SessionClientRequest{PublicSuffixList: sessionFixtureSuffixes{sessionFixtureSuffix}})
			if err != nil {
				t.Fatalf("isolated constructor = %v, want nil", err)
			}
			if got := isolated.http.Jar.Cookies(&target); len(got) != 0 {
				t.Fatalf("independent jar cookies = %v, want empty", got)
			}
		})
	}
}

func TestSessionHTTPCookieCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		cookies   []http.Cookie
		wantCount int
		wantValue string
	}{
		{name: "positive real response cookie returns with exact value", cookies: []http.Cookie{{Name: sessionCookieName, Value: sessionCookieValue, Path: "/"}}, wantCount: 1, wantValue: sessionCookieValue},
		{name: "negative path excluded cookie never appears on wire", cookies: []http.Cookie{{Name: sessionCookieName, Value: sessionCookieValue, Path: sessionCookiePath}}},
		{name: "boundary later deletion removes earlier response cookie", cookies: []http.Cookie{{Name: sessionCookieName, Value: sessionCookieValue, Path: "/"}, {Name: sessionCookieName, Path: "/", MaxAge: -1}}},
		{name: "boundary repeated identity replaces value without duplication", cookies: []http.Cookie{{Name: sessionCookieName, Value: sessionCookieValue, Path: "/"}, {Name: sessionCookieName, Value: sessionOtherCookieValue, Path: "/"}}, wantCount: 1, wantValue: sessionOtherCookieValue},
		{name: "neutral response without cookies creates no cookie header"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			observations := make(chan []*http.Cookie, 3)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				observations <- r.Cookies()
				if r.URL.Path == sessionEstablishPath {
					for _, cookie := range tc.cookies {
						http.SetCookie(w, &cookie)
					}
				}
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(server.Close)
			client, err := NewSessionClient(SessionClientRequest{PublicSuffixList: sessionFixtureSuffixes{sessionFixtureSuffix}})
			if err != nil {
				t.Fatalf("NewSessionClient() = %v, want nil", err)
			}
			isolated, err := NewSessionClient(SessionClientRequest{PublicSuffixList: sessionFixtureSuffixes{sessionFixtureSuffix}})
			if err != nil {
				t.Fatalf("isolated constructor = %v, want nil", err)
			}
			timeout, err := temporal.DurationFromMilliseconds(30000)
			if err != nil {
				t.Fatalf("timeout = %v, want nil", err)
			}
			calls := []struct {
				client    Client
				path      string
				wantCount int
				wantValue string
			}{
				{client: client, path: sessionEstablishPath},
				{client: client, path: sessionObservePath, wantCount: tc.wantCount, wantValue: tc.wantValue},
				{client: isolated, path: sessionObservePath},
			}
			for _, step := range calls {
				target, err := core.ParseHTTPEndpoint(server.URL + step.path)
				if err != nil {
					t.Fatalf("endpoint = %v, want nil", err)
				}
				got, err := SendNoBodyBounded(NoBodyBoundedCall{
					Context: t.Context(), Client: step.client,
					Request: NoBodyBoundedRequest{Target: target, Semantics: RequestSemantics{Method: MethodGet, Replay: ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK()},
					Policy:  NoBodyBoundedPolicy{Operation: OperationPolicy{OperationTimeout: timeout, AttemptTimeout: timeout, Retry: RetryPolicy{MaximumAttempts: 1}, Redirect: RedirectPolicy{Mode: RedirectReject}}},
				})
				if err != nil {
					t.Fatalf("SendNoBodyBounded() = %v, want nil", err)
				}
				if err := got.Validate(); err != nil {
					t.Fatalf("response.Validate() = %v, want nil", err)
				}
				if len(got.Body) != 0 || got.Metadata.Bytes.Uint64() != 0 || got.Metadata.Attempts != 1 || got.Metadata.Status != core.HTTPStatusOK() {
					t.Fatalf("response = %+v, want empty HTTP OK in one attempt", got)
				}
				select {
				case cookies := <-observations:
					if len(cookies) != step.wantCount {
						t.Fatalf("wire cookies = %v, want count %d", cookies, step.wantCount)
					}
					for _, cookie := range cookies {
						if cookie.Name != sessionCookieName || cookie.Value != step.wantValue {
							t.Fatalf("wire cookie = %+v, want (%q, %q)", cookie, sessionCookieName, step.wantValue)
						}
					}
				default:
					t.Fatalf("completed HTTP response has no request observation; owned completion channel=%p", observations)
				}
			}
			select {
			case cookies := <-observations:
				t.Fatalf("unexpected extra HTTP request: %v", cookies)
			default:
			}
		})
	}
}
