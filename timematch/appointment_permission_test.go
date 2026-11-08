package timematch

import (
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/googleidentity"
)

// One appointment needs its original visitor identity and refresh capability,
// without adding the business calendar-list permission. Actual token exchange
// and expiry remain separate native boundaries.
func TestAppointmentPermissionKeepsVisitorScopesAndRefresh(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		provider Provider
		tenant   MicrosoftTenant
		want     []string
	}{
		{name: "Google appointment preserves visitor profile without calendar discovery", provider: ProviderGoogle, want: []string{googleScopeFreeBusy, googleScopeOwnedEvents, googleScopeOpenID, googleScopeEmail, googleScopeProfile, googleidentity.BrowserPhoneScope}},
		{name: "Microsoft common accepts personal and corporate appointment grants", provider: ProviderMicrosoft, tenant: MicrosoftTenantCommon, want: []string{microsoftScopeReadWrite, microsoftScopeUserRead, microsoftScopeOffline}},
		{name: "Microsoft personal appointment retains bounded refresh capability", provider: ProviderMicrosoft, tenant: MicrosoftTenantConsumers, want: []string{microsoftScopeReadWrite, microsoftScopeUserRead, microsoftScopeOffline}},
		{name: "Microsoft corporate appointment retains bounded refresh capability", provider: ProviderMicrosoft, tenant: MicrosoftTenantOrganizations, want: []string{microsoftScopeReadWrite, microsoftScopeUserRead, microsoftScopeOffline}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			state, err := core.ParseOAuthState("appointment-booking-state")
			if err != nil {
				t.Fatal(err)
			}
			verifier, err := core.ParseOAuthPKCEVerifier(strings.Repeat("v", 43))
			if err != nil {
				t.Fatal(err)
			}
			challenge, err := verifier.Challenge()
			if err != nil {
				t.Fatal(err)
			}
			target, err := AuthorizationEndpoint(AuthorizationRequest{Application: application(t, tc.provider, tc.tenant), State: state, Challenge: challenge, Access: AccessManageEvents, Lifetime: LifetimeAppointment})
			if err != nil {
				t.Fatalf("appointment authorization error = %v, want nil", err)
			}
			parsed, err := url.Parse(target.String())
			if err != nil {
				t.Fatal(err)
			}
			values := parsed.Query()
			got := strings.Fields(values.Get("scope"))
			if !slices.Equal(got, tc.want) || values.Get("state") != state.String() || values.Get("code_challenge") != challenge.String() || values.Get("code_challenge_method") != "S256" {
				t.Fatalf("appointment scopes/proof = %q/%q/%q/%q, want %q and original state/PKCE", got, values.Get("state"), values.Get("code_challenge"), values.Get("code_challenge_method"), tc.want)
			}
			if tc.provider == ProviderGoogle && (values.Get("access_type") != "offline" || values.Get("prompt") != "consent") {
				t.Fatalf("Google access/prompt = %q/%q, want offline/consent", values.Get("access_type"), values.Get("prompt"))
			}
		})
	}
}
