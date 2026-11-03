package plunk

import (
	"mime"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

// Attachment is one documented /v1/send attachment object: a safe file name,
// a media type and the file bytes, which the JSON projection carries as the
// provider's base64 text. Bounds follow the published provider limits; the
// caller still owns what it attaches and never relies on provider scanning.
// https://docs.useplunk.com/guides/send-a-transactional-email-with-attachments
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Content     []byte `json:"content"`
}

// Validate refuses path-like, control-bearing or format-rune-bearing names,
// media types the standard library cannot parse or that carry their own
// name/filename parameter, empty content and oversized content. A format
// rune such as a right-to-left override can render a safe name with a
// misleading extension; a media-type name is a second file name that legacy
// clients display instead of Filename.
func (a Attachment) Validate() error {
	if a.Filename == "" || len(a.Filename) > core.PlunkAttachmentFilenameMaximumBytes || !utf8.ValidString(a.Filename) || strings.TrimSpace(a.Filename) != a.Filename || strings.ContainsAny(a.Filename, "/\\") || a.Filename == "." || a.Filename == ".." {
		return core.ErrPlunkContract
	}
	for _, r := range a.Filename {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return core.ErrPlunkContract
		}
	}
	if a.ContentType == "" || len(a.ContentType) > core.PlunkAttachmentMediaTypeMaximumBytes {
		return core.ErrPlunkContract
	}
	media, parameters, err := mime.ParseMediaType(a.ContentType)
	if err != nil || !strings.Contains(media, "/") {
		return core.ErrPlunkContract
	}
	// ParseMediaType lower-cases parameter keys, so one spelling covers all.
	if _, named := parameters["name"]; named {
		return core.ErrPlunkContract
	}
	if _, named := parameters["filename"]; named {
		return core.ErrPlunkContract
	}
	if len(a.Content) == 0 || len(a.Content) > core.PlunkAttachmentsMaximumBytes {
		return core.ErrPlunkContract
	}
	return nil
}

// validateAttachments bounds the array and the summed content extent.
func validateAttachments(attachments []Attachment) error {
	if len(attachments) > core.PlunkAttachmentMaximumCount {
		return core.ErrPlunkContract
	}
	total := 0
	for _, attachment := range attachments {
		if err := attachment.Validate(); err != nil {
			return err
		}
		total += len(attachment.Content)
		if total > core.PlunkAttachmentsMaximumBytes {
			return core.ErrPlunkContract
		}
	}
	return nil
}

var _ core.Validatable = Attachment{}
