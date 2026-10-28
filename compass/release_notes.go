package compass

import (
	"strings"
	"unicode/utf8"
)

// ReleaseNote is one authored change in Project.ReleaseNotes. Git retains
// earlier versions; the project declaration describes its current release.
type ReleaseNote string

// Validate requires actual UTF-8 text without imposing an extent quota or
// rewriting authored whitespace, paragraphs or punctuation.
func (n ReleaseNote) Validate() error {
	if !utf8.ValidString(string(n)) || strings.TrimSpace(string(n)) == "" {
		return contractError("release note is absent, blank or invalid UTF-8", nil)
	}
	return nil
}

// ParseReleaseNote admits one change description without normalizing its text.
func ParseReleaseNote(text string) (ReleaseNote, error) {
	note := ReleaseNote(text)
	if err := note.Validate(); err != nil {
		return ReleaseNote(""), err
	}
	return note, nil
}

// String returns the authored description unchanged.
func (n ReleaseNote) String() string { return string(n) }
