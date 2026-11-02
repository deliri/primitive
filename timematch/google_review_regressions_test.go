package timematch

import (
	json "encoding/json/v2"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoogleCollisionReadFailureIsIndeterminate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		status int
		body   string
		cut    bool
	}{
		{name: "lost read after accepted create", cut: true},
		{name: "gateway unavailable", status: 503, body: `{"error":{"message":"private"}}`},
		{name: "gateway timeout", status: 504, body: `{}`},
		{name: "rate limit", status: 429, body: `{}`},
		{name: "quota forbidden", status: 403, body: `{"error":{"errors":[{"reason":"rateLimitExceeded"}]}}`},
		{name: "invalid credentials do not prove foreign event", status: 401, body: `{}`},
		{name: "resource disappeared between collision and read", status: 404, body: `{}`},
		{name: "resource gone between collision and read", status: 410, body: `{}`},
		{name: "truncated follow-up document", status: 200, body: `{"id":`},
		{name: "null follow-up document", status: 200, body: `null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method == http.MethodPost {
					writeJSON(w, 409, `{}`)
					return
				}
				if tc.cut {
					cutConnection(w)
					return
				}
				writeJSON(w, tc.status, tc.body)
			})
			owned := intent(t, "primary", "Review", "2026-11-02T15:00:00Z", "2026-11-02T15:30:00Z")
			receipt, err := client.PutEvent(t.Context(), eventWrite(t, oauthConnection(t, ProviderGoogle), owned))
			if !errors.Is(err, core.ErrTimeMatchIndeterminate) || errors.Is(err, core.ErrTimeMatchConflict) || receipt != (EventReceipt{}) {
				t.Fatalf("unreadable collision=(%+v,%v),want zero,indeterminate and no conflict", receipt, err)
			}
			requests := fixture.record()
			if len(requests) != 2 || requests[0].method != http.MethodPost || requests[1].method != http.MethodGet {
				t.Fatalf("collision requests=%+v,want one create then one own-ID read", requests)
			}
		})
	}
}

func TestGoogleRecoveryNeverRecreatesCancelledOrInvisibleEvent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		status    int
		cancelled bool
		want      error
	}{
		{name: "unknown initial outcome is not a second create", status: 404, want: core.ErrTimeMatchIndeterminate},
		{name: "deleted identity stays retired", status: 200, cancelled: true, want: core.ErrTimeMatchConflict},
		{name: "existing identical event is recovered", status: 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := intent(t, "primary", "Review", "2026-11-02T15:00:00Z", "2026-11-02T15:30:00Z")
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				status := "confirmed"
				if tc.cancelled {
					status = "cancelled"
				}
				writeJSON(w, tc.status, googleStored(t, owned, "Review", status))
			})
			request := eventWrite(t, oauthConnection(t, ProviderGoogle), owned)
			request.Attempt = EventCreateRecoverOnly
			receipt, err := client.PutEvent(t.Context(), request)
			if !errors.Is(err, tc.want) {
				t.Fatalf("recovery error=%v,want %v", err, tc.want)
			}
			if tc.want != nil && receipt != (EventReceipt{}) {
				t.Fatalf("failed recovery receipt=%+v,want zero", receipt)
			}
			calls := fixture.record()
			if len(calls) != 1 || calls[0].method != http.MethodGet {
				t.Fatalf("recovery requests=%+v,want one GET and no create/update", calls)
			}
		})
	}
}

func TestGoogleDocumentedRestrictedWriterKeepsCalendarList(t *testing.T) {
	t.Parallel()
	roles := []string{"freeBusyReader", "reader", "writer", "writerWithoutPrivateAccess", "owner"}
	type roleItem struct {
		ID         string `json:"id"`
		AccessRole string `json:"accessRole"`
	}
	document := struct {
		Items []roleItem `json:"items"`
	}{Items: make([]roleItem, len(roles))}
	for i, role := range roles {
		document.Items[i] = roleItem{ID: role + "@example.test", AccessRole: role}
	}
	wire, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("role fixture error=%v,want nil", err)
	}
	client, _ := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeJSON(w, 200, string(wire)) })
	got, err := client.ListCalendars(t.Context(), CalendarListRequest{Connection: oauthConnection(t, ProviderGoogle), Policy: fixturePolicy(t)})
	if err != nil || len(got.Calendars) != len(roles) {
		t.Fatalf("calendar list=(%d,%v),want all%d,nil", len(got.Calendars), err, len(roles))
	}
	for i, calendar := range got.Calendars {
		if calendar.ID.String() != document.Items[i].ID || calendar.Writable != (roles[i] == "owner") {
			t.Fatalf("calendar%d=%+v,want owned-scope writes only for owner", i, calendar)
		}
	}
}

func TestGoogleQuotaDoesNotInvalidateConsent(t *testing.T) {
	t.Parallel()
	for _, reason := range []string{"rateLimitExceeded", "userRateLimitExceeded", "quotaExceeded", "forbidden"} {
		t.Run(reason, func(t *testing.T) {
			t.Parallel()
			client, _ := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
				writeJSON(w, 403, `{"error":{"errors":[{"reason":"`+reason+`"}],"message":"`+privateMarker+`"}}`)
			})
			_, err := client.ListCalendars(t.Context(), CalendarListRequest{Connection: oauthConnection(t, ProviderGoogle), Policy: fixturePolicy(t)})
			if !errors.Is(err, core.ErrTimeMatchResponse) || errors.Is(err, core.ErrTimeMatchConsent) || strings.Contains(err.Error(), privateMarker) {
				t.Fatalf("quota error=%v,want response refusal without lost-consent or private data", err)
			}
		})
	}
}

func TestOAuthOptionalScopeUsesRequestedPurposeOnly(t *testing.T) {
	t.Parallel()
	for _, provider := range []Provider{ProviderGoogle, ProviderMicrosoft} {
		for _, scope := range []struct {
			name, member string
			want         error
		}{
			{name: "omitted scope is unchanged requested consent"},
			{name: "explicit empty scope never grants requested consent", member: `,"scope":""`, want: core.ErrTimeMatchConsent},
			{name: "explicit unrelated permission never grants requested consent", member: `,"scope":"openid"`, want: core.ErrTimeMatchConsent},
			{name: "null scope is malformed", member: `,"scope":null`, want: core.ErrTimeMatchResponse},
			{name: "array scope is malformed", member: `,"scope":[]`, want: core.ErrTimeMatchResponse},
		} {
			t.Run(provider.String()+"_"+scope.name, func(t *testing.T) {
				t.Parallel()
				client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
					if r.URL.Path == "/revoke" {
						w.WriteHeader(200)
						return
					}
					writeJSON(w, 200, `{"access_token":"valid-access-token","expires_in":3600,"token_type":"Bearer"`+scope.member+`}`)
				})
				tenant := MicrosoftTenantInvalid
				if provider == ProviderMicrosoft {
					tenant = MicrosoftTenantCommon
				}
				request := codeExchange(t, application(t, provider, tenant), AccessAvailability, LifetimeVisitor)
				grant, err := client.ExchangeCode(t.Context(), request)
				if !errors.Is(err, scope.want) {
					t.Fatalf("optional scope error=%v,want %v", err, scope.want)
				}
				if scope.want == nil {
					if grant.Provider != provider || grant.Access != AccessAvailability || grant.Lifetime != LifetimeVisitor || grant.RefreshToken.IsSet() {
						t.Fatalf("grant=%+v,want exact requested visitor availability", grant)
					}
					if err := grant.Close(); err != nil {
						t.Fatalf("grant close=%v,want nil", err)
					}
				} else if grant.AccessToken.IsSet() || grant.RefreshToken.IsSet() {
					t.Fatal("refused scope retained credentials, want zero grant")
				}
				if len(fixture.refusedDials()) != 0 {
					t.Fatalf("foreign dials=%q,want none", fixture.refusedDials())
				}
			})
		}
	}
}
