package timematch

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func alertOffsets(t testing.TB, minutes ...uint64) EventAlerts {
	t.Helper()
	leads := make(EventAlerts, len(minutes))
	for index, value := range minutes {
		lead, err := temporal.DurationFromMinutes(value)
		if err != nil {
			t.Fatalf("duration %d error=%v,want nil", value, err)
		}
		leads[index] = lead
	}
	return leads
}

func TestEventAlertsExhaustiveLeadAndCardinality(t *testing.T) {
	t.Parallel()
	for value := 0; value <= 65535; value++ {
		err := (alertOffsets(t, uint64(value))).Validate()
		if (value <= 40320 && err != nil) || (value > 40320 && !errors.Is(err, core.ErrTimeMatchContract)) {
			t.Fatalf("lead %d error=%v, want accepted iff0..40320", value, err)
		}
	}
	for count := 0; count <= 6; count++ {
		alerts := make(EventAlerts, count)
		for index := range alerts {
			alerts[index] = alertOffsets(t, uint64(index))[0]
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
		{name: "owner selected day and half hour", alerts: alertOffsets(t, 1440, 30)},
		{name: "empty preserves calendar defaults"},
		{name: "at start and four week endpoints", alerts: alertOffsets(t, 0, 40320)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var mutex sync.Mutex
			var saved struct {
				Reminders *struct {
					UseDefault bool `json:"useDefault"`
					Overrides  []struct {
						Method  string `json:"method"`
						Minutes int64  `json:"minutes"`
					} `json:"overrides"`
				} `json:"reminders"`
			}
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, body []byte) {
				mutex.Lock()
				defer mutex.Unlock()
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
			mutex.Lock()
			defer mutex.Unlock()
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
				if alert.Method != "popup" || alert.Minutes != tc.alerts[index].Nanoseconds()/int64(temporal.NanosecondsPerMinute) {
					t.Fatalf("alert%d=%+v, want popup/%v", index, alert, tc.alerts[index])
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
		{name: "beyond four weeks", provider: ProviderGoogle, alerts: alertOffsets(t, 40321)},
		{name: "duplicate alerts", provider: ProviderGoogle, alerts: alertOffsets(t, 30, 30)},
		{name: "six overrides", provider: ProviderGoogle, alerts: alertOffsets(t, 1, 2, 3, 4, 5, 6)},
		{name: "Microsoft cannot silently drop two alerts", provider: ProviderMicrosoft, alerts: alertOffsets(t, 1440, 30)},
		{name: "Apple cannot silently drop two alerts", provider: ProviderApple, alerts: alertOffsets(t, 1440, 30)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeJSON(w, 503, `{}`) })
			connection := oauthConnection(t, tc.provider)
			if tc.provider == ProviderApple {
				connection = appleConnection(t)
			}
			owned := googleOwnershipIntent(t)
			if tc.provider == ProviderApple {
				owned = appleMutationIntent(t)
			}
			write := eventWrite(t, connection, owned)
			write.Alerts = tc.alerts
			got, err := client.PutEvent(t.Context(), write)
			if !errors.Is(err, core.ErrTimeMatchContract) || got != (EventReceipt{}) || len(fixture.record()) != 0 {
				t.Fatalf("alerts=(%+v,%v),calls%d,want typed refusal and no effect", got, err, len(fixture.record()))
			}
		})
	}
}

// A user may replace our defaults. The provider moves those alerts with the
// event; an owned summary/time PATCH must not reset them.
func TestGoogleCalendarAlertsUpdatePreservesUserChoice(t *testing.T) {
	t.Parallel()
	client, requests, state, owned := googleOwnedProvider(t, eventIDFixture)
	state.event.Reminders = &googleEventReminders{Overrides: []googleEventReminder{{Method: "popup", Minutes: 7}}}
	owned.Interval = window(t, "2026-11-03T15:00:00Z", "2026-11-03T16:00:00Z")
	ref := EventRef{ID: owned.ID, Provider: ProviderGoogle, Calendar: owned.Calendar, Resource: owned.ID.String()}
	got, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: oauthConnection(t, ProviderGoogle), Ref: ref, Intent: owned, ObservedAt: at(t, "2026-11-02T08:00:00Z"), Policy: fixturePolicy(t)})
	if err != nil || got.Ref != ref {
		t.Fatalf("update=(%+v,%v),want exact owned receipt", got, err)
	}
	state.mutex.Lock()
	defer state.mutex.Unlock()
	if state.event.Reminders == nil || len(state.event.Reminders.Overrides) != 1 || state.event.Reminders.Overrides[0].Minutes != 7 || state.mutations != 1 {
		t.Fatalf("provider reminders=%+v, mutations%d,want user's7minutes and1update", state.event.Reminders, state.mutations)
	}
	patches := 0
	for _, request := range requests.record() {
		if request.method == http.MethodPatch {
			patches++
			if bytes.Contains(request.body, []byte(`"reminders"`)) {
				t.Fatalf("PATCH=%s,want reminders omitted", request.body)
			}
		}
	}
	if patches != 1 {
		t.Fatalf("patches=%d,want1", patches)
	}
}

func TestEventAlertsRejectSubminutePrecision(t *testing.T) {
	t.Parallel()
	for _, nanos := range []int64{1, int64(temporal.NanosecondsPerMinute) - 1, int64(temporal.NanosecondsPerMinute) + 1, 40320*int64(temporal.NanosecondsPerMinute) + 1, 1<<63 - 1} {
		lead, err := temporal.DurationFromNanoseconds(nanos)
		if err != nil {
			t.Fatal(err)
		}
		if err := (EventAlerts{lead}).Validate(); !errors.Is(err, core.ErrTimeMatchContract) {
			t.Fatalf("lead %d error=%v,want exact whole-minute bound refusal", nanos, err)
		}
	}
}
