package timematch

import (
	json "encoding/json/v2"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoogleMeetUpdateConferencePreservation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		status      string
		before      func(*googleMeetTestEvent)
		after       func(*googleMeetTestEvent)
		readStatus  int
		patchStatus int
		want        EventVenueStatus
		wantErr     error
		wantPatch   int64
		wantCreate  bool
	}{
		{name: "ready conference survives a different appointment interval", status: googleConferenceSuccess, want: EventVenueReady, wantPatch: 1},
		{name: "pending conference is retained without another creation intent", status: googleConferencePending, want: EventVenuePending, wantPatch: 1},
		{name: "failed conference is retained without an automatic replacement", status: googleConferenceFailure, want: EventVenueFailed, wantPatch: 1},
		{name: "absent conference permits explicit automatic conversion", status: googleConferenceSuccess, before: func(e *googleMeetTestEvent) { e.ConferenceData = nil }, want: EventVenueReady, wantPatch: 1, wantCreate: true},
		{name: "changed ready destination refuses the update receipt", status: googleConferenceSuccess, after: func(e *googleMeetTestEvent) {
			e.ConferenceData.EntryPoints[0].URI = "https://meet.google.com/xyz-abcd-efg"
		}, wantErr: core.ErrTimeMatchConflict, wantPatch: 1},
		{name: "foreign marker is refused before patch", status: googleConferenceSuccess, before: func(e *googleMeetTestEvent) { e.ExtendedProperties.Private.EventID = "other01" }, wantErr: core.ErrTimeMatchConflict},
		{name: "foreign event is refused before patch", status: googleConferenceSuccess, before: func(e *googleMeetTestEvent) { e.ID = "other01" }, wantErr: core.ErrTimeMatchConflict},
		{name: "cancelled event is refused before patch", status: googleConferenceSuccess, before: func(e *googleMeetTestEvent) { e.Status = "cancelled" }, wantErr: core.ErrTimeMatchConflict},
		{name: "changed observed revision is refused before patch", status: googleConferenceSuccess, before: func(e *googleMeetTestEvent) { e.ETag = `"changed-revision"` }, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "foreign conference request is refused before patch", status: googleConferenceSuccess, before: func(e *googleMeetTestEvent) { e.ConferenceData.CreateRequest.RequestID = "other01" }, wantErr: core.ErrTimeMatchConflict},
		{name: "malformed existing conference is refused before patch", status: googleConferenceSuccess, before: func(e *googleMeetTestEvent) { e.ConferenceData.CreateRequest = nil }, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "missing ready entry cannot be replaced", status: googleConferenceSuccess, before: func(e *googleMeetTestEvent) { e.ConferenceData.EntryPoints = nil }, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "conditional observation failure has no mutation", status: googleConferenceSuccess, readStatus: http.StatusServiceUnavailable, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "owner edit after observation refuses conditional patch", status: googleConferenceSuccess, patchStatus: http.StatusPreconditionFailed, wantErr: core.ErrTimeMatchConflict, wantPatch: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := meetIntent(t)
			next := owned
			next.Interval = window(t, "2026-11-02T17:00:00Z", "2026-11-02T18:00:00Z")
			wire := meetEvent(t, owned, tc.status)
			if tc.before != nil {
				tc.before(&wire)
			}
			var metadata, details, patches atomic.Int64
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
				switch {
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/events"):
					metadata.Add(1)
					if r.URL.Query().Get("privateExtendedProperty") != googleOwnershipProperty+"="+owned.ID.String() || r.URL.Query().Get("fields") != googleOwnedMetadataFields {
						t.Errorf("ownership query = %q, want narrow owned metadata", r.URL.RawQuery)
					}
					writeJSON(w, http.StatusOK, meetMetadata(t, owned))
				case r.Method == http.MethodGet:
					details.Add(1)
					if r.Header.Get("If-Match") != `"meet-revision"` || r.URL.Query().Get("fields") != googleOwnedDetailsFields+googleOwnedConferenceFields || len(body) != 0 {
						t.Errorf("owned observation = (%q,%q,%d), want exact conditional conference GET", r.Header.Get("If-Match"), r.URL.RawQuery, len(body))
					}
					if tc.readStatus != 0 {
						writeJSON(w, tc.readStatus, `{}`)
						return
					}
					writeJSON(w, http.StatusOK, string(seedJSON(t, wire)))
				case r.Method == http.MethodPatch:
					patches.Add(1)
					var patch googleEventBody
					if err := json.Unmarshal(body, &patch); err != nil {
						t.Error(err)
					}
					if details.Load() != 1 || r.Header.Get("If-Match") != `"meet-revision"` || r.URL.Query().Get("conferenceDataVersion") != "1" || patch.ID != "" || patch.ExtendedProperties != nil || (patch.ConferenceData != nil) != tc.wantCreate {
						t.Errorf("patch = (observations %d, header %q, query %q, conference %v), want prior observation and create %v", details.Load(), r.Header.Get("If-Match"), r.URL.RawQuery, patch.ConferenceData, tc.wantCreate)
					}
					if tc.patchStatus != 0 {
						writeJSON(w, tc.patchStatus, `{}`)
						return
					}
					// The peer applies the actual patch to the previously observed
					// event. An omitted conference preserves its saved value.
					updated := wire
					updated.Start, updated.End, updated.Summary = patch.Start, patch.End, patch.Summary
					if tc.wantCreate {
						updated.ConferenceData = meetEvent(t, owned, googleConferenceSuccess).ConferenceData
					}
					if tc.after != nil {
						tc.after(&updated)
					}
					writeJSON(w, http.StatusOK, string(seedJSON(t, updated)))
				default:
					t.Errorf("method = %q, want only metadata GET, owned GET and PATCH", r.Method)
					writeJSON(w, http.StatusBadRequest, `{}`)
				}
			})
			ref := EventRef{ID: owned.ID, Calendar: owned.Calendar, Provider: ProviderGoogle, Resource: owned.ID.String()}
			got, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: oauthConnection(t, ProviderGoogle), Ref: ref, Intent: next, ObservedAt: at(t, "2026-11-02T09:00:00Z"), Policy: fixturePolicy(t)})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (EventReceipt{}) {
					t.Fatalf("UpdateEvent() = (%+v,%v), want zero/%v", got, err, tc.wantErr)
				}
			} else if err != nil || got.VenueStatus != tc.want || got.Ref != ref {
				t.Fatalf("UpdateEvent() = (%+v,%v), want same ref/state %v", got, err, tc.want)
			} else if tc.want == EventVenueReady && got.Venue.JoinURL != "https://meet.google.com/abc-defg-hij" {
				t.Fatalf("ready URL = %q, want exact retained URI", got.Venue.JoinURL)
			}
			if metadata.Load() != 1 || details.Load() != 1 || patches.Load() != tc.wantPatch || len(fixture.record()) != int(2+tc.wantPatch) {
				t.Fatalf("request counts = (%d,%d,%d,total%d), want (1,1,%d,total%d)", metadata.Load(), details.Load(), patches.Load(), len(fixture.record()), tc.wantPatch, 2+tc.wantPatch)
			}
		})
	}
}
