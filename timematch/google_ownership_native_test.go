package timematch

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoogleOwnedScopeUsesDataOwnerNotManagerRole(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		role        string
		owner       string
		primaryLast bool
		omitPrimary bool
		want        bool
	}{
		{name: "manager of someone elses calendar cannot write through owned scope", role: "owner", owner: "other@example.test"},
		{name: "actual secondary owner can write", role: "owner", owner: "me@example.test", want: true},
		{name: "primary discovered on later page still proves secondary ownership", role: "owner", owner: "me@example.test", primaryLast: true, want: true},
		{name: "missing data owner does not prove ownership", role: "owner"},
		{name: "missing primary identity does not prove ownership", role: "owner", owner: "me@example.test", omitPrimary: true},
		{name: "write role alone cannot broaden granted owned scope", role: "writer", owner: "me@example.test"},
		{name: "restricted write role alone cannot broaden scope", role: "writerWithoutPrivateAccess", owner: "me@example.test"},
		{name: "read role cannot become writable from matching metadata", role: "reader", owner: "me@example.test"},
		{name: "busy only role cannot become writable", role: "freeBusyReader", owner: "me@example.test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			primary := seedGoogleItem{ID: "me@example.test", AccessRole: "owner", Primary: true}
			secondary := seedGoogleItem{ID: "team@group.calendar.google.com", AccessRole: tc.role, DataOwner: tc.owner}
			pages := [][]byte{seedJSON(t, seedGooglePage{Items: []seedGoogleItem{primary}, NextPageToken: "secondary"}), seedJSON(t, seedGooglePage{Items: []seedGoogleItem{secondary}})}
			if tc.primaryLast {
				pages[0], pages[1] = seedJSON(t, seedGooglePage{Items: []seedGoogleItem{secondary}, NextPageToken: "primary"}), seedJSON(t, seedGooglePage{Items: []seedGoogleItem{primary}})
			}
			if tc.omitPrimary {
				pages = pages[1:]
			}
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				index := 0
				if r.URL.Query().Get("pageToken") != "" {
					index = 1
				}
				writeJSON(w, 200, string(pages[index]))
			})
			got, err := client.ListCalendars(t.Context(), CalendarListRequest{Connection: oauthConnection(t, ProviderGoogle), Policy: fixturePolicy(t)})
			if err != nil {
				t.Fatalf("ListCalendars error=%v,want nil", err)
			}
			found := false
			for _, item := range got.Calendars {
				if item.ID.String() == secondary.ID {
					found = true
					if item.Writable != tc.want {
						t.Fatalf("secondary writable=%t,want %t", item.Writable, tc.want)
					}
				}
			}
			if !found {
				t.Fatalf("secondary present=%t,want true", found)
			}
			for _, request := range fixture.record() {
				if strings.Contains(request.query, "summary") {
					t.Fatalf("calendar query=%q,want no display-name projection", request.query)
				}
			}
		})
	}
}

func TestGoogleSubsecondMutationRefusedBeforeDispatch(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"2026-11-02T15:00:00.000000001Z", "2026-11-02T15:00:00.000001Z", "2026-11-02T15:00:00.999999999Z", "1969-12-31T23:59:59.999999999Z"} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeJSON(w, 503, `{}`) })
			owned := intent(t, "primary", "Review", text, "2026-11-02T16:00:00Z")
			connection := oauthConnection(t, ProviderGoogle)
			_, createErr := client.PutEvent(t.Context(), eventWrite(t, connection, owned))
			_, updateErr := client.UpdateEvent(t.Context(), EventUpdate{Connection: connection, Ref: EventRef{ID: owned.ID, Provider: ProviderGoogle, Calendar: owned.Calendar, Resource: owned.ID.String()}, Intent: owned, ObservedAt: at(t, "2026-11-02T09:00:00Z"), Policy: fixturePolicy(t)})
			if !errors.Is(createErr, core.ErrTimeMatchContract) || !errors.Is(updateErr, core.ErrTimeMatchContract) || len(fixture.record()) != 0 {
				t.Fatalf("subsecond create/update errors=(%v,%v),calls=%d,want contract refusals/zero", createErr, updateErr, len(fixture.record()))
			}
		})
	}
}
