package timematch

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

func TestParseICSHostileTable(t *testing.T) {
	t.Parallel()
	const hour = int64(time.Hour)
	base := func(t *testing.T) int64 { v, _ := at(t, "2026-11-02T10:00:00Z").Nanoseconds(); return v }
	cases := []struct {
		name           string
		text           string
		wantErr        bool
		wantEvents     int
		wantResolvable bool
		wantBlocking   bool
		wantLength     int64
	}{
		{name: "crlf utc event with dtend", text: vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DTEND:20261102T110000Z")), wantEvents: 1, wantResolvable: true, wantBlocking: true, wantLength: hour},
		{name: "lf only line endings", text: strings.ReplaceAll(vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DTEND:20261102T110000Z")), "\r\n", "\n"), wantEvents: 1, wantResolvable: true, wantBlocking: true, wantLength: hour},
		{name: "folded dtstart value with space continuation", text: vcalendar(vevent("UID:a", "DTSTART:2026110\r\n 2T100000Z", "DTEND:20261102T110000Z")), wantEvents: 1, wantResolvable: true, wantBlocking: true, wantLength: hour},
		{name: "folded with tab continuation", text: vcalendar(vevent("UID:a", "DTSTART:20261102T10\r\n\t0000Z", "DTEND:20261102T110000Z")), wantEvents: 1, wantResolvable: true, wantBlocking: true, wantLength: hour},
		{name: "week duration", text: vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DURATION:P1W")), wantEvents: 1, wantResolvable: true, wantBlocking: true, wantLength: 7 * 24 * hour},
		{name: "day and time duration", text: vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DURATION:P1DT2H30M15S")), wantEvents: 1, wantResolvable: true, wantBlocking: true, wantLength: 26*hour + 30*int64(time.Minute) + 15*int64(time.Second)},
		{name: "explicit plus duration", text: vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DURATION:+PT1H")), wantEvents: 1, wantResolvable: true, wantBlocking: true, wantLength: hour},
		{name: "negative duration is unresolvable", text: vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DURATION:-PT1H")), wantEvents: 1, wantBlocking: true},
		{name: "hour without time designator is unresolvable", text: vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DURATION:P1H")), wantEvents: 1, wantBlocking: true},
		{name: "empty time part is unresolvable", text: vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DURATION:PT")), wantEvents: 1, wantBlocking: true},
		{name: "overflowing duration is unresolvable", text: vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DURATION:P99999999999999W")), wantEvents: 1, wantBlocking: true},
		{name: "dtend and duration together are unresolvable", text: vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DTEND:20261102T110000Z", "DURATION:PT1H")), wantEvents: 1, wantBlocking: true},
		{name: "duplicate dtstart is unresolvable", text: vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DTSTART:20261102T120000Z", "DTEND:20261102T130000Z")), wantEvents: 1, wantBlocking: true},
		{name: "dtend before dtstart is unresolvable", text: vcalendar(vevent("UID:a", "DTSTART:20261102T110000Z", "DTEND:20261102T100000Z")), wantEvents: 1, wantBlocking: true},
		{name: "no end takes no time", text: vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z")), wantEvents: 1, wantResolvable: true, wantBlocking: true, wantLength: 0},
		{name: "floating local time is unresolvable", text: vcalendar(vevent("UID:a", "DTSTART:20261102T100000", "DTEND:20261102T110000")), wantEvents: 1, wantBlocking: true},
		{name: "quoted parameter containing a colon", text: vcalendar(vevent("UID:a", `DTSTART;X-NOTE="a:b":20261102T100000Z`, "DTEND:20261102T110000Z")), wantEvents: 1, wantResolvable: true, wantBlocking: true, wantLength: hour},
		{name: "valarm properties do not replace event times", text: vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DTEND:20261102T110000Z", "BEGIN:VALARM", "TRIGGER:-PT15M", "DURATION:PT5M", "END:VALARM")), wantEvents: 1, wantResolvable: true, wantBlocking: true, wantLength: hour},
		{name: "lowercase status cancelled is not blocking", text: vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DTEND:20261102T110000Z", "status:cancelled")), wantEvents: 1, wantResolvable: true, wantLength: hour},
		{name: "rdate means unexpanded recurrence", text: vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DTEND:20261102T110000Z", "RDATE:20261103T100000Z")), wantEvents: 1, wantBlocking: true},
		{name: "calendar with no events", text: vcalendar(), wantEvents: 0},
		{name: "missing vcalendar header", text: vevent("UID:a"), wantErr: true},
		{name: "unterminated vevent", text: "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nDTSTART:20261102T100000Z\r\nEND:VCALENDAR\r\n", wantErr: true},
		{name: "nested vevent", text: "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nBEGIN:VEVENT\r\nEND:VEVENT\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", wantErr: true},
		{name: "unbalanced nested end", text: "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", wantErr: true},
		{name: "invalid utf8", text: "BEGIN:VCALENDAR\r\nSUMMARY:\xff\r\nEND:VCALENDAR\r\n", wantErr: true},
		{name: "empty input", text: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			events, err := parseICS(tc.text)
			if tc.wantErr {
				if !errors.Is(err, core.ErrTimeMatchResponse) || len(events) != 0 {
					t.Fatalf("parseICS() = (%d events, %v), want none and %v", len(events), err, core.ErrTimeMatchResponse)
				}
				return
			}
			if err != nil || len(events) != tc.wantEvents {
				t.Fatalf("parseICS() = (%d events, %v), want %d and nil", len(events), err, tc.wantEvents)
			}
			if tc.wantEvents == 0 {
				return
			}
			got := events[0]
			if got.resolvable != tc.wantResolvable || got.blocking != tc.wantBlocking || tc.wantResolvable && (got.start != base(t) || got.end-got.start != tc.wantLength) {
				t.Fatalf("event = %+v, want resolvable %t blocking %t length %d from 10:00Z", got, tc.wantResolvable, tc.wantBlocking, tc.wantLength)
			}
		})
	}
}

func TestWriteICSRoundTripsThroughParser(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		summary string
	}{
		{name: "escaped separators", summary: `a;b,c\d`},
		{name: "multibyte summary folded beyond 75 octets", summary: strings.Repeat("é☕", 40)},
		{name: "ascii exactly crossing the fold", summary: strings.Repeat("x", 66)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned := intent(t, "https://p42-caldav.icloud.com/1/calendars/home/", tc.summary, "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")
			body, err := writeICS(owned, at(t, "2026-11-02T08:00:00Z"))
			if err != nil {
				t.Fatalf("writeICS() error = %v, want nil", err)
			}
			for line := range strings.SplitSeq(strings.TrimSuffix(string(body), "\r\n"), "\r\n") {
				if len(line) > icsLineMaximum {
					t.Fatalf("line %q has %d octets, want at most %d", line, len(line), icsLineMaximum)
				}
			}
			events, err := parseICS(string(body))
			if err != nil || len(events) != 1 || events[0].summary != tc.summary || events[0].uid != eventIDFixture || !events[0].resolvable || !sameSpan([2]int64{events[0].start, events[0].end}, owned.Interval) {
				t.Fatalf("parseICS(writeICS()) = (%+v, %v), want the owned intent back", events, err)
			}
		})
	}
}
