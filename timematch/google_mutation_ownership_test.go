package timematch

import (
	json "encoding/json/v2"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func googleOwnershipIntent(t testing.TB) EventIntent {
	t.Helper()
	return intent(t, "primary", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")
}

// Foreign content deliberately has identical ID/title/time, but never a
// TimeMatch private marker. It is not emitted by the owned-event writer.
func googleForeignWire(t testing.TB, owned EventIntent) string {
	t.Helper()
	start, err := owned.Interval.Start.RFC3339Nano()
	if err != nil {
		t.Fatal(err)
	}
	end, err := owned.Interval.End.RFC3339Nano()
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(struct {
		ID          string          `json:"id"`
		ETag        string          `json:"etag"`
		Summary     string          `json:"summary"`
		Status      string          `json:"status"`
		Start       googleEventTime `json:"start"`
		End         googleEventTime `json:"end"`
		Description string          `json:"description"`
	}{ID: owned.ID.String(), ETag: `"revision1"`, Summary: owned.Summary.String(), Status: "confirmed", Start: googleEventTime{DateTime: start}, End: googleEventTime{DateTime: end}, Description: privateMarker})
	if err != nil {
		t.Fatal(err)
	}
	return string(wire)
}

func TestGoogleRemoteOwnershipForeignAppointmentCannotBeMutated(t *testing.T) {
	t.Parallel()
	for _, remove := range []bool{true, false} {
		name := "foreign appointment cannot be patched"
		if remove {
			name = "foreign appointment cannot be deleted"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			owned := googleOwnershipIntent(t)
			foreign := googleForeignWire(t, owned)
			var mutations, details atomic.Int64
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/events") {
					writeJSON(w, 200, `{"items":[]}`)
					return
				}
				if r.Method == http.MethodGet {
					details.Add(1)
					writeJSON(w, 200, foreign)
					return
				}
				mutations.Add(1)
				if r.Method == http.MethodDelete {
					w.WriteHeader(204)
					return
				}
				writeJSON(w, 200, foreign)
			})
			ref := EventRef{ID: owned.ID, Calendar: owned.Calendar, Resource: owned.ID.String(), Provider: ProviderGoogle}
			connection := oauthConnection(t, ProviderGoogle)
			if remove {
				got, err := client.DeleteEvent(t.Context(), EventDelete{Connection: connection, Ref: ref, Policy: fixturePolicy(t)})
				if !errors.Is(err, core.ErrTimeMatchIndeterminate) || got != DeleteInvalid || mutations.Load() != 0 || details.Load() != 0 {
					t.Fatalf("foreign delete=(%v,%v),mutations/details%d/%d,want Invalid/Indeterminate/0/0", got, err, mutations.Load(), details.Load())
				}
			} else {
				got, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: connection, Ref: ref, Intent: owned, ObservedAt: at(t, "2026-11-02T08:00:00Z"), Policy: fixturePolicy(t)})
				if !errors.Is(err, core.ErrTimeMatchIndeterminate) || got != (EventReceipt{}) || mutations.Load() != 0 || details.Load() != 0 {
					t.Fatalf("foreign update=(%+v,%v),mutations/details%d/%d,want zero/Indeterminate/0/0", got, err, mutations.Load(), details.Load())
				}
			}
			if len(fixture.record()) != 1 {
				t.Fatalf("foreign metadata requests=%d,want one filtered ownership query", len(fixture.record()))
			}
		})
	}
}

func TestGoogleRemoteOwnershipForeignCollisionCannotIssueReceipt(t *testing.T) {
	t.Parallel()
	owned := googleOwnershipIntent(t)
	foreign := googleForeignWire(t, owned)
	var details, mutations atomic.Int64
	client, _ := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if r.Method != http.MethodGet {
			mutations.Add(1)
			writeJSON(w, 409, `{}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/events") {
			writeJSON(w, 200, `{"items":[]}`)
			return
		}
		details.Add(1)
		writeJSON(w, 200, foreign)
	})
	request := eventWrite(t, oauthConnection(t, ProviderGoogle), owned)
	request.Attempt = EventCreateRecoverOnly
	got, err := client.PutEvent(t.Context(), request)
	if !errors.Is(err, core.ErrTimeMatchIndeterminate) || got != (EventReceipt{}) || mutations.Load() != 0 || details.Load() != 0 {
		t.Fatalf("foreign identical recovery=(%+v,%v),mutations/details%d/%d,want zero/Indeterminate/0/0", got, err, mutations.Load(), details.Load())
	}
}

func TestGoogleRemoteOwnershipCreationEmitsPrivateIdentity(t *testing.T) {
	t.Parallel()
	owned := googleOwnershipIntent(t)
	client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, body []byte) { writeJSON(w, 200, string(body)) })
	got, err := client.PutEvent(t.Context(), eventWrite(t, oauthConnection(t, ProviderGoogle), owned))
	if err != nil || got.Ref.ID != owned.ID {
		t.Fatalf("owned create=(%+v,%v),want exact owned receipt", got, err)
	}
	var observation struct {
		ExtendedProperties struct {
			Private struct {
				TimeMatchEventID string `json:"timematchEventId"`
			} `json:"private"`
		} `json:"extendedProperties"`
	}
	if err := json.Unmarshal(fixture.record()[0].body, &observation); err != nil || observation.ExtendedProperties.Private.TimeMatchEventID != owned.ID.String() {
		t.Fatalf("create marker=%q decode%v,want exact private EventID%q", observation.ExtendedProperties.Private.TimeMatchEventID, err, owned.ID.String())
	}
}
