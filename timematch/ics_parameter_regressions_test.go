package timematch

import (
	"testing"
)

func TestICSQuotedParameterCannotInjectTimeRules(t *testing.T) {
	t.Parallel()
	for _, param := range []string{`X-NOTE="x;VALUE=DATE"`, `X-NOTE="x;TZID=Europe/Berlin"`, `X-NOTE="x;VALUE=DATE;TZID=Europe/Berlin"`, `X-NOTE="x;y:z"`} {
		t.Run(param, func(t *testing.T) {
			t.Parallel()
			events, err := parseICS(vcalendar(vevent("UID:owned", "DTSTART;"+param+":20261102T100000Z", "DTEND:20261102T110000Z")))
			if err != nil || len(events) != 1 || !events[0].resolvable || !events[0].blocking {
				t.Fatalf("quoted parameter events=(%+v,%v),want one exact busy interval", events, err)
			}
			start, _ := at(t, "2026-11-02T10:00:00Z").Nanoseconds()
			end, _ := at(t, "2026-11-02T11:00:00Z").Nanoseconds()
			if events[0].start != start || events[0].end != end {
				t.Fatalf("quoted parameter span=%d/%d,want %d/%d", events[0].start, events[0].end, start, end)
			}
		})
	}
}

func TestFreeBusyQuotedExtensionCannotHideBusyTime(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, line string
		known      bool
		count      int
	}{
		{name: "quoted extension semicolon cannot become free parameter", line: `FREEBUSY;X-NOTE="x;FBTYPE=FREE":20261102T100000Z/20261102T110000Z`, known: true, count: 1},
		{name: "quoted extension cannot split a colon", line: `FREEBUSY;X-NOTE="x;y:z":20261102T100000Z/20261102T110000Z`, known: true, count: 1},
		{name: "genuinely free block stays neutral", line: `FREEBUSY;FBTYPE=FREE:20261102T100000Z/20261102T110000Z`, known: true},
		{name: "malformed busy line is unknown not free", line: `FREEBUSY`},
		{name: "unclosed quoted busy parameter is unknown", line: `FREEBUSY;X-NOTE="x;FBTYPE=FREE:20261102T100000Z/20261102T110000Z`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, known := parseFreeBusy(vcalendar("BEGIN:VFREEBUSY\r\n" + tc.line + "\r\nEND:VFREEBUSY\r\n"))
			if known != tc.known || len(got) != tc.count {
				t.Fatalf("freebusy=(%+v,%t),want count%d,known%t", got, known, tc.count, tc.known)
			}
		})
	}
}
