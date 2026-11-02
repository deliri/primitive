package timematch

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const immutableFixtureID = "AAk-immutable-owned"

func microsoftMetadata(id, owner string) graphOwnedMetadata {
	return graphOwnedMetadata{ID: id, TransactionID: owner, Ownership: []graphOwnedProperty{{ID: graphOwnershipProperty, Value: owner}}}
}

func graphEncoded(t testing.TB, value core.Validatable) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v, want nil", err)
	}
	return string(encoded)
}

func graphOwnedDetail(t testing.TB, owned EventIntent) graphCreated {
	t.Helper()
	body, err := graphEvent(owned, true)
	if err != nil {
		t.Fatalf("graphEvent() error = %v, want nil", err)
	}
	var detail graphCreated
	if err := json.Unmarshal(body, &detail); err != nil {
		t.Fatalf("decode emitted intent error = %v, want nil", err)
	}
	detail.ID = immutableFixtureID
	return detail
}

// The filtered collection projection deliberately has no extended property.
type graphCandidateFixture struct {
	ID            string `json:"id"`
	TransactionID string `json:"transactionId"`
}
type graphCandidatePageFixture struct {
	Value []graphCandidateFixture `json:"value"`
	Next  string                  `json:"@odata.nextLink,omitzero"`
}

func graphCandidateCollection(t testing.TB, page graphOwnedPage) string {
	t.Helper()
	wire := graphCandidatePageFixture{Value: make([]graphCandidateFixture, 0, len(page.Value)), Next: page.Next}
	for _, item := range page.Value {
		wire.Value = append(wire.Value, graphCandidateFixture{ID: item.ID, TransactionID: item.TransactionID})
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		t.Fatalf("candidate collection encode error = %v, want nil", err)
	}
	return string(encoded)
}

// Graph deliberately ignores transactionId deduplication. Only a read-only
// recovery path, not another POST carrying the same transactionId, is safe.
func TestMicrosoftLostCreateRecoveryNeverRepeatsPOST(t *testing.T) {
	t.Parallel()
	owned := intent(t, "primary", "Appointment", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")
	var mutex sync.Mutex
	posts, lookups, ownerships, details := 0, 0, 0, 0
	committed, visible := false, false
	detail := graphOwnedDetail(t, owned)
	client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		mutex.Lock()
		defer mutex.Unlock()
		if r.Header.Get("Prefer") != graphPreferUTC {
			t.Errorf("Prefer = %q, want %q", r.Header.Get("Prefer"), graphPreferUTC)
		}
		switch {
		case r.Method == http.MethodPost:
			posts++
			committed = true
			var sent graphEventBody
			if err := json.Unmarshal(body, &sent); err != nil || len(sent.Ownership) != 1 || sent.Ownership[0] != (graphOwnedProperty{ID: graphOwnershipProperty, Value: owned.ID.value}) {
				t.Errorf("POST ownership = %+v error %v, want exact private marker", sent.Ownership, err)
			}
			cutConnection(w)
		case r.URL.Path == graphEventsPath:
			lookups++
			page := graphOwnedPage{Value: []graphOwnedMetadata{}}
			if visible {
				page.Value = append(page.Value, microsoftMetadata(immutableFixtureID, owned.ID.value))
			}
			writeJSON(w, 200, graphCandidateCollection(t, page))
		case r.URL.Path == graphEventsPath+"/"+immutableFixtureID:
			if r.URL.Query().Has("$expand") {
				ownerships++
				writeJSON(w, 200, graphEncoded(t, microsoftMetadata(immutableFixtureID, owned.ID.value)))
				return
			}
			details++
			encoded, err := json.Marshal(detail)
			if err != nil {
				t.Errorf("detail encode error = %v, want nil", err)
			}
			writeJSON(w, 200, string(encoded))
		default:
			writeJSON(w, 404, `{}`)
		}
	})
	write := eventWrite(t, oauthConnection(t, ProviderMicrosoft), owned)
	first, firstErr := client.PutEvent(t.Context(), write)
	if !errors.Is(firstErr, core.ErrTimeMatchIndeterminate) || first != (EventReceipt{}) {
		t.Fatalf("lost reply = (%+v,%v), want zero indeterminate", first, firstErr)
	}
	write.Attempt = EventCreateRecoverOnly
	lagged, laggedErr := client.PutEvent(t.Context(), write)
	if !errors.Is(laggedErr, core.ErrTimeMatchIndeterminate) || lagged != (EventReceipt{}) {
		t.Fatalf("unindexed committed write = (%+v,%v), want zero indeterminate", lagged, laggedErr)
	}
	mutex.Lock()
	visible = true
	mutex.Unlock()
	recovered, recoverErr := client.PutEvent(t.Context(), write)
	again, againErr := client.PutEvent(t.Context(), write)
	mutex.Lock()
	gotPosts, gotLookups, gotOwnerships, gotDetails, gotCommitted := posts, lookups, ownerships, details, committed
	mutex.Unlock()
	if recoverErr != nil || againErr != nil || recovered != again || recovered.Ref.Resource != immutableFixtureID || recovered.Ref.ID != owned.ID || gotPosts != 1 || gotLookups != 3 || gotOwnerships != 2 || gotDetails != 2 || !gotCommitted {
		t.Fatalf("recovery = (%+v,%v), replay (%+v,%v), effects %d/%d/%d/%d committed %t, want one POST and same owned receipt", recovered, recoverErr, again, againErr, gotPosts, gotLookups, gotOwnerships, gotDetails, gotCommitted)
	}
	for _, request := range fixture.record() {
		if request.method == http.MethodGet && request.path == graphEventsPath {
			query, err := http.NewRequest(http.MethodGet, "https://example.com?"+request.query, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := query.URL.Query().Get("$select"); got != "id,transactionId" {
				t.Fatalf("lookup select = %q, want ownership metadata only", got)
			}
			if !strings.Contains(query.URL.Query().Get("$filter"), "ep/value eq '"+owned.ID.value+"'") {
				t.Fatalf("lookup filter = %q, want exact owned marker", query.URL.Query().Get("$filter"))
			}
		}
	}
}

func TestMicrosoftRecoveryHostileNative(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		mutate       func(*graphOwnedPage, *graphCreated)
		lookupStatus int
		detailStatus int
		raw          string
		wantErr      error
		wantDetails  int
	}{
		{name: "owned event in another calendar retains immutable identity", wantDetails: 1},
		{name: "index empty cannot prove first create failed", mutate: func(p *graphOwnedPage, _ *graphCreated) { p.Value = []graphOwnedMetadata{} }, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "two owned events cannot be silently chosen", mutate: func(p *graphOwnedPage, _ *graphCreated) {
			p.Value = append(p.Value, microsoftMetadata("another", eventIDFixture))
		}, wantErr: core.ErrTimeMatchConflict},
		{name: "duplicate same id is not a unique authoritative observation", mutate: func(p *graphOwnedPage, _ *graphCreated) { p.Value = append(p.Value, p.Value[0]) }, wantErr: core.ErrTimeMatchConflict},
		{name: "foreign transaction is rejected before details", mutate: func(p *graphOwnedPage, _ *graphCreated) { p.Value[0].TransactionID = "another" }, wantErr: core.ErrTimeMatchConflict},
		{name: "foreign marker value is rejected before details", mutate: func(p *graphOwnedPage, _ *graphCreated) { p.Value[0].Ownership[0].Value = "another" }, wantErr: core.ErrTimeMatchConflict},
		{name: "foreign property namespace cannot claim event", mutate: func(p *graphOwnedPage, _ *graphCreated) {
			p.Value[0].Ownership[0].ID = "String {different} Name TimeMatchEventID"
		}, wantErr: core.ErrTimeMatchConflict},
		{name: "missing marker leaves ownership unknown", mutate: func(p *graphOwnedPage, _ *graphCreated) { p.Value[0].Ownership = nil }, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "duplicate markers are ambiguous", mutate: func(p *graphOwnedPage, _ *graphCreated) {
			p.Value[0].Ownership = append(p.Value[0].Ownership, p.Value[0].Ownership[0])
		}, wantErr: core.ErrTimeMatchConflict},
		{name: "empty provider id cannot issue receipt", mutate: func(p *graphOwnedPage, _ *graphCreated) { p.Value[0].ID = "" }, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "newline provider id cannot address resource", mutate: func(p *graphOwnedPage, _ *graphCreated) { p.Value[0].ID = "owned\nforeign" }, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "private content injected into error is sanitized", lookupStatus: 503, raw: `{"error":"PRIVATE-EVENT-TITLE-attendee@example.com"}`, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "consent refusal retains strongest identity", lookupStatus: 401, wantErr: core.ErrTimeMatchConsent},
		{name: "malformed page never implies absence", raw: `{"value":`, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "omitted collection never implies absence", raw: `{}`, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "null collection never implies absence", raw: `{"value":null}`, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "duplicate collection refused", raw: `{"value":[],"value":[]}`, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "wrong collection type refused", raw: `{"value":1}`, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "foreign paging cannot receive bearer", mutate: func(p *graphOwnedPage, _ *graphCreated) { p.Next = "https://other.example/v1.0/me/events?$skip=1" }, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "wrong graph path cannot receive bearer", mutate: func(p *graphOwnedPage, _ *graphCreated) {
			p.Next = "https://graph.microsoft.com/v1.0/me/messages?$skip=1"
		}, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "lookup page loop never certifies first observation", mutate: func(p *graphOwnedPage, _ *graphCreated) {
			p.Value = []graphOwnedMetadata{}
			p.Next = "https://graph.microsoft.com/v1.0/me/events?$skip=1"
		}, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "event disappears before detail read stays unknown", detailStatus: 404, wantErr: core.ErrTimeMatchIndeterminate, wantDetails: 1},
		{name: "event changed summary is conflict", mutate: func(_ *graphOwnedPage, d *graphCreated) { d.Subject = "changed" }, wantErr: core.ErrTimeMatchConflict, wantDetails: 1},
		{name: "event changed start by one tick is conflict", mutate: func(_ *graphOwnedPage, d *graphCreated) { d.Start.DateTime = "2026-11-02T15:00:00.0000001" }, wantErr: core.ErrTimeMatchConflict, wantDetails: 1},
		{name: "event changed end by one tick is conflict", mutate: func(_ *graphOwnedPage, d *graphCreated) { d.End.DateTime = "2026-11-02T16:00:00.0000001" }, wantErr: core.ErrTimeMatchConflict, wantDetails: 1},
		{name: "cancelled owned event is conflict", mutate: func(_ *graphOwnedPage, d *graphCreated) { d.IsCancelled = true }, wantErr: core.ErrTimeMatchConflict, wantDetails: 1},
		{name: "detail response changed immutable id is conflict", mutate: func(_ *graphOwnedPage, d *graphCreated) { d.ID = "different" }, wantErr: core.ErrTimeMatchConflict, wantDetails: 1},
		{name: "detail response changed transaction is conflict", mutate: func(_ *graphOwnedPage, d *graphCreated) { d.TransactionID = "different" }, wantErr: core.ErrTimeMatchConflict, wantDetails: 1},
		{name: "offset zone rather than UTC is indeterminate", mutate: func(_ *graphOwnedPage, d *graphCreated) { d.Start.TimeZone = "America/Toronto" }, wantErr: core.ErrTimeMatchIndeterminate, wantDetails: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := intent(t, "primary", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")
			page := graphOwnedPage{Value: []graphOwnedMetadata{microsoftMetadata(immutableFixtureID, owned.ID.value)}}
			detail := graphOwnedDetail(t, owned)
			if tc.mutate != nil {
				tc.mutate(&page, &detail)
			}
			details := 0
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method != http.MethodGet {
					t.Errorf("method = %q, want recovery GET only", r.Method)
				}
				if r.Header.Get("Prefer") != graphPreferUTC {
					t.Errorf("Prefer = %q, want immutable UTC", r.Header.Get("Prefer"))
				}
				status := tc.lookupStatus
				if status == 0 {
					status = 200
				}
				if r.URL.Path == graphEventsPath {
					body := tc.raw
					if body == "" {
						body = graphCandidateCollection(t, page)
					}
					writeJSON(w, status, body)
					return
				}
				if r.URL.Query().Has("$expand") {
					requested := strings.TrimPrefix(r.URL.Path, graphEventsPath+"/")
					for _, candidate := range page.Value {
						if candidate.ID == requested {
							writeJSON(w, 200, graphEncoded(t, candidate))
							return
						}
					}
					writeJSON(w, 404, `{}`)
					return
				}
				details++
				status = tc.detailStatus
				if status == 0 {
					status = 200
				}
				body, err := json.Marshal(detail)
				if err != nil {
					t.Errorf("encode detail error = %v, want nil", err)
				}
				writeJSON(w, status, string(body))
			})
			write := eventWrite(t, oauthConnection(t, ProviderMicrosoft), owned)
			write.Attempt = EventCreateRecoverOnly
			got, err := client.PutEvent(t.Context(), write)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (EventReceipt{}) {
					t.Fatalf("recovery = (%+v,%v), want zero and %v", got, err, tc.wantErr)
				}
				noLeak(t, err)
			} else if err != nil || got.Ref.Resource != immutableFixtureID || got.Ref.ID != owned.ID {
				t.Fatalf("recovery = (%+v,%v), want immutable owned receipt", got, err)
			}
			if details != tc.wantDetails || len(fixture.refusedDials()) != 0 {
				t.Fatalf("details = %d refused dials %v, want %d and none", details, fixture.refusedDials(), tc.wantDetails)
			}
		})
	}
}

func TestMicrosoftMutationOwnershipNative(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		mutate         func(*graphOwnedMetadata)
		readStatus     int
		mutationStatus int
		wantErr        error
		wantEffects    int
	}{
		{name: "owned event moved calendar still updates immutable id", wantEffects: 1},
		{name: "foreign transaction cannot be mutated", mutate: func(m *graphOwnedMetadata) { m.TransactionID = "another" }, wantErr: core.ErrTimeMatchConflict},
		{name: "foreign ownership cannot be mutated", mutate: func(m *graphOwnedMetadata) { m.Ownership[0].Value = "another" }, wantErr: core.ErrTimeMatchConflict},
		{name: "unmarked event cannot be mutated", mutate: func(m *graphOwnedMetadata) { m.Ownership = nil }, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "different immutable id cannot be mutated", mutate: func(m *graphOwnedMetadata) { m.ID = "different" }, wantErr: core.ErrTimeMatchConflict},
		{name: "missing metadata cannot certify deletion", readStatus: 404, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "gone metadata cannot certify deletion", readStatus: 410, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "revoked authorization never mutates", readStatus: 401, wantErr: core.ErrTimeMatchConsent},
		{name: "rate limited ownership is not absent", readStatus: 429, wantErr: core.ErrTimeMatchResponse},
		{name: "server refused ownership is not absent", readStatus: 503, wantErr: core.ErrTimeMatchResponse},
		{name: "disappears after ownership before write", mutationStatus: 404, wantErr: core.ErrTimeMatchIndeterminate, wantEffects: 1},
		{name: "gone after ownership before write", mutationStatus: 410, wantErr: core.ErrTimeMatchIndeterminate, wantEffects: 1},
		{name: "gateway loses write response", mutationStatus: 503, wantErr: core.ErrTimeMatchIndeterminate, wantEffects: 1},
	}
	for _, tc := range cases {
		for _, deleting := range []bool{false, true} {
			name := tc.name + " update"
			if deleting {
				name = tc.name + " delete"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				owned := intent(t, "primary", "Moved", "2026-11-03T15:00:00Z", "2026-11-03T16:00:00Z")
				metadata := microsoftMetadata(immutableFixtureID, owned.ID.value)
				if tc.mutate != nil {
					tc.mutate(&metadata)
				}
				effects := 0
				client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
					if r.Header.Get("Prefer") != graphPreferUTC {
						t.Errorf("Prefer = %q, want immutable UTC", r.Header.Get("Prefer"))
					}
					if r.URL.Path != graphEventsPath+"/"+immutableFixtureID {
						t.Errorf("mutation path = %q, want immutable mailbox path", r.URL.Path)
					}
					if r.Method == http.MethodGet {
						status := tc.readStatus
						if status == 0 {
							status = 200
						}
						writeJSON(w, status, graphEncoded(t, metadata))
						return
					}
					effects++
					if tc.mutationStatus != 0 {
						writeJSON(w, tc.mutationStatus, `{}`)
						return
					}
					if deleting {
						w.WriteHeader(http.StatusNoContent)
						return
					}
					var detail graphCreated
					if err := json.Unmarshal(body, &detail); err != nil {
						t.Errorf("patch decode error = %v, want nil", err)
					}
					detail.ID = immutableFixtureID
					encoded, err := json.Marshal(detail)
					if err != nil {
						t.Errorf("patch encode error = %v, want nil", err)
					}
					writeJSON(w, 200, string(encoded))
				})
				ref := EventRef{ID: owned.ID, Calendar: owned.Calendar, Provider: ProviderMicrosoft, Resource: immutableFixtureID}
				var gotErr error
				if deleting {
					got, err := client.DeleteEvent(t.Context(), EventDelete{Connection: oauthConnection(t, ProviderMicrosoft), Ref: ref, Policy: fixturePolicy(t)})
					gotErr = err
					if err == nil && got != Deleted || err != nil && got != DeleteInvalid {
						t.Fatalf("DeleteEvent() = (%v,%v), want truthful outcome", got, err)
					}
				} else {
					got, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: oauthConnection(t, ProviderMicrosoft), Ref: ref, Intent: owned, ObservedAt: at(t, "2026-11-02T08:00:00Z"), Policy: fixturePolicy(t)})
					gotErr = err
					if err == nil && got.Ref != ref || err != nil && got != (EventReceipt{}) {
						t.Fatalf("UpdateEvent() = (%+v,%v), want exact reference or zero refusal", got, err)
					}
				}
				wantErr := tc.wantErr
				// PATCH404/410 is a definitive refusal to this update, but cannot prove
				// deletion. DELETE's public outcome must retain that uncertainty.
				if !deleting && (tc.mutationStatus == 404 || tc.mutationStatus == 410) {
					wantErr = core.ErrTimeMatchResponse
				}
				if wantErr == nil && gotErr != nil || wantErr != nil && !errors.Is(gotErr, wantErr) || effects != tc.wantEffects {
					t.Fatalf("mutation error = %v effects %d, want %v effects %d", gotErr, effects, wantErr, tc.wantEffects)
				}
				noLeak(t, gotErr)
				if got := len(fixture.record()); got != 1+tc.wantEffects {
					t.Fatalf("requests = %d, want ownership read plus %d effects", got, tc.wantEffects)
				}
			})
		}
	}
}

func FuzzMicrosoftOwnedMetadataSemanticClosure(f *testing.F) {
	seed := microsoftMetadata(immutableFixtureID, eventIDFixture)
	if err := seed.Validate(); err != nil {
		f.Fatalf("seed Validate error = %v, want nil", err)
	}
	raw, err := json.Marshal(seed)
	if err != nil {
		f.Fatalf("seed encode error = %v, want nil", err)
	}
	f.Add(raw)
	f.Add([]byte(`{"id":"foreign","transactionId":"other"}`))
	f.Add([]byte(`{"id":`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := decodeJSON[graphOwnedMetadata](data)
		if err != nil {
			if !errors.Is(err, core.ErrTimeMatchResponse) || got.ID != "" || got.TransactionID != "" || got.Ownership != nil {
				t.Fatalf("decode refusal = (%+v,%v), want zero typed response", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted Validate error = %v, want nil", err)
		}
		id, err := ParseEventID(eventIDFixture)
		if err != nil {
			t.Fatal(err)
		}
		ownErr := got.owns(id)
		wantOwned := got.TransactionID == eventIDFixture && len(got.Ownership) == 1 && got.Ownership[0].ID == graphOwnershipProperty && got.Ownership[0].Value == eventIDFixture
		contradictory := got.TransactionID != "" && got.TransactionID != eventIDFixture || len(got.Ownership) > 1
		if len(got.Ownership) == 1 {
			p := got.Ownership[0]
			contradictory = contradictory || p.ID != "" && p.ID != graphOwnershipProperty || p.Value != "" && p.Value != eventIDFixture
		}
		wantErr := core.ErrTimeMatchIndeterminate
		if contradictory {
			wantErr = core.ErrTimeMatchConflict
		}
		if wantOwned && ownErr != nil || !wantOwned && !errors.Is(ownErr, wantErr) {
			t.Fatalf("ownership error = %v facts %+v, want owned iff exact deterministic metadata", ownErr, got)
		}
		encoded, encodeErr := json.Marshal(got)
		if encodeErr != nil {
			t.Fatalf("encode error = %v, want nil", encodeErr)
		}
		again, decodeErr := decodeJSON[graphOwnedMetadata](encoded)
		if decodeErr != nil || again.ID != got.ID || again.TransactionID != got.TransactionID || len(again.Ownership) != len(got.Ownership) {
			t.Fatalf("round trip = (%+v,%v), want exact facts %+v", again, decodeErr, got)
		}
		second, secondErr := json.Marshal(again)
		if secondErr != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("second encoding = (%q,%v), want exact %q", second, secondErr, encoded)
		}
		for i := range got.Ownership {
			if got.Ownership[i] != again.Ownership[i] {
				t.Fatalf("round trip property %d = %+v, want %+v", i, again.Ownership[i], got.Ownership[i])
			}
		}
	})
}

func TestMicrosoftTickPrecisionNative(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, start, end string
		wantErr          error
	}{
		{name: "whole second", start: "2026-11-02T15:00:00Z", end: "2026-11-02T16:00:00Z"},
		{name: "first representable tick", start: "2026-11-02T15:00:00.0000001Z", end: "2026-11-02T16:00:00Z"},
		{name: "tick immediately below microsecond", start: "2026-11-02T15:00:00.0000009Z", end: "2026-11-02T16:00:00Z"},
		{name: "microsecond boundary", start: "2026-11-02T15:00:00.000001Z", end: "2026-11-02T16:00:00Z"},
		{name: "last tick below next second", start: "2026-11-02T15:00:00.9999999Z", end: "2026-11-02T16:00:00Z"},
		{name: "negative epoch representable tick", start: "1969-12-31T23:59:59.9999999Z", end: "1970-01-01T00:00:00Z"},
		{name: "cross epoch one tick each side", start: "1969-12-31T23:59:59.9999999Z", end: "1970-01-01T00:00:00.0000001Z"},
		{name: "earliest representable tick near minimum", start: "1677-09-21T00:12:43.1452242Z", end: "1677-09-21T00:12:43.1452243Z"},
		{name: "latest representable tick near maximum", start: "2262-04-11T23:47:16.8547756Z", end: "2262-04-11T23:47:16.8547758Z"},
		{name: "one tick interval at midnight", start: "2026-11-02T23:59:59.9999999Z", end: "2026-11-03T00:00:00Z"},
		{name: "one nanosecond below first tick", start: "2026-11-02T15:00:00.000000099Z", end: "2026-11-02T16:00:00Z", wantErr: core.ErrTimeMatchContract},
		{name: "one nanosecond above first tick", start: "2026-11-02T15:00:00.000000101Z", end: "2026-11-02T16:00:00Z", wantErr: core.ErrTimeMatchContract},
		{name: "one nanosecond above second in start", start: "2026-11-02T15:00:00.000000001Z", end: "2026-11-02T16:00:00Z", wantErr: core.ErrTimeMatchContract},
		{name: "one nanosecond above second in end", start: "2026-11-02T15:00:00Z", end: "2026-11-02T16:00:00.000000001Z", wantErr: core.ErrTimeMatchContract},
		{name: "below microsecond by one nanosecond", start: "2026-11-02T15:00:00.000000999Z", end: "2026-11-02T16:00:00Z", wantErr: core.ErrTimeMatchContract},
		{name: "above microsecond by one nanosecond", start: "2026-11-02T15:00:00.000001001Z", end: "2026-11-02T16:00:00Z", wantErr: core.ErrTimeMatchContract},
		{name: "negative epoch one nanosecond below tick", start: "1969-12-31T23:59:59.999999899Z", end: "1970-01-01T00:00:00Z", wantErr: core.ErrTimeMatchContract},
		{name: "negative epoch one nanosecond above tick", start: "1969-12-31T23:59:59.999999901Z", end: "1970-01-01T00:00:00Z", wantErr: core.ErrTimeMatchContract},
		{name: "minimum instant cannot be silently rounded", start: "1677-09-21T00:12:43.145224192Z", end: "1677-09-21T00:12:43.1452243Z", wantErr: core.ErrTimeMatchContract},
		{name: "maximum instant cannot be silently rounded", start: "2262-04-11T23:47:16.8547756Z", end: "2262-04-11T23:47:16.854775807Z", wantErr: core.ErrTimeMatchContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, body []byte) {
				var detail graphCreated
				if err := json.Unmarshal(body, &detail); err != nil {
					t.Errorf("body decode error = %v, want nil", err)
				}
				detail.ID = immutableFixtureID
				encoded, err := json.Marshal(detail)
				if err != nil {
					t.Errorf("encode error = %v, want nil", err)
				}
				writeJSON(w, 201, string(encoded))
			})
			owned := intent(t, "primary", "Tick", tc.start, tc.end)
			got, err := client.PutEvent(t.Context(), eventWrite(t, oauthConnection(t, ProviderMicrosoft), owned))
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (EventReceipt{}) || len(fixture.record()) != 0 {
					t.Fatalf("lossy write = (%+v,%v) requests %d, want zero contract refusal before HTTP", got, err, len(fixture.record()))
				}
				return
			}
			if err != nil || got.Ref.ID != owned.ID || len(fixture.record()) != 1 {
				t.Fatalf("exact tick write = (%+v,%v) requests %d, want exact receipt and one write", got, err, len(fixture.record()))
			}
		})
	}
}

func FuzzMicrosoftOwnedPageSemanticClosure(f *testing.F) {
	seed := graphOwnedPage{Value: []graphOwnedMetadata{microsoftMetadata(immutableFixtureID, eventIDFixture)}}
	documented := graphOwnedPage{Value: []graphOwnedMetadata{{ID: immutableFixtureID, TransactionID: eventIDFixture}}}
	if err := documented.Validate(); err != nil {
		f.Fatalf("documented candidate seed error = %v, want nil", err)
	}
	documentedBytes, documentedErr := json.Marshal(documented)
	if documentedErr != nil {
		f.Fatalf("documented candidate encode error = %v, want nil", documentedErr)
	}
	f.Add(documentedBytes)
	if err := seed.Validate(); err != nil {
		f.Fatalf("seed Validate error = %v, want nil", err)
	}
	raw, err := json.Marshal(seed)
	if err != nil {
		f.Fatalf("seed encode error = %v, want nil", err)
	}
	f.Add(raw)
	empty, emptyErr := json.Marshal(graphOwnedPage{Value: []graphOwnedMetadata{}})
	if emptyErr != nil {
		f.Fatalf("empty seed encode error = %v, want nil", emptyErr)
	}
	f.Add(empty)
	f.Add([]byte(`{"value":null}`))
	f.Add([]byte(`{"value":`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := decodeJSON[graphOwnedPage](data)
		if err != nil {
			if !errors.Is(err, core.ErrTimeMatchResponse) || got.Value != nil || got.Next != "" {
				t.Fatalf("page refusal = (%+v,%v), want zero typed response", got, err)
			}
			return
		}
		if got.Value == nil {
			t.Fatal("accepted page value = nil, want explicit collection")
		}
		for _, item := range got.Value {
			if !visibleASCII([]byte(item.ID)) {
				t.Fatalf("accepted resource id = %q, want visible ASCII", item.ID)
			}
		}
		encoded, encodeErr := json.Marshal(got)
		if encodeErr != nil {
			t.Fatalf("encode error = %v, want nil", encodeErr)
		}
		again, decodeErr := decodeJSON[graphOwnedPage](encoded)
		if decodeErr != nil || again.Next != got.Next || len(again.Value) != len(got.Value) {
			t.Fatalf("page round trip = (%+v,%v), want exact page %+v", again, decodeErr, got)
		}
		second, secondErr := json.Marshal(again)
		if secondErr != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("second page encoding = (%q,%v), want exact %q", second, secondErr, encoded)
		}
		for i := range got.Value {
			a, b := got.Value[i], again.Value[i]
			if a.ID != b.ID || a.TransactionID != b.TransactionID || len(a.Ownership) != len(b.Ownership) {
				t.Fatalf("item %d = %+v, want %+v", i, b, a)
			}
			for j := range a.Ownership {
				if a.Ownership[j] != b.Ownership[j] {
					t.Fatalf("property %d/%d = %+v, want %+v", i, j, b.Ownership[j], a.Ownership[j])
				}
			}
		}
	})
}

func TestMicrosoftCalendarMoveKeepsOwnedMutationReachable(t *testing.T) {
	t.Parallel()
	owned := intent(t, "primary", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")
	var mutex sync.Mutex
	moved, deleted := false, false
	mutableID := "changes-on-calendar-move"
	client, _ := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		mutex.Lock()
		defer mutex.Unlock()
		immutable := strings.Contains(r.Header.Get("Prefer"), `IdType="ImmutableId"`)
		if r.Method == http.MethodPost {
			detail := graphOwnedDetail(t, owned)
			if !immutable {
				detail.ID = mutableID
			}
			encoded, err := json.Marshal(detail)
			if err != nil {
				t.Errorf("encode error = %v, want nil", err)
			}
			writeJSON(w, 201, string(encoded))
			return
		}
		requested := strings.TrimPrefix(r.URL.Path, graphEventsPath+"/")
		if deleted || requested != immutableFixtureID && (!moved && requested != mutableID || moved) {
			writeJSON(w, 404, `{}`)
			return
		}
		if !immutable && requested == immutableFixtureID {
			writeJSON(w, 404, `{}`)
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, graphEncoded(t, microsoftMetadata(immutableFixtureID, owned.ID.value)))
		case http.MethodPatch:
			var detail graphCreated
			if err := json.Unmarshal(body, &detail); err != nil {
				t.Errorf("decode patch error = %v, want nil", err)
			}
			detail.ID = immutableFixtureID
			encoded, err := json.Marshal(detail)
			if err != nil {
				t.Errorf("encode patch error = %v, want nil", err)
			}
			writeJSON(w, 200, string(encoded))
		case http.MethodDelete:
			deleted = true
			w.WriteHeader(204)
		default:
			writeJSON(w, 405, `{}`)
		}
	})
	connection := oauthConnection(t, ProviderMicrosoft)
	created, err := client.PutEvent(t.Context(), eventWrite(t, connection, owned))
	if err != nil {
		t.Fatalf("create = (%+v,%v), want receipt", created, err)
	}
	mutex.Lock()
	moved = true
	mutex.Unlock()
	changed := owned
	changed.Interval = window(t, "2026-11-03T15:00:00Z", "2026-11-03T16:00:00Z")
	updated, updateErr := client.UpdateEvent(t.Context(), EventUpdate{Connection: connection, Ref: created.Ref, Intent: changed, ObservedAt: at(t, "2026-11-02T08:00:00Z"), Policy: fixturePolicy(t)})
	if updateErr != nil || updated.Ref != created.Ref {
		t.Fatalf("update after actual fixture calendar move = (%+v,%v), want stable receipt %+v", updated, updateErr, created.Ref)
	}
	outcome, deleteErr := client.DeleteEvent(t.Context(), EventDelete{Connection: connection, Ref: updated.Ref, Policy: fixturePolicy(t)})
	mutex.Lock()
	gotDeleted := deleted
	mutex.Unlock()
	if deleteErr != nil || outcome != Deleted || !gotDeleted {
		t.Fatalf("delete after calendar move = (%v,%v) effect %t, want actual deletion", outcome, deleteErr, gotDeleted)
	}
}

func TestMicrosoftPagingNeverBroadensPrivacyNative(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		change    func(url.Values)
		wantKnown bool
		wantCalls int
	}{
		{name: "opaque paging retains exact timing projection", wantKnown: true, wantCalls: 2},
		{name: "dropped timing selection blocks second request", change: func(q url.Values) { q.Del("$select") }, wantCalls: 1},
		{name: "full event selection blocks second request", change: func(q url.Values) { q.Set("$select", "id,subject,body,attendees") }, wantCalls: 1},
		{name: "private expansion cannot be added", change: func(q url.Values) { q.Set("$expand", "attachments") }, wantCalls: 1},
		{name: "filter cannot be injected", change: func(q url.Values) { q.Set("$filter", "subject ne ''") }, wantCalls: 1},
		{name: "window start cannot change", change: func(q url.Values) { q.Set("startDateTime", "2020-01-01T00:00:00Z") }, wantCalls: 1},
		{name: "window end cannot change", change: func(q url.Values) { q.Set("endDateTime", "2030-01-01T00:00:00Z") }, wantCalls: 1},
		{name: "duplicate select cannot add fields", change: func(q url.Values) { q.Add("$select", "subject") }, wantCalls: 1},
		{name: "original page size cannot change", change: func(q url.Values) { q.Set("$top", "999") }, wantCalls: 1},
		{name: "unknown query option cannot broaden scope", change: func(q url.Values) { q.Set("$search", "private") }, wantCalls: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			client, _ := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				calls++
				if r.URL.Query().Has("$skiptoken") {
					writeJSON(w, 200, `{"value":[]}`)
					return
				}
				query := r.URL.Query()
				query.Set("$skiptoken", "opaque+/==")
				if tc.change != nil {
					tc.change(query)
				}
				page := seedGraphView{Value: []seedGraphEvent{}, NextLink: "https://" + graphHost + r.URL.Path + "?" + query.Encode()}
				encoded, err := json.Marshal(page)
				if err != nil {
					t.Errorf("page encode error = %v, want nil", err)
				}
				writeJSON(w, 200, string(encoded))
			})
			got, err := client.Availability(t.Context(), AvailabilityRequest{Connection: oauthConnection(t, ProviderMicrosoft), Calendars: []CalendarID{calendarID(t, "primary")}, Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Policy: fixturePolicy(t)})
			want := AvailabilityUnknown
			if tc.wantKnown {
				want = AvailabilityKnown
			}
			if err != nil || len(got.Calendars) != 1 || got.Calendars[0].State != want || calls != tc.wantCalls || len(got.Calendars[0].Busy) != 0 {
				t.Fatalf("paged availability = (%+v,%v) calls %d, want state %v and exactly %d privacy-scoped calls", got, err, calls, want, tc.wantCalls)
			}
		})
	}
}

func TestMicrosoftAbsentCollectionNeverMeansFreeNative(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body string
		wantKnown  bool
	}{
		{name: "explicit empty collection is known free", body: `{"value":[]}`, wantKnown: true},
		{name: "absent collection is unknown", body: `{}`},
		{name: "null collection is unknown", body: `{"value":null}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeJSON(w, 200, tc.body) })
			connection := oauthConnection(t, ProviderMicrosoft)
			got, err := client.Availability(t.Context(), AvailabilityRequest{Connection: connection, Calendars: []CalendarID{calendarID(t, "primary")}, Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), Policy: fixturePolicy(t)})
			want := AvailabilityUnknown
			if tc.wantKnown {
				want = AvailabilityKnown
			}
			if err != nil || len(got.Calendars) != 1 || got.Calendars[0].State != want || len(got.Calendars[0].Busy) != 0 {
				t.Fatalf("availability = (%+v,%v), want state %v without invented busy data", got, err, want)
			}
			listed, listErr := client.ListCalendars(t.Context(), CalendarListRequest{Connection: connection, Policy: fixturePolicy(t)})
			if tc.wantKnown {
				if listErr != nil || len(listed.Calendars) != 0 {
					t.Fatalf("explicit empty list = (%+v,%v), want complete empty list", listed, listErr)
				}
			} else if !errors.Is(listErr, core.ErrTimeMatchResponse) || listed.Provider != Provider(0) || listed.Calendars != nil {
				t.Fatalf("absent list = (%+v,%v), want zero typed refusal", listed, listErr)
			}
			if len(fixture.record()) != 2 {
				t.Fatalf("requests = %d, want actual availability and list calls", len(fixture.record()))
			}
		})
	}
}

func FuzzMicrosoftPagingProjectionSemanticClosure(f *testing.F) {
	required := url.Values{"$select": {"id,transactionId"}, "$expand": {"singleValueExtendedProperties($filter=id eq '" + graphOwnershipProperty + "')"}, "$filter": {"singleValueExtendedProperties/Any(ep: ep/value eq '" + eventIDFixture + "')"}}
	seed, err := endpoint("https://" + graphHost + graphEventsPath + "?" + required.Encode())
	if err != nil {
		f.Fatalf("seed endpoint error = %v, want nil", err)
	}
	seedURL := seed.HTTPURL()
	f.Add(seedURL.String())
	f.Add("https://graph.microsoft.com/v1.0/me/events?$select=subject")
	f.Add("")
	f.Fuzz(func(t *testing.T, text string) {
		got, err := graphNext(text, graphEventsPath, required)
		if err != nil {
			if !errors.Is(err, core.ErrTimeMatchResponse) || got != (core.HTTPEndpoint{}) {
				t.Fatalf("paging refusal = (%+v,%v), want zero typed response", got, err)
			}
			return
		}
		parsed, parseErr := url.Parse(text)
		if parseErr != nil {
			t.Fatalf("accepted native URL error = %v, want nil", parseErr)
		}
		query, queryErr := url.ParseQuery(parsed.RawQuery)
		if queryErr != nil {
			t.Fatalf("accepted native query error = %v, want nil", queryErr)
		}
		if parsed.Scheme != "https" || parsed.Host != graphHost || parsed.Path != graphEventsPath || parsed.User != nil {
			t.Fatalf("accepted URL = %+v, want bound Graph events origin", parsed)
		}
		for name, values := range required {
			if !slices.Equal(query[name], values) {
				t.Fatalf("accepted %s = %q, want exact %q", name, query[name], values)
			}
		}
		for name := range query {
			if _, original := required[name]; !original && name != "$skip" && name != "$skiptoken" {
				t.Fatalf("accepted added query = %q, want only native paging", name)
			}
		}
		projected := got.HTTPURL()
		again, againErr := graphNext(projected.String(), graphEventsPath, required)
		if againErr != nil || again != got {
			t.Fatalf("paging round trip = (%+v,%v), want exact endpoint %+v", again, againErr, got)
		}
	})
}

func TestMicrosoftMetadataPagingNative(t *testing.T) {
	t.Parallel()
	for _, recovering := range []bool{false, true} {
		name := "calendar list preserves metadata query across pages"
		if recovering {
			name = "owned recovery preserves marker query across pages"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			owned := intent(t, "primary", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")
			calls := 0
			client, _ := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				calls++
				if r.URL.Path == graphEventsPath+"/"+immutableFixtureID {
					if r.URL.Query().Has("$expand") {
						writeJSON(w, 200, graphEncoded(t, microsoftMetadata(immutableFixtureID, owned.ID.value)))
						return
					}
					body, err := json.Marshal(graphOwnedDetail(t, owned))
					if err != nil {
						t.Errorf("detail encode error = %v, want nil", err)
					}
					writeJSON(w, 200, string(body))
					return
				}
				if r.URL.Query().Has("$skiptoken") {
					if recovering {
						writeJSON(w, 200, graphCandidateCollection(t, graphOwnedPage{Value: []graphOwnedMetadata{microsoftMetadata(immutableFixtureID, owned.ID.value)}}))
					} else {
						writeJSON(w, 200, string(seedJSON(t, seedGraphCalendars{Value: []seedGraphCalendar{{ID: "primary", IsDefaultCalendar: true, CanEdit: true}}})))
					}
					return
				}
				query := r.URL.Query()
				query.Set("$skiptoken", "opaque+/==")
				next := "https://" + graphHost + r.URL.Path + "?" + query.Encode()
				if recovering {
					writeJSON(w, 200, graphCandidateCollection(t, graphOwnedPage{Value: []graphOwnedMetadata{}, Next: next}))
				} else {
					writeJSON(w, 200, string(seedJSON(t, seedGraphCalendars{Value: []seedGraphCalendar{}, NextLink: next})))
				}
			})
			if recovering {
				write := eventWrite(t, oauthConnection(t, ProviderMicrosoft), owned)
				write.Attempt = EventCreateRecoverOnly
				got, err := client.PutEvent(t.Context(), write)
				if err != nil || got.Ref.ID != owned.ID || got.Ref.Resource != immutableFixtureID || calls != 4 {
					t.Fatalf("paged recovery = (%+v,%v) calls %d, want owned receipt and4GETs", got, err, calls)
				}
			} else {
				got, err := client.ListCalendars(t.Context(), CalendarListRequest{Connection: oauthConnection(t, ProviderMicrosoft), Policy: fixturePolicy(t)})
				if err != nil || len(got.Calendars) != 1 || got.Calendars[0].ID != owned.Calendar || calls != 2 {
					t.Fatalf("paged list = (%+v,%v) calls %d, want one calendar and2GETs", got, err, calls)
				}
			}
		})
	}
}

// Microsoft's documented filtered collection omits the matching property even
// though the filter matched it. Only a single-resource expansion returns it.
func TestMicrosoftRecoveryUsesDocumentedSingleResourceExpansion(t *testing.T) {
	t.Parallel()
	owned := intent(t, "primary", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")
	collectionReads, ownershipReads, detailReads, mutations := 0, 0, 0, 0
	client, _ := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if r.Method != http.MethodGet {
			mutations++
			writeJSON(w, 500, `{}`)
			return
		}
		if r.URL.Path == graphEventsPath {
			collectionReads++
			// Deliberately omit the property even if the caller asks for expansion.
			page := struct {
				Value []struct {
					ID            string `json:"id"`
					TransactionID string `json:"transactionId"`
				} `json:"value"`
			}{Value: []struct {
				ID            string `json:"id"`
				TransactionID string `json:"transactionId"`
			}{{ID: immutableFixtureID, TransactionID: owned.ID.value}}}
			encoded, err := json.Marshal(page)
			if err != nil {
				t.Errorf("collection encode error = %v, want nil", err)
			}
			writeJSON(w, 200, string(encoded))
			return
		}
		if r.URL.Query().Has("$expand") {
			ownershipReads++
			writeJSON(w, 200, graphEncoded(t, microsoftMetadata(immutableFixtureID, owned.ID.value)))
			return
		}
		detailReads++
		encoded, err := json.Marshal(graphOwnedDetail(t, owned))
		if err != nil {
			t.Errorf("detail encode error = %v, want nil", err)
		}
		writeJSON(w, 200, string(encoded))
	})
	write := eventWrite(t, oauthConnection(t, ProviderMicrosoft), owned)
	write.Attempt = EventCreateRecoverOnly
	got, err := client.PutEvent(t.Context(), write)
	if err != nil || got.Ref.ID != owned.ID || got.Ref.Resource != immutableFixtureID || collectionReads != 1 || ownershipReads != 1 || detailReads != 1 || mutations != 0 {
		t.Fatalf("documented recovery = (%+v,%v), reads collection/ownership/detail %d/%d/%d mutations %d, want verified receipt through1/1/1 reads and0mutations", got, err, collectionReads, ownershipReads, detailReads, mutations)
	}
}

func TestMicrosoftRecoveryCandidateRequiresResourceAuthority(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		mutate      func(*graphOwnedMetadata)
		status      int
		raw         string
		wantErr     error
		wantDetails int
	}{
		{name: "exact individual expansion authorizes one own detail read", wantDetails: 1},
		{name: "omitted individual expansion stays indeterminate", mutate: func(m *graphOwnedMetadata) { m.Ownership = nil }, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "missing individual transaction stays indeterminate", mutate: func(m *graphOwnedMetadata) { m.TransactionID = "" }, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "missing property id stays indeterminate", mutate: func(m *graphOwnedMetadata) { m.Ownership[0].ID = "" }, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "missing property value stays indeterminate", mutate: func(m *graphOwnedMetadata) { m.Ownership[0].Value = "" }, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "contradictory individual transaction is conflict", mutate: func(m *graphOwnedMetadata) { m.TransactionID = "another" }, wantErr: core.ErrTimeMatchConflict},
		{name: "contradictory property namespace is conflict", mutate: func(m *graphOwnedMetadata) { m.Ownership[0].ID = "String {foreign} Name Other" }, wantErr: core.ErrTimeMatchConflict},
		{name: "contradictory property value is conflict", mutate: func(m *graphOwnedMetadata) { m.Ownership[0].Value = "another" }, wantErr: core.ErrTimeMatchConflict},
		{name: "explicit contradiction outranks another missing field", mutate: func(m *graphOwnedMetadata) { m.TransactionID = ""; m.Ownership[0].Value = "another" }, wantErr: core.ErrTimeMatchConflict},
		{name: "duplicate individual ownership is conflict", mutate: func(m *graphOwnedMetadata) { m.Ownership = append(m.Ownership, m.Ownership[0]) }, wantErr: core.ErrTimeMatchConflict},
		{name: "candidate disappeared before ownership read", status: 404, wantErr: core.ErrTimeMatchIndeterminate},
		{name: "ownership consent revoked", status: 401, wantErr: core.ErrTimeMatchConsent},
		{name: "malformed ownership response cannot imply conflict", raw: `{"id":`, wantErr: core.ErrTimeMatchIndeterminate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := intent(t, "primary", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")
			metadata := microsoftMetadata(immutableFixtureID, owned.ID.value)
			if tc.mutate != nil {
				tc.mutate(&metadata)
			}
			ownershipReads, detailReads, mutations := 0, 0, 0
			client, _ := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method != http.MethodGet {
					mutations++
					writeJSON(w, 500, `{}`)
					return
				}
				if r.URL.Path == graphEventsPath {
					if r.URL.Query().Has("$expand") {
						t.Errorf("collection expand = %q, want absent documented candidate query", r.URL.Query().Get("$expand"))
					}
					writeJSON(w, 200, graphCandidateCollection(t, graphOwnedPage{Value: []graphOwnedMetadata{microsoftMetadata(immutableFixtureID, owned.ID.value)}}))
					return
				}
				if r.URL.Query().Has("$expand") {
					ownershipReads++
					body := tc.raw
					if body == "" {
						body = graphEncoded(t, metadata)
					}
					status := tc.status
					if status == 0 {
						status = 200
					}
					writeJSON(w, status, body)
					return
				}
				detailReads++
				body, err := json.Marshal(graphOwnedDetail(t, owned))
				if err != nil {
					t.Errorf("detail encode error = %v, want nil", err)
				}
				writeJSON(w, 200, string(body))
			})
			write := eventWrite(t, oauthConnection(t, ProviderMicrosoft), owned)
			write.Attempt = EventCreateRecoverOnly
			got, err := client.PutEvent(t.Context(), write)
			if tc.wantErr == nil {
				if err != nil || got.Ref.ID != owned.ID {
					t.Fatalf("owned expansion = (%+v,%v), want verified own receipt", got, err)
				}
			} else if !errors.Is(err, tc.wantErr) || got != (EventReceipt{}) {
				t.Fatalf("unverified expansion = (%+v,%v), want zero and %v", got, err, tc.wantErr)
			}
			if ownershipReads != 1 || detailReads != tc.wantDetails || mutations != 0 {
				t.Fatalf("ownership/detail reads = %d/%d mutations%d, want1/%d and0", ownershipReads, detailReads, mutations, tc.wantDetails)
			}
			noLeak(t, err)
		})
	}
}
