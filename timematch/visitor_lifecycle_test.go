package timematch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// oneShotGoogle serves the documented Google endpoints for one visitor: the
// first code exchange is the free/busy match grant, the second the separate
// confirmation grant. insertStatus lets a row fail the confirmation write.
func oneShotGoogle(insertStatus int) func(w http.ResponseWriter, r *http.Request, body []byte) {
	var exchanges atomic.Int64
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		switch r.URL.Path {
		case "/token":
			if exchanges.Add(1) == 1 {
				writeJSON(w, 200, tokenBody("ya29.match", "", googleVisitorScope, "Bearer", "3600"))
				return
			}
			writeJSON(w, 200, tokenBody("ya29.confirm", "", googleConfirmScope, "Bearer", "3600"))
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

// visitorGrant redeems one visitor code and returns its connection and the
// credential bytes the test later proves were zeroed.
func visitorGrant(t *testing.T, client Client, access Access) (Grant, Connection, []byte) {
	t.Helper()
	grant, err := client.ExchangeCode(t.Context(), codeExchange(t, application(t, ProviderGoogle, MicrosoftTenantInvalid), access, LifetimeVisitor))
	if err != nil || grant.RefreshToken.IsSet() || grant.Lifetime != LifetimeVisitor || grant.Access != access {
		t.Fatalf("ExchangeCode(visitor %v) = (refresh %t, %v), want online grant", access, grant.RefreshToken.IsSet(), err)
	}
	return grant, Connection{Provider: ProviderGoogle, AccessToken: grant.AccessToken}, grant.AccessToken.value
}

// finishVisitor revokes at Google and destroys every local copy.
func finishVisitor(t *testing.T, client Client, grant Grant, connection Connection, backing []byte) {
	t.Helper()
	outcome, revokeErr := client.Revoke(t.Context(), RevokeRequest{Provider: ProviderGoogle, Token: connection.AccessToken, Policy: fixturePolicy(t)})
	closeErr := errors.Join(connection.Close(), grant.Close())
	if revokeErr != nil || outcome != RevokedAtProvider || closeErr != nil {
		t.Fatalf("Revoke/Close = (%v, %v, %v), want revoked and closed", outcome, revokeErr, closeErr)
	}
	requireZeroed(t, backing, grant, connection)
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
	matchWindow := func(t *testing.T, connection Connection) AvailabilityRequest {
		return AvailabilityRequest{Connection: connection, Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Calendars: []CalendarID{calendarID(t, PrimaryCalendar)}, Policy: fixturePolicy(t)}
	}
	t.Run("match then separately consented confirmation each revoke and close", func(t *testing.T) {
		t.Parallel()
		client, fixture := newNative(t, oneShotGoogle(200))
		match, matchConnection, matchBacking := visitorGrant(t, client, AccessAvailability)
		availability, err := client.Availability(t.Context(), matchWindow(t, matchConnection))
		if err != nil || availability.Calendars[0].State != AvailabilityKnown {
			t.Fatalf("Availability() = (%+v, %v), want known", availability, err)
		}
		finishVisitor(t, client, match, matchConnection, matchBacking)
		confirm, confirmConnection, confirmBacking := visitorGrant(t, client, AccessManageEvents)
		receipt, err := client.PutEvent(t.Context(), eventWrite(t, confirmConnection, intent(t, PrimaryCalendar, "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")))
		if text := fmt.Sprintf("%+v %+v", receipt, availability); err != nil || strings.Contains(text, privateMarker) {
			t.Fatalf("PutEvent() = (%s, %v), want a receipt without provider-private text", text, err)
		}
		finishVisitor(t, client, confirm, confirmConnection, confirmBacking)
		requirePaths(t, fixture, "/token", "/calendar/v3/freeBusy", "/revoke", "/token", "/calendar/v3/calendars/primary/events", "/revoke")
		requests := fixture.record()
		if string(requests[2].body) != "token=ya29.match" || string(requests[5].body) != "token=ya29.confirm" {
			t.Fatalf("revocations = %q, %q, want each grant revoked", requests[2].body, requests[5].body)
		}
	})
	t.Run("abandoned after consent is revoked without calendar access", func(t *testing.T) {
		t.Parallel()
		client, fixture := newNative(t, oneShotGoogle(200))
		grant, connection, backing := visitorGrant(t, client, AccessAvailability)
		finishVisitor(t, client, grant, connection, backing)
		requirePaths(t, fixture, "/token", "/revoke")
	})
	t.Run("failed confirmation write still revokes and closes", func(t *testing.T) {
		t.Parallel()
		client, fixture := newNative(t, oneShotGoogle(503))
		_, _, _ = visitorGrant(t, client, AccessAvailability)
		grant, connection, backing := visitorGrant(t, client, AccessManageEvents)
		_, err := client.PutEvent(t.Context(), eventWrite(t, connection, intent(t, PrimaryCalendar, "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")))
		if !errors.Is(err, core.ErrTimeMatchIndeterminate) {
			t.Fatalf("PutEvent(503) error = %v, want %v", err, core.ErrTimeMatchIndeterminate)
		}
		finishVisitor(t, client, grant, connection, backing)
		requirePaths(t, fixture, "/token", "/token", "/calendar/v3/calendars/primary/events", "/revoke")
	})
	t.Run("cancelled matching still revokes and closes", func(t *testing.T) {
		t.Parallel()
		client, fixture := newNative(t, oneShotGoogle(200))
		grant, connection, backing := visitorGrant(t, client, AccessAvailability)
		cancelled, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := client.Availability(cancelled, matchWindow(t, connection)); !errors.Is(err, core.ErrTimeMatchContract) {
			t.Fatalf("Availability(cancelled) error = %v, want %v", err, core.ErrTimeMatchContract)
		}
		finishVisitor(t, client, grant, connection, backing)
		requirePaths(t, fixture, "/token", "/revoke")
	})
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
		receipt, putErr := client.PutEvent(t.Context(), eventWrite(t, connection, intent(t, PrimaryCalendar, "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")))
		outcome, revokeErr := client.Revoke(t.Context(), RevokeRequest{Provider: ProviderMicrosoft, Token: connection.AccessToken, Policy: fixturePolicy(t)})
		closeErr := errors.Join(connection.Close(), grant.Close())
		if putErr != nil || receipt.Ref.Resource != "AAMk-visit" || revokeErr != nil || outcome != RevokeProviderHasNoRevocation || closeErr != nil {
			t.Fatalf("microsoft one-shot = (%+v, %v, %v, %v, %v), want receipt, no revocation door, closed", receipt, putErr, outcome, revokeErr, closeErr)
		}
		requirePaths(t, fixture, "/common/oauth2/v2.0/token", "/v1.0/me/calendar/events")
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
