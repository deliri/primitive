package timematch

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func appleReviewConnection(t testing.TB) Connection {
	t.Helper()
	connection := appleConnection(t)
	t.Cleanup(func() {
		if err := connection.Close(); err != nil {
			t.Error(err)
		}
	})
	return connection
}

func appleReviewUnfold(text string) string {
	// Independent RFC5545 CRLF continuation oracle, not the production parser.
	return strings.NewReplacer("\r\n ", "", "\r\n\t", "").Replace(text)
}
func appleReviewResource(t testing.TB, additions, timezone string, duration bool) []byte {
	t.Helper()
	raw := string(appleMutationBody(t))
	if duration {
		raw = strings.Replace(raw, "DTEND:20261102T160000Z\r\n", "DURATION:PT1H\r\n", 1)
	}
	raw = strings.Replace(raw, icsEndEvent+icsCRLF, additions+icsEndEvent+icsCRLF, 1)
	if timezone != "" {
		raw = strings.Replace(raw, icsBeginEvent+icsCRLF, timezone+icsBeginEvent+icsCRLF, 1)
	}
	events, err := parseICS(raw)
	if err != nil || len(events) != 1 || !events[0].resolvable || !events[0].blocking {
		t.Fatalf("owned resource fixture events%d error%v want one exact event", len(events), err)
	}
	return []byte(raw)
}
func TestAppleUpdatePreservesOwnedCalendarUserProperties(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, additions, timezone string
		duration                  bool
		want                      []string
	}{
		{name: "plain private note survives", additions: "DESCRIPTION:Keep this private note\r\n", want: []string{"DESCRIPTION:Keep this private note"}},
		{name: "quoted property parameters survive", additions: "DESCRIPTION;ALTREP=\"cid:note;part=1\";LANGUAGE=en:Quoted note\r\n", want: []string{"DESCRIPTION;ALTREP=\"cid:note;part=1\";LANGUAGE=en:Quoted note"}},
		{name: "folded Unicode note survives", additions: "DESCRIPTION:Café notes first\r\n second\r\n\tthird\r\n", want: []string{"DESCRIPTION:Café notes firstsecondthird"}},
		{name: "binary attachment remains encoded", additions: "ATTACH;ENCODING=BASE64;VALUE=BINARY:YXR0YWNobWVudA==\r\n", want: []string{"ATTACH;ENCODING=BASE64;VALUE=BINARY:YXR0YWNobWVudA=="}},
		{name: "display alarm retains its own description", additions: "DESCRIPTION:Visit note\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT15M\r\nDESCRIPTION:Alarm note\r\nEND:VALARM\r\n", want: []string{"DESCRIPTION:Visit note", "BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT15M\r\nDESCRIPTION:Alarm note\r\nEND:VALARM"}},
		{name: "event duration replacement preserves alarm repeat duration", duration: true, additions: "BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT15M\r\nDESCRIPTION:Repeat alarm\r\nDURATION:PT5M\r\nREPEAT:2\r\nEND:VALARM\r\n", want: []string{"DURATION:PT5M", "REPEAT:2", "DESCRIPTION:Repeat alarm"}},
		{name: "separate alarms retain both triggers", additions: "BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT15M\r\nDESCRIPTION:First alarm\r\nEND:VALARM\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT5M\r\nDESCRIPTION:Second alarm\r\nEND:VALARM\r\n", want: []string{"TRIGGER:-PT15M", "TRIGGER:-PT5M", "DESCRIPTION:First alarm", "DESCRIPTION:Second alarm"}},
		{name: "timezone sibling component survives", timezone: "BEGIN:VTIMEZONE\r\nTZID:Example/Fixed\r\nBEGIN:STANDARD\r\nDTSTART:19700101T000000\r\nTZOFFSETFROM:+0000\r\nTZOFFSETTO:+0000\r\nEND:STANDARD\r\nEND:VTIMEZONE\r\n", want: []string{"BEGIN:VTIMEZONE\r\nTZID:Example/Fixed", "DTSTART:19700101T000000", "TZOFFSETTO:+0000", "END:VTIMEZONE"}},
		{name: "opaque vendor component survives", additions: "BEGIN:X-PRIVATE\r\nX-SECRET-NOTE:Vendor note\r\nEND:X-PRIVATE\r\n", want: []string{"BEGIN:X-PRIVATE\r\nX-SECRET-NOTE:Vendor note\r\nEND:X-PRIVATE"}},
		{name: "user scheduling metadata survives", additions: "LOCATION:Room 7\r\nSEQUENCE:23\r\nCREATED:20261001T120000Z\r\nX-APPLE-STRUCTURED-LOCATION;VALUE=URI:geo:1,2\r\n", want: []string{"LOCATION:Room 7", "SEQUENCE:23", "CREATED:20261001T120000Z", "X-APPLE-STRUCTURED-LOCATION;VALUE=URI:geo:1,2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			original := appleReviewResource(t, tc.additions, tc.timezone, tc.duration)
			client, provider := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method == http.MethodGet {
					w.Header().Set("Content-Type", "text/calendar")
					w.Header().Set("ETag", `"owned-revision"`)
					if _, err := w.Write(original); err != nil {
						t.Error(err)
					}
					return
				}
				if r.Method == http.MethodPut && r.Header.Get("If-Match") == `"owned-revision"` {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.WriteHeader(http.StatusPreconditionFailed)
			})
			next := intent(t, appleMutationHome, "Moved appointment", "2026-11-03T17:00:00Z", "2026-11-03T18:00:00Z")
			ref := appleMutationRef(t)
			got, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: appleReviewConnection(t), Ref: ref, Intent: next, ObservedAt: at(t, "2026-11-02T09:00:00Z"), Policy: fixturePolicy(t)})
			calls := provider.record()
			if err != nil || got.Ref != ref || len(calls) != 2 || calls[0].method != http.MethodGet || calls[1].method != http.MethodPut || calls[1].header.Get("If-Match") != `"owned-revision"` {
				t.Fatalf("owned update receipt%v error%v calls%d want GET then guarded PUT", got.Ref == ref, err, len(calls))
			}
			submitted := appleReviewUnfold(string(calls[1].body))
			for _, want := range tc.want {
				if !strings.Contains(submitted, want) {
					t.Fatalf("preserved property %q absent from owned update", want)
				}
			}
			for _, want := range []string{"UID:" + next.ID.String(), "DTSTART:20261103T170000Z", "DTEND:20261103T180000Z", "SUMMARY:Moved appointment"} {
				if strings.Count(submitted, want) != 1 {
					t.Fatalf("controlled property %q count%d want1", want, strings.Count(submitted, want))
				}
			}
			if strings.Contains(submitted, "DURATION:PT1H") || strings.Contains(submitted, "SUMMARY:Owned visit") || len(provider.refusedDials()) != 0 {
				t.Fatal("owned update retained replaced facts or used foreign endpoint")
			}
		})
	}
}
func TestAppleMissingPreflightNeverCertifiesOwnedAbsence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		status   int
		deleting bool
	}{{"update missing", 404, false}, {"update gone", 410, false}, {"delete missing", 404, true}, {"delete gone", 410, true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, provider := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { w.WriteHeader(tc.status) })
			var err error
			if tc.deleting {
				var got DeleteOutcome
				got, err = client.DeleteEvent(t.Context(), EventDelete{Connection: appleReviewConnection(t), Ref: appleMutationRef(t), Policy: fixturePolicy(t)})
				if got != DeleteInvalid {
					t.Fatalf("unproven delete outcome%v want Invalid", got)
				}
			} else {
				var got EventReceipt
				got, err = client.UpdateEvent(t.Context(), EventUpdate{Connection: appleReviewConnection(t), Ref: appleMutationRef(t), Intent: appleMutationIntent(t), ObservedAt: at(t, "2026-11-02T09:00:00Z"), Policy: fixturePolicy(t)})
				if got != (EventReceipt{}) {
					t.Fatal("missing preflight produced mutation receipt")
				}
			}
			calls := provider.record()
			if !errors.Is(err, core.ErrTimeMatchIndeterminate) || len(calls) != 1 || calls[0].method != http.MethodGet || len(provider.refusedDials()) != 0 {
				t.Fatalf("preflight error%v requests%d want Indeterminate and one exact GET", err, len(calls))
			}
		})
	}
}

// This compiler-visible custody distinction cannot be observed on the wire:
// both forms emit Basic auth, but only the byte slice can be cleared by its owner.
func TestAppleBasicHeaderOwnsClearableBytes(t *testing.T) {
	t.Parallel()
	connection := appleReviewConnection(t)
	auth, err := basic(connection)
	if err != nil {
		t.Fatal(err)
	}
	value := reflect.ValueOf(auth)
	field := value.FieldByName("Value")
	if !field.IsValid() || field.Type() != reflect.TypeFor[[]byte]() {
		t.Fatalf("Apple auth type%v want clearable byte custody", value.Type())
	}
	raw := field.Bytes()
	defer clear(raw)
	if len(raw) == 0 {
		t.Fatal("Apple header has no credential bytes")
	}
	before := bytes.Clone(connection.ApplePassword.value)
	clear(raw)
	for _, v := range raw {
		if v != 0 {
			t.Fatal("owner could not clear Basic header bytes")
		}
	}
	if !bytes.Equal(before, connection.ApplePassword.value) {
		t.Fatal("header cleanup changed borrowed account password")
	}
}

func TestAppleUpdateRejectsAmbiguousOwnedRepresentations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		edit func(string) string
	}{
		{"recurring rule", func(s string) string { return strings.Replace(s, icsEndEvent, "RRULE:FREQ=DAILY\r\n"+icsEndEvent, 1) }},
		{"recurrence dates", func(s string) string {
			return strings.Replace(s, icsEndEvent, "RDATE:20261103T150000Z\r\n"+icsEndEvent, 1)
		}},
		{"single occurrence cannot rewrite series identity", func(s string) string {
			return strings.Replace(s, icsEndEvent, "RECURRENCE-ID:20261102T150000Z\r\n"+icsEndEvent, 1)
		}},
		{"recurrence exclusions", func(s string) string {
			return strings.Replace(s, icsEndEvent, "EXDATE:20261103T150000Z\r\n"+icsEndEvent, 1)
		}},
		{"two events", func(s string) string {
			return strings.Replace(s, "END:VCALENDAR", string(appleMutationBody(t))[strings.Index(s, icsBeginEvent):strings.Index(s, "END:VCALENDAR")]+"END:VCALENDAR", 1)
		}},
		{"duplicate UID", func(s string) string {
			return strings.Replace(s, icsEndEvent, "UID:"+appleMutationIntent(t).ID.String()+"\r\n"+icsEndEvent, 1)
		}},
		{"unresolved local timezone", func(s string) string {
			return strings.Replace(s, "DTSTART:20261102T150000Z", "DTSTART;TZID=America/Toronto:20261102T100000", 1)
		}},
		{"cancelled owned event", func(s string) string { return strings.Replace(s, icsEndEvent, "STATUS:CANCELLED\r\n"+icsEndEvent, 1) }},
		{"transparent owned event", func(s string) string { return strings.Replace(s, "TRANSP:OPAQUE", "TRANSP:TRANSPARENT", 1) }},
		{"missing calendar terminator", func(s string) string { return strings.Replace(s, "END:VCALENDAR\r\n", "", 1) }},
		{"mismatched nested component", func(s string) string {
			return strings.Replace(s, icsEndEvent, "BEGIN:VALARM\r\nACTION:DISPLAY\r\nEND:X-OTHER\r\n"+icsEndEvent, 1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			original := tc.edit(string(appleMutationBody(t)))
			client, p := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method != http.MethodGet {
					t.Error("ambiguous representation reached mutation")
					w.WriteHeader(500)
					return
				}
				w.Header().Set("Content-Type", "text/calendar")
				w.Header().Set("ETag", `"owned-revision"`)
				if _, err := w.Write([]byte(original)); err != nil {
					t.Error(err)
				}
			})
			got, err := client.UpdateEvent(t.Context(), EventUpdate{Connection: appleReviewConnection(t), Ref: appleMutationRef(t), Intent: appleMutationIntent(t), ObservedAt: at(t, "2026-11-02T09:00:00Z"), Policy: fixturePolicy(t)})
			if !errors.Is(err, core.ErrTimeMatchResponse) || got != (EventReceipt{}) || len(p.record()) != 1 {
				t.Fatalf("ambiguous resource receipt%t error%v requests%d want zero,Response,1", got != (EventReceipt{}), err, len(p.record()))
			}
		})
	}
}
func FuzzAppleOwnedUpdatePropertyConservation(f *testing.F) {
	seed := appleMutationBody(f)
	f.Add([]byte("canonical user note"))
	f.Add([]byte{})
	f.Add([]byte("é;,:\\\r\nBEGIN:VEVENT"))
	next, err := writeICS(appleMutationIntent(f), at(f, "2026-11-02T09:00:00Z"))
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, note []byte) {
		// Base64 is an independent property-safe reversible representation of every
		// fuzz byte sequence; mutation can never escape its single user property.
		encoded := base64.StdEncoding.EncodeToString(note)
		original := bytes.Replace(seed, []byte(icsEndEvent), []byte("X-USER-NOTE;ENCODING=BASE64:"+encoded+icsCRLF+icsEndEvent), 1)
		before := bytes.Clone(original)
		got, err := appleUpdatedResource(original, next)
		if err != nil {
			t.Fatalf("valid owned property refused: %v", err)
		}
		if !bytes.Equal(original, before) {
			t.Fatal("owned source mutated")
		}
		unfolded := appleReviewUnfold(string(got))
		prefix := "X-USER-NOTE;ENCODING=BASE64:"
		var observed string
		count := 0
		for _, line := range strings.Split(unfolded, icsCRLF) {
			if strings.HasPrefix(line, prefix) {
				observed = strings.TrimPrefix(line, prefix)
				count++
			}
		}
		recovered, err := base64.StdEncoding.DecodeString(observed)
		if err != nil || count != 1 || !bytes.Equal(recovered, note) {
			t.Fatalf("user property conservation count%d decode%v exact%t want1,nil,true", count, err, bytes.Equal(recovered, note))
		}
		second, err := appleUpdatedResource(got, next)
		if err != nil || !bytes.Equal(second, got) {
			t.Fatalf("second same-intent write stable%t error%v want true,nil", bytes.Equal(second, got), err)
		}
	})
}
