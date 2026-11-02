package timematch

import (
	"context"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestGoogleCalendarListNativePaging(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		pages     []string
		status    int
		wantErr   error
		wantIDs   []string
		wantWrite []bool
	}{
		{name: "two pages concatenate with owner-only write", pages: []string{
			`{"items":[{"id":"primary@example.com","accessRole":"owner","primary":true,"summary":"` + privateMarker + `"}],"nextPageToken":"page-2"}`,
			`{"items":[{"id":"team@group.calendar.google.com","accessRole":"writer"},{"id":"holidays","accessRole":"freeBusyReader"}],"nextSyncToken":"sync"}`,
		}, wantIDs: []string{"primary@example.com", "team@group.calendar.google.com", "holidays"}, wantWrite: []bool{true, false, false}},
		{name: "empty account is an empty complete list", pages: []string{`{"items":[]}`}, wantIDs: []string{}},
		{name: "repeated page token is a provider loop", pages: []string{`{"items":[],"nextPageToken":"same"}`, `{"items":[],"nextPageToken":"same"}`}, wantErr: core.ErrTimeMatchResponse},
		{name: "duplicate calendar across pages is refused", pages: []string{`{"items":[{"id":"a","accessRole":"owner"}],"nextPageToken":"p2"}`, `{"items":[{"id":"a","accessRole":"owner"}]}`}, wantErr: core.ErrTimeMatchResponse},
		{name: "unknown access role is refused", pages: []string{`{"items":[{"id":"a","accessRole":"admin"}]}`}, wantErr: core.ErrTimeMatchResponse},
		{name: "calendar id with a space is refused", pages: []string{`{"items":[{"id":"a b","accessRole":"owner"}]}`}, wantErr: core.ErrTimeMatchResponse},
		{name: "unauthorized is consent loss", status: 401, pages: []string{`{"error":{"message":"` + privateMarker + `"}}`}, wantErr: core.ErrTimeMatchConsent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				index := 0
				if r.URL.Query().Get("pageToken") != "" {
					index = 1
				}
				status := 200
				if tc.status != 0 {
					status = tc.status
				}
				writeJSON(w, status, tc.pages[min(index, len(tc.pages)-1)])
			})
			got, err := client.ListCalendars(t.Context(), CalendarListRequest{Connection: oauthConnection(t, ProviderGoogle), Policy: fixturePolicy(t)})
			noLeak(t, err, "access-token-fixture")
			requests := fixture.record()
			if requests[0].header.Get("Authorization") != "Bearer access-token-fixture" || strings.Contains(requests[0].query, "access-token") || requests[0].query == "" ||
				requests[0].path != "/calendar/v3/users/me/calendarList" || !strings.Contains(requests[0].query, "maxResults=250") {
				t.Fatalf("first request = %s?%s auth %q, want bearer header and maxResults=250", requests[0].path, requests[0].query, requests[0].header.Get("Authorization"))
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || len(got.Calendars) != 0 {
					t.Fatalf("ListCalendars() = (%d calendars, %v), want none and %v", len(got.Calendars), err, tc.wantErr)
				}
				return
			}
			if err != nil || len(got.Calendars) != len(tc.wantIDs) {
				t.Fatalf("ListCalendars() = (%d, %v), want %d and nil", len(got.Calendars), err, len(tc.wantIDs))
			}
			for i, calendar := range got.Calendars {
				if calendar.ID.value != tc.wantIDs[i] || calendar.Writable != tc.wantWrite[i] || calendar.Primary != (i == 0 && len(tc.pages) == 2) {
					t.Fatalf("calendar[%d] = %+v, want id %q writable %t", i, calendar, tc.wantIDs[i], tc.wantWrite[i])
				}
			}
			if len(tc.pages) == 2 && (len(requests) != 2 || !strings.Contains(requests[1].query, "pageToken=page-2")) {
				t.Fatalf("requests = %d second query %q, want pageToken=page-2", len(requests), requests[len(requests)-1].query)
			}
		})
	}
}

type freeBusyRequestWire struct {
	TimeMin string `json:"timeMin"`
	TimeMax string `json:"timeMax"`
	Items   []struct {
		ID string `json:"id"`
	} `json:"items"`
}

func TestGoogleAvailabilityNative(t *testing.T) {
	t.Parallel()
	t.Run("per-calendar errors and absence are unknown while busy is merged and clipped", func(t *testing.T) {
		t.Parallel()
		client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			writeJSON(w, 200, `{"kind":"calendar#freeBusy","timeMin":"2026-11-02T09:00:00.000Z","timeMax":"2026-11-02T17:00:00.000Z","calendars":{
				"a":{"busy":[{"start":"2026-11-02T08:00:00Z","end":"2026-11-02T09:30:00Z"},{"start":"2026-11-02T10:00:00Z","end":"2026-11-02T11:00:00Z"},{"start":"2026-11-02T10:30:00-05:00","end":"2026-11-02T16:00:00Z"},{"start":"2026-11-02T11:00:00Z","end":"2026-11-02T12:00:00Z"}]},
				"b":{"errors":[{"domain":"global","reason":"notFound"}],"busy":[]},
				"d":{"busy":[{"start":"2026-11-02T12:00:00Z","end":"2026-11-02T11:00:00Z"}]}}}`)
		})
		request := AvailabilityRequest{Connection: oauthConnection(t, ProviderGoogle), Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Calendars: []CalendarID{calendarID(t, "a"), calendarID(t, "b"), calendarID(t, "c"), calendarID(t, "d")}, Policy: fixturePolicy(t)}
		got, err := client.Availability(t.Context(), request)
		if err != nil {
			t.Fatalf("Availability() error = %v, want nil", err)
		}
		wantA := []string{"2026-11-02T09:00:00Z/2026-11-02T09:30:00Z", "2026-11-02T10:00:00Z/2026-11-02T12:00:00Z", "2026-11-02T15:30:00Z/2026-11-02T16:00:00Z"}
		if spans := spanText(t, got.Calendars[0].Busy); got.Calendars[0].State != AvailabilityKnown || strings.Join(spans, ",") != strings.Join(wantA, ",") {
			t.Fatalf("calendar a = %v %q, want known %q", got.Calendars[0].State, spans, wantA)
		}
		for i := 1; i < 4; i++ {
			if got.Calendars[i].State != AvailabilityUnknown || len(got.Calendars[i].Busy) != 0 {
				t.Fatalf("calendar %s = %v with %d busy, want unknown and none", got.Calendars[i].ID, got.Calendars[i].State, len(got.Calendars[i].Busy))
			}
		}
		requests := fixture.record()
		var sent freeBusyRequestWire
		if err := json.Unmarshal(requests[0].body, &sent); err != nil || requests[0].path != "/calendar/v3/freeBusy" || sent.TimeMin != "2026-11-02T09:00:00Z" || sent.TimeMax != "2026-11-02T17:00:00Z" || len(sent.Items) != 4 || sent.Items[2].ID != "c" {
			t.Fatalf("freeBusy request = %s %s (%v), want ordered four-item window body", requests[0].path, requests[0].body, err)
		}
	})
	t.Run("fifty-one calendars split at the documented fifty expansion maximum", func(t *testing.T) {
		t.Parallel()
		client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, body []byte) {
			var sent freeBusyRequestWire
			_ = json.Unmarshal(body, &sent)
			parts := make([]string, 0, len(sent.Items))
			for _, item := range sent.Items {
				parts = append(parts, `"`+item.ID+`":{"busy":[]}`)
			}
			writeJSON(w, 200, `{"timeMin":"`+sent.TimeMin+`","timeMax":"`+sent.TimeMax+`","calendars":{`+strings.Join(parts, ",")+`}}`)
		})
		ids := make([]CalendarID, 0, 51)
		for i := range 51 {
			ids = append(ids, calendarID(t, "calendar-"+strconv.Itoa(i)))
		}
		got, err := client.Availability(t.Context(), AvailabilityRequest{Connection: oauthConnection(t, ProviderGoogle), Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Calendars: ids, Policy: fixturePolicy(t)})
		requests := fixture.record()
		var first, second freeBusyRequestWire
		_ = json.Unmarshal(requests[0].body, &first)
		_ = json.Unmarshal(requests[len(requests)-1].body, &second)
		if err != nil || len(requests) != 2 || len(first.Items) != 50 || len(second.Items) != 1 || len(got.Calendars) != 51 || got.Calendars[50].ID.value != "calendar-50" || got.Calendars[50].State != AvailabilityKnown {
			t.Fatalf("Availability(51) = (%d, %v) in %d requests, want 50+1 batches and 51 known", len(got.Calendars), err, len(requests))
		}
	})
	t.Run("response for another window is refused", func(t *testing.T) {
		t.Parallel()
		client, _ := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			writeJSON(w, 200, `{"timeMin":"2026-11-02T09:00:00Z","timeMax":"2026-11-02T16:59:59Z","calendars":{"a":{"busy":[]}}}`)
		})
		got, err := client.Availability(t.Context(), AvailabilityRequest{Connection: oauthConnection(t, ProviderGoogle), Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Calendars: []CalendarID{calendarID(t, "a")}, Policy: fixturePolicy(t)})
		if !errors.Is(err, core.ErrTimeMatchResponse) || len(got.Calendars) != 0 {
			t.Fatalf("Availability(foreign window) = (%d, %v), want none and %v", len(got.Calendars), err, core.ErrTimeMatchResponse)
		}
	})
	t.Run("forbidden is consent and never free", func(t *testing.T) {
		t.Parallel()
		client, _ := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			writeJSON(w, 403, `{"error":{"message":"`+privateMarker+`"}}`)
		})
		got, err := client.Availability(t.Context(), AvailabilityRequest{Connection: oauthConnection(t, ProviderGoogle), Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Calendars: []CalendarID{calendarID(t, "a")}, Policy: fixturePolicy(t)})
		if !errors.Is(err, core.ErrTimeMatchConsent) || len(got.Calendars) != 0 {
			t.Fatalf("Availability(403) = (%d, %v), want none and %v", len(got.Calendars), err, core.ErrTimeMatchConsent)
		}
		noLeak(t, err, "access-token-fixture")
	})
	t.Run("caller cancellation ends the request without a result", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		client, _ := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
			cancel()
			<-r.Context().Done()
		})
		done := make(chan error, 1)
		go func() {
			_, err := client.Availability(ctx, AvailabilityRequest{Connection: oauthConnection(t, ProviderGoogle), Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Calendars: []CalendarID{calendarID(t, "a")}, Policy: fixturePolicy(t)})
			done <- err
		}()
		select {
		case err := <-done:
			if !errors.Is(err, core.ErrTimeMatchResponse) || !errors.Is(err, core.ErrExchangeCancelled) {
				t.Fatalf("Availability(cancelled) error = %v, want %v with %v", err, core.ErrTimeMatchResponse, core.ErrExchangeCancelled)
			}
		case <-time.After(fixtureBackstop):
			t.Fatalf("Availability(cancelled) still running after %v, want a cancelled return", fixtureBackstop)
		}
	})
}

func spanText(t *testing.T, spans []temporal.IntervalBounds) []string {
	t.Helper()
	out := make([]string, 0, len(spans))
	for _, span := range spans {
		start, startErr := span.Start.RFC3339()
		end, endErr := span.End.RFC3339()
		if err := errors.Join(startErr, endErr); err != nil {
			t.Fatalf("span RFC3339 error = %v, want nil", err)
		}
		out = append(out, start+"/"+end)
	}
	return out
}

func graphView(events ...string) string {
	return `{"@odata.context":"x","value":[` + strings.Join(events, ",") + `]}`
}

func graphItem(showAs, start, end string, extra string) string {
	return `{"subject":"` + privateMarker + `","showAs":"` + showAs + `","isCancelled":false,"isAllDay":false,"start":{"dateTime":"` + start + `","timeZone":"UTC"},"end":{"dateTime":"` + end + `","timeZone":"UTC"}` + extra + `}`
}

func TestMicrosoftAvailabilityNative(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		pages     []string
		status    int
		wantState AvailabilityState
		wantBusy  []string
		wantErr   error
	}{
		{name: "busy tentative and oof block while free and working elsewhere do not", pages: []string{graphView(
			graphItem("busy", "2026-11-02T10:00:00.0000000", "2026-11-02T11:00:00.0000000", ""),
			graphItem("free", "2026-11-02T11:00:00.0000000", "2026-11-02T12:00:00.0000000", ""),
			graphItem("workingElsewhere", "2026-11-02T12:00:00.0000000", "2026-11-02T13:00:00.0000000", ""),
			graphItem("tentative", "2026-11-02T13:00:00.0000000", "2026-11-02T13:30:00.0000000", ""),
			graphItem("oof", "2026-11-02T14:00:00.0000000", "2026-11-02T15:00:00.0000000", ""))},
			wantState: AvailabilityKnown, wantBusy: []string{"2026-11-02T10:00:00Z/2026-11-02T11:00:00Z", "2026-11-02T13:00:00Z/2026-11-02T13:30:00Z", "2026-11-02T14:00:00Z/2026-11-02T15:00:00Z"}},
		{name: "cancelled busy occurrence does not block", pages: []string{graphView(strings.Replace(graphItem("busy", "2026-11-02T10:00:00.0000000", "2026-11-02T11:00:00.0000000", ""), `"isCancelled":false`, `"isCancelled":true`, 1))}, wantState: AvailabilityKnown, wantBusy: []string{}},
		{name: "showAs unknown cannot be classified", pages: []string{graphView(graphItem("unknown", "2026-11-02T10:00:00.0000000", "2026-11-02T11:00:00.0000000", ""))}, wantState: AvailabilityUnknown},
		{name: "future showAs value cannot be classified", pages: []string{graphView(graphItem("away", "2026-11-02T10:00:00.0000000", "2026-11-02T11:00:00.0000000", ""))}, wantState: AvailabilityUnknown},
		{name: "blocking all-day event has no absolute instants", pages: []string{graphView(strings.Replace(graphItem("busy", "2026-11-02T00:00:00.0000000", "2026-11-03T00:00:00.0000000", ""), `"isAllDay":false`, `"isAllDay":true`, 1))}, wantState: AvailabilityUnknown},
		{name: "free all-day event is not busy", pages: []string{graphView(strings.Replace(graphItem("free", "2026-11-02T00:00:00.0000000", "2026-11-03T00:00:00.0000000", ""), `"isAllDay":false`, `"isAllDay":true`, 1))}, wantState: AvailabilityKnown, wantBusy: []string{}},
		{name: "non-UTC zone despite Prefer is unknown", pages: []string{graphView(strings.ReplaceAll(graphItem("busy", "2026-11-02T10:00:00.0000000", "2026-11-02T11:00:00.0000000", ""), `"UTC"`, `"Pacific Standard Time"`))}, wantState: AvailabilityUnknown},
		{name: "two pages through the same calendarView path are complete", pages: []string{
			`{"value":[` + graphItem("busy", "2026-11-02T10:00:00.0000000", "2026-11-02T11:00:00.0000000", "") + `],"@odata.nextLink":"https://graph.microsoft.com/v1.0/me/calendars/cal-1/calendarView?$skip=1"}`,
			graphView(graphItem("busy", "2026-11-02T11:00:00.0000000", "2026-11-02T12:00:00.0000000", "")),
		}, wantState: AvailabilityKnown, wantBusy: []string{"2026-11-02T10:00:00Z/2026-11-02T12:00:00Z"}},
		{name: "next link to a foreign host is never followed", pages: []string{`{"value":[],"@odata.nextLink":"https://attacker.example/v1.0/me/calendars/cal-1/calendarView"}`}, wantState: AvailabilityUnknown},
		{name: "next link to another graph path is never followed", pages: []string{`{"value":[],"@odata.nextLink":"https://graph.microsoft.com/v1.0/me/messages"}`}, wantState: AvailabilityUnknown},
		{name: "server error for the calendar is unknown", status: 500, pages: []string{`{"error":{"message":"` + privateMarker + `"}}`}, wantState: AvailabilityUnknown},
		{name: "unauthorized is consent loss for the whole request", status: 401, pages: []string{`{}`}, wantErr: core.ErrTimeMatchConsent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				index := 0
				if r.URL.Query().Has("$skip") {
					index = 1
				}
				status := 200
				if tc.status != 0 {
					status = tc.status
				}
				writeJSON(w, status, tc.pages[index])
			})
			got, err := client.Availability(t.Context(), AvailabilityRequest{Connection: oauthConnection(t, ProviderMicrosoft), Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Calendars: []CalendarID{calendarID(t, "cal-1")}, Policy: fixturePolicy(t)})
			noLeak(t, err, "access-token-fixture")
			requests := fixture.record()
			query, _ := url.ParseQuery(requests[0].query)
			if requests[0].path != "/v1.0/me/calendars/cal-1/calendarView" || requests[0].header.Get("Prefer") != `outlook.timezone="UTC"` || query.Get("startDateTime") != "2026-11-02T09:00:00Z" ||
				query.Get("endDateTime") != "2026-11-02T17:00:00Z" || query.Get("$select") != "start,end,showAs,isCancelled,isAllDay" || len(fixture.refusedDials()) != 0 {
				t.Fatalf("calendarView request = %s?%s prefer %q refused %q, want documented UTC projection with no foreign dial", requests[0].path, requests[0].query, requests[0].header.Get("Prefer"), fixture.refusedDials())
			}
			for _, request := range requests {
				if request.host != "graph.microsoft.com" {
					t.Fatalf("request host = %q, want only graph.microsoft.com", request.host)
				}
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || len(got.Calendars) != 0 {
					t.Fatalf("Availability() = (%d, %v), want none and %v", len(got.Calendars), err, tc.wantErr)
				}
				return
			}
			if err != nil || got.Calendars[0].State != tc.wantState || strings.Join(spanText(t, got.Calendars[0].Busy), ",") != strings.Join(tc.wantBusy, ",") {
				t.Fatalf("Availability() = (%v %q, %v), want %v %q", got.Calendars[0].State, spanText(t, got.Calendars[0].Busy), err, tc.wantState, tc.wantBusy)
			}
		})
	}
}

func multistatus(responses ...string) string {
	return `<?xml version="1.0" encoding="UTF-8"?><d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">` + strings.Join(responses, "") + `</d:multistatus>`
}

func davOK(href, props string) string {
	return `<d:response><d:href>` + href + `</d:href><d:propstat><d:prop>` + props + `</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`
}

func writeXML(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/xml; charset=UTF-8")
	w.WriteHeader(207)
	_, _ = io.WriteString(w, body)
}

func appleDiscovery(home string) func(w http.ResponseWriter, r *http.Request, body []byte) bool {
	return func(w http.ResponseWriter, r *http.Request, body []byte) bool {
		switch {
		case r.Method == "PROPFIND" && r.URL.Path == "/":
			writeXML(w, multistatus(davOK("/", `<d:current-user-principal><d:href>/123456/principal/</d:href></d:current-user-principal>`)))
		case r.Method == "PROPFIND" && r.URL.Path == "/123456/principal/":
			writeXML(w, multistatus(davOK("/123456/principal/", `<c:calendar-home-set><d:href>`+home+`</d:href></c:calendar-home-set>`)))
		default:
			return false
		}
		return true
	}
}

func TestAppleDiscoveryNative(t *testing.T) {
	t.Parallel()
	t.Run("principal home and event calendars are discovered on admitted hosts", func(t *testing.T) {
		t.Parallel()
		discover := appleDiscovery("https://p42-caldav.icloud.com:443/123456/calendars/")
		client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
			if discover(w, r, body) {
				return
			}
			writeXML(w, multistatus(
				davOK("/123456/calendars/", `<d:resourcetype><d:collection/></d:resourcetype>`),
				davOK("/123456/calendars/home/", `<d:resourcetype><d:collection/><c:calendar/></d:resourcetype><c:supported-calendar-component-set><c:comp name="VEVENT"/></c:supported-calendar-component-set><d:current-user-privilege-set><d:privilege><d:write/></d:privilege></d:current-user-privilege-set>`),
				davOK("/123456/calendars/tasks/", `<d:resourcetype><d:collection/><c:calendar/></d:resourcetype><c:supported-calendar-component-set><c:comp name="VTODO"/></c:supported-calendar-component-set>`),
				davOK("/123456/calendars/shared/", `<d:resourcetype><d:collection/><c:calendar/></d:resourcetype><d:current-user-privilege-set><d:privilege><d:read/></d:privilege></d:current-user-privilege-set>`),
			))
		})
		got, err := client.ListCalendars(t.Context(), CalendarListRequest{Connection: appleConnection(t), Policy: fixturePolicy(t)})
		if err != nil || len(got.Calendars) != 2 || got.Calendars[0].ID.value != "https://p42-caldav.icloud.com:443/123456/calendars/home/" || !got.Calendars[0].Writable || got.Calendars[1].Writable {
			t.Fatalf("ListCalendars(apple) = (%+v, %v), want home writable and shared read-only", got.Calendars, err)
		}
		wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("visitor@icloud.com:abcd-efgh-ijkl-mnop"))
		requests := fixture.record()
		wantDepth := []string{"0", "0", "1"}
		wantHost := []string{"caldav.icloud.com", "caldav.icloud.com", "p42-caldav.icloud.com:443"}
		for i, request := range requests {
			if request.method != "PROPFIND" || request.header.Get("Authorization") != wantAuth || request.header.Get("Depth") != wantDepth[i] || request.host != wantHost[i] {
				t.Fatalf("request %d = %s %s depth %q host %q, want PROPFIND depth %q host %q with basic credential", i, request.method, request.path, request.header.Get("Depth"), request.host, wantDepth[i], wantHost[i])
			}
		}
	})
	for _, row := range []struct{ name, home string }{
		{name: "home on a foreign host is refused before any request", home: "https://attacker.example/123456/calendars/"},
		{name: "home on a look-alike apple host is refused", home: "https://p42-caldav.icloud.com.attacker.example/123456/calendars/"},
		{name: "home over plain http is refused", home: "http://p42-caldav.icloud.com/123456/calendars/"},
		{name: "home on a non-numeric partition is refused", home: "https://px-caldav.icloud.com/123456/calendars/"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			discover := appleDiscovery(row.home)
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
				if !discover(w, r, body) {
					writeXML(w, multistatus())
				}
			})
			_, err := client.ListCalendars(t.Context(), CalendarListRequest{Connection: appleConnection(t), Policy: fixturePolicy(t)})
			if !errors.Is(err, core.ErrTimeMatchResponse) || len(fixture.record()) != 2 || len(fixture.refusedDials()) != 0 {
				t.Fatalf("ListCalendars(%s) = %v with %d requests, refused %q, want response refusal after two admitted requests", row.home, err, len(fixture.record()), fixture.refusedDials())
			}
			noLeak(t, err, "abcd-efgh-ijkl-mnop")
		})
	}
	t.Run("revoked app-specific password is consent loss", func(t *testing.T) {
		t.Parallel()
		client, _ := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { w.WriteHeader(http.StatusUnauthorized) })
		_, err := client.ListCalendars(t.Context(), CalendarListRequest{Connection: appleConnection(t), Policy: fixturePolicy(t)})
		if !errors.Is(err, core.ErrTimeMatchConsent) {
			t.Fatalf("ListCalendars(401) error = %v, want %v", err, core.ErrTimeMatchConsent)
		}
		noLeak(t, err, "abcd-efgh-ijkl-mnop")
	})
	t.Run("html instead of multistatus xml is refused", func(t *testing.T) {
		t.Parallel()
		client, _ := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(207)
			_, _ = io.WriteString(w, "<html></html>")
		})
		_, err := client.ListCalendars(t.Context(), CalendarListRequest{Connection: appleConnection(t), Policy: fixturePolicy(t)})
		if !errors.Is(err, core.ErrTimeMatchResponse) {
			t.Fatalf("ListCalendars(html) error = %v, want %v", err, core.ErrTimeMatchResponse)
		}
	})
}

func vcalendar(events ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Apple Inc.//iCloud//EN\r\n" + strings.Join(events, "") + "END:VCALENDAR\r\n"
}

func vevent(lines ...string) string {
	return "BEGIN:VEVENT\r\nSUMMARY:" + privateMarker + "\r\n" + strings.Join(lines, "\r\n") + "\r\nEND:VEVENT\r\n"
}

func calendarData(ics string) string {
	escaped := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(ics)
	return `<c:calendar-data>` + escaped + `</c:calendar-data>`
}

func vfreebusy(lines ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Apple Inc.//iCloud//EN\r\nBEGIN:VFREEBUSY\r\nDTSTART:20261102T090000Z\r\nDTEND:20261102T170000Z\r\n" + strings.Join(append(lines, ""), "\r\n") + "END:VFREEBUSY\r\nEND:VCALENDAR\r\n"
}

func TestAppleAvailabilityNative(t *testing.T) {
	t.Parallel()
	home := "https://p42-caldav.icloud.com/123456/calendars/home/"
	cases := []struct {
		name      string
		body      string
		media     string
		status    int
		wantState AvailabilityState
		wantBusy  []string
		wantErr   error
	}{
		{name: "busy tentative and unavailable periods block while free does not", status: 200, media: "text/calendar", body: vfreebusy(
			"FREEBUSY:20261102T100000Z/20261102T110000Z",
			"FREEBUSY;FBTYPE=BUSY-TENTATIVE:20261102T120000Z/PT30M",
			"FREEBUSY;FBTYPE=FREE:20261102T130000Z/20261102T140000Z",
			"FREEBUSY;FBTYPE=BUSY-UNAVAILABLE:20261102T150000Z/20261102T153000Z,20261102T160000Z/20261102T163000Z"),
			wantState: AvailabilityKnown, wantBusy: []string{"2026-11-02T10:00:00Z/2026-11-02T11:00:00Z", "2026-11-02T12:00:00Z/2026-11-02T12:30:00Z", "2026-11-02T15:00:00Z/2026-11-02T15:30:00Z", "2026-11-02T16:00:00Z/2026-11-02T16:30:00Z"}},
		{name: "folded period list is one property", status: 200, media: "text/calendar", body: vfreebusy("FREEBUSY:20261102T100000Z/20261102T11\r\n 0000Z"), wantState: AvailabilityKnown, wantBusy: []string{"2026-11-02T10:00:00Z/2026-11-02T11:00:00Z"}},
		{name: "empty free busy component is known and free", status: 200, media: "text/calendar", body: vfreebusy(), wantState: AvailabilityKnown, wantBusy: []string{}},
		{name: "unknown free busy type is unknown", status: 200, media: "text/calendar", body: vfreebusy("FREEBUSY;FBTYPE=X-AWAY:20261102T100000Z/20261102T110000Z"), wantState: AvailabilityUnknown},
		{name: "local time period is unknown", status: 200, media: "text/calendar", body: vfreebusy("FREEBUSY:20261102T100000/20261102T110000"), wantState: AvailabilityUnknown},
		{name: "reversed period is unknown", status: 200, media: "text/calendar", body: vfreebusy("FREEBUSY:20261102T110000Z/20261102T100000Z"), wantState: AvailabilityUnknown},
		{name: "answer carrying event data is refused as unknown", status: 200, media: "text/calendar", body: vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DTEND:20261102T110000Z")), wantState: AvailabilityUnknown},
		{name: "answer without a free busy component is unknown", status: 200, media: "text/calendar", body: vcalendar(), wantState: AvailabilityUnknown},
		{name: "multistatus instead of free busy is unknown", status: 207, media: "application/xml", body: multistatus(), wantState: AvailabilityUnknown},
		{name: "free busy text under a non-calendar media type is unknown", status: 200, media: "text/html", body: vfreebusy("FREEBUSY:20261102T100000Z/20261102T110000Z"), wantState: AvailabilityUnknown},
		{name: "server without free busy report support is unknown", status: 501, media: "text/plain", body: "not implemented", wantState: AvailabilityUnknown},
		{name: "revoked app-specific password is consent loss", status: 401, media: "text/plain", body: "", wantErr: core.ErrTimeMatchConsent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
				w.Header().Set("Content-Type", tc.media)
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})
			got, err := client.Availability(t.Context(), AvailabilityRequest{Connection: appleConnection(t), Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Calendars: []CalendarID{calendarID(t, home)}, Policy: fixturePolicy(t)})
			requests := fixture.record()
			if len(requests) != 1 {
				t.Fatalf("Availability(apple) sent %d requests (error %v), want exactly one REPORT", len(requests), err)
			}
			body := string(requests[0].body)
			if requests[0].method != "REPORT" || requests[0].header.Get("Depth") != "1" || requests[0].path != "/123456/calendars/home/" ||
				!strings.Contains(body, `<c:free-busy-query`) || !strings.Contains(body, `<c:time-range start="20261102T090000Z" end="20261102T170000Z"/>`) || strings.Contains(body, "calendar-data") {
				t.Fatalf("availability request = %s %s depth %q body %s, want a free-busy-query REPORT that never asks for calendar-data", requests[0].method, requests[0].path, requests[0].header.Get("Depth"), body)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || len(got.Calendars) != 0 {
					t.Fatalf("Availability(apple) = (%d, %v), want none and %v", len(got.Calendars), err, tc.wantErr)
				}
				return
			}
			if err != nil || got.Calendars[0].State != tc.wantState || strings.Join(spanText(t, got.Calendars[0].Busy), ",") != strings.Join(tc.wantBusy, ",") {
				t.Fatalf("Availability(apple) = (%v %q, %v), want %v %q", got.Calendars[0].State, spanText(t, got.Calendars[0].Busy), err, tc.wantState, tc.wantBusy)
			}
		})
	}
	t.Run("calendar id on a foreign host is refused before sending credentials", func(t *testing.T) {
		t.Parallel()
		client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeXML(w, multistatus()) })
		_, err := client.Availability(t.Context(), AvailabilityRequest{Connection: appleConnection(t), Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Calendars: []CalendarID{calendarID(t, "https://attacker.example/cal/")}, Policy: fixturePolicy(t)})
		if !errors.Is(err, core.ErrTimeMatchContract) || len(fixture.record()) != 0 || len(fixture.refusedDials()) != 0 {
			t.Fatalf("Availability(foreign apple id) = %v with %d requests, want contract refusal and none", err, len(fixture.record()))
		}
	})
}
