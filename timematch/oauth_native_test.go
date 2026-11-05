package timematch

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/googleidentity"
)

const (
	googleVisitorScope              = googleScopeFreeBusy
	googleConfirmScope              = googleScopeFreeBusy + " " + googleScopeOwnedEvents + " openid https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile " + googleidentity.BrowserPhoneScope
	googleBusinessAvailabilityScope = googleScopeFreeBusy + " " + googleScopeCalendarList
	googleBusinessScope             = googleBusinessAvailabilityScope + " " + googleScopeOwnedEvents
)

func tokenBody(access, refresh, scope, tokenType string, expires string) string {
	body := `{"access_token":"` + access + `","expires_in":` + expires + `,"token_type":"` + tokenType + `","scope":"` + scope + `"`
	if refresh != "" {
		body += `,"refresh_token":"` + refresh + `"`
	}
	return body + `,"id_token_absent":true}`
}

func codeExchange(t *testing.T, app Application, access Access, lifetime Lifetime) CodeExchange {
	t.Helper()
	code, codeErr := core.ParseOAuthAuthorizationCode("4/code-fixture")
	verifier, verifierErr := core.ParseOAuthPKCEVerifier(strings.Repeat("v", 43))
	if codeErr != nil || verifierErr != nil {
		t.Fatalf("code fixture errors = (%v, %v), want nil", codeErr, verifierErr)
	}
	return CodeExchange{Application: app, Code: code, Verifier: verifier, Access: access, Lifetime: lifetime, ObservedAt: at(t, "2026-11-02T10:00:00Z"), Policy: fixturePolicy(t)}
}

func TestExchangeCodeNativeOutcomes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		provider     Provider
		tenant       MicrosoftTenant
		access       Access
		lifetime     Lifetime
		status       int
		body         string
		redirect     string
		wantErr      error
		wantRefresh  bool
		wantRequests int
		wantExpires  string
		wantPath     string
		wantRevoke   string
	}{
		{name: "google visitor with both availability scopes is an online grant", provider: ProviderGoogle, access: AccessAvailability, lifetime: LifetimeVisitor, status: 200, body: tokenBody("ya29.visitor", "", googleVisitorScope, "Bearer", "3600"), wantRequests: 1, wantExpires: "2026-11-02T11:00:00Z", wantPath: "/token"},
		{name: "google expiry boundary of one second derives from ObservedAt", provider: ProviderGoogle, access: AccessAvailability, lifetime: LifetimeVisitor, status: 200, body: tokenBody("ya29.visitor", "", googleVisitorScope, "Bearer", "1"), wantRequests: 1, wantExpires: "2026-11-02T10:00:01Z", wantPath: "/token"},
		{name: "google expires_in zero is refused", provider: ProviderGoogle, access: AccessAvailability, lifetime: LifetimeVisitor, status: 200, body: tokenBody("ya29.visitor", "", googleVisitorScope, "Bearer", "0"), wantErr: core.ErrTimeMatchResponse, wantRequests: 1},
		{name: "google visitor consent granting only the calendar list is refused", provider: ProviderGoogle, access: AccessAvailability, lifetime: LifetimeVisitor, status: 200, body: tokenBody("ya29.visitor", "", googleScopeCalendarList, "Bearer", "3600"), wantErr: core.ErrTimeMatchConsent, wantRequests: 2, wantRevoke: "ya29.visitor"},
		{name: "google visitor confirmation grant is online with owned events", provider: ProviderGoogle, access: AccessManageEvents, lifetime: LifetimeVisitor, status: 200, body: tokenBody("ya29.visitor", "", googleConfirmScope, "Bearer", "3600"), wantRequests: 3, wantExpires: "2026-11-02T11:00:00Z", wantPath: "/token"},
		{name: "google visitor confirmation missing canonical email scope refuses", provider: ProviderGoogle, access: AccessManageEvents, lifetime: LifetimeVisitor, status: 200, body: tokenBody("ya29.visitor", "", strings.Replace(googleConfirmScope, " https://www.googleapis.com/auth/userinfo.email", "", 1), "Bearer", "3600"), wantErr: core.ErrTimeMatchConsent, wantRequests: 2, wantRevoke: "ya29.visitor"},
		{name: "google visitor confirmation missing canonical profile scope refuses", provider: ProviderGoogle, access: AccessManageEvents, lifetime: LifetimeVisitor, status: 200, body: tokenBody("ya29.visitor", "", strings.Replace(googleConfirmScope, " https://www.googleapis.com/auth/userinfo.profile", "", 1), "Bearer", "3600"), wantErr: core.ErrTimeMatchConsent, wantRequests: 2, wantRevoke: "ya29.visitor"},
		{name: "google visitor confirmation without owned events consent is refused", provider: ProviderGoogle, access: AccessManageEvents, lifetime: LifetimeVisitor, status: 200, body: tokenBody("ya29.visitor", "", googleVisitorScope, "Bearer", "3600"), wantErr: core.ErrTimeMatchConsent, wantRequests: 2, wantRevoke: "ya29.visitor"},
		{name: "google visitor confirmation given a refresh token revokes it and refuses", provider: ProviderGoogle, access: AccessManageEvents, lifetime: LifetimeVisitor, status: 200, body: tokenBody("ya29.visitor", "1//unexpected", googleConfirmScope, "Bearer", "3600"), wantErr: core.ErrTimeMatchResponse, wantRequests: 2, wantRevoke: "ya29.visitor"},
		{name: "microsoft visitor confirmation grant has no refresh", provider: ProviderMicrosoft, tenant: MicrosoftTenantCommon, access: AccessManageEvents, lifetime: LifetimeVisitor, status: 200, body: tokenBody("eyJ.visitor", "", "Calendars.ReadWrite", "Bearer", "3600"), wantRequests: 1, wantExpires: "2026-11-02T11:00:00Z", wantPath: "/common/oauth2/v2.0/token"},
		{name: "google business with owned events and refresh is retained", provider: ProviderGoogle, access: AccessManageEvents, lifetime: LifetimeBusiness, status: 200, body: tokenBody("ya29.business", "1//refresh", googleBusinessScope, "Bearer", "3599"), wantRefresh: true, wantRequests: 1, wantExpires: "2026-11-02T10:59:59Z", wantPath: "/token"},
		{name: "google business missing owned events scope is refused", provider: ProviderGoogle, access: AccessManageEvents, lifetime: LifetimeBusiness, status: 200, body: tokenBody("ya29.business", "1//refresh", googleVisitorScope, "Bearer", "3599"), wantErr: core.ErrTimeMatchConsent, wantRequests: 2, wantRevoke: "ya29.business"},
		{name: "google business without refresh token is refused", provider: ProviderGoogle, access: AccessManageEvents, lifetime: LifetimeBusiness, status: 200, body: tokenBody("ya29.business", "", googleBusinessScope, "Bearer", "3599"), wantErr: core.ErrTimeMatchConsent, wantRequests: 2, wantRevoke: "ya29.business"},
		{name: "google visitor given a refresh token revokes it and refuses", provider: ProviderGoogle, access: AccessAvailability, lifetime: LifetimeVisitor, status: 200, body: tokenBody("ya29.visitor", "1//unexpected", googleVisitorScope, "Bearer", "3600"), wantErr: core.ErrTimeMatchResponse, wantRequests: 2, wantRevoke: "ya29.visitor"},
		{name: "google non-bearer token type is refused", provider: ProviderGoogle, access: AccessAvailability, lifetime: LifetimeVisitor, status: 200, body: tokenBody("ya29.visitor", "", googleVisitorScope, "mac", "3600"), wantErr: core.ErrTimeMatchResponse, wantRequests: 1},
		{name: "invalid_grant is revoked or expired consent", provider: ProviderGoogle, access: AccessAvailability, lifetime: LifetimeVisitor, status: 400, body: `{"error":"invalid_grant","error_description":"` + privateMarker + `"}`, wantErr: core.ErrTimeMatchConsent, wantRequests: 1},
		{name: "invalid_client is a provider refusal not consent", provider: ProviderGoogle, access: AccessAvailability, lifetime: LifetimeVisitor, status: 400, body: `{"error":"invalid_client"}`, wantErr: core.ErrTimeMatchResponse, wantRequests: 1},
		{name: "server error is a provider refusal", provider: ProviderGoogle, access: AccessAvailability, lifetime: LifetimeVisitor, status: 500, body: `{"error":"` + privateMarker + `"}`, wantErr: core.ErrTimeMatchResponse, wantRequests: 1},
		{name: "duplicate access_token member is refused", provider: ProviderGoogle, access: AccessAvailability, lifetime: LifetimeVisitor, status: 200, body: `{"access_token":"a","access_token":"b","expires_in":10,"token_type":"Bearer","scope":"` + googleVisitorScope + `"}`, wantErr: core.ErrTimeMatchResponse, wantRequests: 1},
		{name: "redirect from token endpoint is refused without following", provider: ProviderGoogle, access: AccessAvailability, lifetime: LifetimeVisitor, status: 302, redirect: "https://attacker.example/steal", wantErr: core.ErrTimeMatchResponse, wantRequests: 1},
		{name: "microsoft personal visitor with short graph scope", provider: ProviderMicrosoft, tenant: MicrosoftTenantConsumers, access: AccessAvailability, lifetime: LifetimeVisitor, status: 200, body: tokenBody("eyJ.visitor", "", "Calendars.ReadBasic", "Bearer", "3600"), wantRequests: 1, wantExpires: "2026-11-02T11:00:00Z", wantPath: "/consumers/oauth2/v2.0/token"},
		{name: "microsoft work business rotates in a refresh token", provider: ProviderMicrosoft, tenant: MicrosoftTenantOrganizations, access: AccessManageEvents, lifetime: LifetimeBusiness, status: 200, body: tokenBody("eyJ.business", "0.refresh", microsoftScopeReadWrite+" offline_access", "Bearer", "4000"), wantRefresh: true, wantRequests: 1, wantExpires: "2026-11-02T11:06:40Z", wantPath: "/organizations/oauth2/v2.0/token"},
		{name: "microsoft visitor given a refresh token refuses without a revocation request", provider: ProviderMicrosoft, tenant: MicrosoftTenantCommon, access: AccessAvailability, lifetime: LifetimeVisitor, status: 200, body: tokenBody("eyJ.visitor", "0.unexpected", microsoftScopeReadBasic, "Bearer", "3600"), wantErr: core.ErrTimeMatchResponse, wantRequests: 1},
		{name: "microsoft consent_required is consent", provider: ProviderMicrosoft, tenant: MicrosoftTenantCommon, access: AccessAvailability, lifetime: LifetimeVisitor, status: 400, body: `{"error":"consent_required"}`, wantErr: core.ErrTimeMatchConsent, wantRequests: 1},
		{name: "microsoft omitted scope inherits the requested scopes per RFC6749", provider: ProviderMicrosoft, tenant: MicrosoftTenantCommon, access: AccessAvailability, lifetime: LifetimeVisitor, status: 200, body: `{"access_token":"eyJ.visitor","expires_in":3600,"token_type":"Bearer"}`, wantExpires: "2026-11-02T11:00:00Z", wantPath: "/common/oauth2/v2.0/token", wantRequests: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.URL.Path == "/oauth2/v3/userinfo" {
					writeJSON(w, http.StatusOK, `{"sub":"subject","given_name":"Ada","family_name":"Lovelace","email":"ada@example.test","email_verified":true}`)
					return
				}
				if r.URL.Path == "/v1/people/me" {
					writeJSON(w, http.StatusOK, `{"resourceName":"people/subject","phoneNumbers":[{"canonicalForm":"+14165550123","metadata":{"primary":true,"source":{"type":"PROFILE"}}}]}`)
					return
				}
				if r.URL.Path == "/revoke" {
					w.WriteHeader(http.StatusOK)
					return
				}
				if tc.redirect != "" {
					http.Redirect(w, r, tc.redirect, tc.status)
					return
				}
				writeJSON(w, tc.status, tc.body)
			})
			app := application(t, tc.provider, tc.tenant)
			grant, err := client.ExchangeCode(t.Context(), codeExchange(t, app, tc.access, tc.lifetime))
			requests := fixture.record()
			if len(requests) != tc.wantRequests || len(fixture.refusedDials()) != 0 {
				t.Fatalf("requests = %d, refused dials = %q, want %d and none", len(requests), fixture.refusedDials(), tc.wantRequests)
			}
			form, parseErr := url.ParseQuery(string(requests[0].body))
			if parseErr != nil || requests[0].method != http.MethodPost || requests[0].query != "" || form.Get("client_secret") != "client-secret-fixture" ||
				form.Get("code_verifier") != strings.Repeat("v", 43) || form.Get("grant_type") != "authorization_code" || form.Get("code") != "4/code-fixture" ||
				(form.Has("scope") != (tc.provider == ProviderMicrosoft)) || !strings.HasPrefix(requests[0].header.Get("Content-Type"), "application/x-www-form-urlencoded") {
				t.Fatalf("token request = %s %s?%s form=%v, want POST form with secret only in the body", requests[0].method, requests[0].path, requests[0].query, form)
			}
			noLeak(t, err, "client-secret-fixture", "1//unexpected", "ya29", "eyJ")
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || grant.AccessToken.IsSet() || grant.RefreshToken.IsSet() {
					t.Fatalf("ExchangeCode() = (set=%t, %v), want zero grant and %v", grant.AccessToken.IsSet(), err, tc.wantErr)
				}
				if tc.wantRequests == 2 && (requests[1].path != "/revoke" || string(requests[1].body) != "token="+tc.wantRevoke) {
					t.Fatalf("revocation request = %s body %q, want POST /revoke token=%s", requests[1].path, requests[1].body, tc.wantRevoke)
				}
				return
			}
			if err != nil || requests[0].path != tc.wantPath {
				t.Fatalf("ExchangeCode() error = %v path %q, want nil and %q", err, requests[0].path, tc.wantPath)
			}
			if grant.ExpiresAt != at(t, tc.wantExpires) || grant.RefreshToken.IsSet() != tc.wantRefresh || grant.Lifetime != tc.lifetime || grant.Provider != tc.provider {
				t.Fatalf("grant = expires %v refresh %t lifetime %v, want %s %t %v", grant.ExpiresAt, grant.RefreshToken.IsSet(), grant.Lifetime, tc.wantExpires, tc.wantRefresh, tc.lifetime)
			}
			if err := grant.Close(); err != nil || grant.AccessToken.IsSet() {
				t.Fatalf("Grant.Close() = %v, want nil and destroyed", err)
			}
		})
	}
}

func TestRefreshAndRevokeNative(t *testing.T) {
	t.Parallel()
	t.Run("microsoft refresh returns the rotated refresh token", func(t *testing.T) {
		t.Parallel()
		client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			writeJSON(w, 200, tokenBody("eyJ.next", "0.rotated", microsoftScopeReadBasic+" offline_access", "Bearer", "3600"))
		})
		grant, err := client.Refresh(t.Context(), RefreshRequest{Application: application(t, ProviderMicrosoft, MicrosoftTenantCommon), RefreshToken: secret(t, "0.previous"), Access: AccessAvailability, ObservedAt: at(t, "2026-11-02T10:00:00Z"), Policy: fixturePolicy(t)})
		form, _ := url.ParseQuery(string(fixture.record()[0].body))
		if err != nil || string(grant.RefreshToken.value) != "0.rotated" || form.Get("refresh_token") != "0.previous" || form.Get("grant_type") != "refresh_token" || !strings.Contains(form.Get("scope"), "offline_access") {
			t.Fatalf("Refresh() = (%q, %v) form %v, want rotated token from refresh_token grant", grant.RefreshToken.value, err, form)
		}
	})
	t.Run("google refresh without a new refresh token keeps none", func(t *testing.T) {
		t.Parallel()
		client, _ := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			writeJSON(w, 200, tokenBody("ya29.next", "", googleBusinessScope, "Bearer", "3600"))
		})
		grant, err := client.Refresh(t.Context(), RefreshRequest{Application: application(t, ProviderGoogle, MicrosoftTenantInvalid), RefreshToken: secret(t, "1//previous"), Access: AccessManageEvents, ObservedAt: at(t, "2026-11-02T10:00:00Z"), Policy: fixturePolicy(t)})
		if err != nil || grant.RefreshToken.IsSet() || grant.Lifetime != LifetimeBusiness {
			t.Fatalf("Refresh() = (refresh=%t, %v), want no refresh and nil", grant.RefreshToken.IsSet(), err)
		}
	})
	t.Run("revoked refresh token is consent loss", func(t *testing.T) {
		t.Parallel()
		client, _ := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			writeJSON(w, 400, `{"error":"invalid_grant"}`)
		})
		_, err := client.Refresh(t.Context(), RefreshRequest{Application: application(t, ProviderGoogle, MicrosoftTenantInvalid), RefreshToken: secret(t, "1//revoked"), Access: AccessAvailability, ObservedAt: at(t, "2026-11-02T10:00:00Z"), Policy: fixturePolicy(t)})
		if !errors.Is(err, core.ErrTimeMatchConsent) {
			t.Fatalf("Refresh(revoked) error = %v, want %v", err, core.ErrTimeMatchConsent)
		}
		noLeak(t, err, "1//revoked")
	})
	for _, row := range []struct {
		name     string
		provider Provider
	}{
		{name: "google revocation posts the token form", provider: ProviderGoogle},
		{name: "microsoft has no documented revocation door", provider: ProviderMicrosoft},
		{name: "apple app-specific passwords are revoked by the owner", provider: ProviderApple},
	} {
		provider := row.provider
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { w.WriteHeader(http.StatusOK) })
			got, err := client.Revoke(t.Context(), RevokeRequest{Provider: provider, Token: secret(t, "token/value"), Policy: fixturePolicy(t)})
			requests := fixture.record()
			want, wantRequests := RevokeProviderHasNoRevocation, 0
			if provider == ProviderGoogle {
				want, wantRequests = RevokedAtProvider, 1
			}
			if err != nil || got != want || len(requests) != wantRequests || wantRequests == 1 && (requests[0].path != "/revoke" || string(requests[0].body) != "token=token%2Fvalue" || requests[0].query != "") {
				t.Fatalf("Revoke(%v) = (%v, %v) with %d requests, want (%v, nil) and %d", provider, got, err, len(requests), want, wantRequests)
			}
		})
	}
}

func TestAuthorizationEndpointRequestsOnlyCalendarScopes(t *testing.T) {
	t.Parallel()
	state, stateErr := core.ParseOAuthState("state-fixture")
	verifier, verifierErr := core.ParseOAuthPKCEVerifier(strings.Repeat("v", 43))
	challenge, challengeErr := verifier.Challenge()
	if err := errors.Join(stateErr, verifierErr, challengeErr); err != nil {
		t.Fatalf("authorization fixture error = %v, want nil", err)
	}
	cases := []struct {
		name       string
		provider   Provider
		tenant     MicrosoftTenant
		access     Access
		lifetime   Lifetime
		wantErr    error
		wantHost   string
		wantPath   string
		wantScope  string
		wantAccess string
		wantPrompt string
	}{
		{name: "google visitor is online free/busy only without calendar list", provider: ProviderGoogle, access: AccessAvailability, lifetime: LifetimeVisitor, wantHost: "accounts.google.com", wantPath: "/o/oauth2/v2/auth", wantScope: googleVisitorScope, wantAccess: "online"},
		{name: "google business availability is offline with consent prompt", provider: ProviderGoogle, access: AccessAvailability, lifetime: LifetimeBusiness, wantHost: "accounts.google.com", wantPath: "/o/oauth2/v2/auth", wantScope: googleBusinessAvailabilityScope, wantAccess: "offline", wantPrompt: "consent"},
		{name: "google business manage adds owned events", provider: ProviderGoogle, access: AccessManageEvents, lifetime: LifetimeBusiness, wantHost: "accounts.google.com", wantPath: "/o/oauth2/v2/auth", wantScope: googleBusinessScope, wantAccess: "offline", wantPrompt: "consent"},
		{name: "microsoft common visitor read basic without offline access", provider: ProviderMicrosoft, tenant: MicrosoftTenantCommon, access: AccessAvailability, lifetime: LifetimeVisitor, wantHost: "login.microsoftonline.com", wantPath: "/common/oauth2/v2.0/authorize", wantScope: microsoftScopeReadBasic},
		{name: "microsoft personal business adds offline access", provider: ProviderMicrosoft, tenant: MicrosoftTenantConsumers, access: AccessAvailability, lifetime: LifetimeBusiness, wantHost: "login.microsoftonline.com", wantPath: "/consumers/oauth2/v2.0/authorize", wantScope: microsoftScopeReadBasic + " offline_access"},
		{name: "microsoft work manage uses read write", provider: ProviderMicrosoft, tenant: MicrosoftTenantOrganizations, access: AccessManageEvents, lifetime: LifetimeBusiness, wantHost: "login.microsoftonline.com", wantPath: "/organizations/oauth2/v2.0/authorize", wantScope: microsoftScopeReadWrite + " offline_access"},
		{name: "google visitor confirmation adds owned events online without consent prompt", provider: ProviderGoogle, access: AccessManageEvents, lifetime: LifetimeVisitor, wantHost: "accounts.google.com", wantPath: "/o/oauth2/v2/auth", wantScope: googleConfirmScope, wantAccess: "online"},
		{name: "microsoft visitor confirmation uses read write without offline access", provider: ProviderMicrosoft, tenant: MicrosoftTenantConsumers, access: AccessManageEvents, lifetime: LifetimeVisitor, wantHost: "login.microsoftonline.com", wantPath: "/consumers/oauth2/v2.0/authorize", wantScope: microsoftScopeReadWrite + " " + microsoftScopeUserRead},
		{name: "microsoft without tenant is refused", provider: ProviderMicrosoft, access: AccessAvailability, lifetime: LifetimeVisitor, wantErr: core.ErrTimeMatchContract},
		{name: "google with a tenant is refused", provider: ProviderGoogle, tenant: MicrosoftTenantCommon, access: AccessAvailability, lifetime: LifetimeVisitor, wantErr: core.ErrTimeMatchContract},
		{name: "apple has no oauth application", provider: ProviderApple, access: AccessAvailability, lifetime: LifetimeVisitor, wantErr: core.ErrTimeMatchContract},
		{name: "future provider is refused", provider: ProviderApple + 1, access: AccessAvailability, lifetime: LifetimeVisitor, wantErr: core.ErrTimeMatchContract},
		{name: "zero access is refused", provider: ProviderGoogle, lifetime: LifetimeVisitor, wantErr: core.ErrTimeMatchContract},
		{name: "future lifetime is refused", provider: ProviderGoogle, access: AccessAvailability, lifetime: LifetimeBusiness + 1, wantErr: core.ErrTimeMatchContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := AuthorizationEndpoint(AuthorizationRequest{Application: application(t, tc.provider, tc.tenant), State: state, Challenge: challenge, Access: tc.access, Lifetime: tc.lifetime})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got.Validate() == nil {
					t.Fatalf("AuthorizationEndpoint() = (%v, %v), want zero and %v", got, err, tc.wantErr)
				}
				return
			}
			u := got.HTTPURL()
			q := u.Query()
			if err != nil || u.Host != tc.wantHost || u.Path != tc.wantPath || q.Get("scope") != tc.wantScope || q.Get("access_type") != tc.wantAccess ||
				q.Get("prompt") != tc.wantPrompt || q.Get("code_challenge") != challenge.String() || q.Get("code_challenge_method") != "S256" ||
				q.Get("state") != "state-fixture" || q.Get("response_type") != "code" || q.Has("client_secret") {
				t.Fatalf("AuthorizationEndpoint() = (%s, %v), want %s%s scope %q access %q prompt %q", got, err, tc.wantHost, tc.wantPath, tc.wantScope, tc.wantAccess, tc.wantPrompt)
			}
			for scope := range strings.FieldsSeq(q.Get("scope")) {
				if (scope == "openid" || scope == "profile" || scope == "email") && !(tc.lifetime == LifetimeVisitor && tc.access == AccessManageEvents) {
					t.Fatalf("scope %q requests identity, want calendar scopes only", scope)
				}
			}
		})
	}
}
