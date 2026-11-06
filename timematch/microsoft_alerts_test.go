package timematch

import (
	json "encoding/json/v2"
	"errors"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// A controlled Graph TLS peer observes the real Exchange POST. No live
// Microsoft calendar or notification participates in this proof.
func TestMicrosoftCalendarAlertNativeCreation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		alerts      EventAlerts
		wantMinutes int64
		wantEnabled bool
	}{
		{name: "owner selected half hour", alerts: alertOffsets(t, 30), wantMinutes: 30, wantEnabled: true},
		{name: "at appointment start", alerts: alertOffsets(t, 0), wantEnabled: true},
		{name: "four week maximum", alerts: alertOffsets(t, 40320), wantMinutes: 40320, wantEnabled: true},
		{name: "empty retains existing disabled create preference"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
				if r.Method != http.MethodPost {
					t.Errorf("method=%s, want POST", r.Method)
				}
				var created graphCreated
				if err := json.Unmarshal(body, &created); err != nil {
					t.Errorf("create decode=%v, want nil", err)
					writeJSON(w, 400, `{}`)
					return
				}
				created.ID = immutableFixtureID
				encoded, err := json.Marshal(created)
				if err != nil {
					t.Errorf("response encode=%v, want nil", err)
					writeJSON(w, 500, `{}`)
					return
				}
				writeJSON(w, 201, string(encoded))
			})
			owned := intent(t, "primary", "Consultation", "2026-11-06T15:00:00Z", "2026-11-06T15:15:00Z")
			write := eventWrite(t, oauthConnection(t, ProviderMicrosoft), owned)
			write.Alerts = tc.alerts
			got, err := client.PutEvent(t.Context(), write)
			requests := fixture.record()
			if err != nil || got.Ref.ID != owned.ID || len(requests) != 1 {
				t.Fatalf("create=(%+v,%v), requests=%d, want owned receipt and one POST", got, err, len(requests))
			}
			var sent struct {
				Enabled *bool  `json:"isReminderOn"`
				Minutes *int64 `json:"reminderMinutesBeforeStart"`
			}
			if err := json.Unmarshal(requests[0].body, &sent); err != nil {
				t.Fatal(err)
			}
			if sent.Enabled == nil || *sent.Enabled != tc.wantEnabled {
				t.Fatalf("reminder=%+v, want enabled=%t", sent, tc.wantEnabled)
			}
			if tc.wantEnabled {
				if sent.Minutes == nil || *sent.Minutes != tc.wantMinutes {
					t.Fatalf("reminder=%+v, want minutes=%d", sent, tc.wantMinutes)
				}
			} else if sent.Minutes != nil {
				t.Fatalf("minutes=%d, want omitted with disabled reminder", *sent.Minutes)
			}
		})
	}
}

func TestMicrosoftCalendarAlertRejectsUnsupportedPolicyBeforeHTTP(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		alerts EventAlerts
	}{
		{name: "two reminders cannot be silently truncated", alerts: alertOffsets(t, 1440, 30)},
		{name: "duplicate reminder", alerts: alertOffsets(t, 30, 30)},
		{name: "beyond bounded lead", alerts: alertOffsets(t, 40321)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeJSON(w, 503, `{}`) })
			owned := intent(t, "primary", "Consultation", "2026-11-06T15:00:00Z", "2026-11-06T15:15:00Z")
			write := eventWrite(t, oauthConnection(t, ProviderMicrosoft), owned)
			write.Alerts = tc.alerts
			got, err := client.PutEvent(t.Context(), write)
			if !errors.Is(err, core.ErrTimeMatchContract) || got != (EventReceipt{}) || len(fixture.record()) != 0 {
				t.Fatalf("create=(%+v,%v), requests=%d, want typed refusal and no HTTP", got, err, len(fixture.record()))
			}
		})
	}
}

func TestCalendarAlertsExhaustiveProviderCardinality(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		provider Provider
		maximum  int
	}{
		{name: "Google five native overrides", provider: ProviderGoogle, maximum: 5},
		{name: "Microsoft one native reminder", provider: ProviderMicrosoft, maximum: 1},
		{name: "Apple no supported override", provider: ProviderApple, maximum: 0},
		{name: "zero provider never admitted", maximum: -1},
		{name: "future provider never admitted", provider: Provider(255), maximum: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for count := 0; count <= 6; count++ {
				alerts := make(EventAlerts, count)
				for index := range alerts {
					alerts[index] = alertOffsets(t, uint64(index))[0]
				}
				err := alerts.ValidateForProvider(tc.provider)
				if (count <= tc.maximum && err != nil) || (count > tc.maximum && !errors.Is(err, core.ErrTimeMatchContract)) {
					t.Fatalf("count=%d error=%v, want accepted iff count<=%d", count, err, tc.maximum)
				}
			}
		})
	}
}
