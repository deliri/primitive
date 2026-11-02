package timematch

import (
	json "encoding/json/v2"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// Official Graph event-update: omitted properties retain their prior values.
// The native provider double applies exactly the fields PATCH actually sends.
func TestMicrosoftPatchPreservesUserCalendarPreferences(t *testing.T) {
	t.Parallel()
	owned := intent(t, "primary", "Rescheduled visit", "2026-11-02T17:00:00Z", "2026-11-02T18:00:00Z")
	var mutex sync.Mutex
	showAs, reminder := "oof", true
	patches := 0
	client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		mutex.Lock()
		defer mutex.Unlock()
		if r.Method == http.MethodGet {
			writeJSON(w, 200, string(seedJSON(t, struct {
				graphOwnedMetadata
				ETag string `json:"@odata.etag"`
			}{graphOwnedMetadata: microsoftMetadata(immutableFixtureID, owned.ID.value), ETag: `W/"observed-revision"`})))
			return
		}
		if r.Method != http.MethodPatch {
			t.Errorf("mutation method%s, want PATCH", r.Method)
			w.WriteHeader(500)
			return
		}
		var patch struct {
			ShowAs   *string `json:"showAs"`
			Reminder *bool   `json:"isReminderOn"`
		}
		if err := json.Unmarshal(body, &patch); err != nil {
			t.Errorf("PATCH decode%v, want typed document", err)
			w.WriteHeader(500)
			return
		}
		if patch.ShowAs != nil {
			showAs = *patch.ShowAs
		}
		if patch.Reminder != nil {
			reminder = *patch.Reminder
		}
		patches++
		writeJSON(w, 200, string(seedJSON(t, graphOwnedDetail(t, owned))))
	})
	got, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: oauthConnection(t, ProviderMicrosoft), Ref: EventRef{ID: owned.ID, Provider: ProviderMicrosoft, Calendar: owned.Calendar, Resource: immutableFixtureID}, Intent: owned, ObservedAt: at(t, "2026-11-02T08:00:00Z"), Policy: fixturePolicy(t)})
	mutex.Lock()
	defer mutex.Unlock()
	if err != nil || got.Ref.ID != owned.ID || patches != 1 || showAs != "oof" || !reminder || len(fixture.record()) != 2 {
		t.Fatalf("PATCH receipt%+v/error%v/effects%d/showAs%s/reminder%t/HTTP%d, want new owned interval while preserving user preferences", got, err, patches, showAs, reminder, len(fixture.record()))
	}
}

func TestMicrosoftBoundedUTCBlocksDoNotBlankCalendar(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, status, zone string
		allDay, cancelled  bool
		wantKnown          bool
		wantCount          int
	}{
		{name: "UTC all-day busy is bounded", status: "busy", zone: "UTC", allDay: true, wantKnown: true, wantCount: 1},
		{name: "UTC all-day tentative is bounded", status: "tentative", zone: "UTC", allDay: true, wantKnown: true, wantCount: 1},
		{name: "UTC all-day out of office is bounded", status: "oof", zone: "UTC", allDay: true, wantKnown: true, wantCount: 1},
		{name: "documented unknown status blocks its known UTC interval", status: "unknown", zone: "UTC", wantKnown: true, wantCount: 1},
		{name: "future status conservatively blocks known UTC interval", status: "future", zone: "UTC", wantKnown: true, wantCount: 1},
		{name: "missing status conservatively blocks known UTC interval", zone: "UTC", wantKnown: true, wantCount: 1},
		{name: "cancelled unknown event never blocks", status: "unknown", zone: "UTC", cancelled: true, wantKnown: true},
		{name: "free all-day stays free", status: "free", zone: "UTC", allDay: true, wantKnown: true},
		{name: "unresolved all-day timezone remains unknown", status: "busy", zone: "Pacific Standard Time", allDay: true},
		{name: "unknown status without absolute timezone remains unknown", status: "unknown", zone: "Pacific Standard Time"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			event := seedGraphEvent{Start: graphDateTime{DateTime: "2026-11-02T00:00:00.0000000", TimeZone: row.zone}, End: graphDateTime{DateTime: "2026-11-03T00:00:00.0000000", TimeZone: row.zone}, ShowAs: row.status, IsAllDay: row.allDay, IsCancelled: row.cancelled}
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
				writeJSON(w, 200, string(seedJSON(t, seedGraphView{Value: []seedGraphEvent{event}})))
			})
			extent := window(t, "2026-11-01T00:00:00Z", "2026-11-04T00:00:00Z")
			got, err := client.Availability(t.Context(), AvailabilityRequest{Connection: oauthConnection(t, ProviderMicrosoft), Calendars: []CalendarID{calendarID(t, PrimaryCalendar)}, Window: extent, Policy: fixturePolicy(t)})
			wantState := AvailabilityUnknown
			if row.wantKnown {
				wantState = AvailabilityKnown
			}
			if err != nil || len(got.Calendars) != 1 || got.Calendars[0].State != wantState || len(got.Calendars[0].Busy) != row.wantCount || len(fixture.record()) != 1 {
				t.Fatalf("bounded UTC projection%+v/error%v/HTTP%d, want state%v/busycount%d", got, err, len(fixture.record()), wantState, row.wantCount)
			}
			if row.wantCount == 1 && got.Calendars[0].Busy[0] != window(t, "2026-11-02T00:00:00Z", "2026-11-03T00:00:00Z") {
				t.Fatalf("bounded busy%+v, want exact source interval and free surrounding days", got.Calendars[0].Busy)
			}
		})
	}
}

func TestMicrosoftRecoveryUsesSelectedCalendarCollection(t *testing.T) {
	t.Parallel()
	owned := intent(t, "secondary/calendar+opaque==", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")
	path := graphCalendarPath(owned.Calendar) + "/events"
	client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if r.Method != http.MethodGet {
			t.Errorf("recovery method%s, want read-only GET", r.Method)
			w.WriteHeader(500)
			return
		}
		switch {
		case r.URL.EscapedPath() == path:
			writeJSON(w, 200, graphCandidateCollection(t, graphOwnedPage{Value: []graphOwnedMetadata{microsoftMetadata(immutableFixtureID, owned.ID.value)}}))
		case r.URL.Path == graphEventsPath+"/"+immutableFixtureID && r.URL.Query().Has("$expand"):
			writeJSON(w, 200, graphEncoded(t, microsoftMetadata(immutableFixtureID, owned.ID.value)))
		case r.URL.Path == graphEventsPath+"/"+immutableFixtureID:
			writeJSON(w, 200, string(seedJSON(t, graphOwnedDetail(t, owned))))
		default:
			writeJSON(w, 200, graphCandidateCollection(t, graphOwnedPage{Value: []graphOwnedMetadata{}}))
		}
	})
	request := eventWrite(t, oauthConnection(t, ProviderMicrosoft), owned)
	request.Attempt = EventCreateRecoverOnly
	got, err := client.PutEvent(t.Context(), request)
	records := fixture.record()
	if err != nil || got.Ref.Calendar != owned.Calendar || got.Ref.ID != owned.ID || got.Ref.Resource != immutableFixtureID || len(records) != 3 || records[0].path != path {
		t.Fatalf("selected recovery%+v/error%v/HTTP%d, want exact selected-calendar receipt and three reads", got, err, len(records))
	}
}

func TestMicrosoftEscapedCalendarPagingKeepsTimingProjection(t *testing.T) {
	t.Parallel()
	calendar := calendarID(t, "calendar/escaped+opaque==")
	client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		event := seedGraphEvent{Start: graphDateTime{DateTime: "2026-11-02T10:00:00.0000000", TimeZone: "UTC"}, End: graphDateTime{DateTime: "2026-11-02T11:00:00.0000000", TimeZone: "UTC"}, ShowAs: "busy"}
		page := seedGraphView{Value: []seedGraphEvent{event}}
		if !r.URL.Query().Has("$skiptoken") {
			query := r.URL.Query()
			query.Set("$skiptoken", "opaque/+==")
			page.NextLink = "https://" + graphHost + r.URL.EscapedPath() + "?" + query.Encode()
		} else {
			page.Value[0].Start.DateTime = "2026-11-02T13:00:00.0000000"
			page.Value[0].End.DateTime = "2026-11-02T14:00:00.0000000"
		}
		writeJSON(w, 200, string(seedJSON(t, page)))
	})
	got, err := client.Availability(t.Context(), AvailabilityRequest{Connection: oauthConnection(t, ProviderMicrosoft), Calendars: []CalendarID{calendar}, Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Policy: fixturePolicy(t)})
	records := fixture.record()
	path := "/v1.0/me/calendars/" + url.PathEscape(calendar.value) + "/calendarView"
	if err != nil || len(got.Calendars) != 1 || got.Calendars[0].State != AvailabilityKnown || len(got.Calendars[0].Busy) != 2 || len(records) != 2 {
		t.Fatalf("escaped paging%+v/error%v/HTTP%d, want both exact timing pages", got, err, len(records))
	}
	for _, record := range records {
		query, parseErr := url.ParseQuery(record.query)
		if parseErr != nil || record.path != path || query.Get("$select") != "start,end,showAs,isCancelled,isAllDay" || strings.Contains(query.Get("$select"), "subject") {
			t.Fatalf("escaped page path%s/query%s/error%v, want unchanged selected calendar and timing-only projection", record.path, record.query, parseErr)
		}
	}
}
