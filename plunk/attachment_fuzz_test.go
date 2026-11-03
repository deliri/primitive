package plunk

import (
	"bytes"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"mime"
	"path"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

// Attachment.Validate and the attachment-bearing EncodeSendRequest are the
// new external doors. Native MIME grammar and base64/JSON decode independently
// check accepted facts; rejected requests must emit no bytes. Ordinary send
// fields remain fixed here and retain their existing dedicated fuzz target.
func FuzzAttachmentRequestSemanticClosure(f *testing.F) {
	seed := attachmentFixture()
	request := SendRequest[struct{}]{To: "recipient@example.test", Subject: "Fixture", Body: "Fixture", Attachments: []Attachment{seed}}
	wire, err := EncodeSendRequest(request)
	if err != nil {
		f.Fatalf("EncodeSendRequest(seed) error = %v, want nil", err)
	}
	var canonical SendRequest[struct{}]
	if err := json.Unmarshal(wire, &canonical); err != nil || len(canonical.Attachments) != 1 {
		f.Fatalf("decode seed error = %v, want one typed attachment", err)
	}
	seed = canonical.Attachments[0]
	f.Add(seed.Filename, seed.ContentType, seed.Content, uint8(1))
	f.Add("", seed.ContentType, seed.Content, uint8(1))
	f.Add(seed.Filename, "text/calendar; method=REQUEST", []byte{0, 0xff, '\r', '\n'}, uint8(5))
	f.Add(seed.Filename, seed.ContentType, seed.Content, uint8(0))
	f.Add(seed.Filename, seed.ContentType, seed.Content, uint8(6))
	f.Add("appointment\u202Esci.exe", seed.ContentType, seed.Content, uint8(1))
	f.Add(seed.Filename, "text/calendar; name=\"../x\"", seed.Content, uint8(1))
	f.Fuzz(func(t *testing.T, filename, media string, content []byte, count uint8) {
		attachment := Attachment{Filename: filename, ContentType: media, Content: content}
		nameSafe := filename != "" && filename != "." && filename != ".." && len(filename) <= core.PlunkAttachmentFilenameMaximumBytes && utf8.ValidString(filename) && strings.TrimSpace(filename) == filename && path.Base(strings.ReplaceAll(filename, "\\", "/")) == filename
		for _, r := range filename {
			if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
				nameSafe = false
			}
		}
		parsedMedia, parameters, mimeErr := mime.ParseMediaType(media)
		_, namesFile := parameters["name"]
		_, namesFilename := parameters["filename"]
		valid := nameSafe && len(media) <= core.PlunkAttachmentMediaTypeMaximumBytes && strings.Contains(parsedMedia, "/") && mimeErr == nil && !namesFile && !namesFilename && len(content) > 0 && len(content) <= core.PlunkAttachmentsMaximumBytes
		gotErr := attachment.Validate()
		if valid != (gotErr == nil) || gotErr != nil && !errors.Is(gotErr, core.ErrPlunkContract) {
			t.Fatalf("Attachment.Validate() error = %v, want validity %t and typed refusal", gotErr, valid)
		}
		request := SendRequest[struct{}]{To: "recipient@example.test", Subject: "Fixture", Body: "Fixture"}
		for i := uint8(0); i < count; i++ {
			request.Attachments = append(request.Attachments, attachment)
		}
		wire, gotErr := EncodeSendRequest(request)
		wantValid := count == 0 || valid && int(count) <= core.PlunkAttachmentMaximumCount && uint64(len(content))*uint64(count) <= core.PlunkAttachmentsMaximumBytes
		if !wantValid {
			if !errors.Is(gotErr, core.ErrPlunkContract) || wire != nil {
				t.Fatalf("EncodeSendRequest() = (%d bytes, %v), want zero wire and typed refusal", len(wire), gotErr)
			}
			return
		}
		if gotErr != nil {
			t.Fatalf("EncodeSendRequest() error = %v, want nil", gotErr)
		}
		var projected struct {
			Attachments []struct {
				Filename    string `json:"filename"`
				ContentType string `json:"contentType"`
				Content     string `json:"content"`
			} `json:"attachments"`
		}
		if err := json.Unmarshal(wire, &projected); err != nil || len(projected.Attachments) != int(count) {
			t.Fatalf("wire count = %d, error = %v, want %d and nil", len(projected.Attachments), err, count)
		}
		for _, got := range projected.Attachments {
			decoded, err := base64.StdEncoding.DecodeString(got.Content)
			if err != nil || got.Filename != filename || got.ContentType != media || !bytes.Equal(decoded, content) {
				t.Fatalf("wire changed authored attachment facts, base64 error = %v", err)
			}
		}
		var again SendRequest[struct{}]
		if err := json.Unmarshal(wire, &again); err != nil || again.Validate() != nil {
			t.Fatalf("canonical request decode error = %v, want nil and admitted request", err)
		}
		second, err := EncodeSendRequest(again)
		if err != nil || !bytes.Equal(second, wire) {
			t.Fatalf("second encoding error = %v, want identical canonical wire", err)
		}
	})
}
