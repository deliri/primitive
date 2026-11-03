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
		venue EventVenue
	}{
		{name: "physical business address", venue: EventVenue{Address: "Off Grid Software, 123 Main Street, Toronto, ON M5V 1A1, Canada"}},
		{name: "Zoom join link", venue: EventVenue{JoinURL: "https://zoom.us/j/123456789?pwd=meeting-passcode"}},
		{name: "Teams join link", venue: EventVenue{JoinURL: "https://teams.microsoft.com/l/meetup-join/meeting-id?context=booking"}},
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
	venue := EventVenue{Address: "Toronto\r\nATTENDEE:mailto:other@example.com"}
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
		venue  EventVenue
		change bool
		want   error
	}{
		{name: "physical destination retained", venue: EventVenue{Address: "123 Main St, Toronto, ON M5V 1A1, Canada"}},
		{name: "video destination retained", venue: EventVenue{JoinURL: "https://zoom.us/j/123456789?pwd=code"}},
		{name: "provider changed destination cannot confirm", venue: EventVenue{Address: "123 Main St, Toronto"}, change: true, want: core.ErrTimeMatchConflict},
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
				t.Fatalf("venue replay inserts%d calls%d, want exactly one write and actual owned readback", calendar.inserts, len(fixture.record()))
			}
		})
	}
}

func TestAppleVenueReplacementPreservesUnownedProperties(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		next      EventVenue
		wantVenue EventVenue
	}{
		{name: "physical move replaces old video URL", next: EventVenue{Address: "Café, 123 Main St; Toronto"}, wantVenue: EventVenue{Address: "Café, 123 Main St; Toronto"}},
		{name: "new video link replaces old video URL", next: EventVenue{JoinURL: "https://teams.microsoft.com/l/meetup-join/new"}, wantVenue: EventVenue{JoinURL: "https://teams.microsoft.com/l/meetup-join/new"}},
		{name: "unspecified venue leaves existing destination untouched", wantVenue: EventVenue{JoinURL: "https://zoom.us/j/original"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := intent(t, "primary", "Consultation", "2026-11-02T10:00:00Z", "2026-11-02T10:30:00Z")
			owned.Venue = EventVenue{JoinURL: "https://zoom.us/j/original"}
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
