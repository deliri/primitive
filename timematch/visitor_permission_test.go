package timematch

import (
	"net/url"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Public consent URL, not a second authorization implementation. The existing
// calendar exchange already owns state, PKCE, application binding and tokens.
func TestVisitorBookingPermissionRequestsProfileAndCalendarTogether(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		provider Provider
		tenant   MicrosoftTenant
		want     []string
	}{
		{name: "Google visitor can match and book with one permission", provider: ProviderGoogle, want: []string{googleScopeFreeBusy, googleScopeOwnedEvents, "openid", "email", "profile", "https://www.googleapis.com/auth/user.phonenumbers.read"}},
		{name: "Outlook personal visitor can match and book with one permission", provider: ProviderMicrosoft, tenant: MicrosoftTenantConsumers, want: []string{microsoftScopeReadWrite, "https://graph.microsoft.com/User.Read"}},
		{name: "Outlook work visitor can match and book with one permission", provider: ProviderMicrosoft, tenant: MicrosoftTenantOrganizations, want: []string{microsoftScopeReadWrite, "https://graph.microsoft.com/User.Read"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			state, e1 := core.ParseOAuthState("visitor-booking-state")
			challenge, e2 := core.ParseOAuthPKCEChallenge(strings.Repeat("c", 43))
			if e1 != nil || e2 != nil {
				t.Fatalf("proof fixture=%v/%v,want nil", e1, e2)
			}
			target, err := AuthorizationEndpoint(AuthorizationRequest{Application: application(t, tc.provider, tc.tenant), State: state, Challenge: challenge, Access: AccessManageEvents, Lifetime: LifetimeVisitor})
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := url.Parse(target.String())
			if err != nil {
				t.Fatal(err)
			}
			values := parsed.Query()
			scopes := " " + values.Get("scope") + " "
			for _, want := range tc.want {
				if !strings.Contains(scopes, " "+want+" ") {
					t.Errorf("visitor permission scopes=%q,want %q in the single redirect", scopes, want)
				}
			}
			if strings.Contains(scopes, "offline_access") || values.Get("access_type") == "offline" || values.Get("state") != state.String() || values.Get("code_challenge") != challenge.String() {
				t.Fatalf("visitor authorization=%v,want online-only owned state and PKCE", values)
			}
		})
	}
}
