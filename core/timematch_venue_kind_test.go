package core

import (
	"errors"
	"testing"
)

func TestTimeMatchAutomaticVenueClosedDomain(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		kind := TimeMatchAppointmentVenueKind(raw)
		v := TimeMatchAppointmentVenue{Kind: kind}
		got := v.Validate()
		want := raw <= int(TimeMatchVenueAutomaticVideo)
		if (got == nil) != want || !want && !errors.Is(got, ErrTimeMatchContract) {
			t.Fatalf("venue kind%d admission%v,want admitted%t", raw, got, want)
		}
	}
	for _, v := range []TimeMatchAppointmentVenue{{Kind: TimeMatchVenueAutomaticVideo, Address: "Business address"}, {Kind: TimeMatchVenueAutomaticVideo, JoinURL: "https://meet.google.com/abc-defg-hij"}, {Kind: TimeMatchVenueAutomaticVideo, Address: "Business address", JoinURL: "https://meet.google.com/abc-defg-hij"}} {
		if err := v.Validate(); !errors.Is(err, ErrTimeMatchContract) {
			t.Fatalf("competing generated venue%+v admitted%v,want refusal", v, err)
		}
	}
	for _, v := range []TimeMatchAppointmentVenue{{}, {Address: "Business address"}, {JoinURL: "https://video.example.test/join"}, {Kind: TimeMatchVenueAutomaticVideo}} {
		if err := v.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}
