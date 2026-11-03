package plunk

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Two review findings: a media-type name or filename parameter is a second
// file name that legacy clients display, and Unicode format runes let a safe
// name render with a misleading extension. Both are refused like the file name
// itself; ordinary parameters and ordinary non-ASCII names stay admitted.
func TestAttachmentRefusesParameterNamesAndFormatRunes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		filename string
		media    string
		wantErr  bool
	}{
		{name: "charset and method parameters stay admitted", filename: "appointment.ics", media: "text/calendar; charset=utf-8; method=REQUEST"},
		{name: "non-ascii display name stays admitted", filename: "rendez-vous 夏.ics", media: "text/calendar"},
		{name: "name parameter with a path is refused", filename: "appointment.ics", media: "text/calendar; name=\"../../x.exe\"", wantErr: true},
		{name: "name parameter even when safe is refused", filename: "appointment.ics", media: "text/calendar; name=\"other.ics\"", wantErr: true},
		{name: "filename parameter is refused", filename: "appointment.ics", media: "text/calendar; filename=\"other.ics\"", wantErr: true},
		{name: "upper case parameter key is refused", filename: "appointment.ics", media: "text/calendar; NAME=\"other.ics\"", wantErr: true},
		{name: "right to left override is refused", filename: "appointment\u202Esci.exe", media: "text/calendar", wantErr: true},
		{name: "zero width joiner is refused", filename: "appoint\u200Dment.ics", media: "text/calendar", wantErr: true},
		{name: "byte order mark is refused", filename: "\uFEFFappointment.ics", media: "text/calendar", wantErr: true},
		{name: "soft hyphen is refused", filename: "appoint\u00ADment.ics", media: "text/calendar", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			attachment := attachmentFixture()
			attachment.Filename, attachment.ContentType = tc.filename, tc.media
			err := attachment.Validate()
			if tc.wantErr {
				if !errors.Is(err, core.ErrPlunkContract) {
					t.Fatalf("Validate() error = %v, want typed refusal", err)
				}
				request := SendRequest[struct{}]{To: "recipient@example.test", Subject: "Fixture", Body: "Fixture", Attachments: []Attachment{attachment}}
				if wire, err := EncodeSendRequest(request); !errors.Is(err, core.ErrPlunkContract) || wire != nil {
					t.Fatalf("EncodeSendRequest() = (%d bytes, %v), want zero wire and typed refusal", len(wire), err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}
