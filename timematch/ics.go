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
		switch {
		case strings.EqualFold(line, icsBeginEvent):
			if current != nil {
				return nil, core.ErrTimeMatchResponse
			}
			current = &icsRawEvent{}
			depth = 0
			continue
		case strings.EqualFold(line, icsEndEvent):
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
		if icsPrefixFold(line, icsBeginGeneric) {
			depth++
			continue
		}
		if icsPrefixFold(line, icsEndGeneric) {
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

// unfoldICS is linear in the text: a folded logical line is assembled in one
// growing builder, never by re-copying the line for every continuation. The
// response body has no byte cap, so a hostile answer folded into thousands of
// continuation lines must cost thousands of appends, not millions of copies.
func unfoldICS(text string) []string {
	lines := make([]string, 0, strings.Count(text, "\n")+1)
	var folded strings.Builder
	current, open := "", false
	for text != "" {
		line, rest, _ := strings.Cut(text, "\n")
		text = rest
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		if open && (line[0] == ' ' || line[0] == '\t') {
			if folded.Len() == 0 {
				folded.WriteString(current)
			}
			folded.WriteString(line[1:])
			continue
		}
		if open {
			lines = append(lines, icsLogicalLine(current, &folded))
		}
		current, open = line, true
	}
	if open {
		lines = append(lines, icsLogicalLine(current, &folded))
	}
	return lines
}

// icsLogicalLine returns the assembled line when continuations were folded
// into the builder, otherwise the single physical line, and resets the builder.
func icsLogicalLine(current string, folded *strings.Builder) string {
	if folded.Len() == 0 {
		return current
	}
	line := folded.String()
	folded.Reset()
	return line
}

// icsPrefixFold reports whether line starts with the ASCII prefix, ignoring
// case, without copying the line.
func icsPrefixFold(line, prefix string) bool {
	return len(line) >= len(prefix) && strings.EqualFold(line[:len(prefix)], prefix)
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
	parts, parametersOK := splitICSParameters(nameAndParams)
	if !parametersOK {
		e.unresolvable = true
		return
	}
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
		// parseICS reads back only the owned event this package wrote, which
		// never recurs: a recurrence rule means the span set is not ours.
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

// splitICSParameters preserves semicolons inside RFC5545 quoted parameter
// values; a quoted extension cannot inject a separate VALUE/TZID/FBTYPE.
func splitICSParameters(text string) ([]string, bool) {
	var result []string
	quoted := false
	start := 0
	for index := range len(text) {
		switch text[index] {
		case '"':
			quoted = !quoted
		case ';':
			if !quoted {
				result = append(result, text[start:index])
				start = index + 1
			}
		}
	}
	if quoted {
		return nil, false
	}
	return append(result, text[start:]), true
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

// parseFreeBusy reads the VFREEBUSY of an RFC 4791 free-busy-query answer:
// FREEBUSY periods (RFC 5545 section 3.8.2.6, always UTC) of FBTYPE BUSY,
// BUSY-UNAVAILABLE or BUSY-TENTATIVE block; FREE does not. An unknown FBTYPE,
// a malformed period, a missing VFREEBUSY, or any VEVENT (event data this
// package never requests) makes the answer unusable.
func parseFreeBusy(text string) ([][2]int64, bool) {
	if !utf8.ValidString(text) || !strings.HasPrefix(text, "BEGIN:VCALENDAR") {
		return nil, false
	}
	var spans [][2]int64
	inside, seen := false, false
	for _, line := range unfoldICS(text) {
		switch {
		case strings.EqualFold(line, "BEGIN:VFREEBUSY"):
			if inside {
				return nil, false
			}
			inside, seen = true, true
			continue
		case strings.EqualFold(line, "END:VFREEBUSY"):
			inside = false
			continue
		case strings.EqualFold(line, icsBeginEvent):
			return nil, false
		}
		if !inside || !icsPrefixFold(line, "FREEBUSY") {
			continue
		}
		nameAndParams, value, ok := cutICSValue(line)
		parts, parametersOK := splitICSParameters(nameAndParams)
		if !parametersOK {
			return nil, false
		}
		if !ok {
			return nil, false
		}
		if !strings.EqualFold(parts[0], "FREEBUSY") {
			continue
		}
		blocking := true
		for _, param := range parts[1:] {
			if !icsPrefixFold(param, "FBTYPE=") {
				continue
			}
			kind := strings.ToUpper(param[len("FBTYPE="):])
			switch kind {
			case "BUSY", "BUSY-UNAVAILABLE", "BUSY-TENTATIVE":
			case "FREE":
				blocking = false
			default:
				return nil, false
			}
		}
		for period := range strings.SplitSeq(value, ",") {
			span, ok := freeBusyPeriod(period)
			if !ok {
				return nil, false
			}
			if blocking {
				spans = append(spans, span)
			}
		}
	}
	return spans, seen && !inside
}

// freeBusyPeriod admits start/end or start/duration in UTC.
func freeBusyPeriod(period string) ([2]int64, bool) {
	startText, endText, ok := strings.Cut(period, "/")
	if !ok {
		return [2]int64{}, false
	}
	start, ok := icsUTCText(startText)
	if !ok {
		return [2]int64{}, false
	}
	end, ok := icsUTCText(endText)
	if !ok {
		length, durationOK := icsDuration(endText)
		if !durationOK || length > (1<<62)-start {
			return [2]int64{}, false
		}
		end = start + length
	}
	return [2]int64{start, end}, end > start
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
	return icsUTCText(property.value)
}

// icsUTCText admits RFC 5545 form 2 (UTC): exactly YYYYMMDDTHHMMSSZ. Temporal
// parses the equivalent RFC 3339 spelling and owns range validation.
func icsUTCText(v string) (int64, bool) {
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
// durations: P[n]W or P[nD][T[nH][nM][nS]]. Digit runs are sliced from the
// text, never accumulated byte by byte, so an oversized run costs one parse.
func icsDuration(text string) (int64, bool) {
	text = strings.TrimPrefix(text, "+")
	rest, ok := strings.CutPrefix(text, "P")
	if !ok || rest == "" {
		return 0, false
	}
	var total int64
	inTime, digits, seen := false, 0, false
	for i := range len(rest) {
		b := rest[i]
		switch {
		case b >= '0' && b <= '9':
			digits++
		case b == 'T' && !inTime && digits == 0:
			inTime = true
		default:
			unit, known := icsDurationUnit(b, inTime)
			if !known || digits == 0 {
				return 0, false
			}
			value, err := strconv.ParseInt(rest[i-digits:i], 10, 64)
			if err != nil || value > (1<<62)/unit || total > (1<<62)-value*unit {
				return 0, false
			}
			total += value * unit
			digits, seen = 0, true
		}
	}
	return total, seen && digits == 0
}

// icsDurationUnit admits W and D before the time designator and H, M and S
// after it, in nanoseconds.
func icsDurationUnit(designator byte, inTime bool) (int64, bool) {
	const second, minute, hour = int64(1e9), int64(60e9), int64(3600e9)
	switch {
	case designator == 'W' && !inTime:
		return 7 * 24 * hour, true
	case designator == 'D' && !inTime:
		return 24 * hour, true
	case designator == 'H' && inTime:
		return hour, true
	case designator == 'M' && inTime:
		return minute, true
	case designator == 'S' && inTime:
		return second, true
	default:
		return 0, false
	}
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
	nanos, err := instant.Nanoseconds()
	if err != nil {
		return "", err
	}
	if nanos%int64(temporal.NanosecondsPerSecond) != 0 {
		return "", core.ErrTimeMatchContract
	}
	return icsStampTime(instant)
}

func icsStampTime(instant temporal.Instant) (string, error) {
	utc, err := temporal.NewLocation(temporal.LocationUTC)
	if err != nil {
		return "", err
	}
	return utc.Format(instant, temporal.TimeLayout(icsUTCLayout))
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
