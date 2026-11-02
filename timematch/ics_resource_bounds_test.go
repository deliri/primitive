package timematch

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

// icsHostileFolded is one VEVENT whose SUMMARY is folded across n physical
// continuation lines and whose DURATION spells 1 day 2 hours with n leading
// zeros in each digit run. Exchange admits the whole body without a byte cap,
// so both shapes arrive at the parser exactly as a hostile server sends them.
func icsHostileFolded(n int) string {
	return "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:owned\r\nSUMMARY:" + strings.Repeat("x\r\n ", n) +
		"\r\nDTSTART:20261102T100000Z\r\nDURATION:P" + strings.Repeat("0", n) + "1DT" + strings.Repeat("0", n) + "2H\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
}

func icsHostileEvent(t testing.TB, text string, n int) float64 {
	t.Helper()
	const twentySixHours = int64(26 * 3600e9)
	var events []icsEvent
	var err error
	allocations := testing.AllocsPerRun(5, func() { events, err = parseICS(text) })
	if err != nil || len(events) != 1 || !events[0].resolvable || !events[0].blocking || events[0].end-events[0].start != twentySixHours || len(events[0].summary) != n {
		t.Fatalf("parseICS(%d continuations) = (%+v, %v), want one resolvable 26 hour event with a %d byte summary", n, events, err, n)
	}
	return allocations
}

// Working storage must follow the number of logical lines, not the number of
// physical continuation lines or duration digits: doubling both must not
// double the allocations. The previous unfold re-copied the logical line for
// every continuation and the duration parser re-copied its digit run for every
// digit, which this measurement refuses.
func TestParseICSFoldedAndDigitRunsCostLinearWorkingStorage(t *testing.T) {
	// AllocsPerRun observes process-wide allocation and sets GOMAXPROCS to 1,
	// so this test runs serially under the declared isolation hazard.
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardRuntimeAllocation,
		Scope:  core.TestIsolationScopePackageProcess,
	})
	const base = 20000
	one := icsHostileEvent(t, icsHostileFolded(base), base)
	two := icsHostileEvent(t, icsHostileFolded(2*base), 2*base)
	t.Logf("parseICS allocations: %v for %d continuations, %v for %d", one, base, two, 2*base)
	if two >= one*3/2 {
		t.Fatalf("parseICS allocations grew from %v to %v when continuation lines and digits doubled, want working storage independent of physical line and digit count", one, two)
	}
}

// Fold, unfold and duration semantics are unchanged by the linear parser:
// lone CR, LF-only endings, empty lines between continuations, a continuation
// before any line, and digit runs with leading zeros keep their meaning.
func TestUnfoldICSLinearKeepsFoldingSemantics(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		text string
		want []string
	}{
		{name: "crlf folds", text: "A:1\r\n 2\r\nB:3\r\n", want: []string{"A:12", "B:3"}},
		{name: "lf folds", text: "A:1\n\t2\nB:3", want: []string{"A:12", "B:3"}},
		{name: "empty line does not break a fold", text: "A:1\r\n\r\n 2\r\n", want: []string{"A:12"}},
		{name: "continuation before any line keeps its leading space", text: " A\r\nB\r\n", want: []string{" A", "B"}},
		{name: "lone cr stays inside the line", text: "A:1\rB\r\n", want: []string{"A:1\rB"}},
		{name: "whitespace-only continuation adds nothing", text: "A:1\r\n \r\n \r\nB:2\r\n", want: []string{"A:1", "B:2"}},
		{name: "no trailing newline", text: "A:1\r\n 2", want: []string{"A:12"}},
		{name: "empty", text: "", want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := unfoldICS(tc.text)
			if len(got) != len(tc.want) {
				t.Fatalf("unfoldICS(%q) = %q, want %q", tc.text, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("unfoldICS(%q) = %q, want %q", tc.text, got, tc.want)
				}
			}
		})
	}
	for _, tc := range []struct {
		text string
		want int64
		ok   bool
	}{
		{"P1W", 7 * 24 * 3600e9, true}, {"P0001DT0002H", 26 * 3600e9, true}, {"PT1H30M", 5400e9, true}, {"+PT15S", 15e9, true},
		{"P1H", 0, false}, {"PT1D", 0, false}, {"PT", 0, false}, {"P", 0, false}, {"P1", 0, false}, {"P" + strings.Repeat("9", 40) + "W", 0, false}, {"PT1X", 0, false}, {"-PT1H", 0, false},
	} {
		got, ok := icsDuration(tc.text)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("icsDuration(%q) = (%d, %t), want (%d, %t)", tc.text, got, ok, tc.want, tc.ok)
		}
	}
}

// Lowercase component and property names stay recognised without copying
// the line; the answer is still typed per the existing hostile tables.
func TestParseICSCaseFoldedTokensWithoutCopies(t *testing.T) {
	t.Parallel()
	text := "begin:vcalendar\r\nbegin:vevent\r\nuid:owned\r\ndtstart:20261102T100000Z\r\ndtend:20261102T110000Z\r\nbegin:valarm\r\nduration:PT5M\r\nend:valarm\r\nend:vevent\r\nend:vcalendar\r\n"
	if events, err := parseICS(text); !errors.Is(err, core.ErrTimeMatchResponse) || len(events) != 0 {
		t.Fatalf("parseICS(lowercase header) = (%+v, %v), want refusal: the VCALENDAR prefix contract is case-exact", events, err)
	}
	text = "BEGIN:VCALENDAR" + text[len("begin:vcalendar"):]
	events, err := parseICS(text)
	if err != nil || len(events) != 1 || !events[0].resolvable || events[0].end-events[0].start != 3600e9 || events[0].uid != "owned" {
		t.Fatalf("parseICS(lowercase tokens) = (%+v, %v), want one resolvable hour", events, err)
	}
	spans, known := parseFreeBusy("BEGIN:VCALENDAR\r\nbegin:vfreebusy\r\nfreebusy;fbtype=busy-tentative:20261102T100000Z/PT1H\r\nfreebusy;fbtype=free:20261102T120000Z/PT1H\r\nend:vfreebusy\r\nEND:VCALENDAR\r\n")
	if !known || len(spans) != 1 || spans[0][1]-spans[0][0] != 3600e9 {
		t.Fatalf("parseFreeBusy(lowercase tokens) = (%v, %t), want one known busy hour", spans, known)
	}
}

// invitationLineRecorder refuses any write that is not exactly one complete
// physical line of at most 75 octets plus CRLF.
type invitationLineRecorder struct {
	output bytes.Buffer
	calls  int
	broken bool
}

func (r *invitationLineRecorder) Write(data []byte) (int, error) {
	r.calls++
	if len(data) > icsLineMaximum+len(icsCRLF) || !bytes.HasSuffix(data, []byte(icsCRLF)) || bytes.Count(data, []byte(icsCRLF)) != 1 {
		r.broken = true
	}
	return r.output.Write(data)
}

// Every destination write is one complete physical line: the writer no longer
// issues one write per rune and per fold, while the message bytes, folding and
// recipient handling are unchanged.
func TestInvitationWritesEachPhysicalLineOnce(t *testing.T) {
	t.Parallel()
	value := invitationFixture(t)
	summary, err := ParseEventSummary(strings.Repeat("é", 120) + `; and \ also, this`)
	if err != nil {
		t.Fatalf("ParseEventSummary() error = %v, want nil", err)
	}
	value.Summary = summary
	for range 4 {
		value.Attendees = append(value.Attendees, value.Organizer)
	}
	var full bytes.Buffer
	if err := WriteInvitation(&full, value); err != nil {
		t.Fatalf("WriteInvitation(buffer) error = %v, want nil", err)
	}
	recorder := &invitationLineRecorder{}
	if err := WriteInvitation(recorder, value); err != nil {
		t.Fatalf("WriteInvitation(recorder) error = %v, want nil", err)
	}
	lines := bytes.Count(full.Bytes(), []byte(icsCRLF))
	if recorder.broken || recorder.calls != lines || !bytes.Equal(recorder.output.Bytes(), full.Bytes()) {
		t.Fatalf("invitation writes = %d calls (broken chunk %t) for %d physical lines, want one complete line per write and identical bytes", recorder.calls, recorder.broken, lines)
	}
	if lines < 12 {
		t.Fatalf("physical lines = %d, want a folded multi-recipient message", lines)
	}
}

var (
	icsBenchmarkEvents []icsEvent
	icsBenchmarkSpans  [][2]int64
)

func BenchmarkParseICSFolded(b *testing.B) {
	b.ReportAllocs()
	text := icsHostileFolded(20000)
	for b.Loop() {
		events, err := parseICS(text)
		if err != nil || len(events) != 1 {
			b.Fatalf("parseICS() = (%d, %v), want one event", len(events), err)
		}
		icsBenchmarkEvents = events
	}
}

func BenchmarkParseFreeBusyDay(b *testing.B) {
	b.ReportAllocs()
	periods := make([]string, 0, 48)
	for hour := range 24 {
		periods = append(periods, "FREEBUSY;FBTYPE=BUSY:20261102T"+twoDigits(hour)+"0000Z/PT30M", "FREEBUSY;FBTYPE=BUSY-TENTATIVE:20261102T"+twoDigits(hour)+"3000Z/PT15M")
	}
	text := vfreebusy(periods...)
	for b.Loop() {
		spans, known := parseFreeBusy(text)
		if !known || len(spans) != 48 {
			b.Fatalf("parseFreeBusy() = (%d, %t), want 48 known periods", len(spans), known)
		}
		icsBenchmarkSpans = spans
	}
}

func twoDigits(value int) string { return string(rune('0'+value/10)) + string(rune('0'+value%10)) }

func BenchmarkWriteInvitation(b *testing.B) {
	b.ReportAllocs()
	value := invitationFixture(b)
	summary, err := ParseEventSummary(strings.Repeat("Website review and planning ", 4))
	if err != nil {
		b.Fatalf("ParseEventSummary() error = %v, want nil", err)
	}
	value.Summary = summary
	for range 4 {
		value.Attendees = append(value.Attendees, value.Organizer)
	}
	for b.Loop() {
		if err := WriteInvitation(io.Discard, value); err != nil {
			b.Fatalf("WriteInvitation() error = %v, want nil", err)
		}
	}
}
