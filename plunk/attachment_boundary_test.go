package plunk

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// These are the previously unpinned exact limits. The original hostile table
// owns malformed names/media and individual over-limit content; this table
// adds accepted limit-adjacent values and aggregate/count admission.
func TestAttachmentExactBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		filename string
		media    string
		bytes    int
		wantErr  bool
	}{
		{name: "one byte filename", filename: "a", media: "a/b", bytes: 1},
		{name: "filename below ceiling", filename: strings.Repeat("a", core.PlunkAttachmentFilenameMaximumBytes-1), media: "text/calendar", bytes: 1},
		{name: "filename at byte ceiling", filename: strings.Repeat("a", core.PlunkAttachmentFilenameMaximumBytes), media: "text/calendar", bytes: 1},
		{name: "unicode filename counts bytes", filename: strings.Repeat("é", 127) + "a", media: "text/calendar", bytes: 1},
		{name: "unicode filename crosses byte ceiling", filename: strings.Repeat("é", 128), media: "text/calendar", bytes: 1, wantErr: true},
		{name: "unicode display filename", filename: "rendez-vous 夏.ics", media: "text/calendar", bytes: 1},
		{name: "interior name space", filename: "my appointment.ics", media: "text/calendar", bytes: 1},
		{name: "double dot is not a directory component", filename: "appointment..ics", media: "text/calendar", bytes: 1},
		{name: "media below ceiling", filename: "a", media: "a/" + strings.Repeat("b", core.PlunkAttachmentMediaTypeMaximumBytes-3), bytes: 1},
		{name: "media at ceiling", filename: "a", media: "a/" + strings.Repeat("b", core.PlunkAttachmentMediaTypeMaximumBytes-2), bytes: 1},
		{name: "media parameter preserved", filename: "a", media: "text/calendar; charset=utf-8; method=REQUEST", bytes: 1},
		{name: "media quoted parameter", filename: "a", media: "application/x-test; name=\"a b\"", bytes: 1},
		{name: "content below ceiling", filename: "a", media: "a/b", bytes: core.PlunkAttachmentsMaximumBytes - 1},
		{name: "content at ceiling", filename: "a", media: "a/b", bytes: core.PlunkAttachmentsMaximumBytes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			attachment := Attachment{Filename: tc.filename, ContentType: tc.media, Content: bytes.Repeat([]byte{0xa5}, tc.bytes)}
			gotErr := attachment.Validate()
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrPlunkContract) {
					t.Fatalf("Validate() error = %v, want typed refusal", gotErr)
				}
			} else if gotErr != nil {
				t.Fatalf("Validate() error = %v, want nil", gotErr)
			}
		})
	}
}

func TestAttachmentRequestCountAndAggregateBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		count   int
		total   int
		wantErr bool
	}{
		{name: "no attachment member", count: 0},
		{name: "count below ceiling", count: core.PlunkAttachmentMaximumCount - 1, total: 4},
		{name: "count at ceiling", count: core.PlunkAttachmentMaximumCount, total: 5},
		{name: "count above ceiling", count: core.PlunkAttachmentMaximumCount + 1, total: 6, wantErr: true},
		{name: "aggregate below ceiling", count: 2, total: core.PlunkAttachmentsMaximumBytes - 1},
		{name: "aggregate at ceiling", count: 2, total: core.PlunkAttachmentsMaximumBytes},
		{name: "aggregate above ceiling individually valid", count: 2, total: core.PlunkAttachmentsMaximumBytes + 1, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := SendRequest[struct{}]{To: "recipient@example.test", Subject: "Fixture", Body: "Fixture"}
			for i := 0; i < tc.count; i++ {
				size := tc.total / tc.count
				if i == tc.count-1 {
					size += tc.total % tc.count
				}
				attachment := attachmentFixture()
				attachment.Content = bytes.Repeat([]byte{byte(i + 1)}, size)
				request.Attachments = append(request.Attachments, attachment)
			}
			wire, gotErr := EncodeSendRequest(request)
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrPlunkContract) || wire != nil {
					t.Fatalf("EncodeSendRequest() = (%d bytes, %v), want zero wire and typed refusal", len(wire), gotErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("EncodeSendRequest() error = %v, want nil", gotErr)
			}
			var got SendRequest[struct{}]
			if err := json.Unmarshal(wire, &got); err != nil || len(got.Attachments) != tc.count {
				t.Fatalf("decoded count = %d, error = %v, want %d and nil", len(got.Attachments), err, tc.count)
			}
			for i, a := range got.Attachments {
				want := request.Attachments[i]
				if a.Filename != want.Filename || a.ContentType != want.ContentType || !bytes.Equal(a.Content, want.Content) {
					t.Fatalf("decoded attachment %d = (%q, %q, %d bytes), want (%q, %q, %d identical bytes)", i, a.Filename, a.ContentType, len(a.Content), want.Filename, want.ContentType, len(want.Content))
				}
			}
			if tc.count == 0 && bytes.Contains(wire, []byte("attachments")) {
				t.Fatalf("EncodeSendRequest(zero attachments) = %s, want absent attachments member", wire)
			}
		})
	}
}
