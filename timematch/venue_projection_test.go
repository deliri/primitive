package timematch

import (
	"bytes"
	"errors"
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
