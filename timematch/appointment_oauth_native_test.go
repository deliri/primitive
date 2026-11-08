package timematch

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Native Exchange and the actual profile parsers execute against local TLS.
// No live provider, calendar, mailbox or SMS is used here.
func TestAppointmentCodeExchangeKeepsContactAndRefreshNative(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		provider      Provider
		tenant        MicrosoftTenant
		absentPhone   bool
		absentRefresh bool
		absentProfile bool
		wantErr       error
	}{
		{name: "Google appointment retains profile and refresh together", provider: ProviderGoogle},
		{name: "Microsoft personal appointment retains profile and refresh together", provider: ProviderMicrosoft, tenant: MicrosoftTenantConsumers},
		{name: "Microsoft work appointment uses its original organizations endpoint", provider: ProviderMicrosoft, tenant: MicrosoftTenantOrganizations},
		{name: "Microsoft common appointment keeps both account families", provider: ProviderMicrosoft, tenant: MicrosoftTenantCommon},
		{name: "Google absent phone cannot discard retained calendar access", provider: ProviderGoogle, absentPhone: true},
		{name: "Microsoft absent phone cannot discard retained calendar access", provider: ProviderMicrosoft, tenant: MicrosoftTenantConsumers, absentPhone: true},
		{name: "Google missing refresh refuses before contact reads", provider: ProviderGoogle, absentRefresh: true, wantErr: core.ErrTimeMatchConsent},
		{name: "Microsoft missing refresh refuses before contact reads", provider: ProviderMicrosoft, tenant: MicrosoftTenantCommon, absentRefresh: true, wantErr: core.ErrTimeMatchConsent},
		{name: "Google missing profile scope cannot become retained identity", provider: ProviderGoogle, absentProfile: true, wantErr: core.ErrTimeMatchConsent},
		{name: "Microsoft missing profile scope cannot become retained identity", provider: ProviderMicrosoft, tenant: MicrosoftTenantOrganizations, absentProfile: true, wantErr: core.ErrTimeMatchConsent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, peer := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/token"):
					scope, err := scopes(tc.provider, AccessManageEvents, LifetimeAppointment)
					if err != nil {
						t.Errorf("appointment scope error = %v, want nil", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					if tc.absentProfile {
						remove := googleScopeProfile
						if tc.provider == ProviderMicrosoft {
							remove = microsoftScopeUserRead
						}
						scope = slices.DeleteFunc(scope, func(value string) bool { return value == remove })
					}
					refresh := "appointment-refresh"
					if tc.absentRefresh {
						refresh = ""
					}
					writeJSON(w, http.StatusOK, tokenBody("appointment-access", refresh, strings.Join(scope, " "), bearerTokenType, "3600"))
				case r.URL.Path == "/oauth2/v3/userinfo":
					writeJSON(w, http.StatusOK, string(seedJSON(t, struct {
						Subject  string `json:"sub"`
						Given    string `json:"given_name"`
						Family   string `json:"family_name"`
						Email    string `json:"email"`
						Verified bool   `json:"email_verified"`
					}{"appointment-subject", "Ada", "Lovelace", "ada@example.test", true})))
				case r.URL.Path == "/v1/people/me":
					if tc.absentPhone {
						writeJSON(w, http.StatusOK, `{"resourceName":"people/appointment-subject"}`)
						return
					}
					writeJSON(w, http.StatusOK, `{"resourceName":"people/appointment-subject","phoneNumbers":[{"canonicalForm":"+14165550123","metadata":{"primary":true,"source":{"type":"PROFILE"}}}]}`)
				case r.URL.Path == "/v1.0/me":
					var phone *string
					if !tc.absentPhone {
						value := "+14165550123"
						phone = &value
					}
					writeJSON(w, http.StatusOK, string(seedJSON(t, struct {
						Subject string  `json:"id"`
						Given   string  `json:"givenName"`
						Family  string  `json:"surname"`
						Email   string  `json:"mail"`
						Phone   *string `json:"mobilePhone"`
					}{"appointment-subject", "Ada", "Lovelace", "ada@example.test", phone})))
				case r.URL.Path == "/revoke":
					w.WriteHeader(http.StatusOK)
				default:
					t.Errorf("appointment provider path = %q, want token/profile or refused-grant revocation", r.URL.Path)
					w.WriteHeader(http.StatusInternalServerError)
				}
			})
			grant, err := client.ExchangeCode(t.Context(), codeExchange(t, application(t, tc.provider, tc.tenant), AccessManageEvents, LifetimeAppointment))
			defer func() {
				if err := grant.Close(); err != nil {
					t.Errorf("appointment grant close error = %v, want nil", err)
				}
			}()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("appointment exchange error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if grant.AccessToken.IsSet() || grant.RefreshToken.IsSet() || grant.Contact != (VisitorContact{}) {
					t.Fatalf("refused appointment access/refresh/contact = %t/%t/%v, want zero authority", grant.AccessToken.IsSet(), grant.RefreshToken.IsSet(), grant.Contact)
				}
				for _, request := range peer.record() {
					if !strings.HasSuffix(request.path, "/token") && request.path != "/revoke" {
						t.Fatalf("refused appointment request = %q, want no profile reads", request.path)
					}
				}
				return
			}
			wantPhone := "+14165550123"
			if tc.absentPhone {
				wantPhone = ""
			}
			if grant.Lifetime != LifetimeAppointment || grant.Contact.Subject.String() != "appointment-subject" || grant.Contact.Email.String() != "ada@example.test" || grant.Contact.Phone.String() != wantPhone || !grant.RefreshToken.IsSet() || grant.Contact.Validate() != nil {
				t.Fatalf("appointment lifetime/contact/refresh = %v/%v/%t, want original identity, phone %q and retained capability", grant.Lifetime, grant.Contact, grant.RefreshToken.IsSet(), wantPhone)
			}
			if grant.ExpiresAt != at(t, "2026-11-02T11:00:00Z") || len(peer.refusedDials()) != 0 {
				t.Fatalf("appointment token expiry/refused hosts = %v/%v, want exact observed expiry and no foreign host", grant.ExpiresAt, peer.refusedDials())
			}
		})
	}
}

func TestAppointmentRefreshPreservesLifetimeNative(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		provider Provider
		tenant   MicrosoftTenant
		rotated  string
	}{
		{name: "Google omitted replacement preserves appointment lifetime", provider: ProviderGoogle},
		{name: "Microsoft rotated refresh preserves appointment lifetime", provider: ProviderMicrosoft, tenant: MicrosoftTenantCommon, rotated: "next-refresh"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wantScope, err := scopes(tc.provider, AccessManageEvents, LifetimeAppointment)
			if err != nil {
				t.Fatal(err)
			}
			client, peer := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
				writeJSON(w, http.StatusOK, tokenBody("next-access", tc.rotated, strings.Join(wantScope, " "), bearerTokenType, "3600"))
			})
			grant, err := client.Refresh(t.Context(), RefreshRequest{Application: application(t, tc.provider, tc.tenant), RefreshToken: secret(t, "prior-refresh"), ObservedAt: at(t, "2026-11-02T10:00:00Z"), Policy: fixturePolicy(t), Access: AccessManageEvents, Lifetime: LifetimeAppointment})
			defer func() {
				if err := grant.Close(); err != nil {
					t.Errorf("refreshed appointment close error = %v, want nil", err)
				}
			}()
			if err != nil || grant.Lifetime != LifetimeAppointment || string(grant.RefreshToken.value) != tc.rotated || grant.ExpiresAt != at(t, "2026-11-02T11:00:00Z") {
				t.Fatalf("appointment refresh lifetime/token/expiry/error = %v/%q/%v/%v, want appointment/%q/exact expiry/nil", grant.Lifetime, grant.RefreshToken.value, grant.ExpiresAt, err, tc.rotated)
			}
			requests := peer.record()
			if len(requests) != 1 {
				t.Fatalf("appointment refresh requests = %d, want one", len(requests))
			}
			form, err := url.ParseQuery(string(requests[0].body))
			if err != nil || form.Get("grant_type") != "refresh_token" || form.Get("refresh_token") != "prior-refresh" {
				t.Fatalf("appointment refresh grant/source/error = %q/%q/%v, want original refresh intent", form.Get("grant_type"), form.Get("refresh_token"), err)
			}
			if tc.provider == ProviderMicrosoft && !slices.Equal(strings.Fields(form.Get("scope")), wantScope) {
				t.Fatalf("appointment refresh scopes = %q, want %q", form.Get("scope"), wantScope)
			}
		})
	}
}
