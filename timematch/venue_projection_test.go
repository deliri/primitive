package timematch

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"

	json "encoding/json/v2"
	"github.com/deliri/primitive/v2026/core"
)

// Real typed provider writers must carry the organizer's destination. These
// direct wire tests are not a claim that a live phone rendered a directions UI.
func TestOwnedAppointmentWritersCarryVenue(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		venue core.TimeMatchAppointmentVenue
	}{
		{name: "physical business address", venue: core.TimeMatchAppointmentVenue{Address: "Off Grid Software, 123 Main Street, Toronto, ON M5V 1A1, Canada"}},
		{name: "Zoom join link", venue: core.TimeMatchAppointmentVenue{JoinURL: "https://zoom.us/j/123456789?pwd=meeting-passcode"}},
		{name: "Teams join link", venue: core.TimeMatchAppointmentVenue{JoinURL: "https://teams.microsoft.com/l/meetup-join/meeting-id?context=booking"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := intent(t, "primary", "Consultation", "2026-11-02T10:00:00Z", "2026-11-02T10:30:00Z")
			owned.Venue = tc.venue
			if err := owned.Validate(); err != nil {
				t.Fatal(err)
			}
			google, err := googleEvent(owned)
			if err != nil {
				t.Fatal(err)
			}
			var googleWire struct {
				Location string `json:"location"`
			}
			if err := json.Unmarshal(google, &googleWire); err != nil || googleWire.Location != tc.venue.Text() {
				t.Fatalf("Google event location = (%q,%v), want %q", googleWire.Location, err, tc.venue.Text())
			}
			graph, err := graphEvent(owned, true)
			if err != nil {
				t.Fatal(err)
			}
			var graphWire struct {
				Location struct {
					DisplayName string `json:"displayName"`
					LocationURI string `json:"locationUri"`
				} `json:"location"`
			}
			if err := json.Unmarshal(graph, &graphWire); err != nil || graphWire.Location.DisplayName != tc.venue.Text() || graphWire.Location.LocationURI != tc.venue.JoinURL {
				t.Fatalf("Graph event venue = (%+v,%v), want organizer destination and optional join URL", graphWire.Location, err)
			}
			calendar, err := writeICS(owned, at(t, "2026-11-01T10:00:00Z"))
			if err != nil {
				t.Fatal(err)
			}
			unfolded := strings.Join(unfoldICS(string(calendar)), "\n")
			if !strings.Contains(unfolded, "LOCATION:"+escapeICSText(tc.venue.Text())+"\n") {
				t.Fatalf("owned iCalendar = %q, want exact escaped destination", unfolded)
			}
			if tc.venue.JoinURL != "" && !strings.Contains(unfolded, "URL:"+tc.venue.JoinURL+"\n") {
				t.Fatalf("video iCalendar = %q, want supplied join URL", unfolded)
			}
			invitation := invitationFixture(t)
			invitation.Venue = tc.venue
			var message bytes.Buffer
			if err := WriteInvitation(&message, invitation); err != nil {
				t.Fatal(err)
			}
			invitationLines := strings.Join(unfoldICS(message.String()), "\n")
			if !strings.Contains(invitationLines, "LOCATION:"+escapeICSText(tc.venue.Text())+"\n") || tc.venue.JoinURL != "" && !strings.Contains(invitationLines, "URL:"+tc.venue.JoinURL+"\n") {
				t.Fatalf("calendar invitation = %q, want organizer address or join link", invitationLines)
			}

		})
	}
}

func TestVenueRefusalEmitsNoInvitation(t *testing.T) {
	t.Parallel()
	// Existing typed invitation fixture is built by the owning writer tests.
	venue := core.TimeMatchAppointmentVenue{Address: "Toronto\r\nATTENDEE:mailto:other@example.com"}
	if err := venue.Validate(); !errors.Is(err, core.ErrTimeMatchContract) {
		t.Fatalf("venue injection error = %v, want typed refusal", err)
	}
	invitation := invitationFixture(t)
	if err := invitation.Validate(); err != nil {
		t.Fatal(err)
	}
	invitation.Venue = venue
	var destination bytes.Buffer
	err := WriteInvitation(&destination, invitation)
	if !errors.Is(err, core.ErrTimeMatchContract) || destination.Len() != 0 {
		t.Fatalf("rejected invitation = (%d bytes,%v), want no output and typed refusal", destination.Len(), err)
	}
}

func TestGoogleOwnedVenueNativeWriteAndReadback(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		venue  core.TimeMatchAppointmentVenue
		change bool
		want   error
	}{
		{name: "physical destination retained", venue: core.TimeMatchAppointmentVenue{Address: "123 Main St, Toronto, ON M5V 1A1, Canada"}},
		{name: "video destination retained", venue: core.TimeMatchAppointmentVenue{JoinURL: "https://zoom.us/j/123456789?pwd=code"}},
		{name: "provider changed destination cannot confirm", venue: core.TimeMatchAppointmentVenue{Address: "123 Main St, Toronto"}, change: true, want: core.ErrTimeMatchConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := intent(t, "primary", "Consultation", "2026-11-02T10:00:00Z", "2026-11-02T10:30:00Z")
			owned.Venue = tc.venue
			calendar := &googleCalendar{events: map[string]string{}}
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
				if tc.change && r.Method == http.MethodPost {
					body = bytes.ReplaceAll(body, []byte(tc.venue.Text()), []byte("Different business address"))
				}
				calendar.serve(w, r, body)
			})
			request := eventWrite(t, oauthConnection(t, ProviderGoogle), owned)
			got, err := client.PutEvent(t.Context(), request)
			if !errors.Is(err, tc.want) || err == nil && got.Ref.ID != owned.ID {
				t.Fatalf("native owned venue = (%+v,%v), want error %v with exact ownership on success", got, err, tc.want)
			}
			if tc.want != nil {
				return
			}
			request.Attempt = EventCreateRecoverOnly
			again, err := client.PutEvent(t.Context(), request)
			if err != nil || again != got {
				t.Fatalf("native venue recovery = (%+v,%v), want unchanged owned receipt", again, err)
			}
			if calendar.inserts != 1 || len(fixture.record()) < 3 {
				t.Fatalf("venue replay inserts%d calls%d, want exactly one write and native owned readback", calendar.inserts, len(fixture.record()))
			}
		})
	}
}

func TestAppleVenueReplacementPreservesUnownedProperties(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		next      core.TimeMatchAppointmentVenue
		wantVenue core.TimeMatchAppointmentVenue
	}{
		{name: "physical move replaces old video URL", next: core.TimeMatchAppointmentVenue{Address: "Café, 123 Main St; Toronto"}, wantVenue: core.TimeMatchAppointmentVenue{Address: "Café, 123 Main St; Toronto"}},
		{name: "new video link replaces old video URL", next: core.TimeMatchAppointmentVenue{JoinURL: "https://teams.microsoft.com/l/meetup-join/new"}, wantVenue: core.TimeMatchAppointmentVenue{JoinURL: "https://teams.microsoft.com/l/meetup-join/new"}},
		{name: "unspecified venue leaves existing destination untouched", wantVenue: core.TimeMatchAppointmentVenue{JoinURL: "https://zoom.us/j/original"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := intent(t, "primary", "Consultation", "2026-11-02T10:00:00Z", "2026-11-02T10:30:00Z")
			owned.Venue = core.TimeMatchAppointmentVenue{JoinURL: "https://zoom.us/j/original"}
			original, err := writeICS(owned, at(t, "2026-11-01T10:00:00Z"))
			if err != nil {
				t.Fatal(err)
			}
			original = bytes.Replace(original, []byte("END:VEVENT"), []byte("DESCRIPTION:Keep user notes\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nDESCRIPTION:Keep reminder\r\nTRIGGER:-PT15M\r\nEND:VALARM\r\nEND:VEVENT"), 1)
			owned.Venue = tc.next
			replacement, err := writeICS(owned, at(t, "2026-11-01T11:00:00Z"))
			if err != nil {
				t.Fatal(err)
			}
			got, err := appleUpdatedResource(original, replacement)
			if err != nil {
				t.Fatal(err)
			}
			events, err := parseICS(string(got))
			if err != nil || len(events) != 1 || events[0].location != tc.wantVenue.Text() || events[0].joinURL != tc.wantVenue.JoinURL {
				t.Fatalf("owned venue replacement = (%+v,%v), want exact destination %+v", events, err, tc.wantVenue)
			}
			if !bytes.Contains(got, []byte("DESCRIPTION:Keep user notes")) || !bytes.Contains(got, []byte("DESCRIPTION:Keep reminder")) || !bytes.Contains(got, []byte("TRIGGER:-PT15M")) {
				t.Fatalf("owned replacement = %q, want existing notes and alarm conserved", got)
			}
		})
	}
}

func TestMicrosoftOwnedVenueNativeWrite(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		venue     core.TimeMatchAppointmentVenue
		alterName bool
		alterURI  bool
		omit      bool
		want      error
	}{
		{name: "physical location echoed", venue: core.TimeMatchAppointmentVenue{Address: "123 Main St, Toronto, Canada"}},
		{name: "video join URI echoed", venue: core.TimeMatchAppointmentVenue{JoinURL: "https://teams.microsoft.com/l/meetup-join/meeting"}},
		{name: "changed physical location refuses confirmation", venue: core.TimeMatchAppointmentVenue{Address: "123 Main St"}, alterName: true, want: core.ErrTimeMatchConflict},
		{name: "changed video URI refuses confirmation", venue: core.TimeMatchAppointmentVenue{JoinURL: "https://zoom.us/j/123"}, alterURI: true, want: core.ErrTimeMatchConflict},
		{name: "missing destination refuses confirmation", venue: core.TimeMatchAppointmentVenue{Address: "123 Main St"}, omit: true, want: core.ErrTimeMatchConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, body []byte) {
				var observed struct {
					graphEventBody
					ID string `json:"id"`
				}
				if err := json.Unmarshal(body, &observed); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				observed.ID = "AAMk-owned-event"
				if observed.Location == nil {
					t.Error("submitted venue = nil, want organizer destination")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if tc.alterName {
					observed.Location.DisplayName = "Different business"
				}
				if tc.alterURI {
					observed.Location.URI = "https://video.example.test/other"
				}
				if tc.omit {
					observed.Location = nil
				}
				encoded, err := json.Marshal(observed)
				if err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				writeJSON(w, http.StatusCreated, string(encoded))
			})
			owned := intent(t, "cal-1", "Consultation", "2026-11-02T10:00:00Z", "2026-11-02T10:30:00Z")
			owned.Venue = tc.venue
			got, err := client.PutEvent(t.Context(), eventWrite(t, oauthConnection(t, ProviderMicrosoft), owned))
			if !errors.Is(err, tc.want) || err != nil && got != (EventReceipt{}) {
				t.Fatalf("Graph venue response = (%+v,%v), want error %v with no partial receipt", got, err, tc.want)
			}
			calls := fixture.record()
			if len(calls) != 1 || calls[0].method != http.MethodPost || strings.Contains(string(calls[0].body), "onlineMeetingUrl") || strings.Contains(string(calls[0].body), "attendees") {
				t.Fatalf("Graph venue calls = %+v, want one owned create without read-only conferencing fields or attendees", calls)
			}
		})
	}
}

func TestAppleOwnedVenueNativeReadback(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		venue   core.TimeMatchAppointmentVenue
		changed bool
		want    error
	}{
		{name: "physical destination recovered", venue: core.TimeMatchAppointmentVenue{Address: "123 Main St; Toronto, Canada"}},
		{name: "video destination recovered", venue: core.TimeMatchAppointmentVenue{JoinURL: "https://zoom.us/j/123?pwd=join"}},
		{name: "different location is not the same intent", venue: core.TimeMatchAppointmentVenue{Address: "123 Main St"}, changed: true, want: core.ErrTimeMatchConflict},
		{name: "different join link is not the same intent", venue: core.TimeMatchAppointmentVenue{JoinURL: "https://teams.microsoft.com/l/meetup-join/meeting"}, changed: true, want: core.ErrTimeMatchConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := intent(t, appleMutationHome, "Consultation", "2026-11-02T10:00:00Z", "2026-11-02T10:30:00Z")
			owned.Venue = tc.venue
			observed := owned
			if tc.changed {
				if observed.Venue.Address != "" {
					observed.Venue.Address = "Other business address"
				} else {
					observed.Venue.JoinURL = "https://video.example.test/other"
				}
			}
			raw, err := writeICS(observed, at(t, "2026-11-01T10:00:00Z"))
			if err != nil {
				t.Fatal(err)
			}
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method != http.MethodGet {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				w.Header().Set("Content-Type", "text/calendar")
				if _, err := w.Write(raw); err != nil {
					t.Error(err)
				}
			})
			request := eventWrite(t, appleConnection(t), owned)
			request.Attempt = EventCreateRecoverOnly
			got, err := client.PutEvent(t.Context(), request)
			if !errors.Is(err, tc.want) || err != nil && got != (EventReceipt{}) {
				t.Fatalf("Apple venue readback = (%+v,%v), want error %v with no partial receipt", got, err, tc.want)
			}
			calls := fixture.record()
			if len(calls) != 1 || calls[0].method != http.MethodGet || !strings.HasSuffix(calls[0].path, "/"+owned.ID.value+".ics") {
				t.Fatalf("Apple venue recovery calls = %+v, want one read of the exact owned child", calls)
			}
		})
	}
}
