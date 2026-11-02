package timematch

import (
	json "encoding/json/v2"
	"errors"
	"net/http"
	"strings"
	"sync"
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
	if !errors.Is(err, core.ErrTimeMatchIndeterminate) || got != (EventReceipt{}) || mutations.Load() != 1 || details.Load() != 0 {
		t.Fatalf("foreign identical recovery=(%+v,%v),mutations/details%d/%d,want zero/Indeterminate/1 rejected create/0 private reads", got, err, mutations.Load(), details.Load())
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

// This provider double implements the documented property filter and native
// revision compare-and-swap. It deliberately allows a credentialed client to
// mutate a foreign event if the SDK omits either ownership or revision guard.
type googleOwnedProviderState struct {
	mutex                                               sync.Mutex
	event                                               googleEventBody
	etag                                                string
	description                                         string
	otherPrivate                                        string
	metadataCode                                        int
	metadataReply                                       string
	race                                                bool
	paginate                                            bool
	queries, mutationAttempts, mutations, detailReplies int
	deleted                                             bool
	metadataOverride                                    bool
}

func googleOwnedProvider(t *testing.T, marker string) (Client, *provider, *googleOwnedProviderState, EventIntent) {
	t.Helper()
	owned := googleOwnershipIntent(t)
	event, err := googleEventProjection(owned)
	if err != nil {
		t.Fatal(err)
	}
	event.ID = owned.ID.String()
	event.ExtendedProperties = &googleExtendedProperties{Private: googlePrivateProperties{EventID: marker}}
	state := &googleOwnedProviderState{event: event, etag: `"revision1"`, description: privateMarker, otherPrivate: "calendar-owner-unrelated-metadata"}
	client, requests := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) { state.serve(t, w, r, body) })
	return client, requests, state, owned
}

func (s *googleOwnedProviderState) serve(t *testing.T, w http.ResponseWriter, r *http.Request, body []byte) {
	t.Helper()
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// A deterministic ID already exists. The provider's create guard rejects
	// this attempt without changing any owned or foreign resource.
	if r.Method == http.MethodPost {
		writeJSON(w, 409, `{}`)
		return
	}
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/events") {
		s.queries++
		if r.URL.Query().Get("fields") != "items(id,etag),nextPageToken" {
			t.Errorf("ownership fields=%q,want only IDs/revisions", r.URL.Query().Get("fields"))
		}
		if s.metadataCode != 0 {
			writeJSON(w, s.metadataCode, `{"error":"metadata unavailable"}`)
			return
		}
		if s.metadataOverride || s.metadataReply != "" {
			writeJSON(w, 200, s.metadataReply)
			return
		}
		type item struct {
			ID   string `json:"id"`
			ETag string `json:"etag"`
		}
		page := struct {
			Items []item `json:"items"`
			Next  string `json:"nextPageToken,omitzero"`
		}{Items: []item{}}
		filter := r.URL.Query().Get("privateExtendedProperty")
		matches := filter == "" || s.event.ExtendedProperties != nil && filter == "timematchEventId="+s.event.ExtendedProperties.Private.EventID
		if s.paginate && r.URL.Query().Get("pageToken") == "" {
			page.Next = "second"
		} else if !s.deleted && matches {
			page.Items = append(page.Items, item{ID: s.event.ID, ETag: s.etag})
		}
		wire, err := json.Marshal(page)
		if err != nil {
			t.Errorf("metadata emission=%v,want nil", err)
			w.WriteHeader(500)
			return
		}
		writeJSON(w, 200, string(wire))
		if s.race {
			s.etag = `"revision2"`
			s.event.ExtendedProperties.Private.EventID = "foreign0"
		}
		return
	}
	if condition := r.Header.Get("If-Match"); condition != "" && condition != s.etag {
		writeJSON(w, 412, `{}`)
		return
	}
	if r.Method == http.MethodGet {
		s.detailReplies++
		s.emit(t, w)
		return
	}
	s.mutationAttempts++
	if r.Method == http.MethodDelete {
		s.deleted = true
		s.mutations++
		w.WriteHeader(204)
		return
	}
	if r.Method == http.MethodPatch {
		if strings.Contains(string(body), "description") || strings.Contains(string(body), "extendedProperties") {
			t.Errorf("PATCH=%s,want owned fields only with other notes/metadata omitted", body)
		}
		if err := json.Unmarshal(body, &s.event); err != nil {
			writeJSON(w, 400, `{}`)
			return
		}
		s.etag = `"updated"`
		s.mutations++
		s.emit(t, w)
		return
	}
	w.WriteHeader(405)
}
func (s *googleOwnedProviderState) emit(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	wire, err := json.Marshal(struct {
		googleEventBody
		ETag   string `json:"etag"`
		Status string `json:"status"`
	}{googleEventBody: s.event, ETag: s.etag, Status: "confirmed"})
	if err != nil {
		t.Errorf("owned event emission=%v,want nil", err)
		w.WriteHeader(500)
		return
	}
	writeJSON(w, 200, string(wire))
}

func TestGoogleOwnedMutationStatefulLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                            string
		marker                          string
		remove, recover, paginate, race bool
		want                            error
		wantEffects                     int
	}{
		{name: "owned event is conditionally removed", marker: eventIDFixture, remove: true, wantEffects: 1},
		{name: "owned update preserves owner notes and private metadata", marker: eventIDFixture, wantEffects: 1},
		{name: "owned recovery collision requires conditional readback", marker: eventIDFixture, recover: true},
		{name: "later native page establishes owned revision", marker: eventIDFixture, paginate: true, remove: true, wantEffects: 1},
		{name: "foreign event with valid same ID is untouched", marker: "foreign0", remove: true, want: core.ErrTimeMatchIndeterminate},
		{name: "missing marker never authorizes update", want: core.ErrTimeMatchIndeterminate},
		{name: "missing marker never authorizes recovery receipt", recover: true, want: core.ErrTimeMatchIndeterminate},
		{name: "delete race loses revision and never retries", marker: eventIDFixture, remove: true, race: true, want: core.ErrTimeMatchConflict},
		{name: "patch race loses revision and never overwrites foreign event", marker: eventIDFixture, race: true, want: core.ErrTimeMatchConflict},
		{name: "recovery race never downloads replacement appointment", marker: eventIDFixture, recover: true, race: true, want: core.ErrTimeMatchIndeterminate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, requests, state, owned := googleOwnedProvider(t, tc.marker)
			state.paginate = tc.paginate
			state.race = tc.race
			ref := EventRef{ID: owned.ID, Provider: ProviderGoogle, Calendar: owned.Calendar, Resource: owned.ID.String()}
			connection := oauthConnection(t, ProviderGoogle)
			var err error
			if tc.remove {
				var got DeleteOutcome
				got, err = client.DeleteEvent(t.Context(), EventDelete{Connection: connection, Ref: ref, Policy: fixturePolicy(t)})
				if tc.want == nil && got != Deleted || tc.want != nil && got != DeleteInvalid {
					t.Fatalf("delete outcome=%v,want success only after owned conditional mutation", got)
				}
			} else if tc.recover {
				request := eventWrite(t, connection, owned)
				request.Attempt = EventCreateRecoverOnly
				var receipt EventReceipt
				receipt, err = client.PutEvent(t.Context(), request)
				if tc.want != nil && receipt != (EventReceipt{}) || tc.want == nil && receipt.Ref != ref {
					t.Fatalf("owned recovery receipt=%+v,want exact ownership only on success", receipt)
				}
			} else {
				var receipt EventReceipt
				receipt, err = client.UpdateEvent(t.Context(), EventUpdate{Connection: connection, Ref: ref, Intent: owned, ObservedAt: at(t, "2026-11-02T08:00:00Z"), Policy: fixturePolicy(t)})
				if tc.want != nil && receipt != (EventReceipt{}) || tc.want == nil && receipt.Ref != ref {
					t.Fatalf("owned update receipt=%+v,want exact ownership only on success", receipt)
				}
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("ownership result=%v,want%v", err, tc.want)
			}
			state.mutex.Lock()
			effects, details, queries, note, other := state.mutations, state.detailReplies, state.queries, state.description, state.otherPrivate
			state.mutex.Unlock()
			if effects != tc.wantEffects || note != privateMarker || other != "calendar-owner-unrelated-metadata" {
				t.Fatalf("provider effects=%d notes=%q/%q,want effects%d and unchanged other data", effects, note, other, tc.wantEffects)
			}
			if tc.want != nil && details != 0 {
				t.Fatalf("foreign/uncertain detail responses=%d,want0", details)
			}
			wantQueries := 1
			if tc.paginate {
				wantQueries = 2
			}
			if queries != wantQueries {
				t.Fatalf("ownership queries=%d,want%d without retry", queries, wantQueries)
			}
			calls := requests.record()
			for _, call := range calls {
				if call.method == http.MethodGet && strings.HasSuffix(call.path, "/events") && call.query == "" {
					t.Fatalf("ownership query = %q for %q, want exact private property", call.query, call.path)
				}
			}
			wantRaceCalls := 2
			if tc.recover {
				wantRaceCalls = 3
			}
			if tc.race && len(calls) != wantRaceCalls {
				t.Fatalf("race requests=%d,want metadata+one conditional attempt and no retry", len(calls))
			}
		})
	}
}

func TestGoogleOwnedMetadataHostileRefusals(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, etag string
		status           int
		want             error
	}{
		{name: "missing items are unknown", body: `{}`, want: core.ErrTimeMatchIndeterminate},
		{name: "null items are unknown", body: `{"items":null}`, want: core.ErrTimeMatchIndeterminate},
		{name: "truncated metadata is unknown", body: `{"items":[`, want: core.ErrTimeMatchIndeterminate},
		{name: "duplicate metadata keys are refused", body: `{"items":[],"items":[]}`, want: core.ErrTimeMatchIndeterminate},
		{name: "null event is not metadata", body: `{"items":[null]}`, want: core.ErrTimeMatchIndeterminate},
		{name: "different filtered resource is a contradiction", body: `{"items":[{"id":"other0","etag":"\"revision1\""}]}`, want: core.ErrTimeMatchConflict},
		{name: "duplicate ownership metadata is ambiguous", body: `{"items":[{"id":"` + eventIDFixture + `","etag":"\"revision1\""},{"id":"` + eventIDFixture + `","etag":"\"revision1\""}]}`, want: core.ErrTimeMatchIndeterminate},
		{name: "weak revision never permits mutate", etag: `W/"weak"`, want: core.ErrTimeMatchIndeterminate},
		{name: "wildcard revision never permits mutate", etag: `*`, want: core.ErrTimeMatchIndeterminate},
		{name: "revision list never permits mutate", etag: `"one","two"`, want: core.ErrTimeMatchIndeterminate},
		{name: "unquoted revision never permits mutate", etag: `revision`, want: core.ErrTimeMatchIndeterminate},
		{name: "missing revision never permits mutate", etag: "missing", want: core.ErrTimeMatchIndeterminate},
		{name: "unavailable metadata cannot become already absent", status: 503, want: core.ErrTimeMatchIndeterminate},
		{name: "missing calendar cannot become already absent", status: 404, want: core.ErrTimeMatchIndeterminate},
		{name: "gone calendar cannot become already absent", status: 410, want: core.ErrTimeMatchIndeterminate},
		{name: "revoked consent remains unknown ownership", status: 401, want: core.ErrTimeMatchIndeterminate},
		{name: "forbidden metadata remains unknown ownership", status: 403, want: core.ErrTimeMatchIndeterminate},
		{name: "quota refusal remains unknown ownership", status: 429, want: core.ErrTimeMatchIndeterminate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, requests, state, owned := googleOwnedProvider(t, eventIDFixture)
			state.metadataReply = tc.body
			state.metadataCode = tc.status
			if tc.etag != "" {
				state.etag = tc.etag
			}
			if tc.etag == "missing" {
				state.etag = ""
			}
			got, err := client.DeleteEvent(t.Context(), EventDelete{Connection: oauthConnection(t, ProviderGoogle), Ref: EventRef{ID: owned.ID, Provider: ProviderGoogle, Calendar: owned.Calendar, Resource: owned.ID.String()}, Policy: fixturePolicy(t)})
			if !errors.Is(err, tc.want) || got != DeleteInvalid || state.mutations != 0 || state.detailReplies != 0 {
				t.Fatalf("uncertain metadata=(%v,%v),effects/details%d/%d,want Invalid/%v/0/0", got, err, state.mutations, state.detailReplies, tc.want)
			}
			if len(requests.record()) != 1 {
				t.Fatalf("refused metadata requests=%d,want one read only", len(requests.record()))
			}
			noLeak(t, err)
		})
	}
}

func TestGoogleOwnedReferenceBindingBeforeHTTP(t *testing.T) {
	t.Parallel()
	for _, resource := range []string{"", "/", "primary", "https://www.googleapis.com/calendar/v3/calendars/primary/events", eventIDFixture + "/other", eventIDFixture + "?anything", "other0"} {
		t.Run("resource "+resource, func(t *testing.T) {
			t.Parallel()
			client, requests, _, owned := googleOwnedProvider(t, eventIDFixture)
			got, err := client.DeleteEvent(t.Context(), EventDelete{Connection: oauthConnection(t, ProviderGoogle), Ref: EventRef{ID: owned.ID, Provider: ProviderGoogle, Calendar: owned.Calendar, Resource: resource}, Policy: fixturePolicy(t)})
			if got != DeleteInvalid || !errors.Is(err, core.ErrTimeMatchContract) || len(requests.record()) != 0 || len(requests.refusedDials()) != 0 {
				t.Fatalf("tampered resource=%q outcome/error%v/%v requests%d,want Invalid/Contract/zeroHTTP", resource, got, err, len(requests.record()))
			}
		})
	}
}

func TestGoogleOwnedReceiptRejectsUnboundReadback(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                         string
		missingMarker, wrongRevision bool
		want                         error
	}{
		{name: "same content without private marker cannot be a receipt", missingMarker: true, want: core.ErrTimeMatchConflict},
		{name: "different readback revision cannot certify prior ownership", wrongRevision: true, want: core.ErrTimeMatchIndeterminate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := googleOwnershipIntent(t)
			body := googleForeignWire(t, owned)
			if !tc.missingMarker {
				encoded, err := googleEvent(owned)
				if err != nil {
					t.Fatal(err)
				}
				body = strings.TrimSuffix(string(encoded), "}") + `,"etag":"\"other-revision\""}`
			}
			client, _ := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method == http.MethodPost {
					writeJSON(w, 409, `{}`)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/events") {
					writeJSON(w, 200, `{"items":[{"id":"`+eventIDFixture+`","etag":"\"revision1\""}]}`)
					return
				}
				writeJSON(w, 200, body)
			})
			request := eventWrite(t, oauthConnection(t, ProviderGoogle), owned)
			request.Attempt = EventCreateRecoverOnly
			got, err := client.PutEvent(t.Context(), request)
			if !errors.Is(err, tc.want) || got != (EventReceipt{}) {
				t.Fatalf("unbound readback=(%+v,%v),want zero/%v", got, err, tc.want)
			}
		})
	}
}

func FuzzGoogleOwnedMetadataMutationClosure(f *testing.F) {
	owned := googleOwnershipIntent(f)
	body, err := googleEvent(owned)
	if err != nil {
		f.Fatal(err)
	}
	var emitted googleEventBody
	if err := json.Unmarshal(body, &emitted); err != nil || emitted.ExtendedProperties == nil || emitted.ExtendedProperties.Private.EventID != owned.ID.String() {
		f.Fatalf("owned seed marker=%+v/%v,want real owned creation", emitted.ExtendedProperties, err)
	}
	type seedItem struct {
		ID   string `json:"id"`
		ETag string `json:"etag"`
	}
	seed, err := json.Marshal(struct {
		Items []seedItem `json:"items"`
	}{Items: []seedItem{{ID: emitted.ID, ETag: `"revision1"`}}})
	if err != nil {
		f.Fatal(err)
	}
	for op := uint8(0); op < 3; op++ {
		f.Add(seed, op)
	}
	f.Add([]byte(`{"items":[{"id":"`+owned.ID.String()+`","etag":"\"revision1\"","unusedProviderDetail":{"summary":7}}],"providerExtension":{"future":true}}`), uint8(0))
	f.Add([]byte{}, uint8(0))
	f.Add([]byte(`{"items":[]}`), uint8(1))
	f.Add([]byte(`{"items":null}`), uint8(2))
	f.Fuzz(func(t *testing.T, data []byte, operation uint8) {
		client, requests, state, owned := googleOwnedProvider(t, eventIDFixture)
		state.metadataReply = string(data)
		state.metadataOverride = true
		operation %= 3
		ref := EventRef{ID: owned.ID, Calendar: owned.Calendar, Provider: ProviderGoogle, Resource: owned.ID.String()}
		connection := oauthConnection(t, ProviderGoogle)
		var err error
		var receipt EventReceipt
		outcome := DeleteInvalid
		switch operation {
		case 0:
			outcome, err = client.DeleteEvent(t.Context(), EventDelete{Connection: connection, Ref: ref, Policy: fixturePolicy(t)})
		case 1:
			receipt, err = client.UpdateEvent(t.Context(), EventUpdate{Connection: connection, Ref: ref, Intent: owned, ObservedAt: at(t, "2026-11-02T08:00:00Z"), Policy: fixturePolicy(t)})
		case 2:
			request := eventWrite(t, connection, owned)
			request.Attempt = EventCreateRecoverOnly
			receipt, err = client.PutEvent(t.Context(), request)
		default:
			t.Fatalf("operation=%d,want exhausted0..2", operation)
		}
		var independent struct {
			Items []seedItem `json:"items"`
			Next  string     `json:"nextPageToken"`
		}
		plainErr := json.Unmarshal(data, &independent)
		bound := plainErr == nil && len(independent.Items) == 1 && independent.Items[0].ID == owned.ID.String() && independent.Items[0].ETag == `"revision1"` && independent.Next == ""
		knownShape := bound
		state.mutex.Lock()
		effects, details, notes := state.mutations, state.detailReplies, state.description
		state.mutex.Unlock()
		if err != nil {
			if knownShape || !errors.Is(err, core.ErrTimeMatchContract) || receipt != (EventReceipt{}) || outcome != DeleteInvalid || effects != 0 {
				t.Fatalf("rejected metadata bound%t=(%+v,%v,%v),effects%d,want typed refusal/zero effects only for unbound input", knownShape, receipt, outcome, err, effects)
			}
			return
		}
		if !bound || notes != privateMarker {
			t.Fatalf("accepted metadata bound%t notes%q,want exact independent ID/revision and unchanged private data", bound, notes)
		}
		wantEffects := 1
		if operation == 2 {
			wantEffects = 0
			if details != 1 {
				t.Fatalf("owned recovery detail replies=%d,want1", details)
			}
		}
		if effects != wantEffects || operation == 0 && outcome != Deleted || operation != 0 && receipt.Ref != ref {
			t.Fatalf("accepted effect/receipt=%d/%v/%+v,want%d and exact owned identity", effects, outcome, receipt, wantEffects)
		}
		calls := requests.record()
		wantCalls := 2
		if operation == 2 {
			wantCalls = 3
		}
		if len(calls) != wantCalls || calls[len(calls)-1].header.Get("If-Match") != `"revision1"` {
			t.Fatalf("accepted requests=%d,want filtered proof then exact conditional operation", len(calls))
		}
	})
}

func TestGoogleOwnedMetadataHarmlessExtrasRemainUsable(t *testing.T) {
	t.Parallel()
	client, _, state, owned := googleOwnedProvider(t, eventIDFixture)
	state.metadataOverride = true
	state.metadataReply = `{"items":[{"id":"` + owned.ID.String() + `","etag":"\"revision1\"","unusedProviderDetail":{"summary":7}}],"providerExtension":{"future":true}}`
	got, err := client.DeleteEvent(t.Context(), EventDelete{Connection: oauthConnection(t, ProviderGoogle), Ref: EventRef{ID: owned.ID, Provider: ProviderGoogle, Calendar: owned.Calendar, Resource: owned.ID.String()}, Policy: fixturePolicy(t)})
	if err != nil || got != Deleted || state.mutations != 1 {
		t.Fatalf("bound metadata with irrelevant extras=(%v,%v),effects%d,want Deleted/nil/1", got, err, state.mutations)
	}
}
