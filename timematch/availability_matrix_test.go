package timematch

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type matrixPrimary string

const (
	primaryContradiction matrixPrimary = "contradiction"
	primaryRefusal       matrixPrimary = "typed-refusal"
	primaryNeutral       matrixPrimary = "neutral"
	primaryBoundary      matrixPrimary = "boundary"
)

func item(showAs, start, end string) string { return graphItem(showAs, start, end, "") }

func cancelledItem(showAs, start, end string) string {
	return strings.Replace(item(showAs, start, end), `"isCancelled":false`, `"isCancelled":true`, 1)
}

func allDayItem(showAs, start, end string) string {
	return strings.Replace(item(showAs, start, end), `"isAllDay":false`, `"isAllDay":true`, 1)
}

const (
	h09 = "2026-11-02T09:00:00.0000000"
	h10 = "2026-11-02T10:00:00.0000000"
	h11 = "2026-11-02T11:00:00.0000000"
	h12 = "2026-11-02T12:00:00.0000000"
	h17 = "2026-11-02T17:00:00.0000000"
)

// produceCalendar runs the real producer: Graph page decode, then the busy
// normalization every provider projection uses. A refused page is Unknown.
func produceCalendar(t *testing.T, id string, page string) (CalendarAvailability, [][2]int64, error) {
	t.Helper()
	decoded, err := decodeJSON[graphViewPage]([]byte(page))
	calendar := CalendarAvailability{ID: CalendarID{value: id}, State: AvailabilityUnknown}
	if err != nil || !decoded.known {
		return calendar, nil, err
	}
	busy, normalizeErr := normalizeBusy(window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), decoded.busy)
	if normalizeErr != nil {
		t.Fatalf("normalizeBusy() error = %v, want nil", normalizeErr)
	}
	return CalendarAvailability{ID: calendar.ID, State: AvailabilityKnown, Busy: busy}, decoded.busy, nil
}

func TestAvailabilityProducerClassifierMatrix(t *testing.T) {
	t.Parallel()
	many := func(n int, page string) []string {
		pages := make([]string, n)
		for i := range pages {
			pages[i] = page
		}
		return pages
	}
	cases := []struct {
		name        string
		primary     matrixPrimary
		pages       []string
		wantRefused int
		wantState   AvailabilityState
		wantBusy    string
	}{
		{name: "busy and free claim the same span: busy wins", primary: primaryContradiction, pages: []string{graphView(item("busy", h10, h11), item("free", h10, h11))}, wantState: AvailabilityKnown, wantBusy: "10:00-11:00"},
		{name: "tentative and free claim the same span: tentative blocks", primary: primaryContradiction, pages: []string{graphView(item("tentative", h10, h11), item("free", h10, h11))}, wantState: AvailabilityKnown, wantBusy: "10:00-11:00"},
		{name: "one calendar free all window, another busy: busy wins", primary: primaryContradiction, pages: []string{graphView(item("free", h09, h17)), graphView(item("busy", h10, h11))}, wantState: AvailabilityKnown, wantBusy: "10:00-11:00"},
		{name: "same facts in reverse calendar order classify identically", primary: primaryContradiction, pages: []string{graphView(item("busy", h10, h11)), graphView(item("free", h09, h17))}, wantState: AvailabilityKnown, wantBusy: "10:00-11:00"},
		{name: "known busy calendar beside an unknown calendar is unknown", primary: primaryContradiction, pages: []string{graphView(item("busy", h10, h11)), graphView(item("unknown", h10, h11))}, wantState: AvailabilityUnknown},
		{name: "cancelled copy does not cancel a live busy span", primary: primaryContradiction, pages: []string{graphView(cancelledItem("busy", h10, h11), item("busy", h10, h11))}, wantState: AvailabilityKnown, wantBusy: "10:00-11:00"},
		{name: "busy and oof overlap merge into one span", primary: primaryContradiction, pages: []string{graphView(item("busy", h10, h11), item("oof", "2026-11-02T10:30:00.0000000", h12))}, wantState: AvailabilityKnown, wantBusy: "10:00-12:00"},
		{name: "two calendars with overlapping busy union", primary: primaryContradiction, pages: []string{graphView(item("busy", h10, h11)), graphView(item("busy", "2026-11-02T10:30:00.0000000", h12))}, wantState: AvailabilityKnown, wantBusy: "10:00-12:00"},
		{name: "timed busy beside blocking all-day in one page is unknown", primary: primaryContradiction, pages: []string{graphView(item("busy", h10, h11), allDayItem("busy", "2026-11-02T00:00:00.0000000", "2026-11-03T00:00:00.0000000"))}, wantState: AvailabilityUnknown},
		{name: "free beside unclassifiable status is unknown", primary: primaryContradiction, pages: []string{graphView(item("free", h10, h11), item("unknown", h11, h12))}, wantState: AvailabilityUnknown},

		{name: "truncated page is refused", primary: primaryRefusal, pages: []string{`{"value":[`}, wantRefused: 1, wantState: AvailabilityUnknown},
		{name: "duplicate value member is refused", primary: primaryRefusal, pages: []string{`{"value":[],"value":[]}`}, wantRefused: 1, wantState: AvailabilityUnknown},
		{name: "invalid utf8 is refused", primary: primaryRefusal, pages: []string{"{\"value\":[],\"x\":\"\xff\"}"}, wantRefused: 1, wantState: AvailabilityUnknown},
		{name: "null document is refused", primary: primaryRefusal, pages: []string{`null`}, wantRefused: 1, wantState: AvailabilityUnknown},
		{name: "top-level array is refused", primary: primaryRefusal, pages: []string{`[]`}, wantRefused: 1, wantState: AvailabilityUnknown},
		{name: "value of wrong type is refused", primary: primaryRefusal, pages: []string{`{"value":"busy"}`}, wantRefused: 1, wantState: AvailabilityUnknown},
		{name: "numeric showAs is refused", primary: primaryRefusal, pages: []string{`{"value":[{"showAs":3,"start":{"dateTime":"` + h10 + `","timeZone":"UTC"},"end":{"dateTime":"` + h11 + `","timeZone":"UTC"}}]}`}, wantRefused: 1, wantState: AvailabilityUnknown},
		{name: "trailing document is refused", primary: primaryRefusal, pages: []string{`{"value":[]}{"value":[]}`}, wantRefused: 1, wantState: AvailabilityUnknown},
		{name: "nesting beyond the strict depth is refused", primary: primaryRefusal, pages: []string{`{"value":[],"x":` + strings.Repeat("[", 70) + strings.Repeat("]", 70) + `}`}, wantRefused: 1, wantState: AvailabilityUnknown},
		{name: "refused page beside a busy page stays unknown", primary: primaryRefusal, pages: []string{graphView(item("busy", h10, h11)), `{"value":`}, wantRefused: 1, wantState: AvailabilityUnknown},

		{name: "empty page is known and free", primary: primaryNeutral, pages: []string{graphView()}, wantState: AvailabilityKnown},
		{name: "only free items add no busy", primary: primaryNeutral, pages: []string{graphView(item("free", h10, h11), item("free", h11, h12))}, wantState: AvailabilityKnown},
		{name: "only working elsewhere adds no busy", primary: primaryNeutral, pages: []string{graphView(item("workingElsewhere", h09, h17))}, wantState: AvailabilityKnown},
		{name: "only cancelled busy adds no busy", primary: primaryNeutral, pages: []string{graphView(cancelledItem("busy", h10, h11))}, wantState: AvailabilityKnown},
		{name: "free all-day adds no busy", primary: primaryNeutral, pages: []string{graphView(allDayItem("free", "2026-11-02T00:00:00.0000000", "2026-11-03T00:00:00.0000000"))}, wantState: AvailabilityKnown},
		{name: "busy wholly before the window is absent", primary: primaryNeutral, pages: []string{graphView(item("busy", "2026-11-02T07:00:00.0000000", "2026-11-02T08:00:00.0000000"))}, wantState: AvailabilityKnown},
		{name: "busy wholly after the window is absent", primary: primaryNeutral, pages: []string{graphView(item("busy", "2026-11-02T18:00:00.0000000", "2026-11-02T19:00:00.0000000"))}, wantState: AvailabilityKnown},
		{name: "identical duplicate busy items equal one", primary: primaryNeutral, pages: []string{graphView(item("busy", h10, h11), item("busy", h10, h11))}, wantState: AvailabilityKnown, wantBusy: "10:00-11:00"},
		{name: "identical duplicate calendars equal one", primary: primaryNeutral, pages: []string{graphView(item("busy", h10, h11)), graphView(item("busy", h10, h11))}, wantState: AvailabilityKnown, wantBusy: "10:00-11:00"},
		{name: "two empty calendars stay free", primary: primaryNeutral, pages: []string{graphView(), graphView()}, wantState: AvailabilityKnown},

		{name: "busy ending 100ns before window start is absent", primary: primaryBoundary, pages: []string{graphView(item("busy", "2026-11-02T08:00:00.0000000", "2026-11-02T08:59:59.9999999"))}, wantState: AvailabilityKnown},
		{name: "busy ending exactly at window start is absent", primary: primaryBoundary, pages: []string{graphView(item("busy", "2026-11-02T08:00:00.0000000", h09))}, wantState: AvailabilityKnown},
		{name: "busy ending 100ns after window start is a 100ns span", primary: primaryBoundary, pages: []string{graphView(item("busy", "2026-11-02T08:00:00.0000000", "2026-11-02T09:00:00.0000001"))}, wantState: AvailabilityKnown, wantBusy: "09:00-09:00:00.0000001"},
		{name: "busy starting at year 9999 is outside representable instants", primary: primaryBoundary, pages: []string{graphView(item("busy", "9999-12-31T00:00:00.0000000", "9999-12-31T01:00:00.0000000"))}, wantState: AvailabilityUnknown},
		{name: "zero-length busy cannot be placed", primary: primaryBoundary, pages: []string{graphView(item("busy", h10, h10))}, wantState: AvailabilityUnknown},
		{name: "100ns busy is the shortest graph span", primary: primaryBoundary, pages: []string{graphView(item("busy", h10, "2026-11-02T10:00:00.0000001"))}, wantState: AvailabilityKnown, wantBusy: "10:00-10:00:00.0000001"},
		{name: "busy equal to the window", primary: primaryBoundary, pages: []string{graphView(item("busy", h09, h17))}, wantState: AvailabilityKnown, wantBusy: "09:00-17:00"},
		{name: "busy wider than the window is clipped", primary: primaryBoundary, pages: []string{graphView(item("busy", "2026-11-02T08:00:00.0000000", "2026-11-02T18:00:00.0000000"))}, wantState: AvailabilityKnown, wantBusy: "09:00-17:00"},
		{name: "one known calendar", primary: primaryBoundary, pages: many(1, graphView()), wantState: AvailabilityKnown},
		{name: "two known calendars", primary: primaryBoundary, pages: many(2, graphView()), wantState: AvailabilityKnown},
		{name: "fifty known calendars", primary: primaryBoundary, pages: many(50, graphView(item("busy", h10, h11))), wantState: AvailabilityKnown, wantBusy: "10:00-11:00"},
		{name: "fifty-one calendars with one unknown at the tail", primary: primaryBoundary, pages: append(many(50, graphView()), graphView(item("unknown", h10, h11))), wantState: AvailabilityUnknown},
		{name: "lowest status free", primary: primaryBoundary, pages: []string{graphView(item("free", h10, h11))}, wantState: AvailabilityKnown},
		{name: "highest status working elsewhere", primary: primaryBoundary, pages: []string{graphView(item("workingElsewhere", h10, h11))}, wantState: AvailabilityKnown},
		{name: "status one beyond the documented set", primary: primaryBoundary, pages: []string{graphView(item("away", h10, h11))}, wantState: AvailabilityUnknown},
		{name: "empty status string", primary: primaryBoundary, pages: []string{graphView(item("", h10, h11))}, wantState: AvailabilityUnknown},
		{name: "weaker precedence: free only", primary: primaryBoundary, pages: []string{graphView(item("free", h10, h12))}, wantState: AvailabilityKnown},
		{name: "tie precedence: busy and tentative over the same span", primary: primaryBoundary, pages: []string{graphView(item("busy", h10, h11), item("tentative", h10, h11))}, wantState: AvailabilityKnown, wantBusy: "10:00-11:00"},
		{name: "stronger precedence: oof inside free", primary: primaryBoundary, pages: []string{graphView(item("free", h09, h17), item("oof", h11, h12))}, wantState: AvailabilityKnown, wantBusy: "11:00-12:00"},
		{name: "saturation: back-to-back busy fills the window", primary: primaryBoundary, pages: []string{graphView(item("busy", h09, h12), item("busy", h12, h17))}, wantState: AvailabilityKnown, wantBusy: "09:00-17:00"},
	}
	counts := map[matrixPrimary]int{}
	for _, tc := range cases {
		counts[tc.primary]++
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			availability := Availability{Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z")}
			var produced [][2]int64
			refused := 0
			for i, page := range tc.pages {
				calendar, spans, err := produceCalendar(t, "cal-"+strconv.Itoa(i), page)
				if err != nil {
					refused++
					if !errors.Is(err, core.ErrTimeMatchResponse) || calendar.State != AvailabilityUnknown || len(calendar.Busy) != 0 {
						t.Fatalf("producer refusal = (%+v, %v), want unknown calendar and ErrTimeMatchResponse", calendar, err)
					}
				}
				produced = append(produced, spans...)
				availability.Calendars = append(availability.Calendars, calendar)
			}
			if refused != tc.wantRefused {
				t.Fatalf("producer refusals = %d, want %d", refused, tc.wantRefused)
			}
			got, err := availability.Browser()
			if err != nil || got.State != tc.wantState || browserBusyText(t, got) != tc.wantBusy {
				t.Fatalf("Browser() = (%v %q, %v), want %v %q", got.State, browserBusyText(t, got), err, tc.wantState, tc.wantBusy)
			}
			if got.State == AvailabilityUnknown && len(got.Busy) != 0 {
				t.Fatalf("unknown projection carried %d busy spans", len(got.Busy))
			}
			// No evidence ex nihilo: every projected busy instant lies inside
			// some span the producer actually emitted.
			for _, busy := range got.Busy {
				start, end, _ := nanosecondBounds(busy)
				for _, edge := range []int64{start, end - 1} {
					covered := false
					for _, span := range produced {
						covered = covered || edge >= span[0] && edge < span[1]
					}
					if !covered {
						t.Fatalf("projected instant %d is not covered by producer spans %v", edge, produced)
					}
				}
			}
		})
	}
	if counts[primaryContradiction] != 10 || counts[primaryRefusal] != 10 || counts[primaryNeutral] != 10 || counts[primaryBoundary] != 20 {
		t.Fatalf("matrix classes = %v, want 10/10/10/20", counts)
	}
}

func browserBusyText(t *testing.T, got BrowserAvailability) string {
	t.Helper()
	parts := make([]string, 0, len(got.Busy))
	for _, busy := range got.Busy {
		start, startErr := busy.Start.Time()
		end, endErr := busy.End.Time()
		if errors.Join(startErr, endErr) != nil {
			t.Fatalf("busy time error = %v, want nil", errors.Join(startErr, endErr))
		}
		parts = append(parts, strings.TrimSuffix(start.UTC().Format("15:04:05.9999999"), ":00")+"-"+strings.TrimSuffix(end.UTC().Format("15:04:05.9999999"), ":00"))
	}
	return strings.Join(parts, ",")
}
