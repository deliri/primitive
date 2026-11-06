package timematch

import (
	json "encoding/json/v2"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestEventAlertsExhaustiveLeadAndCardinality(t *testing.T) {
	t.Parallel()
	for value := 0; value <= 65535; value++ {
		err := (EventAlerts{AlertMinutes(value)}).Validate()
		if (value <= 40320 && err != nil) || (value > 40320 && !errors.Is(err, core.ErrTimeMatchContract)) {
			t.Fatalf("lead %d error=%v, want accepted iff0..40320", value, err)
		}
	}
	for count := 0; count <= 6; count++ {
		alerts := make(EventAlerts, count)
		for index := range alerts {
			alerts[index] = AlertMinutes(index)
		}
		err := alerts.Validate()
		if (count <= 5 && err != nil) || (count > 5 && !errors.Is(err, core.ErrTimeMatchContract)) {
			t.Fatalf("count %d error=%v, want accepted iff0..5", count, err)
		}
		for index := 1; index < count && count <= 5; index++ {
			for prior := 0; prior < index; prior++ {
				duplicate := slices.Clone(alerts)
				duplicate[index] = duplicate[prior]
				if err := duplicate.Validate(); !errors.Is(err, core.ErrTimeMatchContract) {
					t.Fatalf("duplicate positions %d/%d of%d error=%v, want contract refusal", prior, index, count, err)
				}
			}
		}
	}
}

// Real TLS/Exchange with a controlled Google peer. The peer retains exactly
// what the SDK posts; no live calendar or notification is involved.
func TestGoogleCalendarAlertsNativeCreation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		alerts EventAlerts
	}{
		{name: "owner selected day and half hour", alerts: EventAlerts{1440, 30}},
		{name: "empty preserves calendar defaults"},
		{name: "at start and four week endpoints", alerts: EventAlerts{0, 40320}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var saved struct {
				Reminders *struct {
					UseDefault bool `json:"useDefault"`
					Overrides  []struct {
						Method  string       `json:"method"`
						Minutes AlertMinutes `json:"minutes"`
					} `json:"overrides"`
				} `json:"reminders"`
			}
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, body []byte) {
				if err := json.Unmarshal(body, &saved); err != nil {
					t.Error(err)
					writeJSON(w, 400, `{}`)
					return
				}
				writeJSON(w, 200, string(body))
			})
			owned := googleOwnershipIntent(t)
			write := eventWrite(t, oauthConnection(t, ProviderGoogle), owned)
			write.Alerts = tc.alerts
			got, err := client.PutEvent(t.Context(), write)
			if err != nil || got.Ref.ID != owned.ID || len(fixture.record()) != 1 {
				t.Fatalf("create=(%+v,%v), calls%d, want owned receipt and1POST", got, err, len(fixture.record()))
			}
			if len(tc.alerts) == 0 {
				if saved.Reminders != nil {
					t.Fatalf("reminders=%+v, want omitted", saved.Reminders)
				}
				return
			}
			if saved.Reminders == nil || saved.Reminders.UseDefault || len(saved.Reminders.Overrides) != len(tc.alerts) {
				t.Fatalf("reminders=%+v, want explicit overrides%v", saved.Reminders, tc.alerts)
			}
			for index, alert := range saved.Reminders.Overrides {
				if alert.Method != "popup" || alert.Minutes != tc.alerts[index] {
					t.Fatalf("alert%d=%+v, want popup/%d", index, alert, tc.alerts[index])
				}
			}
		})
	}
}

func TestEventAlertsRefuseBeforeProviderEffects(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		provider Provider
		alerts   EventAlerts
	}{
		{name: "beyond four weeks", provider: ProviderGoogle, alerts: EventAlerts{40321}},
		{name: "duplicate alerts", provider: ProviderGoogle, alerts: EventAlerts{30, 30}},
		{name: "six overrides", provider: ProviderGoogle, alerts: EventAlerts{1, 2, 3, 4, 5, 6}},
		{name: "Microsoft cannot silently drop two alerts", provider: ProviderMicrosoft, alerts: EventAlerts{1440, 30}},
		{name: "Apple cannot silently drop two alerts", provider: ProviderApple, alerts: EventAlerts{1440, 30}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeJSON(w, 503, `{}`) })
			connection := oauthConnection(t, tc.provider)
			if tc.provider == ProviderApple {
				connection = appleConnection(t)
			}
			write := eventWrite(t, connection, googleOwnershipIntent(t))
			write.Alerts = tc.alerts
			got, err := client.PutEvent(t.Context(), write)
			if !errors.Is(err, core.ErrTimeMatchContract) || got != (EventReceipt{}) || len(fixture.record()) != 0 {
				t.Fatalf("alerts=(%+v,%v),calls%d,want typed refusal and no effect", got, err, len(fixture.record()))
			}
		})
	}
}
