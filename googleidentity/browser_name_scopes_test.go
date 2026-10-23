package googleidentity

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"strings"
	"testing"
)

func TestBrowserOAuthProfileScopesAreExplicit(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		request BrowserProfileRequest
	}{
		{name: "basic names do not request private fields"},
		{name: "phone explicitly selected", request: BrowserProfileRequest{PhoneNumber: true}},
		{name: "birthday explicitly selected", request: BrowserProfileRequest{Birthday: true}},
		{name: "both private fields explicitly selected", request: BrowserProfileRequest{PhoneNumber: true, Birthday: true}},
		{name: "basic People name selected", request: BrowserProfileRequest{Name: true}},
		{name: "basic name and phone selected", request: BrowserProfileRequest{Name: true, PhoneNumber: true}},
		{name: "basic name and birthday selected", request: BrowserProfileRequest{Name: true, Birthday: true}},
		{name: "all profile fields selected", request: BrowserProfileRequest{Name: true, PhoneNumber: true, Birthday: true}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			id, e1 := core.ParseOAuthClientID("profile-scope-client")
			callback, e2 := core.ParseOAuthRedirectURI("https://api.app.example/v1/auth/oauth/google/callback")
			state, e3 := core.ParseOAuthState("owned-state")
			nonce, e4 := core.ParseOAuthNonce("owned-nonce")
			verifier, e5 := core.ParseOAuthPKCEVerifier(strings.Repeat("v", 43))
			challenge, e6 := verifier.Challenge()
			if err := errors.Join(e1, e2, e3, e4, e5, e6); err != nil {
				t.Fatal(err)
			}
			got, err := BeginBrowserAuthorization(BrowserAuthorizationRequest{Profile: row.request, ClientID: id, RedirectURI: callback, State: state, Nonce: nonce, Challenge: challenge})
			if err != nil {
				t.Fatalf("profile authorization = %v, want nil", err)
			}
			address := got.URL.HTTPURL()
			scopes := strings.Fields(address.Query().Get("scope"))
			if len(scopes) < 3 || scopes[0] != "openid" || scopes[1] != "email" || scopes[2] != "profile" {
				t.Fatalf("basic Google scopes = %q, want exact openid/email/profile", scopes)
			}
			count := 3
			if row.request.PhoneNumber {
				count++
			}
			if row.request.Birthday {
				count++
			}
			if len(scopes) != count {
				t.Fatalf("scope count = %d, want exactly%d", len(scopes), count)
			}
			joined := " " + strings.Join(scopes, " ") + " "
			if strings.Contains(joined, " "+BrowserPhoneScope+" ") != row.request.PhoneNumber || strings.Contains(joined, " "+BrowserBirthdayScope+" ") != row.request.Birthday {
				t.Fatalf("private Google scopes = %q, want phone%t/birthday%t", scopes, row.request.PhoneNumber, row.request.Birthday)
			}
		})
	}
}
