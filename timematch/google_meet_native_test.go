package timematch

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type googleMeetTestEvent struct {
	googleEventBody
	ETag   string `json:"etag"`
	Status string `json:"status"`
}

func meetEvent(t testing.TB, owned EventIntent, status string) googleMeetTestEvent {
	t.Helper()
	request, err := googleEvent(owned)
	if err != nil {
		t.Fatal(err)
	}
	var body googleEventBody
	if err := json.Unmarshal(request, &body); err != nil {
		t.Fatal(err)
	}
	if body.ConferenceData == nil || body.ConferenceData.CreateRequest == nil {
		t.Fatal("typed automatic venue emitted no conference request")
	}
	body.ConferenceData.CreateRequest.Status = &googleConferenceStatus{Code: status}
	if status == googleConferenceSuccess {
		body.ConferenceData.Solution = &googleConferenceSolution{Key: googleConferenceKey{Type: googleMeetSolution}}
		body.ConferenceData.EntryPoints = []googleConferenceEntryPoint{{Type: googleConferenceVideo, URI: "https://meet.google.com/abc-defg-hij"}}
	}
	return googleMeetTestEvent{googleEventBody: body, ETag: `"meet-revision"`, Status: "confirmed"}
}

func meetMetadata(t testing.TB, owned EventIntent) string {
	t.Helper()
	return string(seedJSON(t, googleOwnedEventPage{Items: []googleOwnedEventMetadata{{ID: owned.ID.String(), ETag: `"meet-revision"`}}}))
}

func TestGoogleMeetNativeAsyncLifecycle(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		lost      bool
		collision bool
		failed    bool
		reads     int
		want      EventVenueStatus
	}{
		{name: "pending creation becomes ready through read-only owned recovery", reads: 1, want: EventVenueReady},
		{name: "lost create reply recovers pending then exact ready call", lost: true, reads: 2, want: EventVenueReady},
		{name: "explicit provider failure remains visible without automatic replacement", failed: true, reads: 1, want: EventVenueFailed},
		{name: "duplicate deterministic insert reads existing conference without a second mutation", collision: true, reads: 1, want: EventVenueReady},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := meetIntent(t)
			var inserts, details atomic.Int64
			var saved googleEventBody
			var mutex sync.Mutex
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
				mutex.Lock()
				defer mutex.Unlock()
				if r.Method == http.MethodPost {
					inserts.Add(1)
					if err := json.Unmarshal(body, &saved); err != nil {
						t.Error(err)
						writeJSON(w, 400, `{}`)
						return
					}
					if saved.ID != owned.ID.String() || saved.ConferenceData == nil || saved.ConferenceData.CreateRequest == nil {
						t.Error("create omitted owned conference identity")
						writeJSON(w, 400, `{}`)
						return
					}
					if r.URL.Query().Get("conferenceDataVersion") != "1" {
						t.Error("create omitted conference version1")
					}
					if tc.lost {
						writeJSON(w, 503, `{}`)
						return
					}
					if tc.collision {
						writeJSON(w, 409, `{}`)
						return
					}
					status := googleConferencePending
					if tc.failed {
						status = googleConferenceFailure
					}
					writeJSON(w, 200, string(seedJSON(t, meetEvent(t, owned, status))))
					return
				}
				if r.Method != http.MethodGet {
					t.Error("recovery sent mutation")
					writeJSON(w, 400, `{}`)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/events") {
					if r.URL.Query().Get("privateExtendedProperty") != "timematchEventId="+owned.ID.String() || r.URL.Query().Get("fields") != googleOwnedMetadataFields {
						t.Errorf("ownership query=%q, want exact narrow marker lookup", r.URL.RawQuery)
					}
					writeJSON(w, 200, meetMetadata(t, owned))
					return
				}
				if r.Header.Get("If-Match") != `"meet-revision"` || !strings.Contains(r.URL.Query().Get("fields"), "conferenceData(") {
					t.Error("detail read omitted revision or narrow conference fields")
				}
				n := details.Add(1)
				status := googleConferenceSuccess
				if tc.failed {
					status = googleConferenceFailure
				} else if tc.lost && n == 1 {
					status = googleConferencePending
				}
				wire := meetEvent(t, owned, status)
				// Readback carries the exact event body actually accepted before the
				// lost reply, not an independently pre-baked replacement event.
				wire.ID = saved.ID
				wire.Summary = saved.Summary
				wire.Start = saved.Start
				wire.End = saved.End
				wire.ExtendedProperties = saved.ExtendedProperties
				wire.ConferenceData.CreateRequest.RequestID = saved.ConferenceData.CreateRequest.RequestID
				writeJSON(w, 200, string(seedJSON(t, wire)))
			})
			request := eventWrite(t, oauthConnection(t, ProviderGoogle), owned)
			first, err := client.PutEvent(t.Context(), request)
			if tc.lost {
				if !errors.Is(err, core.ErrTimeMatchIndeterminate) || first != (EventReceipt{}) {
					t.Fatalf("lost create=(%+v,%v),want zero/Indeterminate", first, err)
				}
			} else if err != nil {
				t.Fatal(err)
			} else if !tc.collision && first.VenueStatus != EventVenuePending && !tc.failed {
				t.Fatalf("first receipt=%+v,want Pending/noURL", first)
			}
			if first.VenueStatus != EventVenueReady && first.Venue.JoinURL != "" {
				t.Fatalf("unready first receipt exposed venue%q", first.Venue.JoinURL)
			}
			request.Attempt = EventCreateRecoverOnly
			var got EventReceipt
			for range tc.reads {
				got, err = client.PutEvent(t.Context(), request)
				if err != nil {
					t.Fatal(err)
				}
				if err := got.Validate(); err != nil {
					t.Fatal(err)
				}
			}
			if got.VenueStatus != tc.want || inserts.Load() != 1 {
				t.Fatalf("recovered=(%+v),inserts%d,want state%d/onecreate", got, inserts.Load(), tc.want)
			}
			if tc.want == EventVenueReady && got.Venue.JoinURL != "https://meet.google.com/abc-defg-hij" {
				t.Fatalf("ready URI=%q,want exact owned URI", got.Venue.JoinURL)
			}
			if tc.want == EventVenueFailed && got.Venue != (core.TimeMatchAppointmentVenue{}) {
				t.Fatalf("failed URI=%+v,want absent", got.Venue)
			}
			for _, call := range fixture.record() {
				if call.method == http.MethodGet && len(call.body) != 0 {
					t.Error("recovery GET carried a create body")
				}
			}
		})
	}
}

func TestGoogleMeetOwnedConferenceHostileReadback(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*googleMeetTestEvent)
		want   error
	}{
		{name: "missing conference stays indeterminate", mutate: func(e *googleMeetTestEvent) { e.ConferenceData = nil }, want: core.ErrTimeMatchIndeterminate},
		{name: "missing creation request stays indeterminate", mutate: func(e *googleMeetTestEvent) { e.ConferenceData.CreateRequest = nil }, want: core.ErrTimeMatchIndeterminate},
		{name: "missing creation status stays indeterminate", mutate: func(e *googleMeetTestEvent) { e.ConferenceData.CreateRequest.Status = nil }, want: core.ErrTimeMatchIndeterminate},
		{name: "another conference request cannot satisfy this event", mutate: func(e *googleMeetTestEvent) { e.ConferenceData.CreateRequest.RequestID = "other-owned-event" }, want: core.ErrTimeMatchConflict},
		{name: "another requested solution is not Meet", mutate: func(e *googleMeetTestEvent) { e.ConferenceData.CreateRequest.Solution.Type = "addOn" }, want: core.ErrTimeMatchConflict},
		{name: "unknown generation state cannot become ready", mutate: func(e *googleMeetTestEvent) { e.ConferenceData.CreateRequest.Status.Code = "future" }, want: core.ErrTimeMatchIndeterminate},
		{name: "missing returned solution cannot prove Meet", mutate: func(e *googleMeetTestEvent) { e.ConferenceData.Solution = nil }, want: core.ErrTimeMatchIndeterminate},
		{name: "returned foreign solution cannot prove Meet", mutate: func(e *googleMeetTestEvent) { e.ConferenceData.Solution.Key.Type = "addOn" }, want: core.ErrTimeMatchIndeterminate},
		{name: "missing video entry cannot become ready", mutate: func(e *googleMeetTestEvent) { e.ConferenceData.EntryPoints = nil }, want: core.ErrTimeMatchIndeterminate},
		{name: "two video entries contradict one conference", mutate: func(e *googleMeetTestEvent) {
			e.ConferenceData.EntryPoints = append(e.ConferenceData.EntryPoints, e.ConferenceData.EntryPoints[0])
		}, want: core.ErrTimeMatchIndeterminate},
		{name: "phone only is not a video call", mutate: func(e *googleMeetTestEvent) {
			e.ConferenceData.EntryPoints[0] = googleConferenceEntryPoint{Type: googleConferencePhone, URI: "tel:+14165550123"}
		}, want: core.ErrTimeMatchIndeterminate},
		{name: "future entry type is not a video call", mutate: func(e *googleMeetTestEvent) { e.ConferenceData.EntryPoints[0].Type = "future" }, want: core.ErrTimeMatchIndeterminate},
		{name: "pending status plus ready entries is contradictory", mutate: func(e *googleMeetTestEvent) { e.ConferenceData.CreateRequest.Status.Code = googleConferencePending }, want: core.ErrTimeMatchIndeterminate},
		{name: "failure status plus ready entries is contradictory", mutate: func(e *googleMeetTestEvent) { e.ConferenceData.CreateRequest.Status.Code = googleConferenceFailure }, want: core.ErrTimeMatchIndeterminate},
		{name: "changed event ID cannot supply a conference", mutate: func(e *googleMeetTestEvent) { e.ID = "other01" }, want: core.ErrTimeMatchConflict},
		{name: "foreign private marker cannot supply a conference", mutate: func(e *googleMeetTestEvent) { e.ExtendedProperties.Private.EventID = "other01" }, want: core.ErrTimeMatchConflict},
		{name: "cancelled owned event cannot supply a conference", mutate: func(e *googleMeetTestEvent) { e.Status = "cancelled" }, want: core.ErrTimeMatchConflict},
		{name: "changed summary cannot supply an old call", mutate: func(e *googleMeetTestEvent) { e.Summary = "Owner changed this event" }, want: core.ErrTimeMatchConflict},
		{name: "changed interval cannot supply an old call", mutate: func(e *googleMeetTestEvent) { e.End.DateTime = "2026-11-02T17:00:00Z" }, want: core.ErrTimeMatchConflict},
		{name: "changed revision refuses captured conference", mutate: func(e *googleMeetTestEvent) { e.ETag = `"foreign-revision"` }, want: core.ErrTimeMatchIndeterminate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := meetIntent(t)
			wire := meetEvent(t, owned, googleConferenceSuccess)
			tc.mutate(&wire)
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method != http.MethodGet {
					t.Error("read-only recovery mutated")
					writeJSON(w, 400, `{}`)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/events") {
					writeJSON(w, 200, meetMetadata(t, owned))
					return
				}
				writeJSON(w, 200, string(seedJSON(t, wire)))
			})
			request := eventWrite(t, oauthConnection(t, ProviderGoogle), owned)
			request.Attempt = EventCreateRecoverOnly
			got, err := client.PutEvent(t.Context(), request)
			if !errors.Is(err, tc.want) || got != (EventReceipt{}) || len(fixture.record()) != 2 {
				t.Fatalf("hostile Meet=(%+v,%v),calls%d,want zero/%v/tworeads", got, err, len(fixture.record()), tc.want)
			}
		})
	}
}

func TestGoogleMeetConditionalUpdateUsesSameConference(t *testing.T) {
	t.Parallel()
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "same request and video link survive reschedule", true: "concurrent owner edit refuses mutation"}[conflict], func(t *testing.T) {
			t.Parallel()
			owned := meetIntent(t)
			next := owned
			next.Interval = window(t, "2026-11-02T17:00:00Z", "2026-11-02T18:00:00Z")
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
				if r.Method == http.MethodGet {
					writeJSON(w, 200, meetMetadata(t, owned))
					return
				}
				var patch googleEventBody
				if err := json.Unmarshal(body, &patch); err != nil {
					t.Error(err)
				}
				if r.Method != http.MethodPatch || r.Header.Get("If-Match") != `"meet-revision"` || r.URL.Query().Get("conferenceDataVersion") != "1" || patch.ConferenceData == nil || patch.ConferenceData.CreateRequest.RequestID != owned.ID.String() || patch.ID != "" || patch.ExtendedProperties != nil {
					t.Errorf("reschedule=(%s,%q,%+v),want guarded existing-request patch", r.Method, r.URL.RawQuery, patch)
				}
				if conflict {
					writeJSON(w, 412, `{}`)
					return
				}
				writeJSON(w, 200, string(seedJSON(t, meetEvent(t, next, googleConferenceSuccess))))
			})
			ref := EventRef{ID: owned.ID, Calendar: owned.Calendar, Provider: ProviderGoogle, Resource: owned.ID.String()}
			got, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: oauthConnection(t, ProviderGoogle), Ref: ref, Intent: next, ObservedAt: at(t, "2026-11-02T09:00:00Z"), Policy: fixturePolicy(t)})
			if conflict {
				if !errors.Is(err, core.ErrTimeMatchConflict) || got != (EventReceipt{}) {
					t.Fatalf("update race=(%+v,%v),want zero/conflict", got, err)
				}
			} else if err != nil || got.VenueStatus != EventVenueReady || got.Venue.JoinURL != "https://meet.google.com/abc-defg-hij" {
				t.Fatalf("updated Meet=(%+v,%v),want exact Ready/unchangedURI", got, err)
			}
			if len(fixture.record()) != 2 {
				t.Fatalf("update calls%d,want one metadata read and one guarded patch", len(fixture.record()))
			}
		})
	}
}

func TestGoogleMeetMissingOwnershipNeverReadsConference(t *testing.T) {
	t.Parallel()
	owned := meetIntent(t)
	client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if !strings.HasSuffix(r.URL.Path, "/events") {
			t.Error("read foreign private event details")
		}
		writeJSON(w, 200, string(seedJSON(t, googleOwnedEventPage{Items: []googleOwnedEventMetadata{}})))
	})
	r := eventWrite(t, oauthConnection(t, ProviderGoogle), owned)
	r.Attempt = EventCreateRecoverOnly
	got, err := client.PutEvent(t.Context(), r)
	if !errors.Is(err, core.ErrTimeMatchIndeterminate) || got != (EventReceipt{}) || len(fixture.record()) != 1 {
		t.Fatalf("unowned recovery=(%+v,%v),calls%d,want zero/unknown/onemetadata", got, err, len(fixture.record()))
	}
}

func TestAutomaticVideoInvitationCannotPublishUnreadyVenue(t *testing.T) {
	t.Parallel()
	invitation := invitationFixture(t)
	invitation.Venue = core.TimeMatchAppointmentVenue{Kind: core.TimeMatchVenueAutomaticVideo}
	var out bytes.Buffer
	err := WriteInvitation(&out, invitation)
	if !errors.Is(err, core.ErrTimeMatchContract) || out.Len() != 0 {
		t.Fatalf("unready invitation=(%dbytes,%v),want zero/contract", out.Len(), err)
	}
}

func TestAutomaticVideoUnsupportedUpdateHasNoEffects(t *testing.T) {
	t.Parallel()
	for _, provider := range []Provider{ProviderMicrosoft, ProviderApple} {
		t.Run(provider.String(), func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeJSON(w, 503, `{}`) })
			connection := oauthConnection(t, provider)
			if provider == ProviderApple {
				connection = appleConnection(t)
			}
			owned := meetIntent(t)
			resource := owned.ID.String()
			if provider == ProviderApple {
				resource = "https://caldav.icloud.com/calendar/" + resource + ".ics"
			}
			ref := EventRef{ID: owned.ID, Calendar: owned.Calendar, Provider: provider, Resource: resource}
			got, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: connection, Ref: ref, Intent: owned, ObservedAt: at(t, "2026-11-02T09:00:00Z"), Policy: fixturePolicy(t)})
			if !errors.Is(err, core.ErrTimeMatchContract) || got != (EventReceipt{}) || len(fixture.record()) != 0 {
				t.Fatalf("unsupported update=(%+v,%v),calls%d,want refusal/noeffects", got, err, len(fixture.record()))
			}
		})
	}
}

func TestGoogleMeetReadyReceiptDiscardsOtherEntryCredentials(t *testing.T) {
	t.Parallel()
	owned := meetIntent(t)
	wire := meetEvent(t, owned, googleConferenceSuccess)
	wire.ConferenceData.EntryPoints = append(wire.ConferenceData.EntryPoints, googleConferenceEntryPoint{Type: googleConferencePhone, URI: "tel:+14165550123,,123456"}, googleConferenceEntryPoint{Type: googleConferenceSIP, URI: "sip:private-code@meet.example.test"}, googleConferenceEntryPoint{Type: googleConferenceMore, URI: "https://example.test/private-phone-code"})
	ref := EventRef{ID: owned.ID, Calendar: owned.Calendar, Provider: ProviderGoogle, Resource: owned.ID.String()}
	got, err := verifyGoogle(seedJSON(t, wire), owned, ref)
	if err != nil || got.Venue != (core.TimeMatchAppointmentVenue{JoinURL: "https://meet.google.com/abc-defg-hij"}) {
		t.Fatalf("ready conference=(%+v,%v),want exact video-only venue", got, err)
	}
}
