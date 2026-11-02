package timematch

import (
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const appleMutationHome = "https://p42-caldav.icloud.com/123456/calendars/home/"

func appleMutationIntent(t testing.TB) EventIntent {
	t.Helper()
	return intent(t, appleMutationHome, "Owned visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")
}
func appleMutationRef(t testing.TB) EventRef {
	t.Helper()
	owned := appleMutationIntent(t)
	return EventRef{ID: owned.ID, Calendar: owned.Calendar, Provider: ProviderApple, Resource: appleMutationHome + owned.ID.String() + icsResourceExt}
}
func appleMutationBody(t testing.TB) []byte {
	t.Helper()
	body, err := writeICS(appleMutationIntent(t), at(t, "2026-11-02T08:00:00Z"))
	if err != nil {
		t.Fatalf("owned ICS fixture error=%v, want nil", err)
	}
	return body
}

// Calendar collection URLs can satisfy a prefix check. They must never reach
// any HTTP operation, since DELETE there removes a whole calendar.
func TestAppleDeleteRefRejectsCalendarCollectionBeforeHTTP(t *testing.T) {
	t.Parallel()
	client, provider := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { w.WriteHeader(http.StatusNoContent) })
	ref := appleMutationRef(t)
	ref.Resource = appleMutationHome
	got, err := client.DeleteEvent(t.Context(), EventDelete{Connection: appleConnection(t), Ref: ref, Policy: fixturePolicy(t)})
	if !errors.Is(err, core.ErrTimeMatchContract) || got != DeleteInvalid || len(provider.record()) != 0 {
		t.Fatalf("calendar collection delete=%v/%v, requests=%d, want Invalid/Contract/zeroHTTP", got, err, len(provider.record()))
	}
}

func TestAppleUpdateUsesObservedStrongETag(t *testing.T) {
	t.Parallel()
	body := appleMutationBody(t)
	client, provider := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/calendar")
			w.Header().Set("ETag", `"revision-7"`)
			_, _ = w.Write(body)
		case http.MethodPut:
			if r.Header.Get("If-Match") != `"revision-7"` {
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	ref := appleMutationRef(t)
	got, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: appleConnection(t), Ref: ref, Intent: appleMutationIntent(t), ObservedAt: at(t, "2026-11-02T09:00:00Z"), Policy: fixturePolicy(t)})
	requests := provider.record()
	if err != nil || got.Ref != ref || len(requests) != 2 || requests[0].method != http.MethodGet || requests[1].method != http.MethodPut || requests[1].header.Get("If-Match") != `"revision-7"` {
		t.Fatalf("conditional update=%+v/%v, requests=%d, want bound receipt and GET then PUT with observed strong ETag", got, err, len(requests))
	}
}

func TestAppleRecoveryNeverCreatesAbsentResource(t *testing.T) {
	t.Parallel()
	client, provider := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { w.WriteHeader(http.StatusNotFound) })
	request := eventWrite(t, appleConnection(t), appleMutationIntent(t))
	request.Attempt = EventCreateRecoverOnly
	got, err := client.PutEvent(t.Context(), request)
	requests := provider.record()
	if !errors.Is(err, core.ErrTimeMatchIndeterminate) || got != (EventReceipt{}) || len(requests) != 1 || requests[0].method != http.MethodGet {
		t.Fatalf("absent recovery=%+v/%v, requests=%d, want zero/Indeterminate/GET-only", got, err, len(requests))
	}
}

func TestAppleCollisionUnreadableStateNeverClaimsConflict(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		status int
		cut    bool
	}{
		{name: "server unavailable after collision", status: 503}, {name: "resource vanished after collision", status: 404}, {name: "lost readback connection", cut: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, provider := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method == http.MethodPut {
					w.WriteHeader(http.StatusPreconditionFailed)
					return
				}
				if tc.cut {
					cutConnection(w)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, "unreadable")
			})
			got, err := client.PutEvent(t.Context(), eventWrite(t, appleConnection(t), appleMutationIntent(t)))
			requests := provider.record()
			if !errors.Is(err, core.ErrTimeMatchIndeterminate) || errors.Is(err, core.ErrTimeMatchConflict) || got != (EventReceipt{}) || len(requests) != 2 || requests[0].method != http.MethodPut || requests[1].method != http.MethodGet {
				t.Fatalf("collision readback=%+v/%v, requests=%d, want zero/Indeterminate/noConflict and PUT then GET", got, err, len(requests))
			}
		})
	}
}
