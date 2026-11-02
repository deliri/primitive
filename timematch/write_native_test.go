package timematch

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	json "encoding/json/v2"

	"github.com/deliri/primitive/v2026/core"
)

const eventIDFixture = "tm0123456789abcdefghijklmnopqrstuv"

func intent(t testing.TB, calendar, summary, start, end string) EventIntent {
	t.Helper()
	id, idErr := ParseEventID(eventIDFixture)
	text, textErr := ParseEventSummary(summary)
	if err := errors.Join(idErr, textErr); err != nil {
		t.Fatalf("intent fixture error = %v, want nil", err)
	}
	return EventIntent{Calendar: calendarID(t, calendar), ID: id, Summary: text, Interval: window(t, start, end)}
}

func eventWrite(t testing.TB, connection Connection, owned EventIntent) EventWrite {
	t.Helper()
	return EventWrite{Attempt: EventCreateFirst, Connection: connection, Intent: owned, ObservedAt: at(t, "2026-11-02T08:00:00.5Z"), Policy: fixturePolicy(t)}
}

// cutConnection closes the socket after the provider applied the effect, so
// the client sent the request and observed no status.
func cutConnection(w http.ResponseWriter) {
	connection, _, err := http.NewResponseController(w).Hijack()
	if err == nil {
		_ = connection.Close()
	}
}

// googleCalendar is a stateful Google events collection for one calendar.
type googleCalendar struct {
	mutex   sync.Mutex
	events  map[string]string
	inserts int
	fail    func(method string, attempt int, w http.ResponseWriter) bool
}

func (g *googleCalendar) serve(w http.ResponseWriter, r *http.Request, body []byte) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	path := strings.TrimPrefix(r.URL.Path, "/calendar/v3/calendars/primary/events")
	if r.Method == http.MethodPost {
		g.inserts++
		var event struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(body, &event)
		_, exists := g.events[event.ID]
		if !exists {
			g.events[event.ID] = string(body)
		}
		if g.fail != nil && g.fail(r.Method, g.inserts, w) {
			return
		}
		if exists {
			writeJSON(w, http.StatusConflict, `{"error":{"code":409,"message":"The requested identifier already exists."}}`)
			return
		}
		writeJSON(w, 200, strings.TrimSuffix(string(body), "}")+`,"status":"confirmed","htmlLink":"x"}`)
		return
	}
	if r.Method == http.MethodGet && path == "" {
		items := []struct {
			ID   string `json:"id"`
			ETag string `json:"etag"`
		}{}
		for id, stored := range g.events {
			var event struct {
				googleEventBody
				Status string `json:"status"`
			}
			if err := json.Unmarshal([]byte(stored), &event); err != nil {
				writeJSON(w, 500, `{}`)
				return
			}
			if event.Status != "cancelled" && event.ExtendedProperties != nil && r.URL.Query().Get("privateExtendedProperty") == googleOwnershipProperty+"="+event.ExtendedProperties.Private.EventID {
				items = append(items, struct {
					ID   string `json:"id"`
					ETag string `json:"etag"`
				}{ID: id, ETag: googleFixtureETag(stored)})
			}
		}
		wire, err := json.Marshal(struct {
			Items []struct {
				ID   string `json:"id"`
				ETag string `json:"etag"`
			} `json:"items"`
		}{Items: items})
		if err != nil {
			writeJSON(w, 500, `{}`)
			return
		}
		writeJSON(w, 200, string(wire))
		return
	}
	id := strings.TrimPrefix(path, "/")
	stored, ok := g.events[id]
	if ok && r.Header.Get("If-Match") != "" && r.Header.Get("If-Match") != googleFixtureETag(stored) {
		writeJSON(w, 412, `{}`)
		return
	}
	switch {
	case r.Method == http.MethodGet && ok:
		writeJSON(w, 200, googleFixtureWithETag(stored))
	case r.Method == http.MethodPut && ok:
		var replacement googleEventBody
		if err := json.Unmarshal(body, &replacement); err != nil {
			writeJSON(w, 400, `{}`)
			return
		}
		replacement.ID = id
		encoded, err := json.Marshal(replacement)
		if err != nil {
			writeJSON(w, 500, `{}`)
			return
		}
		g.events[id] = string(encoded)
		writeJSON(w, 200, string(encoded))
	case r.Method == http.MethodPatch && ok:
		var current struct {
			googleEventBody
			Description string `json:"description,omitzero"`
		}
		if err := json.Unmarshal([]byte(stored), &current); err != nil {
			writeJSON(w, 500, `{}`)
			return
		}
		if err := json.Unmarshal(body, &current); err != nil {
			writeJSON(w, 400, `{}`)
			return
		}
		encoded, err := json.Marshal(current)
		if err != nil {
			writeJSON(w, 500, `{}`)
			return
		}
		g.events[id] = string(encoded)
		writeJSON(w, 200, string(encoded))
	case r.Method == http.MethodDelete && ok:
		delete(g.events, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusGone, `{"error":{"code":410}}`)
	}
}

func googleFixtureETag(stored string) string {
	return fmt.Sprintf("\"%x\"", sha256.Sum256([]byte(stored)))
}
func googleFixtureWithETag(stored string) string {
	revision := fmt.Sprintf("%q", googleFixtureETag(stored))
	return strings.TrimSuffix(stored, "}") + `,"etag":` + revision + `}`
}

func googleStored(t *testing.T, owned EventIntent, summary, status string) string {
	t.Helper()
	body, err := googleEvent(owned)
	if err != nil {
		t.Fatalf("googleEvent() error = %v, want nil", err)
	}
	text := strings.Replace(string(body), `"summary":"`+owned.Summary.value+`"`, `"summary":"`+summary+`"`, 1)
	return strings.TrimSuffix(text, "}") + `,"status":"` + status + `"}`
}

func TestGooglePutEventNative(t *testing.T) {
	t.Parallel()
	owned := func(t *testing.T) EventIntent {
		return intent(t, "primary", "Visit, Jane", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")
	}
	cases := []struct {
		name        string
		existing    func(t *testing.T) string
		fail        func(method string, attempt int, w http.ResponseWriter) bool
		wantErr     error
		wantInserts int
		wantStored  bool
	}{
		{name: "fresh deterministic id is created", wantInserts: 1, wantStored: true},
		{name: "existing identical intent is verified replay", existing: func(t *testing.T) string { return googleStored(t, owned(t), "Visit, Jane", "confirmed") }, wantInserts: 1, wantStored: true},
		{name: "existing event with another summary is a conflict", existing: func(t *testing.T) string { return googleStored(t, owned(t), "Someone else", "confirmed") }, wantErr: core.ErrTimeMatchConflict, wantInserts: 1, wantStored: true},
		{name: "cancelled event cannot establish live private ownership", existing: func(t *testing.T) string { return googleStored(t, owned(t), "Visit, Jane", "cancelled") }, wantErr: core.ErrTimeMatchIndeterminate, wantInserts: 1, wantStored: true},
		{name: "server error after applying is indeterminate", fail: func(_ string, _ int, w http.ResponseWriter) bool { writeJSON(w, 503, `{}`); return true }, wantErr: core.ErrTimeMatchIndeterminate, wantInserts: 1, wantStored: true},
		{name: "connection cut after applying is indeterminate", fail: func(_ string, _ int, w http.ResponseWriter) bool { cutConnection(w); return true }, wantErr: core.ErrTimeMatchIndeterminate, wantInserts: 1, wantStored: true},
		{name: "malformed success body is indeterminate", fail: func(_ string, _ int, w http.ResponseWriter) bool { writeJSON(w, 200, `{"id":`); return true }, wantErr: core.ErrTimeMatchIndeterminate, wantInserts: 1, wantStored: true},
		{name: "forbidden write is a refusal without lost consent", fail: func(_ string, _ int, w http.ResponseWriter) bool { writeJSON(w, 403, `{}`); return true }, wantErr: core.ErrTimeMatchResponse, wantInserts: 1, wantStored: true},
		{name: "bad request is a definitive refusal", fail: func(_ string, _ int, w http.ResponseWriter) bool { writeJSON(w, 400, `{}`); return true }, wantErr: core.ErrTimeMatchResponse, wantInserts: 1, wantStored: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calendar := &googleCalendar{events: map[string]string{}, fail: tc.fail}
			if tc.existing != nil {
				calendar.events[eventIDFixture] = tc.existing(t)
			}
			client, fixture := newNative(t, calendar.serve)
			got, err := client.PutEvent(t.Context(), eventWrite(t, oauthConnection(t, ProviderGoogle), owned(t)))
			requests := fixture.record()
			if requests[0].query != "sendUpdates=none" || strings.Contains(string(requests[0].body), "attendees") || !strings.Contains(string(requests[0].body), `"transparency":"opaque"`) {
				t.Fatalf("insert = ?%s %s, want sendUpdates=none, opaque and no attendees", requests[0].query, requests[0].body)
			}
			_, stored := calendar.events[eventIDFixture]
			if calendar.inserts != tc.wantInserts || stored != tc.wantStored {
				t.Fatalf("provider inserts = %d stored %t, want %d %t", calendar.inserts, stored, tc.wantInserts, tc.wantStored)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (EventReceipt{}) {
					t.Fatalf("PutEvent() = (%+v, %v), want zero and %v", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got.Ref != (EventRef{ID: owned(t).ID, Provider: ProviderGoogle, Calendar: calendarID(t, "primary"), Resource: eventIDFixture}) {
				t.Fatalf("PutEvent() = (%+v, %v), want receipt for %s", got, err, eventIDFixture)
			}
		})
	}
}

func TestGoogleIndeterminateRetryConvergesOnOneEvent(t *testing.T) {
	t.Parallel()
	calendar := &googleCalendar{events: map[string]string{}, fail: func(_ string, attempt int, w http.ResponseWriter) bool {
		if attempt == 1 {
			cutConnection(w)
			return true
		}
		return false
	}}
	client, _ := newNative(t, calendar.serve)
	write := eventWrite(t, oauthConnection(t, ProviderGoogle), intent(t, "primary", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z"))
	_, first := client.PutEvent(t.Context(), write)
	second, err := client.PutEvent(t.Context(), write)
	if !errors.Is(first, core.ErrTimeMatchIndeterminate) || err != nil || second.Ref.Resource != eventIDFixture || len(calendar.events) != 1 || calendar.inserts != 2 {
		t.Fatalf("retry = (%v, %+v, %v) events %d inserts %d, want indeterminate then verified receipt and exactly one event", first, second, err, len(calendar.events), calendar.inserts)
	}
}

func TestGoogleConcurrentIdenticalWritesCreateOneEvent(t *testing.T) {
	t.Parallel()
	calendar := &googleCalendar{events: map[string]string{}}
	client, _ := newNative(t, calendar.serve)
	write := eventWrite(t, oauthConnection(t, ProviderGoogle), intent(t, "primary", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z"))
	const writers = 8
	results := make(chan error, writers)
	var group sync.WaitGroup
	for range writers {
		group.Go(func() {
			receipt, err := client.PutEvent(t.Context(), write)
			if err == nil && receipt.Ref.Resource != eventIDFixture {
				err = core.ErrTimeMatchConflict
			}
			results <- err
		})
	}
	group.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent PutEvent error = %v, want every writer to verify the same event", err)
		}
	}
	if len(calendar.events) != 1 || calendar.inserts != writers {
		t.Fatalf("provider events = %d inserts = %d, want 1 and %d", len(calendar.events), calendar.inserts, writers)
	}
}

func TestGoogleUpdateAndDeleteNative(t *testing.T) {
	t.Parallel()
	calendar := &googleCalendar{events: map[string]string{}}
	client, fixture := newNative(t, calendar.serve)
	connection := oauthConnection(t, ProviderGoogle)
	created, err := client.PutEvent(t.Context(), eventWrite(t, connection, intent(t, "primary", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")))
	if err != nil {
		t.Fatalf("PutEvent() error = %v, want nil", err)
	}
	moved := intent(t, "primary", "Visit moved", "2026-11-03T15:00:00Z", "2026-11-03T16:30:00Z")
	updated, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: connection, Ref: created.Ref, Intent: moved, ObservedAt: at(t, "2026-11-02T09:00:00Z"), Policy: fixturePolicy(t)})
	last := fixture.record()[len(fixture.record())-1]
	if err != nil || updated.Ref != created.Ref || last.method != http.MethodPatch || last.query != "sendUpdates=none" || !strings.Contains(calendar.events[eventIDFixture], "Visit moved") {
		t.Fatalf("UpdateEvent() = (%+v, %v) via %s ?%s, want PATCH of owned fields", updated, err, last.method, last.query)
	}
	foreign := moved
	foreign.Calendar = calendarID(t, "other")
	if _, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: connection, Ref: created.Ref, Intent: foreign, ObservedAt: at(t, "2026-11-02T09:00:00Z"), Policy: fixturePolicy(t)}); !errors.Is(err, core.ErrTimeMatchContract) {
		t.Fatalf("UpdateEvent(foreign calendar) error = %v, want %v", err, core.ErrTimeMatchContract)
	}
	first, firstErr := client.DeleteEvent(t.Context(), EventDelete{Connection: connection, Ref: created.Ref, Policy: fixturePolicy(t)})
	second, secondErr := client.DeleteEvent(t.Context(), EventDelete{Connection: connection, Ref: created.Ref, Policy: fixturePolicy(t)})
	if firstErr != nil || !errors.Is(secondErr, core.ErrTimeMatchIndeterminate) || first != Deleted || second != DeleteInvalid || len(calendar.events) != 0 {
		t.Fatalf("DeleteEvent twice = (%v, %v), (%v, %v), want Deleted then unknown unproved ownership without another DELETE", first, firstErr, second, secondErr)
	}
}

type graphCreated struct {
	ID            string        `json:"id"`
	Subject       string        `json:"subject"`
	TransactionID string        `json:"transactionId,omitzero"`
	Start         graphDateTime `json:"start"`
	End           graphDateTime `json:"end"`
	IsCancelled   bool          `json:"isCancelled"`
}

func TestMicrosoftEventsNative(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		mutate  func(*graphCreated)
		cut     bool
		status  int
		wantErr error
	}{
		{name: "created event bound by transactionId is a receipt", status: 201},
		{name: "returned event with another transactionId is a conflict", status: 201, mutate: func(e *graphCreated) { e.TransactionID = "other" }, wantErr: core.ErrTimeMatchConflict},
		{name: "returned event with another end is a conflict", status: 201, mutate: func(e *graphCreated) { e.End.DateTime = "2026-11-02T16:30:00.0000000" }, wantErr: core.ErrTimeMatchConflict},
		{name: "returned cancelled event is a conflict", status: 201, mutate: func(e *graphCreated) { e.IsCancelled = true }, wantErr: core.ErrTimeMatchConflict},
		{name: "connection cut after create is indeterminate", cut: true, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "gateway timeout is indeterminate", status: 504, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "unauthorized create is consent", status: 401, wantErr: core.ErrTimeMatchConsent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, body []byte) {
				if tc.cut {
					cutConnection(w)
					return
				}
				var created graphCreated
				_ = json.Unmarshal(body, &created)
				created.ID = "AAMkAGI2-event"
				if tc.mutate != nil {
					tc.mutate(&created)
				}
				encoded, _ := json.Marshal(created)
				writeJSON(w, tc.status, string(encoded))
			})
			got, err := client.PutEvent(t.Context(), eventWrite(t, oauthConnection(t, ProviderMicrosoft), intent(t, "cal-1", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")))
			request := fixture.record()[0]
			var sent graphEventBody
			if decodeErr := json.Unmarshal(request.body, &sent); decodeErr != nil || request.path != "/v1.0/me/calendars/cal-1/events" || sent.TransactionID != eventIDFixture || sent.ShowAs != "busy" ||
				sent.Start != (graphDateTime{DateTime: "2026-11-02T15:00:00.0000000", TimeZone: "UTC"}) || request.header.Get("Prefer") != graphPreferUTC || strings.Contains(string(request.body), "attendees") {
				t.Fatalf("create request = %s %s, want transactionId-bound UTC busy event without attendees", request.path, request.body)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (EventReceipt{}) {
					t.Fatalf("PutEvent() = (%+v, %v), want zero and %v", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got.Ref != (EventRef{ID: intent(t, "cal-1", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z").ID, Provider: ProviderMicrosoft, Calendar: calendarID(t, "cal-1"), Resource: "AAMkAGI2-event"}) {
				t.Fatalf("PutEvent() = (%+v, %v), want graph event receipt", got, err)
			}
		})
	}
	t.Run("update patches and delete reports absence", func(t *testing.T) {
		t.Parallel()
		client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/v1.0/me/events/AAMkAGI2-event":
				encoded, encodeErr := json.Marshal(graphOwnedMetadata{ID: "AAMkAGI2-event", TransactionID: eventIDFixture, Ownership: []graphOwnedProperty{{ID: graphOwnershipProperty, Value: eventIDFixture}}})
				if encodeErr != nil {
					t.Errorf("metadata encode error = %v, want nil", encodeErr)
				}
				writeJSON(w, 200, string(encoded))
			case r.Method == http.MethodPatch:
				var patch graphCreated
				_ = json.Unmarshal(body, &patch)
				patch.ID = "AAMkAGI2-event"
				encoded, _ := json.Marshal(patch)
				writeJSON(w, 200, string(encoded))
			case r.Method == http.MethodDelete && r.URL.Path == "/v1.0/me/events/AAMkAGI2-event":
				w.WriteHeader(http.StatusNoContent)
			default:
				writeJSON(w, 404, `{}`)
			}
		})
		connection := oauthConnection(t, ProviderMicrosoft)
		ref := EventRef{ID: intent(t, "cal-1", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z").ID, Provider: ProviderMicrosoft, Calendar: calendarID(t, "cal-1"), Resource: "AAMkAGI2-event"}
		updated, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: connection, Ref: ref, Intent: intent(t, "cal-1", "Moved", "2026-11-03T15:00:00Z", "2026-11-03T16:00:00Z"), ObservedAt: at(t, "2026-11-02T09:00:00Z"), Policy: fixturePolicy(t)})
		patch := fixture.record()[1]
		if err != nil || updated.Ref != ref || patch.path != "/v1.0/me/events/AAMkAGI2-event" || strings.Contains(string(patch.body), "transactionId") {
			t.Fatalf("UpdateEvent() = (%+v, %v) body %s, want PATCH without transactionId", updated, err, patch.body)
		}
		deleted, deleteErr := client.DeleteEvent(t.Context(), EventDelete{Connection: connection, Ref: ref, Policy: fixturePolicy(t)})
		ref.Resource = "gone"
		absent, absentErr := client.DeleteEvent(t.Context(), EventDelete{Connection: connection, Ref: ref, Policy: fixturePolicy(t)})
		if deleteErr != nil || !errors.Is(absentErr, core.ErrTimeMatchIndeterminate) || deleted != Deleted || absent != DeleteInvalid {
			t.Fatalf("DeleteEvent = (%v, %v), (%v, %v), want Deleted and unknown absence", deleted, deleteErr, absent, absentErr)
		}
	})
}

func TestAppleEventsNative(t *testing.T) {
	t.Parallel()
	home := "https://p42-caldav.icloud.com/123456/calendars/home/"
	resource := home + eventIDFixture + ".ics"
	owned := func(t *testing.T) EventIntent {
		return intent(t, home, "Visit; with, escapes", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")
	}
	cases := []struct {
		name     string
		status   int
		existing func(t *testing.T) string
		wantErr  error
	}{
		{name: "absent resource is created", status: 201},
		{name: "no content create is accepted", status: 204},
		{name: "existing identical resource is verified replay", status: 412, existing: func(t *testing.T) string {
			body, _ := writeICS(owned(t), at(t, "2026-11-01T00:00:00Z"))
			return string(body)
		}},
		{name: "existing resource for another time is a conflict", status: 412, existing: func(t *testing.T) string {
			other := owned(t)
			other.Interval = window(t, "2026-11-02T15:00:00Z", "2026-11-02T17:00:00Z")
			body, _ := writeICS(other, at(t, "2026-11-01T00:00:00Z"))
			return string(body)
		}, wantErr: core.ErrTimeMatchConflict},
		{name: "server error is indeterminate", status: 502, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "revoked app password is consent", status: 401, wantErr: core.ErrTimeMatchConsent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method == http.MethodGet && tc.existing != nil {
					w.Header().Set("Content-Type", "text/calendar")
					_, _ = io.WriteString(w, tc.existing(t))
					return
				}
				w.WriteHeader(tc.status)
			})
			got, err := client.PutEvent(t.Context(), eventWrite(t, appleConnection(t), owned(t)))
			if len(fixture.record()) == 0 {
				t.Fatalf("PutEvent(apple) sent no request (error %v), want a conditional PUT", err)
			}
			put := fixture.record()[0]
			body := string(put.body)
			if put.method != http.MethodPut || put.host != "p42-caldav.icloud.com" || put.path != "/123456/calendars/home/"+eventIDFixture+".ics" || put.header.Get("If-None-Match") != "*" ||
				!strings.HasPrefix(put.header.Get("Content-Type"), "text/calendar") || !strings.Contains(body, "UID:"+eventIDFixture+"\r\n") ||
				!strings.Contains(body, "DTSTART:20261102T150000Z\r\nDTEND:20261102T160000Z\r\n") || !strings.Contains(body, "DTSTAMP:20261102T080000Z\r\n") || !strings.Contains(body, `SUMMARY:Visit\; with\, escapes`) {
				t.Fatalf("create = %s %s%s if-none-match %q body %q, want conditional RFC 5545 create", put.method, put.host, put.path, put.header.Get("If-None-Match"), body)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (EventReceipt{}) {
					t.Fatalf("PutEvent(apple) = (%+v, %v), want zero and %v", got, err, tc.wantErr)
				}
				noLeak(t, err, "abcd-efgh-ijkl-mnop")
				return
			}
			if err != nil || got.Ref.Resource != resource {
				t.Fatalf("PutEvent(apple) = (%+v, %v), want resource %s", got, err, resource)
			}
		})
	}
	t.Run("sub-second interval has no iCalendar form and sends nothing", func(t *testing.T) {
		t.Parallel()
		client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { w.WriteHeader(201) })
		precise := owned(t)
		precise.Interval = window(t, "2026-11-02T15:00:00.25Z", "2026-11-02T16:00:00Z")
		_, err := client.PutEvent(t.Context(), eventWrite(t, appleConnection(t), precise))
		if !errors.Is(err, core.ErrTimeMatchContract) || len(fixture.record()) != 0 {
			t.Fatalf("PutEvent(sub-second) = %v with %d requests, want contract refusal and none", err, len(fixture.record()))
		}
	})
	t.Run("update requires existence and delete reports absence", func(t *testing.T) {
		t.Parallel()
		client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
			if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "gone0.ics") {
				w.WriteHeader(404)
				return
			}
			if r.Method == http.MethodGet {
				body, bodyErr := writeICS(owned(t), at(t, "2026-11-02T08:00:00Z"))
				if bodyErr != nil {
					t.Errorf("owned readback fixture error = %v, want nil", bodyErr)
					w.WriteHeader(500)
					return
				}
				w.Header().Set("Content-Type", "text/calendar")
				w.Header().Set("ETag", `"observed"`)
				_, _ = w.Write(body)
				return
			}
			w.WriteHeader(204)
		})
		connection := appleConnection(t)
		ref := EventRef{ID: intent(t, home, "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z").ID, Provider: ProviderApple, Calendar: calendarID(t, home), Resource: resource}
		updated, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: connection, Ref: ref, Intent: owned(t), ObservedAt: at(t, "2026-11-02T09:00:00Z"), Policy: fixturePolicy(t)})
		if err != nil || updated.Ref != ref || len(fixture.record()) != 2 || fixture.record()[0].method != http.MethodGet || fixture.record()[1].header.Get("If-Match") != `"observed"` {
			t.Fatalf("UpdateEvent(apple) = (%+v, %v), want If-Match replacement", updated, err)
		}
		deleted, deleteErr := client.DeleteEvent(t.Context(), EventDelete{Connection: connection, Ref: ref, Policy: fixturePolicy(t)})
		ref.ID, err = ParseEventID("gone0")
		if err != nil {
			t.Fatal(err)
		}
		ref.Resource = home + "gone0.ics"
		absent, absentErr := client.DeleteEvent(t.Context(), EventDelete{Connection: connection, Ref: ref, Policy: fixturePolicy(t)})
		ref.Resource = "https://attacker.example/x.ics"
		_, foreignErr := client.DeleteEvent(t.Context(), EventDelete{Connection: connection, Ref: ref, Policy: fixturePolicy(t)})
		if deleteErr != nil || absentErr != nil || deleted != Deleted || absent != DeleteAlreadyAbsent || !errors.Is(foreignErr, core.ErrTimeMatchContract) || len(fixture.refusedDials()) != 0 {
			t.Fatalf("DeleteEvent(apple) = (%v, %v), (%v, %v), foreign %v, want Deleted, AlreadyAbsent and contract refusal", deleted, deleteErr, absent, absentErr, foreignErr)
		}
	})
}
