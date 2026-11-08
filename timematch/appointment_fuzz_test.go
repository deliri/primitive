package timematch

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/googleidentity"
)

// This is a token-admission unit oracle. The TLS native tests separately prove
// code redemption, exact provider paths and contact reads.
func FuzzAppointmentTokenAdmission(f *testing.F) {
	fuzzAppointmentTokenAdmission(f, ProviderGoogle)
}

func FuzzMicrosoftAppointmentTokenAdmission(f *testing.F) {
	fuzzAppointmentTokenAdmission(f, ProviderMicrosoft)
}

func fuzzAppointmentTokenAdmission(f *testing.F, provider Provider) {
	f.Helper()
	wantScope, err := scopes(provider, AccessManageEvents, LifetimeAppointment)
	if err != nil {
		f.Fatal(err)
	}
	seed := seedToken{AccessToken: "appointment-access", RefreshToken: "appointment-refresh", ExpiresIn: 3600, TokenType: bearerTokenType, Scope: strings.Join(wantScope, " ")}
	f.Add(seedJSON(f, seed))
	withoutRefresh := seed
	withoutRefresh.RefreshToken = ""
	f.Add(seedJSON(f, withoutRefresh))
	withoutProfile := seed
	profileScope := googleScopeProfile
	if provider == ProviderMicrosoft {
		profileScope = microsoftScopeUserRead
	}
	withoutProfile.Scope = strings.Join(slices.DeleteFunc(slices.Clone(wantScope), func(value string) bool { return value == profileScope }), " ")
	f.Add(seedJSON(f, withoutProfile))
	addHostile(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		token, err := decodeJSON[tokenWire](data)
		if err != nil {
			if !errors.Is(err, core.ErrTimeMatchResponse) || token.access != nil || token.refresh != nil {
				t.Fatalf("appointment token decode error/authority = %v/%t/%t, want typed refusal and zero", err, token.access != nil, token.refresh != nil)
			}
			return
		}
		defer token.clear()
		grant, err := admitExchange(token, CodeExchange{Application: Application{Provider: provider}, Access: AccessManageEvents, Lifetime: LifetimeAppointment, ObservedAt: at(t, "2026-11-02T10:00:00Z")})
		defer func() {
			if err := grant.Close(); err != nil {
				t.Errorf("appointment fuzz grant close error = %v, want nil", err)
			}
		}()
		if err != nil {
			if !errors.Is(err, core.ErrTimeMatchResponse) && !errors.Is(err, core.ErrTimeMatchConsent) && !errors.Is(err, core.ErrTimeMatchPhoneDenied) {
				t.Fatalf("appointment admission error = %v, want typed provider/consent refusal", err)
			}
			if grant.AccessToken.IsSet() || grant.RefreshToken.IsSet() {
				t.Fatalf("refused appointment access/refresh = %t/%t, want zero authority", grant.AccessToken.IsSet(), grant.RefreshToken.IsSet())
			}
			return
		}
		if grant.Validate() != nil || grant.Lifetime != LifetimeAppointment || grant.Access != AccessManageEvents || !grant.RefreshToken.IsSet() || string(grant.RefreshToken.value) != string(token.refresh) || string(grant.AccessToken.value) != string(token.access) {
			t.Fatalf("accepted appointment lifetime/access/refresh = %v/%v/%t, want exact retained appointment material", grant.Lifetime, grant.Access, grant.RefreshToken.IsSet())
		}
		if token.scopePresent {
			got := strings.Fields(token.scope)
			if provider == ProviderMicrosoft {
				for _, required := range []string{microsoftScopeReadWrite, microsoftScopeUserRead, microsoftScopeOffline} {
					if !slices.Contains(got, required) {
						t.Fatalf("accepted Microsoft appointment scope = %q, want independently required %q", token.scope, required)
					}
				}
				return
			}
			for _, required := range []string{googleScopeFreeBusy, googleScopeOwnedEvents, googleScopeOpenID, googleidentity.BrowserPhoneScope} {
				if !slices.Contains(got, required) {
					t.Fatalf("accepted appointment scope = %q, want independently required %q", token.scope, required)
				}
			}
			if !slices.Contains(got, googleScopeEmail) && !slices.Contains(got, "email") || !slices.Contains(got, googleScopeProfile) && !slices.Contains(got, "profile") {
				t.Fatalf("accepted appointment profile scope = %q, want email and profile authority", token.scope)
			}
		}
	})
}
