package timematch

import (
	json "encoding/json/v2"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoogleReviewDuplicateAndReadOnlyAbsenceDiffer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		attempt EventCreationAttempt
		want    error
		writes  int64
	}{
		{name: "definitive duplicate without owned identity is conflict", attempt: EventCreateGuardedRecovery, want: core.ErrTimeMatchConflict, writes: 1},
		{name: "read only absence remains unknown", attempt: EventCreateRecoverOnly, want: core.ErrTimeMatchIndeterminate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := meetIntent(t)
			var writes atomic.Int64
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method == http.MethodPost {
					writes.Add(1)
					writeJSON(w, http.StatusConflict, `{}`)
					return
				}
				if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/events") {
					t.Errorf("absence request = %s %s, want metadata only", r.Method, r.URL.Path)
				}
				writeJSON(w, http.StatusOK, string(seedJSON(t, googleOwnedEventPage{Items: []googleOwnedEventMetadata{}})))
			})
			request := eventWrite(t, oauthConnection(t, ProviderGoogle), owned)
			request.Attempt = tc.attempt
			got, err := client.PutEvent(t.Context(), request)
			if !errors.Is(err, tc.want) || got != (EventReceipt{}) || writes.Load() != tc.writes || len(fixture.record()) != int(tc.writes)+1 {
				t.Fatalf("PutEvent() = (%+v,%v), writes/calls %d/%d, want zero/%v/%d/%d", got, err, writes.Load(), len(fixture.record()), tc.want, tc.writes, tc.writes+1)
			}
		})
	}
}

func TestGoogleReviewCancelledIdentityRequiresMinimalObservation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		status string
		id     string
		http   int
		want   DeleteOutcome
		err    error
	}{
		{name: "matching cancelled identity is already absent", status: "cancelled", want: DeleteAlreadyAbsent},
		{name: "active foreign held identity stays unknown", status: "confirmed", want: DeleteInvalid, err: core.ErrTimeMatchIndeterminate},
		{name: "tentative identity stays unknown", status: "tentative", want: DeleteInvalid, err: core.ErrTimeMatchIndeterminate},
		{name: "missing status does not prove absence", want: DeleteInvalid, err: core.ErrTimeMatchIndeterminate},
		{name: "other cancelled identity is conflict", id: "other01", status: "cancelled", want: DeleteInvalid, err: core.ErrTimeMatchConflict},
		{name: "not found can conceal inaccessible calendar", http: http.StatusNotFound, want: DeleteInvalid, err: core.ErrTimeMatchIndeterminate},
		{name: "failed observation remains unknown", http: http.StatusServiceUnavailable, want: DeleteInvalid, err: core.ErrTimeMatchIndeterminate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := meetIntent(t)
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method != http.MethodGet {
					t.Errorf("absence method = %s, want read only", r.Method)
					writeJSON(w, http.StatusBadRequest, `{}`)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/events") {
					writeJSON(w, http.StatusOK, string(seedJSON(t, googleOwnedEventPage{Items: []googleOwnedEventMetadata{}})))
					return
				}
				if r.URL.Query().Get("fields") != "id,status" || r.Header.Get("If-Match") != "" {
					t.Errorf("absence read = (%q,%q), want id,status and no fabricated revision", r.URL.RawQuery, r.Header.Get("If-Match"))
				}
				if tc.http != 0 {
					writeJSON(w, tc.http, `{}`)
					return
				}
				id := owned.ID.String()
				if tc.id != "" {
					id = tc.id
				}
				writeJSON(w, http.StatusOK, string(seedJSON(t, struct {
					ID     string `json:"id"`
					Status string `json:"status,omitempty"`
				}{ID: id, Status: tc.status})))
			})
			ref := EventRef{ID: owned.ID, Calendar: owned.Calendar, Provider: ProviderGoogle, Resource: owned.ID.String()}
			got, err := client.DeleteEvent(t.Context(), EventDelete{Connection: oauthConnection(t, ProviderGoogle), Ref: ref, Policy: fixturePolicy(t)})
			if got != tc.want || (tc.err == nil && err != nil) || (tc.err != nil && !errors.Is(err, tc.err)) || len(fixture.record()) != 2 {
				t.Fatalf("DeleteEvent() = (%v,%v), calls%d, want (%v,%v)/2 reads", got, err, len(fixture.record()), tc.want, tc.err)
			}
		})
	}
}

func TestGoogleReviewWritesRequestOnlyOwnedResponseFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		update bool
		video  bool
	}{
		{name: "ordinary insert"}, {name: "video insert", video: true},
		{name: "ordinary update", update: true}, {name: "video update", update: true, video: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := meetIntent(t)
			wire := meetEvent(t, owned, googleConferenceSuccess)
			fields := googleOwnedDetailsFields
			if tc.video {
				fields += googleOwnedConferenceFields
			} else {
				owned.Venue = core.TimeMatchAppointmentVenue{}
				wire.ConferenceData = nil
			}
			var writes atomic.Int64
			client, _ := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method == http.MethodGet {
					if strings.HasSuffix(r.URL.Path, "/events") {
						writeJSON(w, http.StatusOK, meetMetadata(t, owned))
					} else {
						writeJSON(w, http.StatusOK, string(seedJSON(t, wire)))
					}
					return
				}
				writes.Add(1)
				if r.URL.Query().Get("fields") != fields || (r.URL.Query().Get("conferenceDataVersion") == "1") != tc.video {
					t.Errorf("mutation query = %q, want exact owned fields %q and video %v", r.URL.RawQuery, fields, tc.video)
				}
				writeJSON(w, http.StatusOK, string(seedJSON(t, wire)))
			})
			var got EventReceipt
			var err error
			if tc.update {
				ref := EventRef{ID: owned.ID, Calendar: owned.Calendar, Provider: ProviderGoogle, Resource: owned.ID.String()}
				got, err = client.UpdateEvent(t.Context(), EventUpdate{Connection: oauthConnection(t, ProviderGoogle), Ref: ref, Intent: owned, ObservedAt: at(t, "2026-11-02T09:00:00Z"), Policy: fixturePolicy(t)})
			} else {
				got, err = client.PutEvent(t.Context(), eventWrite(t, oauthConnection(t, ProviderGoogle), owned))
			}
			if err != nil || got.Ref.ID != owned.ID || writes.Load() != 1 {
				t.Fatalf("owned write = (%+v,%v), mutations%d, want same identity and one write", got, err, writes.Load())
			}
		})
	}
}

func TestGoogleReviewFreeBusyDeclaresExpansionForFullAndTailBatch(t *testing.T) {
	t.Parallel()
	client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		var request struct {
			freeBusyRequestWire
			Expansion int `json:"calendarExpansionMax"`
		}
		if err := json.Unmarshal(body, &request); err != nil || request.Expansion != googleFreeBusyBatch || r.Method != http.MethodPost {
			t.Errorf("freebusy expansion = %d, error%v, want %d/POST", request.Expansion, err, googleFreeBusyBatch)
		}
		calendars := make(map[string]struct {
			Busy []struct{} `json:"busy"`
		}, len(request.Items))
		for _, item := range request.Items {
			calendars[item.ID] = struct {
				Busy []struct{} `json:"busy"`
			}{Busy: []struct{}{}}
		}
		writeJSON(w, http.StatusOK, string(seedJSON(t, struct {
			TimeMin   string `json:"timeMin"`
			TimeMax   string `json:"timeMax"`
			Calendars map[string]struct {
				Busy []struct{} `json:"busy"`
			} `json:"calendars"`
		}{TimeMin: request.TimeMin, TimeMax: request.TimeMax, Calendars: calendars})))
	})
	ids := make([]CalendarID, googleFreeBusyBatch+1)
	for i := range ids {
		ids[i] = calendarID(t, "calendar-"+strconv.Itoa(i))
	}
	got, err := client.Availability(t.Context(), AvailabilityRequest{Connection: oauthConnection(t, ProviderGoogle), Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Calendars: ids, Policy: fixturePolicy(t)})
	if err != nil || len(got.Calendars) != len(ids) || got.Calendars[len(ids)-1].State != AvailabilityKnown || len(fixture.record()) != 2 {
		t.Fatalf("Availability() = (%d,%v), calls%d, want 51known/2 batches", len(got.Calendars), err, len(fixture.record()))
	}
}
