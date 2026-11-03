package plunk

import (
	"bytes"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func attachmentFixture() Attachment {
	return Attachment{Filename: "appointment.ics", ContentType: "text/calendar", Content: []byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n")}
}

func TestAttachmentHostileTable(t *testing.T) {
	t.Parallel()
	if err := attachmentFixture().Validate(); err != nil {
		t.Fatalf("fixture error = %v, want nil", err)
	}
	for _, tc := range []struct {
		name   string
		change func(*Attachment)
	}{
		{name: "empty name", change: func(a *Attachment) { a.Filename = "" }},
		{name: "path separator", change: func(a *Attachment) { a.Filename = "../appointment.ics" }},
		{name: "backslash", change: func(a *Attachment) { a.Filename = "x\\y.ics" }},
		{name: "dot", change: func(a *Attachment) { a.Filename = "." }},
		{name: "control", change: func(a *Attachment) { a.Filename = "a\x00.ics" }},
		{name: "padded", change: func(a *Attachment) { a.Filename = " a.ics" }},
		{name: "long name", change: func(a *Attachment) { a.Filename = strings.Repeat("a", core.PlunkAttachmentFilenameMaximumBytes+1) }},
		{name: "invalid utf8", change: func(a *Attachment) { a.Filename = "a\xff.ics" }},
		{name: "empty media type", change: func(a *Attachment) { a.ContentType = "" }},
		{name: "media type without slash", change: func(a *Attachment) { a.ContentType = "calendar" }},
		{name: "media type with control", change: func(a *Attachment) { a.ContentType = "text/\x01calendar" }},
		{name: "long media type", change: func(a *Attachment) {
			a.ContentType = "text/" + strings.Repeat("a", core.PlunkAttachmentMediaTypeMaximumBytes)
		}},
		{name: "empty content", change: func(a *Attachment) { a.Content = nil }},
		{name: "oversized content", change: func(a *Attachment) { a.Content = make([]byte, core.PlunkAttachmentsMaximumBytes+1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			attachment := attachmentFixture()
			tc.change(&attachment)
			if err := attachment.Validate(); !errors.Is(err, core.ErrPlunkContract) {
				t.Fatalf("Validate() error = %v, want plunk contract refusal", err)
			}
		})
	}
}

func TestSendRequestAttachmentsBoundedAndEncodedAsDocumentedBase64(t *testing.T) {
	t.Parallel()
	type metadata struct {
		Purpose string `json:"purpose"`
	}
	request := SendRequest[metadata]{To: "recipient@example.test", Subject: "Appointment booked", Body: "<p>Booked</p>", Attachments: []Attachment{attachmentFixture()}}
	wire, err := EncodeSendRequest(request)
	if err != nil {
		t.Fatalf("EncodeSendRequest() error = %v, want nil", err)
	}
	encoded := base64.StdEncoding.EncodeToString(attachmentFixture().Content)
	for _, want := range []string{`"attachments":[{`, `"filename":"appointment.ics"`, `"contentType":"text/calendar"`, `"content":"` + encoded + `"`} {
		if !bytes.Contains(wire, []byte(want)) {
			t.Fatalf("wire %s lacks %s", wire, want)
		}
	}
	var decoded SendRequest[metadata]
	if err := json.Unmarshal(wire, &decoded); err != nil || len(decoded.Attachments) != 1 || !bytes.Equal(decoded.Attachments[0].Content, attachmentFixture().Content) {
		t.Fatalf("round trip = (%+v, %v), want the exact attachment bytes", decoded.Attachments, err)
	}
	plain := SendRequest[metadata]{To: "recipient@example.test", Subject: "Plain", Body: "<p>Plain</p>"}
	if wire, err := EncodeSendRequest(plain); err != nil || bytes.Contains(wire, []byte("attachments")) {
		t.Fatalf("request without attachments = (%s, %v), want no attachments member", wire, err)
	}
	tooMany := request
	tooMany.Attachments = make([]Attachment, core.PlunkAttachmentMaximumCount+1)
	for i := range tooMany.Attachments {
		tooMany.Attachments[i] = attachmentFixture()
	}
	if wire, err := EncodeSendRequest(tooMany); !errors.Is(err, core.ErrPlunkContract) || wire != nil {
		t.Fatalf("too many attachments = (%v, %v), want refusal before encoding", wire, err)
	}
	half := attachmentFixture()
	half.Content = make([]byte, core.PlunkAttachmentsMaximumBytes/2+1)
	tooLarge := request
	tooLarge.Attachments = []Attachment{half, half}
	if wire, err := EncodeSendRequest(tooLarge); !errors.Is(err, core.ErrPlunkContract) || wire != nil {
		t.Fatalf("summed attachments over the limit = (%v, %v), want refusal before encoding", wire, err)
	}
	broken := request
	broken.Attachments = []Attachment{{Filename: "a/b.ics", ContentType: "text/calendar", Content: []byte("x")}}
	if wire, err := EncodeSendRequest(broken); !errors.Is(err, core.ErrPlunkContract) || wire != nil {
		t.Fatalf("hostile attachment = (%v, %v), want refusal before encoding", wire, err)
	}
}
