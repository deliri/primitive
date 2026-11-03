package timematch

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	json "encoding/json/v2"
	"github.com/deliri/primitive/v2026/core"
)

func TestEventVenueHostileAdmission(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		venue EventVenue
		want  error
	}{
		{name: "unspecified SDK venue remains neutral"},
		{name: "complete Canadian destination", venue: EventVenue{Address: "123 Main St, Toronto, ON M5V 1A1, Canada"}},
		{name: "accented business destination", venue: EventVenue{Address: "Café Montréal, 1 rue Saint-Paul"}},
		{name: "non-Latin destination", venue: EventVenue{Address: "東京都千代田区丸の内1丁目"}},
		{name: "venue with suite delimiter", venue: EventVenue{Address: "Clinic; Suite 204, 5 Main St"}},
		{name: "venue with literal backslash", venue: EventVenue{Address: `North\South Hall, 5 Main St`}},
		{name: "Zoom query retained", venue: EventVenue{JoinURL: "https://zoom.us/j/123456789?pwd=secret"}},
		{name: "Teams escaped path retained", venue: EventVenue{JoinURL: "https://teams.microsoft.com/l/meetup-join/19%3ameeting?context=%7B%7D"}},
		{name: "HTTPS port retained", venue: EventVenue{JoinURL: "https://video.example.test:8443/join"}},
		{name: "HTTPS fragment retained", venue: EventVenue{JoinURL: "https://video.example.test/join#room"}},
		{name: "address and call link cannot compete", venue: EventVenue{Address: "123 Main St", JoinURL: "https://zoom.us/j/1"}, want: core.ErrTimeMatchContract},
		{name: "address leading space refused", venue: EventVenue{Address: " 123 Main St"}, want: core.ErrTimeMatchContract},
		{name: "address trailing space refused", venue: EventVenue{Address: "123 Main St "}, want: core.ErrTimeMatchContract},
		{name: "calendar property CR injection refused", venue: EventVenue{Address: "Clinic\rATTENDEE:other"}, want: core.ErrTimeMatchContract},
		{name: "calendar property LF injection refused", venue: EventVenue{Address: "Clinic\nATTENDEE:other"}, want: core.ErrTimeMatchContract},
		{name: "address embedded tab refused", venue: EventVenue{Address: "Clinic\t1"}, want: core.ErrTimeMatchContract},
		{name: "address null byte refused", venue: EventVenue{Address: "Clinic\x00"}, want: core.ErrTimeMatchContract},
		{name: "address malformed UTF8 refused", venue: EventVenue{Address: string([]byte{'A', 0xff})}, want: core.ErrTimeMatchContract},
		{name: "insecure call link refused", venue: EventVenue{JoinURL: "http://zoom.us/j/1"}, want: core.ErrTimeMatchContract},
		{name: "credential-bearing link refused", venue: EventVenue{JoinURL: "https://person:password@video.example.test/join"}, want: core.ErrTimeMatchContract},
		{name: "one-byte address retained", venue: EventVenue{Address: "A"}},
		{name: "one-rune multibyte address retained", venue: EventVenue{Address: "館"}},
		{name: "Unicode DEL boundary refused", venue: EventVenue{Address: "Clinic\u007f"}, want: core.ErrTimeMatchContract},
		{name: "Unicode C1 control boundary refused", venue: EventVenue{Address: "Clinic\u0080"}, want: core.ErrTimeMatchContract},
		{name: "space alone is not a destination", venue: EventVenue{Address: " "}, want: core.ErrTimeMatchContract},
		{name: "nonbreaking outer whitespace refused", venue: EventVenue{Address: "\u00a0Clinic"}, want: core.ErrTimeMatchContract},
		{name: "Unicode separator outer whitespace refused", venue: EventVenue{Address: "Clinic\u2003"}, want: core.ErrTimeMatchContract},
		{name: "long valid destination spans ICS physical windows", venue: EventVenue{Address: strings.Repeat("Café, Suite 10; ", 512) + "Toronto"}},
		{name: "empty video host refused", venue: EventVenue{JoinURL: "https:///join"}, want: core.ErrTimeMatchContract},
		{name: "scheme-relative URL refused", venue: EventVenue{JoinURL: "//zoom.us/j/1"}, want: core.ErrTimeMatchContract},
		{name: "relative meeting path refused", venue: EventVenue{JoinURL: "/join/1"}, want: core.ErrTimeMatchContract},
		{name: "script URL refused", venue: EventVenue{JoinURL: "javascript:alert(1)"}, want: core.ErrTimeMatchContract},
		{name: "opaque HTTPS URL refused", venue: EventVenue{JoinURL: "https:join"}, want: core.ErrTimeMatchContract},
		{name: "user-only URL credentials refused", venue: EventVenue{JoinURL: "https://person@video.example.test/join"}, want: core.ErrTimeMatchContract},
		{name: "malformed percent escape refused", venue: EventVenue{JoinURL: "https://video.example.test/%zz"}, want: core.ErrTimeMatchContract},
		{name: "malformed URL port refused", venue: EventVenue{JoinURL: "https://video.example.test:port/join"}, want: core.ErrTimeMatchContract},
		{name: "IPv6 meeting endpoint retained", venue: EventVenue{JoinURL: "https://[2001:db8::1]/join"}},
		{name: "query separators remain URL data", venue: EventVenue{JoinURL: "https://video.example.test/join?a=1&b=2"}},
		{name: "encoded delimiters remain URL data", venue: EventVenue{JoinURL: "https://video.example.test/join/room%3Bname?key=a%2Cb"}},
		{name: "link CRLF injection refused", venue: EventVenue{JoinURL: "https://zoom.us/j/1\r\nATTENDEE:other"}, want: core.ErrTimeMatchContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.venue.Validate()
			if !errors.Is(got, tc.want) {
				t.Fatalf("EventVenue.Validate(%+v) error = %v, want %v", tc.venue, got, tc.want)
			}
			if got != nil {
				return
			}
			owned := intent(t, "primary", "Consultation", "2026-11-02T10:00:00Z", "2026-11-02T10:30:00Z")
			owned.Venue = tc.venue
			if err := owned.Validate(); err != nil {
				t.Fatalf("admitted venue intent error = %v, want nil", err)
			}
			calendar, err := writeICS(owned, at(t, "2026-11-01T10:00:00Z"))
			if err != nil {
				t.Fatal(err)
			}
			events, err := parseICS(string(calendar))
			if err != nil || len(events) != 1 || events[0].location != tc.venue.Text() || events[0].joinURL != tc.venue.JoinURL {
				t.Fatalf("venue ICS round trip = (%+v,%v), want exact destination %+v", events, err, tc.venue)
			}
		})
	}
}

func FuzzOwnedEventVenueSemanticConservation(f *testing.F) {
	for _, seed := range []EventVenue{{}, {Address: "Café, Suite 4; 123 Main St"}, {JoinURL: "https://zoom.us/j/123?pwd=a&b=2"}, {JoinURL: "https://teams.microsoft.com/l/meetup-join/19%3Ameeting"}} {
		if err := seed.Validate(); err != nil {
			f.Fatal(err)
		}
		f.Add(seed.Address, seed.JoinURL)
	}
	f.Add("Venue\r\nATTENDEE:other", "")
	f.Add("", "https://user:password@video.example.test/join")
	f.Fuzz(func(t *testing.T, address, joinURL string) {
		owned := intent(t, "primary", "Consultation", "2026-11-02T10:00:00Z", "2026-11-02T10:30:00Z")
		owned.Venue = EventVenue{Address: address, JoinURL: joinURL}
		before := owned
		admissionErr := owned.Validate()
		if admissionErr != nil {
			if !errors.Is(admissionErr, core.ErrTimeMatchContract) || owned != before {
				t.Fatalf("rejected venue = (%+v,%v), want original value and typed refusal", owned, admissionErr)
			}
			if data, err := writeICS(owned, at(t, "2026-11-01T10:00:00Z")); !errors.Is(err, core.ErrTimeMatchContract) || len(data) != 0 {
				t.Fatalf("rejected venue writer = (%d bytes,%v), want zero bytes and typed refusal", len(data), err)
			}
			return
		}
		google, err := googleEvent(owned)
		if err != nil {
			t.Fatal(err)
		}
		var g struct {
			Location string `json:"location"`
		}
		if err := json.Unmarshal(google, &g); err != nil || g.Location != owned.Venue.Text() {
			t.Fatalf("fuzzed Google venue = (%q,%v), want exact accepted destination", g.Location, err)
		}
		graph, err := graphEvent(owned, true)
		if err != nil {
			t.Fatal(err)
		}
		var m struct {
			Location *graphEventLocation `json:"location"`
		}
		if err := json.Unmarshal(graph, &m); err != nil {
			t.Fatal(err)
		}
		if owned.Venue.Text() == "" {
			if m.Location != nil {
				t.Fatalf("neutral Graph venue = %+v, want omitted", m.Location)
			}
		} else if m.Location == nil || m.Location.DisplayName != owned.Venue.Text() || m.Location.URI != joinURL {
			t.Fatalf("Graph venue = %+v, want exact accepted destination", m.Location)
		}
		first, err := writeICS(owned, at(t, "2026-11-01T10:00:00Z"))
		if err != nil {
			t.Fatal(err)
		}
		events, err := parseICS(string(first))
		if err != nil || len(events) != 1 || events[0].location != owned.Venue.Text() || events[0].joinURL != joinURL {
			t.Fatalf("fuzzed ICS venue = (%+v,%v), want exact accepted destination", events, err)
		}
		second, err := writeICS(owned, at(t, "2026-11-01T10:00:00Z"))
		if err != nil || !bytes.Equal(first, second) {
			t.Fatalf("second venue writer = (%v,identical%v), want stable canonical bytes", err, bytes.Equal(first, second))
		}
	})
}
