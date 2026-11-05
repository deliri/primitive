package timematch

import (
	json "encoding/json"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"net/http"
	"strings"
	"testing"
)

// Real Exchange/TLS/code redemption and the native People SDK execute against
// a local provider double. No real account, calendar, email or SMS is touched.
func TestVisitorCodeExchangeReturnsRequiredContact(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		provider       Provider
		profile, phone string
		phoneStatus    int
		wantErr        error
	}{
		{name: "Google complete contact", provider: ProviderGoogle},
		{name: "Outlook complete contact", provider: ProviderMicrosoft},
		{name: "Google unverified email refuses", provider: ProviderGoogle, profile: `{"sub":"subject","given_name":"Ada","family_name":"Lovelace","email":"ada@example.test","email_verified":false}`, wantErr: core.ErrTimeMatchConsent},
		{name: "Google missing given name refuses", provider: ProviderGoogle, profile: `{"sub":"subject","family_name":"Lovelace","email":"ada@example.test","email_verified":true}`, wantErr: core.ErrTimeMatchConsent},
		{name: "Google missing family name refuses", provider: ProviderGoogle, profile: `{"sub":"subject","given_name":"Ada","email":"ada@example.test","email_verified":true}`, wantErr: core.ErrTimeMatchConsent},
		{name: "Google missing email refuses", provider: ProviderGoogle, profile: `{"sub":"subject","given_name":"Ada","family_name":"Lovelace","email_verified":true}`, wantErr: core.ErrTimeMatchConsent},
		{name: "Google missing subject refuses", provider: ProviderGoogle, profile: `{"given_name":"Ada","family_name":"Lovelace","email":"ada@example.test","email_verified":true}`, wantErr: core.ErrTimeMatchConsent},
		{name: "Google absent phone refuses", provider: ProviderGoogle, phone: `{"resourceName":"people/subject"}`, wantErr: core.ErrTimeMatchPhoneMissing},
		{name: "Google foreign phone subject refuses", provider: ProviderGoogle, phone: `{"resourceName":"people/foreign","phoneNumbers":[{"canonicalForm":"+14165550123","metadata":{"primary":true,"source":{"type":"PROFILE"}}}]}`, wantErr: core.ErrTimeMatchPhoneInvalid},
		{name: "Google incomplete canonical phone refuses", provider: ProviderGoogle, phone: `{"resourceName":"people/subject","phoneNumbers":[{"metadata":{"primary":true}}]}`, wantErr: core.ErrTimeMatchPhonePartial},
		{name: "Google invalid canonical phone refuses", provider: ProviderGoogle, phone: `{"resourceName":"people/subject","phoneNumbers":[{"canonicalForm":"not a phone","metadata":{"primary":true}}]}`, wantErr: core.ErrTimeMatchPhoneInvalid},
		{name: "Google denied phone lookup refuses", provider: ProviderGoogle, phoneStatus: 403, wantErr: core.ErrTimeMatchPhoneDenied},
		{name: "Google unavailable phone lookup refuses", provider: ProviderGoogle, phoneStatus: 503, wantErr: core.ErrTimeMatchPhoneUnavailable},
		{name: "Outlook missing phone refuses", provider: ProviderMicrosoft, profile: `{"id":"subject","givenName":"Ada","surname":"Lovelace","mail":"ada@example.test"}`, wantErr: core.ErrTimeMatchConsent},
		{name: "Outlook malformed phone refuses", provider: ProviderMicrosoft, profile: `{"id":"subject","givenName":"Ada","surname":"Lovelace","mail":"ada@example.test","mobilePhone":"call me"}`, wantErr: core.ErrTimeMatchConsent},
		{name: "Outlook missing mailbox refuses", provider: ProviderMicrosoft, profile: `{"id":"subject","givenName":"Ada","surname":"Lovelace","mobilePhone":"+14165550123"}`, wantErr: core.ErrTimeMatchConsent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tenant := MicrosoftTenantInvalid
			if tc.provider == ProviderMicrosoft {
				tenant = MicrosoftTenantConsumers
			}
			client, p := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/token"):
					scope, err := scopes(tc.provider, AccessManageEvents, LifetimeVisitor)
					if err != nil {
						t.Error(err)
						w.WriteHeader(500)
						return
					}
					_, _ = w.Write([]byte(tokenBody("visitor-access", "", strings.Join(scope, " "), "Bearer", "3600")))
				case r.URL.Path == "/revoke":
					w.WriteHeader(http.StatusOK)
				case r.URL.Path == "/oauth2/v3/userinfo":
					body := tc.profile
					if body == "" {
						body = `{"sub":"subject","given_name":"Ada","family_name":"Lovelace","email":"ada@example.test","email_verified":true}`
					}
					_, _ = w.Write([]byte(body))
				case r.URL.Path == "/v1/people/me":
					if tc.phoneStatus != 0 {
						w.WriteHeader(tc.phoneStatus)
						return
					}
					body := tc.phone
					if body == "" {
						body = `{"resourceName":"people/subject","phoneNumbers":[{"canonicalForm":"+14165550123","metadata":{"primary":true,"verified":true,"source":{"type":"PROFILE"}}}]}`
					}
					_, _ = w.Write([]byte(body))
				case r.URL.Path == "/v1.0/me":
					body := tc.profile
					if body == "" {
						body = `{"id":"subject","givenName":"Ada","surname":"Lovelace","mail":"ada@example.test","mobilePhone":"+14165550123"}`
					}
					_, _ = w.Write([]byte(body))
				default:
					t.Errorf("provider path=%s,want only token/profile/revoke", r.URL.Path)
					w.WriteHeader(500)
				}
			})
			grant, err := client.ExchangeCode(t.Context(), codeExchange(t, application(t, tc.provider, tenant), AccessManageEvents, LifetimeVisitor))
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || grant.AccessToken.IsSet() || grant.Contact != (VisitorContact{}) {
					t.Fatalf("ExchangeCode()=(credential %t,contact %v,error %v),want zero and %v", grant.AccessToken.IsSet(), grant.Contact, err, tc.wantErr)
				}
			} else {
				if err != nil || grant.Contact.Validate() != nil || grant.Contact.GivenName.String() != "Ada" || grant.Contact.FamilyName.String() != "Lovelace" || grant.Contact.Email.String() != "ada@example.test" || grant.Contact.Phone.String() != "+14165550123" {
					t.Fatalf("ExchangeCode()=(%v,%v),want complete provider contact", grant.Contact, err)
				}
				if err := grant.Close(); err != nil {
					t.Fatal(err)
				}
			}
			exchanges := 0
			for _, r := range p.record() {
				if strings.HasSuffix(r.path, "/token") {
					exchanges++
				}
				if !strings.HasSuffix(r.path, "/token") && r.path != "/revoke" && r.header.Get("Authorization") != "Bearer visitor-access" {
					t.Fatalf("profile authorization=%q,want scoped bearer", r.header.Get("Authorization"))
				}
			}
			if exchanges != 1 || len(p.refusedDials()) != 0 {
				t.Fatalf("exchanges=%d,refused=%v,want one exchange and no foreign hosts", exchanges, p.refusedDials())
			}
		})
	}
}

// Same production response door with arbitrary provider JSON. An accepted
// result must retain the consenting account's exact fields, not manufacture a
// contact from absent, malformed or unverified provider data.
func FuzzVisitorProviderContactSemanticClosure(f *testing.F) {
	f.Add(false, []byte(`{"sub":"subject","given_name":"Ada","family_name":"Lovelace","email":"ada@example.test","email_verified":true}`))
	f.Add(true, []byte(`{"id":"subject","givenName":"Ada","surname":"Lovelace","mail":"ada@example.test","mobilePhone":"+14165550123"}`))
	f.Add(false, []byte(`{"email_verified":false}`))
	f.Add(true, []byte(`{`))
	f.Fuzz(func(t *testing.T, microsoft bool, body []byte) {
		provider, tenant := ProviderGoogle, MicrosoftTenantInvalid
		if microsoft {
			provider, tenant = ProviderMicrosoft, MicrosoftTenantConsumers
		}
		scope, scopeErr := scopes(provider, AccessManageEvents, LifetimeVisitor)
		if scopeErr != nil {
			t.Fatal(scopeErr)
		}
		client, p := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case strings.HasSuffix(r.URL.Path, "/token"):
				_, _ = w.Write([]byte(tokenBody("visitor-access", "", strings.Join(scope, " "), "Bearer", "3600")))
			case r.URL.Path == "/revoke":
				w.WriteHeader(http.StatusOK)
			case r.URL.Path == "/v1/people/me":
				_, _ = w.Write([]byte(`{"resourceName":"people/subject","phoneNumbers":[{"canonicalForm":"+14165550123","metadata":{"primary":true,"source":{"type":"PROFILE"}}}]}`))
			default:
				_, _ = w.Write(body)
			}
		})
		grant, err := client.ExchangeCode(t.Context(), codeExchange(t, application(t, provider, tenant), AccessManageEvents, LifetimeVisitor))
		if err != nil {
			if grant.AccessToken.IsSet() || grant.Contact != (VisitorContact{}) {
				t.Fatalf("refused response retained contact=%v/access=%t,want zero", grant.Contact, grant.AccessToken.IsSet())
			}
			return
		}
		defer func() {
			if err := grant.Close(); err != nil {
				t.Error(err)
			}
		}()
		if err := grant.Contact.Validate(); err != nil {
			t.Fatalf("accepted contact invalid=%v,want complete provider contact", err)
		}
		var wire struct {
			Sub, ID                    string
			GivenName                  string `json:"givenName"`
			Surname, Mail, MobilePhone string
			Given                      string `json:"given_name"`
			Family                     string `json:"family_name"`
			Email                      string
			Verified                   bool `json:"email_verified"`
		}
		if err := json.Unmarshal(body, &wire); err != nil {
			t.Fatalf("accepted malformed response=%v,want valid provider JSON", err)
		}
		subject, given, family, email, phone := wire.Sub, wire.Given, wire.Family, wire.Email, "+14165550123"
		if microsoft {
			subject, given, family, email, phone = wire.ID, wire.GivenName, wire.Surname, wire.Mail, wire.MobilePhone
		} else if !wire.Verified {
			t.Fatal("unverified Google email accepted,want refusal")
		}
		c := grant.Contact
		if c.Subject.String() != subject || c.GivenName.String() != given || c.FamilyName.String() != family || c.Email.String() != email || c.Phone.String() != phone {
			t.Fatalf("contact=%v,want exact native provider fields", c)
		}
		if len(p.refusedDials()) != 0 {
			t.Fatalf("foreign dials=%v,want none", p.refusedDials())
		}
	})
}
