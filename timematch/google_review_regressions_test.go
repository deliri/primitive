package timematch

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
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
			if len(requests) < 2 || requests[0].method != http.MethodPost {
				t.Fatalf("collision request count=%d,want one create followed by read-only recovery", len(requests))
			}
			for _, request := range requests[1:] {
				// Native transport may retry an idempotent GET after a dropped
				// reused connection. It must never repeat the effectful POST.
				if request.method != http.MethodGet || request.path != requests[1].path {
					t.Fatalf("collision recovery method/path=%s/%s,want GET of same owned ID", request.method, request.path)
				}
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
		{name: "unmarked deleted identity cannot establish ownership", status: 200, cancelled: true, want: core.ErrTimeMatchIndeterminate},
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
				stored := googleStored(t, owned, "Review", status)
				if tc.status == 200 && strings.HasSuffix(r.URL.Path, "/events") {
					if tc.cancelled {
						writeJSON(w, 200, `{"items":[]}`)
						return
					}
					revision := fmt.Sprintf("%q", googleFixtureETag(stored))
					writeJSON(w, 200, `{"items":[{"id":"`+owned.ID.String()+`","etag":`+revision+`}]}`)
					return
				}
				writeJSON(w, tc.status, googleFixtureWithETag(stored))
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
			wantReads := 1
			if tc.want == nil {
				wantReads = 2
			}
			if len(calls) != wantReads {
				t.Fatalf("recovery requests=%d,want%d filtered proof/conditional readback", len(calls), wantReads)
			}
			for _, call := range calls {
				if call.method != http.MethodGet {
					t.Fatalf("recovery method=%s,want GET only", call.method)
				}
			}
			if tc.want == nil && calls[1].header.Get("If-Match") == "" {
				t.Fatalf("owned readback If-Match = %q, want observed conditional revision", calls[1].header.Get("If-Match"))
			}
		})
	}
}

func TestGoogleUnmarkedDeletedTombstoneCannotEstablishOwnership(t *testing.T) {
	t.Parallel()
	owned := intent(t, "primary", "Review", "2026-11-02T15:00:00Z", "2026-11-02T15:30:00Z")
	wire := seedJSON(t, struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}{ID: owned.ID.String(), Status: "cancelled"})
	client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if strings.HasSuffix(r.URL.Path, "/events") {
			writeJSON(w, 200, `{"items":[]}`)
			return
		}
		writeJSON(w, 200, string(wire))
	})
	request := eventWrite(t, oauthConnection(t, ProviderGoogle), owned)
	request.Attempt = EventCreateRecoverOnly
	receipt, err := client.PutEvent(t.Context(), request)
	if !errors.Is(err, core.ErrTimeMatchIndeterminate) || receipt != (EventReceipt{}) {
		t.Fatalf("deleted tombstone recovery=(%+v,%v),want unknown ownership without downloading the deleted event", receipt, err)
	}
	if calls := fixture.record(); len(calls) != 1 || calls[0].method != http.MethodGet {
		t.Fatalf("deleted identity calls=%d,want exactly one read and no resurrection", len(calls))
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
		if calendar.ID.String() != document.Items[i].ID || calendar.Writable {
			t.Fatalf("calendar%d=%+v,want no ownership inferred from access role alone", i, calendar)
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

func TestGoogleMissingBusyCollectionNeverMeansFree(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, calendar string
		known          bool
	}{
		{name: "missing busy is unknown", calendar: `{}`},
		{name: "null busy is unknown", calendar: `{"busy":null}`},
		{name: "explicit empty busy is known free", calendar: `{"busy":[]}`, known: true},
		{name: "calendar errors defeat empty busy", calendar: `{"busy":[],"errors":[{"reason":"internalError"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, _ := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
				writeJSON(w, 200, `{"timeMin":"2026-11-02T09:00:00Z","timeMax":"2026-11-02T17:00:00Z","calendars":{"primary":`+tc.calendar+`}}`)
			})
			got, err := client.Availability(t.Context(), AvailabilityRequest{Connection: oauthConnection(t, ProviderGoogle), Calendars: []CalendarID{calendarID(t, PrimaryCalendar)}, Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Policy: fixturePolicy(t)})
			want := AvailabilityUnknown
			if tc.known {
				want = AvailabilityKnown
			}
			if err != nil || len(got.Calendars) != 1 || got.Calendars[0].State != want || len(got.Calendars[0].Busy) != 0 {
				t.Fatalf("busy collection=(%+v,%v),want one state%v,emptybusy,nil", got, err, want)
			}
		})
	}
}

func TestGoogleReschedulePreservesUserCalendarNotes(t *testing.T) {
	t.Parallel()
	owned := intent(t, "primary", "Review", "2026-11-02T15:00:00Z", "2026-11-02T15:30:00Z")
	const description = "A note the calendar owner added"
	stored := strings.TrimSuffix(googleStored(t, owned, "Review", "confirmed"), "}") + `,"description":"` + description + `"}`
	calendar := &googleCalendar{events: map[string]string{owned.ID.value: stored}}
	client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) { calendar.serve(w, r, body) })
	updated := owned
	updated.Interval = window(t, "2026-11-02T16:00:00Z", "2026-11-02T16:30:00Z")
	ref := EventRef{Provider: ProviderGoogle, ID: owned.ID, Calendar: owned.Calendar, Resource: owned.ID.value}
	receipt, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: oauthConnection(t, ProviderGoogle), Ref: ref, Intent: updated, ObservedAt: at(t, "2026-11-02T14:00:00Z"), Policy: fixturePolicy(t)})
	if err != nil || receipt.Ref != ref {
		t.Fatalf("reschedule=(%+v,%v),want same reference,nil", receipt, err)
	}
	calls := fixture.record()
	if len(calls) != 2 || calls[0].method != http.MethodGet || calls[1].method != http.MethodPatch || calls[1].header.Get("If-Match") == "" || strings.Contains(string(calls[1].body), "description") {
		t.Fatalf("reschedule calls=%+v,want filtered ownership read then conditional partial update without unrequested notes", calls)
	}
	calendar.mutex.Lock()
	after := calendar.events[owned.ID.value]
	calendar.mutex.Unlock()
	if !strings.Contains(after, description) {
		t.Fatalf("rescheduled event=%q,want preserved owner note", after)
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
					t.Fatalf("refused scope credentials access/refresh=%t/%t,want false/false", grant.AccessToken.IsSet(), grant.RefreshToken.IsSet())
				}
				if len(fixture.refusedDials()) != 0 {
					t.Fatalf("foreign dials=%q,want none", fixture.refusedDials())
				}
			})
		}
	}
}
