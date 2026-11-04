package timematch

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoogleMeetExactHTTPSDestination(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, uri string
		want      bool
	}{
		{name: "owned HTTPS video URI", uri: "https://meet.google.com/abc-defg-hij", want: true},
		{name: "foreign HTTPS origin", uri: "https://video.example.test/abc-defg-hij"},
		{name: "host suffix injection", uri: "https://meet.google.com.evil.test/abc-defg-hij"},
		{name: "insecure Meet origin", uri: "http://meet.google.com/abc-defg-hij"},
		{name: "userinfo cannot change navigation identity", uri: "https://person@meet.google.com/abc-defg-hij"},
		{name: "explicit port cannot change destination", uri: "https://meet.google.com:8443/abc-defg-hij"},
		{name: "query cannot carry another destination", uri: "https://meet.google.com/abc-defg-hij?next=https://evil.test"},
		{name: "empty query marker cannot change URI", uri: "https://meet.google.com/abc-defg-hij?"},
		{name: "fragment cannot carry a browser capability", uri: "https://meet.google.com/abc-defg-hij#secret"},
		{name: "host alone does not name a meeting", uri: "https://meet.google.com"},
		{name: "root path does not name a meeting", uri: "https://meet.google.com/"},
		{name: "empty URI cannot become a venue"},
		{name: "opaque URI has no owned authority", uri: "https:meet.google.com/abc-defg-hij"},
		{name: "calendar line injection is refused", uri: "https://meet.google.com/abc-defg-hij\r\nATTENDEE:foreign"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := meetIntent(t)
			event := meetEvent(t, owned, googleConferenceSuccess)
			event.ConferenceData.EntryPoints[0].URI = tc.uri
			ref := EventRef{ID: owned.ID, Calendar: owned.Calendar, Provider: ProviderGoogle, Resource: owned.ID.String()}
			got, err := verifyGoogle(seedJSON(t, event), owned, ref)
			if tc.want {
				if err != nil || got.VenueStatus != EventVenueReady || got.Venue.JoinURL != tc.uri {
					t.Fatalf("admitted video=(%+v,%v),want exactURI%q", got, err, tc.uri)
				}
			} else if !errors.Is(err, core.ErrTimeMatchIndeterminate) || got != (EventReceipt{}) {
				t.Fatalf("rejected video=(%+v,%v),want zero/unknown", got, err)
			}
		})
	}
}

func TestGoogleMeetReceiptClosedOutcomes(t *testing.T) {
	t.Parallel()
	owned := meetIntent(t)
	ref := EventRef{ID: owned.ID, Calendar: owned.Calendar, Provider: ProviderGoogle, Resource: owned.ID.String()}
	for raw := range 256 {
		status := EventVenueStatus(raw)
		r := EventReceipt{Ref: ref, VenueStatus: status}
		if status == EventVenueReady {
			r.Venue = core.TimeMatchAppointmentVenue{JoinURL: "https://meet.google.com/abc-defg-hij"}
		}
		got := r.Validate()
		want := raw <= int(EventVenueFailed)
		if (got == nil) != want || !want && !errors.Is(got, core.ErrTimeMatchContract) {
			t.Fatalf("receipt status%d admission%v,want accepted%t", raw, got, want)
		}
	}
	for _, status := range []EventVenueStatus{EventVenueNone, EventVenuePending, EventVenueFailed} {
		r := EventReceipt{Ref: ref, VenueStatus: status, Venue: core.TimeMatchAppointmentVenue{JoinURL: "https://meet.google.com/abc-defg-hij"}}
		if err := r.Validate(); !errors.Is(err, core.ErrTimeMatchContract) {
			t.Fatalf("unready receipt%+v admitted%v,want refusal", r, err)
		}
	}
	for _, kind := range []core.TimeMatchAppointmentVenueKind{core.TimeMatchVenueAutomaticVideo, 255} {
		r := EventReceipt{Ref: ref, VenueStatus: EventVenueReady, Venue: core.TimeMatchAppointmentVenue{Kind: kind}}
		if err := r.Validate(); !errors.Is(err, core.ErrTimeMatchContract) {
			t.Fatalf("ready unresolved venue%+v admitted%v,want refusal", r, err)
		}
	}
}

func TestGoogleMeetRequestIdentityUsesExistingClosedEventDomain(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"00000", "vvvvv", strings.Repeat("a", eventIDMaximumBytes)} {
		owned := meetIntent(t)
		id, err := ParseEventID(text)
		if err != nil {
			t.Fatal(err)
		}
		owned.ID = id
		body, err := googleEvent(owned)
		if err != nil {
			t.Fatal(err)
		}
		var got googleEventBody
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if got.ConferenceData.CreateRequest.RequestID != text {
			t.Fatalf("conference requestID=%q,want exact%q", got.ConferenceData.CreateRequest.RequestID, text)
		}
	}
	one := meetIntent(t)
	two := one
	id, err := ParseEventID("other01")
	if err != nil {
		t.Fatal(err)
	}
	two.ID = id
	if googleMeetCreateData(one).CreateRequest.RequestID == googleMeetCreateData(two).CreateRequest.RequestID {
		t.Fatal("distinct events share one conference request identity")
	}
}

// Real typed event encoder seeds the external provider-response boundary. The
// oracle binds admitted facts independently to the intended event, request,
// generation outcome and exact HTTPS origin; canonical replay preserves them.
func FuzzGoogleMeetOwnedConferenceSemanticClosure(f *testing.F) {
	owned := meetIntent(f)
	ref := EventRef{ID: owned.ID, Calendar: owned.Calendar, Provider: ProviderGoogle, Resource: owned.ID.String()}
	for _, status := range []string{googleConferencePending, googleConferenceSuccess, googleConferenceFailure} {
		f.Add(seedJSON(f, meetEvent(f, owned, status)))
	}
	f.Add([]byte(`null`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"conferenceData":{"createRequest":{"status":{"statusCode":"future"}}}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := verifyGoogle(bytes.Clone(data), owned, ref)
		if err != nil {
			if got != (EventReceipt{}) || !errors.Is(err, core.ErrTimeMatchConflict) && !errors.Is(err, core.ErrTimeMatchIndeterminate) {
				t.Fatalf("rejected conference=(%+v,%v),want zero/typed conflict or unknown", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil || got.Ref != ref || got.VenueStatus == EventVenueNone {
			t.Fatalf("accepted conference=(%+v,%v),want validated exact owned effect", got, err)
		}
		var source googleMeetTestEvent
		if err := json.Unmarshal(data, &source); err != nil {
			t.Fatal(err)
		}
		if source.ID != owned.ID.String() || source.ExtendedProperties == nil || source.ExtendedProperties.Private.EventID != owned.ID.String() || source.ConferenceData == nil || source.ConferenceData.CreateRequest == nil || source.ConferenceData.CreateRequest.RequestID != owned.ID.String() {
			t.Fatal("accepted conference has foreign event/request binding")
		}
		status := source.ConferenceData.CreateRequest.Status.Code
		if got.VenueStatus == EventVenuePending && status != "pending" || got.VenueStatus == EventVenueFailed && status != "failure" {
			t.Fatal("unready outcome changed provider generation status")
		}
		if got.VenueStatus == EventVenueReady {
			u, err := url.Parse(got.Venue.JoinURL)
			if err != nil || u.Scheme != "https" || u.Host != "meet.google.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
				t.Fatalf("accepted URI%q violates exact Meet origin", got.Venue.JoinURL)
			}
			matches := 0
			for _, e := range source.ConferenceData.EntryPoints {
				if e.Type == "video" && e.URI == got.Venue.JoinURL {
					matches++
				}
			}
			if matches != 1 || source.ConferenceData.CreateRequest.Status.Code != "success" {
				t.Fatal("ready result lacks one exact successful provider video entry")
			}
		} else if got.Venue != (core.TimeMatchAppointmentVenue{}) {
			t.Fatal("unready result exposed a call URI")
		}
		canonical, err := json.Marshal(source)
		if err != nil {
			t.Fatal(err)
		}
		again, err := verifyGoogle(bytes.Clone(canonical), owned, ref)
		if err != nil || again != got {
			t.Fatalf("canonical provider replay=(%+v,%v),want %+v", again, err, got)
		}
		var round googleMeetTestEvent
		if err := json.Unmarshal(canonical, &round); err != nil {
			t.Fatal(err)
		}
		second, err := json.Marshal(round)
		if err != nil || !bytes.Equal(canonical, second) {
			t.Fatal("provider canonical projection changed on second encoding")
		}
	})
}
