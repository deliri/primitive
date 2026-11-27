package gmail

import (
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type consentTokenFixture struct {
	tokenFixture
	Offline       string `json:"refresh_token,omitempty"`
	RefreshExpiry uint64 `json:"refresh_token_expires_in,omitempty"`
}

func workspaceConsentFixture() consentTokenFixture {
	token := grantedTokenFixture()
	token.Scope = ReadOnlyScope + " " + SendScope
	return consentTokenFixture{tokenFixture: token, Offline: "synthetic-offline-grant"}
}

func TestGmailEnrollWorkspaceGrantThroughServer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		configure func(*consentTokenFixture)
		profile   string
		status    int
		fault     string
		want      error
	}{
		{name: "own mailbox offline", profile: fixtureMailbox},
		{name: "wrong mailbox", profile: "other@example.invalid", want: core.ErrGmailBinding},
		{name: "no offline token", configure: func(r *consentTokenFixture) { r.Offline = "" }, want: core.ErrGmailConsent},
		{name: "read-only grant", configure: func(r *consentTokenFixture) { r.Scope = ReadOnlyScope }, want: core.ErrGmailConsent},
		{name: "time limited", configure: func(r *consentTokenFixture) { r.RefreshExpiry = 3600 }, want: core.ErrGmailConsent},
		{name: "browser code reused", status: 400, fault: "invalid_grant", want: core.ErrGmailConsent},
		{name: "admin policy refusal", status: 403, fault: "admin_policy_enforced", want: core.ErrGmailPolicy},
		{name: "client setup refusal", status: 401, fault: "invalid_client", want: core.ErrGmailOAuthClient},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refresh := fixtureRefresh(t)
			code, err := core.ParseOAuthAuthorizationCode("once+synthetic&code")
			if err != nil {
				t.Fatal(err)
			}
			origin, err := core.ParseOAuthRedirectURI("https://admin.example.invalid")
			if err != nil {
				t.Fatal(err)
			}
			request := ConsentRequest{
				Mailbox: refresh.Mailbox, ClientID: refresh.ClientID, ClientSecret: refresh.ClientSecret,
				Code: code, Origin: origin, Policy: refresh.Policy,
			}
			fixture := workspaceConsentFixture()
			if tc.configure != nil {
				tc.configure(&fixture)
			}
			var posts, profiles atomic.Int32
			server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/token":
					posts.Add(1)
					if r.Method != http.MethodPost || r.Header.Get("X-Fixture-Provider") != "oauth2.googleapis.com" ||
						r.URL.RawQuery != "" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
						t.Error("wrong owner code exchange transport")
					}
					if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "authorization_code" ||
						r.PostForm.Get("code") != "once+synthetic&code" || r.PostForm.Get("redirect_uri") != "https://admin.example.invalid" ||
						r.PostForm.Get("client_id") != "fixture+opaque" || r.PostForm.Get("client_secret") != "fixture+secret=&" ||
						len(r.PostForm) != 5 {
						t.Error("wrong bounded OAuth form")
					}
					if tc.status != 0 {
						nativeFixtureWrite(t, w, tc.status, []byte(`{"error":"`+tc.fault+`","error_description":"private-marker"}`))
						return
					}
					nativeFixtureWrite(t, w, http.StatusOK, encodeFixture(t, fixture))
				case "/gmail/v1/users/me/profile":
					profiles.Add(1)
					if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer "+fixtureToken {
						t.Error("profile verification did not use the exchanged bearer")
					}
					nativeFixtureWrite(t, w, http.StatusOK, encodeFixture(t, mailboxWire{Email: mailAddressWire(tc.profile)}))
				default:
					t.Error("unexpected provider call")
					w.WriteHeader(http.StatusNotFound)
				}
			})
			got, err := server.EnrollWorkspaceGrant().Execute(t.Context(), request)
			if !errors.Is(err, tc.want) || err != nil && strings.Contains(err.Error(), "private-marker") {
				t.Fatalf("consent error = %v, want safe typed %v", err, tc.want)
			}
			if posts.Load() != 1 || tc.profile == "" && profiles.Load() != 0 || tc.profile != "" && profiles.Load() != 1 {
				t.Fatal("consent exchange did extra work or skipped mailbox binding")
			}
			if tc.want != nil {
				if got.Validate() == nil {
					t.Fatal("an unverified refresh token escaped")
				}
				return
			}
			defer func() {
				if err := got.Close(); err != nil {
					t.Error(err)
				}
			}()
			secret, err := got.CopyBytes()
			defer clear(secret)
			if err != nil || string(secret) != fixture.Offline {
				t.Fatal("the custody handoff lost the verified offline token")
			}
			if data, err := got.MarshalJSON(); err == nil || len(data) != 0 {
				t.Fatal("refresh credential escaped confidential custody")
			}
		})
	}
}
