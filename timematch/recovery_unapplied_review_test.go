package timematch

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// This local TLS provider double loses the first reply before applying the
// create. It proves recovery from an unapplied attempt, not live-provider
// consistency or a simulated successful booking in Kernel.
func TestRecoveryCompletesUnappliedDeterministicCreate(t *testing.T) {
	t.Parallel()
	for _, providerKind := range []Provider{ProviderGoogle, ProviderApple} {
		t.Run(providerKind.String(), func(t *testing.T) {
			t.Parallel()
			connection := oauthConnection(t, ProviderGoogle)
			owned := intent(t, "primary", "Review", "2026-11-02T15:00:00Z", "2026-11-02T15:30:00Z")
			if providerKind == ProviderApple {
				connection = appleConnection(t)
				owned = appleMutationIntent(t)
			}
			var attempts, entities atomic.Int32
			client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
				effect := r.Method == http.MethodPost || r.Method == http.MethodPut
				if effect {
					if attempts.Add(1) == 1 {
						cutConnection(w)
						return
					}
					if providerKind == ProviderApple && r.Header.Get("If-None-Match") != "*" {
						t.Errorf("Apple recovery If-None-Match = %q, want *", r.Header.Get("If-None-Match"))
						w.WriteHeader(400)
						return
					}
					if entities.CompareAndSwap(0, 1) {
						if providerKind == ProviderGoogle {
							writeJSON(w, 200, googleFixtureWithETag(googleStored(t, owned, owned.Summary.value, "confirmed")))
						} else {
							w.WriteHeader(201)
						}
					} else if providerKind == ProviderGoogle {
						writeJSON(w, 409, `{}`)
					} else {
						w.WriteHeader(412)
					}
					return
				}
				if entities.Load() == 0 {
					if providerKind == ProviderGoogle && strings.HasSuffix(r.URL.Path, "/events") {
						writeJSON(w, 200, `{"items":[]}`)
					} else {
						w.WriteHeader(404)
					}
					return
				}
				if providerKind == ProviderGoogle {
					stored := googleStored(t, owned, owned.Summary.value, "confirmed")
					if strings.HasSuffix(r.URL.Path, "/events") {
						writeJSON(w, 200, fmt.Sprintf(`{"items":[{"id":%q,"etag":%q}]}`, owned.ID.String(), googleFixtureETag(stored)))
					} else {
						writeJSON(w, 200, googleFixtureWithETag(stored))
					}
				} else {
					w.Header().Set("Content-Type", "text/calendar")
					if _, err := w.Write(appleMutationBody(t)); err != nil {
						t.Errorf("owned resource response write=%v,want nil", err)
					}
				}
			})
			request := eventWrite(t, connection, owned)
			first, firstErr := client.PutEvent(t.Context(), request)
			if !errors.Is(firstErr, core.ErrTimeMatchIndeterminate) || first != (EventReceipt{}) || entities.Load() != 0 {
				t.Fatalf("unapplied first create=(%v,%v),entities%d,wantzero,Indeterminate,0", first, firstErr, entities.Load())
			}
			request.Attempt = EventCreateGuardedRecovery
			recovered, recoverErr := client.PutEvent(t.Context(), request)
			if recoverErr != nil || recovered.Ref.ID != owned.ID || entities.Load() != 1 {
				t.Fatalf("recovery=(%v,%v),entities%d,wantownedreceipt,nil,1", recovered, recoverErr, entities.Load())
			}
			again, againErr := client.PutEvent(t.Context(), request)
			if againErr != nil || again != recovered || entities.Load() != 1 {
				t.Fatalf("repeated recovery=(%v,%v),entities%d,wantexactreceipt,nil,1", again, againErr, entities.Load())
			}
			var firstBody []byte
			for _, call := range fixture.record() {
				if call.method == http.MethodPost || call.method == http.MethodPut {
					if firstBody == nil {
						firstBody = call.body
					} else if string(firstBody) != string(call.body) {
						t.Fatalf("recovery body changed %q→%q,wantexactsameintent", firstBody, call.body)
					}
				}
			}
		})
	}
}
