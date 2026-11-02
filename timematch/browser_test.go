package timematch

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Layers: provider facts (Availability) -> aggregate projection (Browser)
// -> wire document (MarshalJSON) -> admitted ingress (UnmarshalJSON).
func TestBrowserProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	w := window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z")
	t.Run("positive known calendars merge into one exact document", func(t *testing.T) {
		t.Parallel()
		availability := Availability{Window: w, Calendars: []CalendarAvailability{
			{ID: CalendarID{value: "a"}, State: AvailabilityKnown, Busy: []temporal.IntervalBounds{window(t, "2026-11-02T10:00:00Z", "2026-11-02T11:00:00Z")}},
			{ID: CalendarID{value: "b"}, State: AvailabilityKnown, Busy: []temporal.IntervalBounds{window(t, "2026-11-02T10:30:00Z", "2026-11-02T12:00:00Z")}},
		}}
		projected, err := availability.Browser()
		encoded, encodeErr := projected.MarshalJSON()
		want := `{"start":"2026-11-02T09:00:00Z","end":"2026-11-02T17:00:00Z","state":"known","busy":[{"start":"2026-11-02T10:00:00Z","end":"2026-11-02T12:00:00Z"}]}`
		if err != nil || encodeErr != nil || string(encoded) != want {
			t.Fatalf("Browser().MarshalJSON() = (%s, %v, %v), want %s", encoded, err, encodeErr, want)
		}
		var admitted BrowserAvailability
		if err := admitted.UnmarshalJSON(encoded); err != nil || admitted.State != AvailabilityKnown || len(admitted.Busy) != 1 {
			t.Fatalf("UnmarshalJSON(projection) = (%+v, %v), want the same known projection", admitted, err)
		}
	})
	t.Run("negative invalid facts and hostile documents are refused at every layer", func(t *testing.T) {
		t.Parallel()
		invalid := Availability{Window: w, Calendars: []CalendarAvailability{{ID: CalendarID{value: "a"}, State: AvailabilityUnknown, Busy: []temporal.IntervalBounds{window(t, "2026-11-02T10:00:00Z", "2026-11-02T11:00:00Z")}}}}
		if _, err := invalid.Browser(); !errors.Is(err, core.ErrTimeMatchContract) {
			t.Fatalf("Browser(unknown with busy) error = %v, want %v", err, core.ErrTimeMatchContract)
		}
		if _, err := (BrowserAvailability{Window: w, State: AvailabilityKnown, Busy: []temporal.IntervalBounds{window(t, "2026-11-02T08:00:00Z", "2026-11-02T10:00:00Z")}}).MarshalJSON(); !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("MarshalJSON(busy outside window) error = %v, want %v", err, core.ErrJSONContract)
		}
		for _, document := range []string{
			`{"start":"2026-11-02T09:00:00Z","end":"2026-11-02T17:00:00Z","state":"unknown","busy":[{"start":"2026-11-02T10:00:00Z","end":"2026-11-02T11:00:00Z"}]}`,
			`{"start":"2026-11-02T09:00:00Z","end":"2026-11-02T17:00:00Z","state":"free","busy":[]}`,
			`{"start":"2026-11-02T09:00:00Z","end":"2026-11-02T17:00:00Z","state":"known","busy":[],"title":"x"}`,
			`{"start":"2026-11-02T17:00:00Z","end":"2026-11-02T09:00:00Z","state":"known","busy":[]}`,
		} {
			var got BrowserAvailability
			if err := got.UnmarshalJSON([]byte(document)); !errors.Is(err, core.ErrJSONContract) || got.Window.Start.IsSet() {
				t.Fatalf("UnmarshalJSON(%s) = (%+v, %v), want refusal and zero receiver", document, got, err)
			}
		}
	})
	t.Run("neutral any unknown calendar yields unknown without busy facts", func(t *testing.T) {
		t.Parallel()
		availability := Availability{Window: w, Calendars: []CalendarAvailability{
			{ID: CalendarID{value: "a"}, State: AvailabilityKnown, Busy: []temporal.IntervalBounds{window(t, "2026-11-02T10:00:00Z", "2026-11-02T11:00:00Z")}},
			{ID: CalendarID{value: "b"}, State: AvailabilityUnknown},
		}}
		projected, err := availability.Browser()
		encoded, encodeErr := projected.MarshalJSON()
		want := `{"start":"2026-11-02T09:00:00Z","end":"2026-11-02T17:00:00Z","state":"unknown","busy":[]}`
		if err != nil || encodeErr != nil || string(encoded) != want {
			t.Fatalf("Browser(unknown).MarshalJSON() = (%s, %v, %v), want %s", encoded, err, encodeErr, want)
		}
	})
}

func TestBrowserIngressDocuments(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		decode  func([]byte) error
		input   string
		wantErr bool
	}{
		{name: "availability request window", decode: func(b []byte) error { var r BrowserAvailabilityRequest; return r.UnmarshalJSON(b) }, input: `{"start":"2026-11-02T09:00:00Z","end":"2026-11-02T17:00:00-05:00"}`},
		{name: "availability request empty window", decode: func(b []byte) error { var r BrowserAvailabilityRequest; return r.UnmarshalJSON(b) }, input: `{"start":"2026-11-02T09:00:00Z","end":"2026-11-02T09:00:00Z"}`, wantErr: true},
		{name: "availability request extra member", decode: func(b []byte) error { var r BrowserAvailabilityRequest; return r.UnmarshalJSON(b) }, input: `{"start":"2026-11-02T09:00:00Z","end":"2026-11-02T10:00:00Z","calendar":"a"}`, wantErr: true},
		{name: "connection google", decode: func(b []byte) error { var r BrowserConnectionRequest; return r.UnmarshalJSON(b) }, input: `{"provider":"google"}`},
		{name: "connection apple uses its own route", decode: func(b []byte) error { var r BrowserConnectionRequest; return r.UnmarshalJSON(b) }, input: `{"provider":"apple"}`, wantErr: true},
		{name: "connection uppercase provider", decode: func(b []byte) error { var r BrowserConnectionRequest; return r.UnmarshalJSON(b) }, input: `{"provider":"Google"}`, wantErr: true},
		{name: "consent location on microsoft", decode: func(b []byte) error { var r BrowserConnectionStart; return r.UnmarshalJSON(b) }, input: `{"location":"https://login.microsoftonline.com/common/oauth2/v2.0/authorize?x=1"}`},
		{name: "consent location on a look-alike host", decode: func(b []byte) error { var r BrowserConnectionStart; return r.UnmarshalJSON(b) }, input: `{"location":"https://accounts.google.com.attacker.example/o/oauth2/v2/auth"}`, wantErr: true},
		{name: "consent location over http", decode: func(b []byte) error { var r BrowserConnectionStart; return r.UnmarshalJSON(b) }, input: `{"location":"http://accounts.google.com/o/oauth2/v2/auth"}`, wantErr: true},
		{name: "apple connection", decode: func(b []byte) error { var r BrowserAppleConnection; return r.UnmarshalJSON(b) }, input: `{"account":"a@icloud.com","password":"abcd-efgh-ijkl-mnop"}`},
		{name: "apple connection with space in password", decode: func(b []byte) error { var r BrowserAppleConnection; return r.UnmarshalJSON(b) }, input: `{"account":"a@icloud.com","password":"abcd efgh"}`, wantErr: true},
		{name: "apple connection numeric password", decode: func(b []byte) error { var r BrowserAppleConnection; return r.UnmarshalJSON(b) }, input: `{"account":"a@icloud.com","password":1234}`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.decode([]byte(tc.input))
			if tc.wantErr != (err != nil) || err != nil && !errors.Is(err, core.ErrJSONContract) && !errors.Is(err, core.ErrTimeMatchContract) {
				t.Fatalf("decode(%s) error = %v, want error %t with typed identity", tc.input, err, tc.wantErr)
			}
		})
	}
}
