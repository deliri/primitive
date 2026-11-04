package timematch

import (
	"errors"
	"io"
	"math"
	"strconv"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// InvitationMethod identifies the two organizer-issued iTIP messages supported
// here. A later request with the same UID updates the existing appointment.
type InvitationMethod uint8

const (
	InvitationRequest InvitationMethod = iota + 1
	InvitationCancel
)

// Validate refuses every method outside the two supported iTIP operations.
func (m InvitationMethod) Validate() error {
	switch m {
	case InvitationRequest, InvitationCancel:
		return nil
	default:
		return core.ErrTimeMatchContract
	}
}

func invitationMethodNames() []string { return []string{"request", "cancel"} }

func (m InvitationMethod) IsValid() bool  { return m.Validate() == nil }
func (m InvitationMethod) String() string { return enumName(m, invitationMethodNames()) }
func (m InvitationMethod) MarshalJSON() ([]byte, error) {
	return marshalEnum(m, invitationMethodNames())
}
func (m *InvitationMethod) UnmarshalJSON(data []byte) error {
	return unmarshalEnum(m, data, invitationMethodNames())
}

// Invitation is one appointment's calendar message, never a downloaded
// customer event. UID and organizer remain stable across updates/cancellation;
// the application owns the increasing sequence and the consenting recipients.
// UTC second precision avoids ambiguous local clocks and VTIMEZONE invention.
// The message is one recipient's by default: listing several attendees in one
// message discloses every address to every recipient (RFC 5546 section 6.3),
// so more than one attendee needs DiscloseAttendees set on purpose.
// Sources: RFC 5546 sections 3.2.2/3.2.5/6.3; RFC 5545 sections 3.1/3.8.7.4.
type Invitation struct {
	Venue             core.TimeMatchAppointmentVenue
	UID               EventID
	Summary           EventSummary
	Interval          temporal.IntervalBounds
	Stamp             temporal.Instant
	Organizer         core.OAuthEmail
	Attendees         []core.OAuthEmail
	Sequence          uint32
	Method            InvitationMethod
	DiscloseAttendees bool
}

// Validate checks the entire invitation before any message bytes are emitted.
func (i Invitation) Validate() error {
	if err := errors.Join(i.UID.Validate(), i.Summary.Validate(), i.Venue.Validate(), positiveInterval(i.Interval), i.Stamp.Validate(), i.Organizer.Validate(), i.Method.Validate()); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	if i.Venue.Kind != core.TimeMatchVenueSupplied || len(i.Attendees) == 0 || len(i.Attendees) > 1 && !i.DiscloseAttendees || i.Sequence > math.MaxInt32 || i.Method == InvitationCancel && i.Sequence == 0 {
		return core.ErrTimeMatchContract
	}
	if !asciiMailDomain(i.Organizer.String()) {
		return core.ErrTimeMatchContract
	}
	if _, err := icsTime(i.Interval.Start); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	if _, err := icsTime(i.Interval.End); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	return i.validateAttendees()
}

// WriteInvitation streams a REQUEST or whole-appointment CANCEL into a
// caller-owned writer. It validates every input before the first byte, folds
// physical lines without splitting UTF-8, preserves writer failures, and keeps
// fixed working storage regardless of recipient count or summary length.
// The UID is the owned event identity qualified by the organizer's domain, so
// two organizers can never collide on one guest's calendar (RFC 5545 section
// 3.8.4.7). Send the bytes as "text/calendar; method=REQUEST|CANCEL;
// charset=UTF-8" (RFC 6047 section 2.4): the writer emits raw UTF-8 text.
// Producing the message proves neither email acceptance nor calendar import.
func WriteInvitation(destination io.Writer, invitation Invitation) error {
	if destination == nil {
		return core.ErrTimeMatchContract
	}
	if err := invitation.Validate(); err != nil {
		return err
	}
	start, err := icsTime(invitation.Interval.Start)
	if err != nil {
		return err
	}
	end, err := icsTime(invitation.Interval.End)
	if err != nil {
		return err
	}
	stamp, err := icsStampTime(invitation.Stamp)
	if err != nil {
		return err
	}
	w := invitationWriter{destination: destination}
	w.line("BEGIN:VCALENDAR")
	w.line("VERSION:2.0")
	w.line("PRODID:", icsProductID)
	switch invitation.Method {
	case InvitationRequest:
		w.line("METHOD:REQUEST")
	case InvitationCancel:
		w.line("METHOD:CANCEL")
	default:
		return core.ErrTimeMatchContract
	}
	w.line(icsBeginEvent)
	w.line("UID:", invitation.UID.value, "@", mailDomain(invitation.Organizer.String()))
	w.line("DTSTAMP:", stamp)
	w.line("DTSTART:", start)
	w.line("DTEND:", end)
	w.line("SEQUENCE:", strconv.FormatUint(uint64(invitation.Sequence), 10))
	w.text("ORGANIZER:mailto:", false)
	w.address(invitation.Organizer.String())
	w.endLine()
	for _, attendee := range invitation.Attendees {
		if w.err != nil {
			return w.err
		}
		w.text("ATTENDEE:mailto:", false)
		w.address(attendee.String())
		w.endLine()
	}
	w.text("SUMMARY:", false)
	w.text(invitation.Summary.value, true)
	w.endLine()
	if invitation.Venue.Text() != "" {
		w.text("LOCATION:", false)
		w.text(invitation.Venue.Text(), true)
		w.endLine()
	}
	if invitation.Venue.JoinURL != "" {
		w.line("URL:", invitation.Venue.JoinURL)
	}
	switch invitation.Method {
	case InvitationRequest:
		w.line("STATUS:CONFIRMED")
	case InvitationCancel:
		w.line("STATUS:CANCELLED")
	default:
		return core.ErrTimeMatchContract
	}
	w.line("TRANSP:OPAQUE")
	w.line(icsEndEvent)
	w.line("END:VCALENDAR")
	return w.err
}

// invitationWriter holds exactly one RFC 5545 physical line (75 octets plus
// CRLF) in fixed storage and hands each completed line to the destination in
// one write. It never buffers the whole message, a whole recipient list or an
// arbitrarily large text line, and it never writes again after a failure.
type invitationWriter struct {
	destination io.Writer
	err         error
	column      int
	physical    [icsLineMaximum + len(icsCRLF)]byte
}

// flush writes the buffered physical line once and records the first failure.
func (w *invitationWriter) flush() {
	if w.err != nil {
		return
	}
	data := w.physical[:w.column]
	w.column = 0
	written, err := w.destination.Write(data)
	if err != nil {
		w.err = err
		return
	}
	if written != len(data) {
		w.err = io.ErrShortWrite
	}
}

// terminate appends CRLF to the buffered line and flushes it.
func (w *invitationWriter) terminate() {
	w.column += copy(w.physical[w.column:], icsCRLF)
	w.flush()
}

func (w *invitationWriter) rune(value rune) {
	var raw [utf8.UTFMax]byte
	n := utf8.EncodeRune(raw[:], value)
	if w.column+n > icsLineMaximum {
		w.terminate()
		w.physical[0] = ' '
		w.column = 1
	}
	w.column += copy(w.physical[w.column:], raw[:n])
}

func (w *invitationWriter) text(value string, escape bool) {
	for _, r := range value {
		if w.err != nil {
			return
		}
		if escape && (r == '\\' || r == ';' || r == ',') {
			w.rune('\\')
		}
		w.rune(r)
	}
}

// address encodes exactly one mailto addr-spec, with no header/body URI
// parameters. Percent-encoding is applied once to the original address bytes.
// Source: RFC 6068 section 2.
func (w *invitationWriter) address(value string) {
	const digits = "0123456789ABCDEF"
	for i := range len(value) {
		if w.err != nil {
			return
		}
		b := value[i]
		if b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '-' || b == '.' || b == '_' || b == '~' || b == '@' {
			w.rune(rune(b))
		} else {
			w.rune('%')
			w.rune(rune(digits[b>>4]))
			w.rune(rune(digits[b&15]))
		}
	}
}

func (w *invitationWriter) endLine() { w.terminate() }
func (w *invitationWriter) line(parts ...string) {
	for _, part := range parts {
		w.text(part, false)
	}
	w.endLine()
}

func (Invitation) timematchProtocolFact()       {}
func (invitationWriter) timematchInternalFlow() {}

func (i Invitation) validateAttendees() error {
	for _, attendee := range i.Attendees {
		if err := attendee.Validate(); err != nil {
			return errors.Join(core.ErrTimeMatchContract, err)
		}
		if !asciiMailDomain(attendee.String()) {
			return core.ErrTimeMatchContract
		}
	}
	return nil
}

// mailDomain is the text after the last '@' of an admitted address.
func mailDomain(address string) string {
	for i := len(address) - 1; i >= 0; i-- {
		if address[i] == '@' {
			return address[i+1:]
		}
	}
	return address
}

// asciiMailDomain refuses an internationalized domain: a mailto URI must carry
// the IDNA form, never a percent-encoded one (RFC 6068 section 2), and this
// package does not transliterate addresses.
func asciiMailDomain(address string) bool {
	domain := mailDomain(address)
	for i := range len(domain) {
		if domain[i] >= 0x80 {
			return false
		}
	}
	return domain != ""
}
