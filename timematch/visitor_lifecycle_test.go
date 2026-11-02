package timematch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// oneShotGoogle serves the documented Google endpoints for one visitor
// confirmation. insertStatus lets a row fail the confirmation write.
func oneShotGoogle(insertStatus int) func(w http.ResponseWriter, r *http.Request, body []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		switch r.URL.Path {
		case "/token":
			writeJSON(w, 200, tokenBody("ya29.oneshot", "", googleBusinessScope, "Bearer", "3600"))
		case "/calendar/v3/freeBusy":
			writeJSON(w, 200, `{"timeMin":"2026-11-02T09:00:00Z","timeMax":"2026-11-02T17:00:00Z","calendars":{"primary":{"busy":[{"start":"2026-11-02T10:00:00Z","end":"2026-11-02T11:00:00Z"}]}}}`)
		case "/calendar/v3/calendars/primary/events":
			if insertStatus != 200 {
				writeJSON(w, insertStatus, `{}`)
				return
			}
			writeJSON(w, 200, strings.TrimSuffix(string(body), "}")+`,"status":"confirmed","description":"`+privateMarker+`"}`)
		case "/revoke":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func requirePaths(t *testing.T, fixture *provider, want ...string) {
	t.Helper()
	requests := fixture.record()
	got := make([]string, 0, len(requests))
	for _, request := range requests {
		got = append(got, request.path)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("provider requests = %q, want %q", got, want)
	}
}

func requireZeroed(t *testing.T, backing []byte, grant Grant, connection Connection) {
	t.Helper()
	if strings.Trim(string(backing), "\x00") != "" || grant.AccessToken.IsSet() || grant.RefreshToken.IsSet() || connection.AccessToken.IsSet() || connection.ApplePassword.IsSet() {
		t.Fatalf("after Close backing = %q grant set %t connection set %t, want zeroed and unset", backing, grant.AccessToken.IsSet(), connection.AccessToken.IsSet() || connection.ApplePassword.IsSet())
	}
}

// TestVisitorOneShotLifecycleNative proves the owner's one-time contract:
// an online visitor grant with the minimal event-write scope matches and
// confirms, is revoked where the provider documents revocation, and leaves
// no credential bytes behind on every exit path.
func TestVisitorOneShotLifecycleNative(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		insert    int
		exit      string
		wantPaths []string
		wantErr   error
	}{
		{name: "confirmed appointment then revoke and close", insert: 200, exit: "confirm", wantPaths: []string{"/token", "/calendar/v3/freeBusy", "/calendar/v3/calendars/primary/events", "/revoke"}},
		{name: "abandoned after consent is revoked without calendar access", insert: 200, exit: "abandon", wantPaths: []string{"/token", "/revoke"}},
		{name: "failed confirmation write still revokes and closes", insert: 503, exit: "confirm", wantPaths: []string{"/token", "/calendar/v3/freeBusy", "/calendar/v3/calendars/primary/events", "/revoke"}, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "cancelled matching still revokes and closes", insert: 200, exit: "cancel", wantPaths: []string{"/token", "/revoke"}, wantErr: core.ErrTimeMatchContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, oneShotGoogle(tc.insert))
			grant, err := client.ExchangeCode(t.Context(), codeExchange(t, application(t, ProviderGoogle, MicrosoftTenantInvalid), AccessManageEvents, LifetimeVisitor))
			if err != nil || grant.RefreshToken.IsSet() || grant.Lifetime != LifetimeVisitor || grant.Access != AccessManageEvents {
				t.Fatalf("ExchangeCode(visitor confirmation) = (refresh %t, %v), want online grant", grant.RefreshToken.IsSet(), err)
			}
			backing := grant.AccessToken.value
			connection := Connection{Provider: ProviderGoogle, AccessToken: grant.AccessToken}
			var stepErr error
			switch tc.exit {
			case "confirm":
				availability, availabilityErr := client.Availability(t.Context(), AvailabilityRequest{Connection: connection, Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Calendars: []CalendarID{calendarID(t, "primary")}, Policy: fixturePolicy(t)})
				if availabilityErr != nil || availability.Calendars[0].State != AvailabilityKnown {
					t.Fatalf("Availability() = (%+v, %v), want known", availability, availabilityErr)
				}
				receipt, putErr := client.PutEvent(t.Context(), eventWrite(t, connection, intent(t, "primary", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")))
				if text := fmt.Sprintf("%+v %+v", receipt, availability); strings.Contains(text, privateMarker) {
					t.Fatalf("SDK output %q retains provider-private event text", text)
				}
				stepErr = putErr
			case "cancel":
				cancelled, cancel := context.WithCancel(t.Context())
				cancel()
				_, stepErr = client.Availability(cancelled, AvailabilityRequest{Connection: connection, Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Calendars: []CalendarID{calendarID(t, "primary")}, Policy: fixturePolicy(t)})
			}
			if tc.wantErr == nil && stepErr != nil || tc.wantErr != nil && !errors.Is(stepErr, tc.wantErr) {
				t.Fatalf("visitor step error = %v, want %v", stepErr, tc.wantErr)
			}
			outcome, revokeErr := client.Revoke(t.Context(), RevokeRequest{Provider: ProviderGoogle, Token: connection.AccessToken, Policy: fixturePolicy(t)})
			closeErr := errors.Join(connection.Close(), grant.Close())
			if revokeErr != nil || outcome != RevokedAtProvider || closeErr != nil {
				t.Fatalf("Revoke/Close = (%v, %v, %v), want revoked and closed", outcome, revokeErr, closeErr)
			}
			requirePaths(t, fixture, tc.wantPaths...)
			if last := fixture.record()[len(fixture.record())-1]; string(last.body) != "token=ya29.oneshot" {
				t.Fatalf("revocation body = %q, want the visitor access token", last.body)
			}
			requireZeroed(t, backing, grant, connection)
		})
	}
	t.Run("microsoft confirmation reports no programmatic revocation and closes", func(t *testing.T) {
		t.Parallel()
		client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
			if r.URL.Path == "/common/oauth2/v2.0/token" {
				writeJSON(w, 200, tokenBody("eyJ.oneshot", "", microsoftScopeReadWrite, "Bearer", "3600"))
				return
			}
			writeJSON(w, 201, strings.TrimSuffix(string(body), "}")+`,"id":"AAMk-visit","bodyPreview":"`+privateMarker+`"}`)
		})
		grant, err := client.ExchangeCode(t.Context(), codeExchange(t, application(t, ProviderMicrosoft, MicrosoftTenantCommon), AccessManageEvents, LifetimeVisitor))
		if err != nil || grant.RefreshToken.IsSet() {
			t.Fatalf("ExchangeCode(microsoft visitor) = (refresh %t, %v), want online grant", grant.RefreshToken.IsSet(), err)
		}
		form := fixture.record()[0].body
		if strings.Contains(string(form), "offline_access") {
			t.Fatalf("visitor token request %q asks for offline access", form)
		}
		backing := grant.AccessToken.value
		connection := Connection{Provider: ProviderMicrosoft, AccessToken: grant.AccessToken}
		receipt, putErr := client.PutEvent(t.Context(), eventWrite(t, connection, intent(t, "cal-1", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")))
		outcome, revokeErr := client.Revoke(t.Context(), RevokeRequest{Provider: ProviderMicrosoft, Token: connection.AccessToken, Policy: fixturePolicy(t)})
		closeErr := errors.Join(connection.Close(), grant.Close())
		if putErr != nil || receipt.Ref.Resource != "AAMk-visit" || revokeErr != nil || outcome != RevokeProviderHasNoRevocation || closeErr != nil {
			t.Fatalf("microsoft one-shot = (%+v, %v, %v, %v, %v), want receipt, no revocation door, closed", receipt, putErr, outcome, revokeErr, closeErr)
		}
		requirePaths(t, fixture, "/common/oauth2/v2.0/token", "/v1.0/me/calendars/cal-1/events")
		requireZeroed(t, backing, grant, connection)
	})
	t.Run("apple confirmation reports no programmatic revocation and destroys the password", func(t *testing.T) {
		t.Parallel()
		client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { w.WriteHeader(201) })
		var ingress BrowserAppleConnection
		if err := ingress.UnmarshalJSON([]byte(`{"account":"visitor@icloud.com","password":"abcd-efgh-ijkl-mnop"}`)); err != nil {
			t.Fatalf("BrowserAppleConnection ingress error = %v, want nil", err)
		}
		connection, err := ingress.Connection()
		if err != nil {
			t.Fatalf("Connection() error = %v, want nil", err)
		}
		backing := connection.ApplePassword.value
		_, putErr := client.PutEvent(t.Context(), eventWrite(t, connection, intent(t, "https://p42-caldav.icloud.com/1/calendars/home/", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")))
		outcome, revokeErr := client.Revoke(t.Context(), RevokeRequest{Provider: ProviderApple, Token: connection.ApplePassword, Policy: fixturePolicy(t)})
		closeErr := errors.Join(connection.Close(), ingress.Close())
		if putErr != nil || revokeErr != nil || outcome != RevokeProviderHasNoRevocation || closeErr != nil || ingress.Password.IsSet() {
			t.Fatalf("apple one-shot = (%v, %v, %v, %v), want written, no revocation door, closed", putErr, outcome, revokeErr, closeErr)
		}
		if len(fixture.record()) != 1 {
			t.Fatalf("apple requests = %d, want only the confirmation PUT", len(fixture.record()))
		}
		requireZeroed(t, backing, Grant{}, connection)
	})
}

// The SDK retains nothing between calls: a Client is only its Exchange
// transport, so no credential, availability or event detail can outlive a call.
func TestClientRetainsNoCalendarState(t *testing.T) {
	t.Parallel()
	client := reflect.TypeFor[Client]()
	if client.NumField() != 1 || client.Field(0).Type != reflect.TypeFor[exchange.Client]() {
		t.Fatalf("Client fields = %d (first %v), want exactly one exchange.Client", client.NumField(), client.Field(0).Type)
	}
}
