package timematch

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// iCalendar, RFC 5545. Only the VEVENT facts needed for busy time and owned
// event verification are read; every other property is ignored.
const (
	icsUTCLayout    = "20060102T150405Z"
	icsLineMaximum  = 75 // RFC 5545 section 3.1: lines SHOULD NOT exceed 75 octets.
	icsProductID    = "-//deliri//Primitive TimeMatch//EN"
	icsCRLF         = "\r\n"
	icsBeginEvent   = "BEGIN:VEVENT"
	icsEndEvent     = "END:VEVENT"
	icsBeginGeneric = "BEGIN:"
	icsEndGeneric   = "END:"
)

// icsEvent is one VEVENT as observed. resolvable is false when the event's
// absolute span cannot be established (DATE values, TZID or floating times,
// unexpanded recurrence, malformed values).
type icsEvent struct {
	uid        string
	summary    string
	start      int64
	end        int64
	blocking   bool
	resolvable bool
}

// parseICS returns every VEVENT in one iCalendar object. Lines are unfolded
// per RFC 5545 section 3.1 (CRLF or LF followed by one space or tab).
func parseICS(text string) ([]icsEvent, error) {
	if !utf8.ValidString(text) || !strings.HasPrefix(text, "BEGIN:VCALENDAR") {
		return nil, core.ErrTimeMatchResponse
	}
	lines := unfoldICS(text)
	var events []icsEvent
	var current *icsRawEvent
	depth := 0
	for _, line := range lines {
		upper := strings.ToUpper(line)
		switch {
		case upper == icsBeginEvent:
			if current != nil {
				return nil, core.ErrTimeMatchResponse
			}
			current = &icsRawEvent{}
			depth = 0
			continue
		case upper == icsEndEvent:
			if current == nil || depth != 0 {
				return nil, core.ErrTimeMatchResponse
			}
			events = append(events, current.event())
			current = nil
			continue
		}
		if current == nil {
			continue
		}
		// Nested components (VALARM) inside a VEVENT are skipped.
		if strings.HasPrefix(upper, icsBeginGeneric) {
			depth++
			continue
		}
		if strings.HasPrefix(upper, icsEndGeneric) {
			if depth == 0 {
				return nil, core.ErrTimeMatchResponse
			}
			depth--
			continue
		}
		if depth == 0 {
			current.add(line)
		}
	}
	if current != nil {
		return nil, core.ErrTimeMatchResponse
	}
	return events, nil
}

func unfoldICS(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && len(lines) > 0 {
			lines[len(lines)-1] += line[1:]
			continue
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

type icsProperty struct {
	params []string
	value  string
	set    bool
}

type icsRawEvent struct {
	uid, summary, transp, status       icsProperty
	dtstart, dtend, duration           icsProperty
	recurring, duplicate, unresolvable bool
}

func (e *icsRawEvent) add(line string) {
	nameAndParams, value, ok := cutICSValue(line)
	if !ok {
		e.unresolvable = true
		return
	}
	parts := strings.Split(nameAndParams, ";")
	property := icsProperty{params: parts[1:], value: value, set: true}
	var slot *icsProperty
	switch strings.ToUpper(parts[0]) {
	case "UID":
		slot = &e.uid
	case "SUMMARY":
		slot = &e.summary
	case "TRANSP":
		slot = &e.transp
	case "STATUS":
		slot = &e.status
	case "DTSTART":
		slot = &e.dtstart
	case "DTEND":
		slot = &e.dtend
	case "DURATION":
		slot = &e.duration
	case "RRULE", "RDATE":
		// A calendar-query with CALDAV:expand returns expanded instances
		// without recurrence rules (RFC 4791 section 9.6.5). A rule here
		// means the server did not expand: the span set is unknown.
		e.recurring = true
		return
	default:
		return
	}
	if slot.set {
		e.duplicate = true
	}
	*slot = property
}

// cutICSValue splits at the first ':' outside a quoted parameter value.
func cutICSValue(line string) (string, string, bool) {
	quoted := false
	for i := range len(line) {
		switch line[i] {
		case '"':
			quoted = !quoted
		case ':':
			if !quoted {
				return line[:i], line[i+1:], i > 0
			}
		}
	}
	return "", "", false
}

func (e *icsRawEvent) event() icsEvent {
	event := icsEvent{uid: e.uid.value, summary: unescapeICSText(e.summary.value)}
	event.blocking = !strings.EqualFold(e.transp.value, "TRANSPARENT") && !strings.EqualFold(e.status.value, "CANCELLED")
	if e.recurring || e.duplicate || e.unresolvable || !e.dtstart.set || e.dtend.set && e.duration.set {
		return event
	}
	start, ok := icsUTC(e.dtstart)
	if !ok {
		return event
	}
	end := start
	switch {
	case e.dtend.set:
		if end, ok = icsUTC(e.dtend); !ok {
			return event
		}
	case e.duration.set:
		length, ok := icsDuration(e.duration.value)
		if !ok || length > (1<<62)-start {
			return event
		}
		end = start + length
	}
	// RFC 5545 3.6.1: a DATE-TIME DTSTART without DTEND/DURATION ends at its
	// start and takes up no time.
	if end < start {
		return event
	}
	event.start, event.end, event.resolvable = start, end, true
	return event
}

// icsUTC admits only the UTC DATE-TIME form. TZID-qualified, floating and
// DATE values have no absolute instant without a zone database decision.
func icsUTC(property icsProperty) (int64, bool) {
	for _, param := range property.params {
		upper := strings.ToUpper(param)
		if strings.HasPrefix(upper, "TZID=") || upper == "VALUE=DATE" {
			return 0, false
		}
	}
	// RFC 5545 form 2 (UTC): exactly YYYYMMDDTHHMMSSZ. Temporal parses the
	// equivalent RFC 3339 spelling and owns range validation.
	v := property.value
	if len(v) != len("20060102T150405Z") || v[8] != 'T' || v[15] != 'Z' {
		return 0, false
	}
	for i, b := range []byte(v) {
		if i != 8 && i != 15 && (b < '0' || b > '9') {
			return 0, false
		}
	}
	instant, err := temporal.ParseRFC3339(v[0:4] + "-" + v[4:6] + "-" + v[6:8] + "T" + v[9:11] + ":" + v[11:13] + ":" + v[13:15] + "Z")
	if err != nil {
		return 0, false
	}
	value, err := instant.Nanoseconds()
	return value, err == nil
}

// icsDuration parses the RFC 5545 section 3.3.6 dur-value for non-negative
// durations: P[n]W or P[nD][T[nH][nM][nS]].
func icsDuration(text string) (int64, bool) {
	text = strings.TrimPrefix(text, "+")
	rest, ok := strings.CutPrefix(text, "P")
	if !ok || rest == "" {
		return 0, false
	}
	const second, minute, hour = int64(1e9), int64(60e9), int64(3600e9)
	units := map[byte]int64{'W': 7 * 24 * hour, 'D': 24 * hour, 'H': hour, 'M': minute, 'S': second}
	var total int64
	inTime, digits, seen := false, "", false
	for i := range len(rest) {
		b := rest[i]
		switch {
		case b >= '0' && b <= '9':
			digits += string(b)
		case b == 'T' && !inTime && digits == "":
			inTime = true
		default:
			unit, known := units[b]
			if !known || digits == "" || inTime != (b == 'H' || b == 'M' || b == 'S') {
				return 0, false
			}
			value, err := strconv.ParseInt(digits, 10, 64)
			if err != nil || value > (1<<62)/unit || total > (1<<62)-value*unit {
				return 0, false
			}
			total += value * unit
			digits, seen = "", true
		}
	}
	return total, seen && digits == ""
}

// unescapeICSText reverses RFC 5545 section 3.3.11 TEXT escaping.
func unescapeICSText(value string) string {
	var builder strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' && i+1 < len(value) {
			i++
			switch value[i] {
			case 'n', 'N':
				builder.WriteByte('\n')
			default:
				builder.WriteByte(value[i])
			}
			continue
		}
		builder.WriteByte(value[i])
	}
	return builder.String()
}

func escapeICSText(value string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`).Replace(value)
}

// writeICS emits one owned VEVENT. Times are whole-second UTC: the iCalendar
// DATE-TIME form has no fractional seconds, so a sub-second intent is refused.
func writeICS(intent EventIntent, stamp temporal.Instant) ([]byte, error) {
	start, startErr := icsTime(intent.Interval.Start)
	end, endErr := icsTime(intent.Interval.End)
	observed, stampErr := icsStampTime(stamp)
	if err := errors.Join(startErr, endErr, stampErr); err != nil {
		return nil, errors.Join(core.ErrTimeMatchContract, err)
	}
	var builder strings.Builder
	for _, line := range []string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:" + icsProductID, icsBeginEvent,
		"UID:" + intent.ID.value, "DTSTAMP:" + observed, "DTSTART:" + start, "DTEND:" + end,
		"SUMMARY:" + escapeICSText(intent.Summary.value), "TRANSP:OPAQUE", icsEndEvent, "END:VCALENDAR",
	} {
		foldICS(&builder, line)
	}
	return []byte(builder.String()), nil
}

func icsTime(instant temporal.Instant) (string, error) {
	value, err := instant.Time()
	if err != nil {
		return "", err
	}
	if value.Nanosecond() != 0 {
		return "", core.ErrTimeMatchContract
	}
	return value.UTC().Format(icsUTCLayout), nil
}

func icsStampTime(instant temporal.Instant) (string, error) {
	value, err := instant.Time()
	if err != nil {
		return "", err
	}
	return value.UTC().Truncate(1e9).Format(icsUTCLayout), nil
}

// foldICS folds at 75 octets without splitting a UTF-8 sequence.
func foldICS(builder *strings.Builder, line string) {
	limit := icsLineMaximum
	for len(line) > limit {
		cut := limit
		for cut > 0 && !utf8.RuneStart(line[cut]) {
			cut--
		}
		builder.WriteString(line[:cut])
		builder.WriteString(icsCRLF + " ")
		line = line[cut:]
		limit = icsLineMaximum - 1
	}
	builder.WriteString(line)
	builder.WriteString(icsCRLF)
}
