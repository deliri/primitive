package timematch

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
			readsOnly := len(requests) >= 2
			for index, request := range requests {
				if index == 0 {
					readsOnly = readsOnly && request.method == http.MethodPut
				} else {
					readsOnly = readsOnly && request.method == http.MethodGet
				}
			}
			if !errors.Is(err, core.ErrTimeMatchIndeterminate) || errors.Is(err, core.ErrTimeMatchConflict) || got != (EventReceipt{}) || !readsOnly {
				t.Fatalf("collision readback=%+v/%v, requests=%d, want zero/Indeterminate/noConflict and PUT then GET", got, err, len(requests))
			}
		})
	}
}

func TestAppleDeleteHostileResourceBindings(t *testing.T) {
	t.Parallel()
	canonical := appleMutationRef(t)
	for _, tc := range []struct{ name, resource string }{
		{"calendar collection", appleMutationHome},
		{"child collection", appleMutationHome + "nested/"},
		{"event as collection", canonical.Resource + "/"},
		{"different owned event", appleMutationHome + "aaaaa.ics"},
		{"missing event extension", appleMutationHome + eventIDFixture},
		{"extension suffix", canonical.Resource + ".bak"},
		{"parent traversal", appleMutationHome + "../" + eventIDFixture + ".ics"},
		{"encoded parent traversal", appleMutationHome + "%2e%2e/" + eventIDFixture + ".ics"},
		{"nested event path", appleMutationHome + "nested/" + eventIDFixture + ".ics"},
		{"duplicate slash alias", appleMutationHome + "/" + eventIDFixture + ".ics"},
		{"event query", canonical.Resource + "?delete=calendar"},
		{"event fragment", canonical.Resource + "#calendar"},
		{"foreign origin", "https://attacker.example/" + eventIDFixture + ".ics"},
		{"unrelated calendar", "https://p42-caldav.icloud.com/123456/calendars/other/" + eventIDFixture + ".ics"},
		{"other partition", "https://p43-caldav.icloud.com/123456/calendars/home/" + eventIDFixture + ".ics"},
		{"insecure transport", "http://p42-caldav.icloud.com/123456/calendars/home/" + eventIDFixture + ".ics"},
		{"relative resource", eventIDFixture + ".ics"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, provider := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { w.WriteHeader(204) })
			ref := canonical
			ref.Resource = tc.resource
			got, err := client.DeleteEvent(t.Context(), EventDelete{Connection: appleConnection(t), Ref: ref, Policy: fixturePolicy(t)})
			if !errors.Is(err, core.ErrTimeMatchContract) || got != DeleteInvalid || len(provider.record()) != 0 || len(provider.refusedDials()) != 0 {
				t.Fatalf("resource binding %q=%v/%v, requests/refused=%d/%d, want Invalid/Contract and zeroHTTP", tc.name, got, err, len(provider.record()), len(provider.refusedDials()))
			}
		})
	}
}

func TestAppleMutationStrongETagHostileBoundaryTable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, etag string
		duplicate  bool
		want       bool
	}{
		{name: "empty opaque tag is strong", etag: `""`, want: true},
		{name: "lowest opaque tag byte", etag: `"!"`, want: true},
		{name: "byte after quote", etag: `"#"`, want: true},
		{name: "upper ASCII opaque byte", etag: `"~"`, want: true},
		{name: "comma inside one opaque tag", etag: `"version,7"`, want: true},
		{name: "backslash is opaque data", etag: `"version\7"`, want: true},
		{name: "ordinary strong revision", etag: `"revision-7"`, want: true},
		{name: "missing validator"},
		{name: "wildcard is not a revision", etag: `*`},
		{name: "weak revision cannot establish CAS", etag: `W/"revision-7"`},
		{name: "unquoted revision", etag: `revision-7`},
		{name: "unterminated tag", etag: `"revision-7`},
		{name: "unopened tag", etag: `revision-7"`},
		{name: "multiple revisions are not one observation", etag: `"one", "two"`},
		{name: "embedded quote is not opaque data", etag: `"bad"quote"`},
		{name: "space inside tag is outside opaque grammar", etag: `"bad revision"`},
		{name: "trailing data", etag: `"revision-7"suffix`},
		{name: "duplicate ETag fields", etag: `"revision-7"`, duplicate: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, deleting := range []bool{false, true} {
				body := appleMutationBody(t)
				client, provider := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
					if r.Method == http.MethodGet {
						w.Header().Set("Content-Type", "text/calendar")
						if tc.etag != "" {
							w.Header().Set("ETag", tc.etag)
						}
						if tc.duplicate {
							w.Header().Add("ETag", `"other"`)
						}
						_, _ = w.Write(body)
						return
					}
					if r.Header.Get("If-Match") != tc.etag {
						w.WriteHeader(412)
						return
					}
					w.WriteHeader(204)
				})
				ref := appleMutationRef(t)
				var err error
				if deleting {
					var out DeleteOutcome
					out, err = client.DeleteEvent(t.Context(), EventDelete{Connection: appleConnection(t), Ref: ref, Policy: fixturePolicy(t)})
					if tc.want && out != Deleted || !tc.want && out != DeleteInvalid {
						t.Fatalf("conditional delete outcome=%v, want success=%t", out, tc.want)
					}
				} else {
					var out EventReceipt
					out, err = client.UpdateEvent(t.Context(), EventUpdate{Connection: appleConnection(t), Ref: ref, Intent: appleMutationIntent(t), ObservedAt: at(t, "2026-11-02T09:00:00Z"), Policy: fixturePolicy(t)})
					if tc.want && out.Ref != ref || !tc.want && out != (EventReceipt{}) {
						t.Fatalf("conditional update output=%+v, want bound receipt=%t", out, tc.want)
					}
				}
				requests := provider.record()
				wantRequests := 1
				if tc.want {
					wantRequests = 2
				}
				if tc.want && err != nil || !tc.want && !errors.Is(err, core.ErrTimeMatchResponse) || len(requests) != wantRequests {
					t.Fatalf("mutation deleting=%t, etag%q = error%v requests%d, want allowed%t requests%d", deleting, tc.etag, err, len(requests), tc.want, wantRequests)
				}
				if tc.want && requests[1].header.Get("If-Match") != tc.etag {
					t.Fatalf("observed revision forwarded=%q, want exact%q", requests[1].header.Get("If-Match"), tc.etag)
				}
			}
		})
	}
}

func TestAppleDeleteHTTPOutcomeAndNoRecreation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                     string
		readStatus, deleteStatus int
		want                     DeleteOutcome
		wantErr                  error
		wantCalls                int
	}{
		{name: "200 confirms enacted deletion", readStatus: 200, deleteStatus: 200, want: Deleted, wantCalls: 2},
		{name: "204 confirms enacted deletion", readStatus: 200, deleteStatus: 204, want: Deleted, wantCalls: 2},
		{name: "202 is pending rather than deleted", readStatus: 200, deleteStatus: 202, want: DeleteInvalid, wantErr: core.ErrTimeMatchIndeterminate, wantCalls: 2},
		{name: "404 preflight is already absent", readStatus: 404, want: DeleteAlreadyAbsent, wantCalls: 1},
		{name: "410 preflight is already absent", readStatus: 410, want: DeleteAlreadyAbsent, wantCalls: 1},
		{name: "404 after preflight is already absent", readStatus: 200, deleteStatus: 404, want: DeleteAlreadyAbsent, wantCalls: 2},
		{name: "changed revision refuses deletion", readStatus: 200, deleteStatus: 412, want: DeleteInvalid, wantErr: core.ErrTimeMatchConflict, wantCalls: 2},
		{name: "server failure is indeterminate", readStatus: 200, deleteStatus: 503, want: DeleteInvalid, wantErr: core.ErrTimeMatchIndeterminate, wantCalls: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := appleMutationBody(t)
			client, provider := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method == http.MethodGet {
					if tc.readStatus != 200 {
						w.WriteHeader(tc.readStatus)
						return
					}
					w.Header().Set("Content-Type", "text/calendar")
					w.Header().Set("ETag", `"observed"`)
					_, _ = w.Write(body)
					return
				}
				w.WriteHeader(tc.deleteStatus)
			})
			got, err := client.DeleteEvent(t.Context(), EventDelete{Connection: appleConnection(t), Ref: appleMutationRef(t), Policy: fixturePolicy(t)})
			if got != tc.want || !errors.Is(err, tc.wantErr) || len(provider.record()) != tc.wantCalls {
				t.Fatalf("delete status%d/%d=%v/%v requests%d, want%v/%v requests%d", tc.readStatus, tc.deleteStatus, got, err, len(provider.record()), tc.want, tc.wantErr, tc.wantCalls)
			}
			for _, request := range provider.record() {
				if request.method != http.MethodGet && request.method != http.MethodDelete {
					t.Fatalf("delete path method=%s, want GET or DELETE and never recreation", request.method)
				}
			}
		})
	}
}

func TestAppleUpdateMissingAndChangedResourceNeverCreates(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                  string
		readStatus, putStatus int
		wantErr               error
		wantCalls             int
	}{
		{name: "missing resource refuses without PUT", readStatus: 404, wantErr: core.ErrTimeMatchConflict, wantCalls: 1},
		{name: "gone resource refuses without PUT", readStatus: 410, wantErr: core.ErrTimeMatchConflict, wantCalls: 1},
		{name: "lost update CAS refuses", readStatus: 200, putStatus: 412, wantErr: core.ErrTimeMatchConflict, wantCalls: 2},
		{name: "deleted between read and PUT refuses", readStatus: 200, putStatus: 404, wantErr: core.ErrTimeMatchConflict, wantCalls: 2},
		{name: "unexpected created response cannot confirm update", readStatus: 200, putStatus: 201, wantErr: core.ErrTimeMatchIndeterminate, wantCalls: 2},
		{name: "asynchronous update cannot confirm completion", readStatus: 200, putStatus: 202, wantErr: core.ErrTimeMatchIndeterminate, wantCalls: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := appleMutationBody(t)
			client, provider := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method == http.MethodGet {
					if tc.readStatus != 200 {
						w.WriteHeader(tc.readStatus)
						return
					}
					w.Header().Set("Content-Type", "text/calendar")
					w.Header().Set("ETag", `"observed"`)
					_, _ = w.Write(body)
					return
				}
				w.WriteHeader(tc.putStatus)
			})
			got, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: appleConnection(t), Ref: appleMutationRef(t), Intent: appleMutationIntent(t), ObservedAt: at(t, "2026-11-02T09:00:00Z"), Policy: fixturePolicy(t)})
			if !errors.Is(err, tc.wantErr) || got != (EventReceipt{}) || len(provider.record()) != tc.wantCalls {
				t.Fatalf("update statuses%d/%d=%+v/%v calls%d, wantzero/%v calls%d", tc.readStatus, tc.putStatus, got, err, len(provider.record()), tc.wantErr, tc.wantCalls)
			}
		})
	}
}

func TestAppleMutationReadbackMustIdentifyOnlyOwnedEvent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		body  func(*testing.T) []byte
		media string
		want  error
	}{
		{name: "different UID supplies no mutation authority", media: "text/calendar", body: func(t *testing.T) []byte {
			return []byte(strings.Replace(string(appleMutationBody(t)), "UID:"+eventIDFixture, "UID:aaaaa", 1))
		}, want: core.ErrTimeMatchConflict},
		{name: "missing UID supplies no mutation authority", media: "text/calendar", body: func(t *testing.T) []byte {
			return []byte(strings.Replace(string(appleMutationBody(t)), "UID:"+eventIDFixture, "X-ID:"+eventIDFixture, 1))
		}, want: core.ErrTimeMatchResponse},
		{name: "malformed owned response supplies no mutation authority", media: "text/calendar", body: func(*testing.T) []byte { return []byte("not a calendar") }, want: core.ErrTimeMatchResponse},
		{name: "HTML must not masquerade as owned event", media: "text/html", body: func(t *testing.T) []byte { return appleMutationBody(t) }, want: core.ErrTimeMatchResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := tc.body(t)
			client, provider := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
				w.Header().Set("Content-Type", tc.media)
				w.Header().Set("ETag", `"observed"`)
				_, _ = w.Write(body)
			})
			got, err := client.DeleteEvent(t.Context(), EventDelete{Connection: appleConnection(t), Ref: appleMutationRef(t), Policy: fixturePolicy(t)})
			if !errors.Is(err, tc.want) || got != DeleteInvalid || len(provider.record()) != 1 || provider.record()[0].method != http.MethodGet {
				t.Fatalf("owned readback=%v/%v, requests%d, want Invalid/%v and GET-only", got, err, len(provider.record()), tc.want)
			}
		})
	}
}

func TestAppleStrongETagExhaustsOpaqueByteAlphabet(t *testing.T) {
	t.Parallel()
	for value := range 256 {
		text := `"before` + string([]byte{byte(value)}) + `after"`
		// RFC9110 etagc is exactly 0x21, 0x23..0x7e and obs-text0x80..0xff.
		want := value == 0x21 || value >= 0x23 && value <= 0x7e || value >= 0x80
		if got := appleStrongETag(text); got != want {
			t.Fatalf("opaque tag byte%02x admitted=%t, want%t", value, got, want)
		}
	}
}

func FuzzAppleDeleteResourceBindingSemantic(f *testing.F) {
	calendar := calendarID(f, appleMutationHome)
	id, err := ParseEventID(eventIDFixture)
	if err != nil {
		f.Fatal(err)
	}
	target, err := appleEventURL(calendar, id)
	if err != nil {
		f.Fatal(err)
	}
	ref := EventRef{ID: id, Calendar: calendar, Resource: target.String(), Provider: ProviderApple}
	if err := ref.Validate(); err != nil {
		f.Fatal(err)
	}
	f.Add(ref.Resource)
	f.Add(appleMutationHome)
	f.Add(appleMutationHome + "nested/")
	f.Add(appleMutationHome + "aaaaa.ics")
	f.Fuzz(func(t *testing.T, resource string) {
		body := appleMutationBody(t)
		client, provider := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "text/calendar")
				w.Header().Set("ETag", `"observed"`)
				_, _ = w.Write(body)
				return
			}
			w.WriteHeader(204)
		})
		candidate := ref
		candidate.Resource = resource
		got, err := client.DeleteEvent(t.Context(), EventDelete{Connection: appleConnection(t), Ref: candidate, Policy: fixturePolicy(t)})
		requests := provider.record()
		if resource != ref.Resource {
			if !errors.Is(err, core.ErrTimeMatchContract) || got != DeleteInvalid || len(requests) != 0 || len(provider.refusedDials()) != 0 {
				t.Fatalf("foreign resource admitted=%v/%v requests/refused%d/%d, want Invalid/Contract and zeroHTTP", got, err, len(requests), len(provider.refusedDials()))
			}
			return
		}
		if err != nil || got != Deleted || len(requests) != 2 || requests[0].method != http.MethodGet || requests[1].method != http.MethodDelete || requests[1].header.Get("If-Match") != `"observed"` {
			t.Fatalf("canonical owned mutation=%v/%v requests%d, want Deleted/nil and GET then conditional DELETE", got, err, len(requests))
		}
	})
}

// Go's real HTTP precondition evaluator is an independent RFC oracle, rather
// than another copy of this SDK's quoted-tag parser.
func FuzzAppleStrongETagMatchesNativeHTTPPreconditions(f *testing.F) {
	field, err := header("ETag", `"native-revision"`)
	if err != nil {
		f.Fatal(err)
	}
	seed, err := field.Values[0].Value()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add(`W/"weak"`)
	f.Add(`*`)
	f.Add(`"one", "two"`)
	f.Add("*\xa3\xf7\x8bl\x97B\xd6") // Native wildcard-prefix acceptance must not stand for a revision.
	f.Add("")
	f.Fuzz(func(t *testing.T, value string) {
		request := httptest.NewRequest(http.MethodGet, "https://example.invalid/owned", nil)
		request.Header.Set("If-Match", value)
		response := httptest.NewRecorder()
		response.Header().Set("ETag", value)
		http.ServeContent(response, request, "owned", time.Time{}, bytes.NewReader(nil))
		// A specific revision must match itself and refuse a different revision.
		// Native HTTP accepts wildcard prefixes permissively; testing only the
		// first response would misclassify those as valid compare-and-swap.
		different := `"different-native-revision"`
		if different == value {
			different = `"another-native-revision"`
		}
		other := httptest.NewRecorder()
		other.Header().Set("ETag", different)
		http.ServeContent(other, request, "owned", time.Time{}, bytes.NewReader(nil))
		want := response.Code == http.StatusOK && other.Code == http.StatusPreconditionFailed
		if got := appleStrongETag(value); got != want {
			t.Fatalf("strong revision%q admitted=%t, native same/different status%d/%d want%t", value, got, response.Code, other.Code, want)
		}
	})
}
